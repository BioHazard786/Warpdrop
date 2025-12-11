package signaling

import (
	"log"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 64 * 1024 // 64 KB - enough for WebRTC SDP messages
)

// Client is a wrapper for a single websocket connection (a peer)
type Client struct {
	// hub is a pointer to the hub that manages this client.
	Hub *Hub

	// conn is the websocket connection.
	Conn *websocket.Conn

	// roomID is the ID of the room the client is in.
	RoomID string

	// send is a buffered channel for all outbound messages.
	// We write to this channel, and a separate goroutine (writePump)
	// reads from it and writes to the websocket.
	Send chan *Message

	// Client metadata for protocol negotiation
	ClientType string // "cli" or "web"
}

// ReadPump pumps messages from the websocket connection to the hub.
//
// The application runs ReadPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Client) ReadPump() {
	// When this function exits (e.g., connection closes), unregister the client
	defer func() {
		c.Hub.Unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// Loop forever, reading messages from the connection
	for {
		// Read a message as JSON
		var msg Message
		err := c.Conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break // Break the loop on error
		}

		// Attach the client pointer to the message
		msg.client = c

		// Send the message to the hub's broadcast channel for processing
		c.Hub.Broadcast <- &msg
	}
}

// WritePump pumps messages from the hub to the websocket connection.
//
// A goroutine running WritePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)

	// When this function exits, stop the ticker and close the connection
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		// Case 1: We have a message to send from our 'send' channel
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Write the message to the websocket
			err := c.Conn.WriteJSON(message) // Write the Message struct as JSON
			if err != nil {
				log.Printf("error writing json: %v", err)
				return // Exit on write error
			}

		// Case 2: The ticker's timer has fired, so we send a ping
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return // Exit on ping error
			}
		}
	}
}
