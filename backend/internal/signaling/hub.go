package signaling

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
)

// Hub is the central brain of the signaling server.
// It manages all active rooms and clients.
type Hub struct {
	// rooms maps room IDs to Room instances.
	Rooms map[string]*Room

	// register is a channel for registering new clients.
	Register chan *Client

	// unregister is a channel for unregistering clients.
	Unregister chan *Client

	// broadcast is a channel for clients to broadcast messages to.
	// The hub will process these messages.
	Broadcast chan *Message
}

// NewHub creates a new Hub instance.
func NewHub() *Hub {
	return &Hub{
		Rooms:      make(map[string]*Room),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		Broadcast:  make(chan *Message),
	}
}

// generateRoomID creates a random, memorable room ID using word combinations.
// Format: word-word-word-word (e.g., "kitten-waffle-stardust-happy")
// Randomly picks 4 words from all available word lists.
func (h *Hub) generateRoomID() string {
	// Combine all word lists into one pool
	allWords := [][]string{animals, dishes, names, randomWords, adjectives, extras}

	// Keep generating until we find one that's not in use
	for {
		// Pick 4 random word lists (without replacement)
		selectedLists := make([][]string, 4)
		usedIndices := make(map[int]bool)

		for i := 0; i < 4; i++ {
			// Pick a random list index that hasn't been used yet
			var listIndex int
			for {
				listIndex = randomIndex(len(allWords))
				if !usedIndices[listIndex] {
					usedIndices[listIndex] = true
					break
				}
			}
			selectedLists[i] = allWords[listIndex]
		}

		// Pick a random word from each selected list
		word1 := selectedLists[0][randomIndex(len(selectedLists[0]))]
		word2 := selectedLists[1][randomIndex(len(selectedLists[1]))]
		word3 := selectedLists[2][randomIndex(len(selectedLists[2]))]
		word4 := selectedLists[3][randomIndex(len(selectedLists[3]))]

		// Combine them with hyphens
		id := fmt.Sprintf("%s-%s-%s-%s", word1, word2, word3, word4)

		// Check if room already exists
		if _, ok := h.Rooms[id]; !ok {
			return id
		}
	}
}

// randomIndex returns a cryptographically secure random index for a slice of given length.
func randomIndex(max int) int {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		log.Panic("Failed to generate random index:", err)
	}
	return int(n.Int64())
}

// Run starts the hub's main processing loop.
// This is the single goroutine that safely manages all state (rooms, clients).
func (h *Hub) Run() {
	// Start an infinite loop to listen for messages on our channels
	for {
		select {
		// --- Client Register ---
		case client := <-h.Register:
			// For now, we just log the registration.
			// The client is not in a room yet. They need to send a
			// "create_room" or "join_room" message first.
			log.Printf("Client registered: %s", client.Conn.RemoteAddr())

		// --- Client Unregister ---
		case client := <-h.Unregister:
			log.Printf("Client unregistered: %s", client.Conn.RemoteAddr())

			// Clean up:
			// 1. Find the room the client was in
			if client.RoomID != "" {
				if room, ok := h.Rooms[client.RoomID]; ok {

					var otherPeer *Client

					// 2. See if they were the sender or receiver and remove them
					if room.Sender == client {
						room.Sender = nil
						otherPeer = room.Receiver
					} else if room.Receiver == client {
						room.Receiver = nil
						otherPeer = room.Sender
					}

					// 3. If the room is now empty, delete it
					if room.Sender == nil && room.Receiver == nil {
						delete(h.Rooms, room.ID)
						log.Printf("Room deleted: %s", room.ID)
					} else {
						// 4. If the room is not empty, notify the other peer
						log.Printf("Peer left room: %s", room.ID)
						if otherPeer != nil {
							otherPeer.Send <- &Message{Type: "peer_left"}
						}
					}
				}
			}

			// 5. Close the client's send channel to stop its writePump
			close(client.Send)

		// --- Broadcast Message ---
		case message := <-h.Broadcast:
			// Log the incoming message
			log.Printf("Broadcast received: Type=%s from %s", message.Type, message.client.Conn.RemoteAddr())

			// This is the core signaling logic
			switch message.Type {

			// Case 1: A client wants to create a new room
			case "create_room":
				// Store client metadata
				message.client.ClientType = message.ClientType

				roomID := h.generateRoomID()
				room := &Room{
					ID:     roomID,
					Sender: message.client,
				}
				h.Rooms[roomID] = room
				message.client.RoomID = roomID

				log.Printf("Room created: %s by %s (type=%s)", roomID, message.client.Conn.RemoteAddr(), message.client.ClientType)

				// Send the "room_created" message back to the sender
				message.client.Send <- &Message{
					Type:   "room_created",
					RoomID: roomID,
				}

			// Case 2: A client wants to join an existing room
			case "join_room":
				// Store client metadata
				message.client.ClientType = message.ClientType

				roomID := message.RoomID
				room, ok := h.Rooms[roomID]

				// Check if room exists
				if !ok {
					log.Printf("Room join failed: Room %s not found", roomID)
					message.client.Send <- &Message{
						Type:    "error",
						Payload: json.RawMessage(`{"error": "Room not found"}`),
					}
					continue // Use 'continue' to skip to the next 'select' iteration
				}

				// Check if room is full
				if room.Receiver != nil {
					log.Printf("Room join failed: Room %s is full", roomID)
					message.client.Send <- &Message{
						Type:    "error",
						Payload: json.RawMessage(`{"error": "Room is full"}`),
					}
					continue
				}

				// Room is valid and has space. Add the client as the receiver.
				room.Receiver = message.client
				message.client.RoomID = roomID

				log.Printf("Client %s joined room %s (type=%s)", message.client.Conn.RemoteAddr(), roomID, message.client.ClientType)

				// Notify the *sender* (Peer A) that the receiver has joined
				// Include receiver's peer info for protocol negotiation
				if room.Sender != nil {
					peerInfo := PeerInfo{
						ClientType: message.client.ClientType,
					}
					peerInfoBytes, _ := json.Marshal(peerInfo)

					room.Sender.Send <- &Message{
						Type:    "peer_joined",
						Payload: peerInfoBytes,
					}
				}

				// Notify the *receiver* (Peer B) that they successfully joined
				// Include sender's peer info for protocol negotiation
				peerInfo := PeerInfo{
					ClientType: room.Sender.ClientType,
				}
				peerInfoBytes, _ := json.Marshal(peerInfo)

				message.client.Send <- &Message{
					Type:    "join_success",
					RoomID:  roomID,
					Payload: peerInfoBytes,
				}

			// Case 3: A client is sending a WebRTC signal (offer, answer, or ICE candidate)
			case "signal":
				roomID := message.client.RoomID

				if roomID == "" {
					log.Printf("Signal failed: Client %s is not in any room", message.client.Conn.RemoteAddr())
					message.client.Send <- &Message{
						Type:    "error",
						Payload: json.RawMessage(`{"error": "You must join a room first"}`),
					}
					continue
				}

				room, ok := h.Rooms[roomID]
				if !ok {
					log.Printf("Signal failed: Room %s not found", roomID)
					message.client.Send <- &Message{
						Type:    "error",
						Payload: json.RawMessage(`{"error": "Room not found"}`),
					}
					continue
				}

				// Find the *other* peer to relay the signal to
				var targetClient *Client
				if message.client == room.Sender {
					targetClient = room.Receiver
				} else {
					targetClient = room.Sender
				}

				// Relay the message only if the other peer exists
				if targetClient != nil {
					log.Printf("Relaying signal from %s to %s in room %s", message.client.Conn.RemoteAddr(), targetClient.Conn.RemoteAddr(), roomID)
					// We can just forward the original message, as it already
					// has the correct type ("signal") and payload.
					targetClient.Send <- message
				} else {
					log.Printf("Signal failed: No other peer in room %s", roomID)
				}

			// Default case: Unknown message type
			default:
				log.Printf("Unknown message type: %s", message.Type)
			}
		}
	}
}
