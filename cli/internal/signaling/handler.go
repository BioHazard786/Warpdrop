package signaling

import (
	"encoding/json"
	"log/slog"
)

// PeerInfo contains information about the connected peer
type PeerInfo struct {
	ClientType string `json:"client_type"`
}

// Handler routes incoming signaling messages to appropriate channels
type Handler struct {
	// client is the WebSocket client connection
	client *Client

	// RoomCreated fires when server sends "room_created" message
	RoomCreated chan string

	// PeerJoined fires when another peer joins our room (for sender)
	PeerJoined chan *PeerInfo

	// JoinSuccess fires when we successfully join a room (for receiver)
	JoinSuccess chan *PeerInfo

	// PeerLeft fires when the peer leaves
	PeerLeft chan struct{}

	// Signal fires when we receive WebRTC signaling data (offer/answer/ICE)
	Signal chan *SignalPayload

	// Error fires when server sends an error
	Error chan string

	// closed tracks if channels are already closed
	closed bool
}

// NewHandler creates a new message handler
func NewHandler(client *Client) *Handler {
	return &Handler{
		client:      client,
		RoomCreated: make(chan string, 1),
		PeerJoined:  make(chan *PeerInfo, 1),
		JoinSuccess: make(chan *PeerInfo, 1),
		PeerLeft:    make(chan struct{}, 1),
		Signal:      make(chan *SignalPayload, 32), // Larger buffer for trickle ICE candidates
		Error:       make(chan string, 1),
	}
}

// Start begins listening to incoming messages and routing them
func (h *Handler) Start() {
	// Loop forever, listening to incoming messages from the client
	for msg := range h.client.Incoming() {
		// Route the message based on its type
		slog.Info("Received message", "type", msg.Type)

		switch msg.Type {

		case MessageTypeRoomCreated:
			h.handleRoomCreated(msg)

		case MessageTypeJoinSuccess:
			h.handleJoinSuccess(msg)

		case MessageTypePeerJoined:
			h.handlePeerJoined(msg)

		case MessageTypePeerLeft:
			slog.Info("Peer left the room")
			h.PeerLeft <- struct{}{}

		case MessageTypeSignal:
			h.handleSignal(msg)

		case MessageTypeError:
			h.handleError(msg)

		default:
			slog.Error("Unknown message", "type", msg.Type)
		}
	}
}

// handleRoomCreated extracts room ID and sends it through the channel
func (h *Handler) handleRoomCreated(msg *Message) {
	// msg.RoomID contains the room ID the server created for us
	slog.Info("Room created with ID", "id", msg.RoomID)
	h.RoomCreated <- msg.RoomID
}

// handleJoinSuccess is called when we successfully joined a room
func (h *Handler) handleJoinSuccess(msg *Message) {
	slog.Info("Successfully joined room", "id", msg.RoomID)

	// Parse peer info from payload
	var peerInfo PeerInfo
	if msg.Payload != nil {
		payloadBytes, err := json.Marshal(msg.Payload)
		if err == nil {
			json.Unmarshal(payloadBytes, &peerInfo)
		}
	}

	slog.Info("Peer info", "type", peerInfo.ClientType)
	h.JoinSuccess <- &peerInfo
}

// handlePeerJoined is called when a peer joins our room (sender receives this)
func (h *Handler) handlePeerJoined(msg *Message) {
	slog.Info("Peer joined the room")

	// Parse peer info from payload
	var peerInfo PeerInfo
	if msg.Payload != nil {
		payloadBytes, err := json.Marshal(msg.Payload)
		if err == nil {
			json.Unmarshal(payloadBytes, &peerInfo)
		}
	}

	slog.Info("Peer info", "type", peerInfo.ClientType)
	h.PeerJoined <- &peerInfo
}

// handleSignal parses the WebRTC signaling payload and sends it
func (h *Handler) handleSignal(msg *Message) {
	// msg.Payload should be a map[string]any or SignalPayload struct
	// It could be an SDP offer/answer or an ICE candidate

	var payload SignalPayload

	// Convert payload to SignalPayload
	// Payload comes as map[string]any from JSON, so we marshal and unmarshal
	payloadBytes, err := json.Marshal(msg.Payload)
	if err != nil {
		slog.Error("Failed to marshal payload", "error", err)
		h.Error <- "Failed to parse signal payload"
		return
	}

	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		slog.Error("Failed to parse signal payload", "error", err)
		h.Error <- "Failed to parse signal payload"
		return
	}

	slog.Info("Received signal",
		"type", payload.Type, "has_sdp", payload.SDP != "", "has_ice", payload.ICECandidate != nil)

	// Send the parsed payload through the channel
	h.Signal <- &payload
}

// handleError parses error message and sends it through the Error channel
func (h *Handler) handleError(msg *Message) {
	var errPayload ErrorPayload

	// Convert payload to ErrorPayload
	payloadBytes, err := json.Marshal(msg.Payload)
	if err != nil {
		slog.Error("Failed to marshal error payload", "error", err)
		h.Error <- "Unknown error from server"
		return
	}

	if err := json.Unmarshal(payloadBytes, &errPayload); err != nil {
		slog.Error("Failed to parse error payload", "error", err)
		h.Error <- "Unknown error from server"
		return
	}

	slog.Error("Server error", "error", errPayload.Error)
	h.Error <- errPayload.Error
}

// Close closes all handler channels
// Call this when shutting down to prevent goroutines from blocking
func (h *Handler) Close() {
	if h.closed {
		return
	}
	h.closed = true

	close(h.RoomCreated)
	close(h.PeerJoined)
	close(h.JoinSuccess)
	close(h.PeerLeft)
	close(h.Signal)
	close(h.Error)
}
