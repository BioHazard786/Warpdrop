package signaling

import (
	"fmt"
	"log"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

// Timing constants for WebSocket health checks (copied from backend)
const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period (must be less than pongWait)
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer (64 KB - enough for WebRTC SDP)
	maxMessageSize = 64 * 1024
)

// Client manages the WebSocket connection to the signaling server.
type Client struct {
	// conn is the WebSocket connection
	conn *websocket.Conn

	// serverURL is the signaling server address
	serverURL string

	// incoming is a channel that receives messages FROM the server
	incoming chan *Message

	// outgoing is a channel that sends messages TO the server
	outgoing chan *Message

	// done signals when the connection should close
	done chan struct{}

	// closed tracks if already closed
	closed bool
}

// NewClient creates a new signaling client
func NewClient(serverURL string) *Client {
	return &Client{
		serverURL: serverURL,
		incoming:  make(chan *Message, 1),
		outgoing:  make(chan *Message, 1),
		done:      make(chan struct{}, 1),
	}
}

// Connect establishes WebSocket connection to the server
func (c *Client) Connect() error {
	// Parse the server URL
	u, err := url.Parse(c.serverURL)
	if err != nil {
		return fmt.Errorf("invalid server URL: %w", err)
	}

	// Dial (connect to) the WebSocket server
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	c.conn = conn
	log.Println("WebSocket Connected")

	// Set read limit to prevent huge messages from crashing us
	c.conn.SetReadLimit(maxMessageSize)

	// Set pong handler - this is called when we receive a pong from server
	// SetReadDeadline updates the deadline, which tells the client we're still alive
	c.conn.SetPongHandler(func(string) error {
		log.Println("Received pong from server")
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// Start background goroutines
	go c.readPump()  // Reads messages FROM server (runs in background)
	go c.writePump() // Writes messages TO server (runs in background)

	return nil
}

// readPump reads messages from the WebSocket connection
// This runs in a GOROUTINE (background thread)
// It also handles the pong deadline - if we don't receive a pong in time, the connection dies
func (c *Client) readPump() {
	defer func() {
		c.conn.Close()
		close(c.incoming)
	}()

	// Set initial read deadline
	// If server doesn't send a pong within pongWait, ReadJSON will timeout and we exit
	c.conn.SetReadDeadline(time.Now().Add(pongWait))

	// Infinite loop - keeps reading messages
	for {
		var msg Message
		if err := c.conn.ReadJSON(&msg); err != nil {
			log.Println("WebSocket read error:", err)
			return
		}

		// Send to incoming channel
		c.incoming <- &msg
	}
}

// writePump writes messages to the WebSocket connection
// This also runs in a GOROUTINE (background thread)
// It sends periodic pings to keep the connection alive
func (c *Client) writePump() {
	// Create a ticker that fires every pingPeriod
	// Like setInterval in JavaScript
	ticker := time.NewTicker(pingPeriod)

	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	// Infinite loop - checks for messages to send and ping timer
	for {
		select {
		case message := <-c.outgoing:
			// We have a message to send
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteJSON(message); err != nil {
				log.Println("WebSocket write error:", err)
				return
			}

		case <-ticker.C:
			// Timer fired - send a ping to keep connection alive
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Println("Failed to send ping:", err)
				return
			}
			log.Println("Sent ping to server")

		case <-c.done:
			// Close signal received - close connection gracefully
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			c.conn.WriteMessage(websocket.CloseMessage, []byte{})
			return
		}
	}
}

// SendMessage sends a message to the server
// This is safe to call from any goroutine
func (c *Client) SendMessage(msg *Message) {
	c.outgoing <- msg // Put message in outgoing channel
}

// Incoming returns the channel for receiving messages
func (c *Client) Incoming() <-chan *Message {
	return c.incoming
}

// Close closes the WebSocket connection and cleans up resources
// Should only be called once, typically via defer in main()
func (c *Client) Close() {
	if c.closed {
		return
	}
	c.closed = true

	close(c.done)     // Signal goroutines to stop (writePump sees this)
	close(c.outgoing) // Close outgoing channel (no more sends allowed)
}
