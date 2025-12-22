package signaling

import (
	"fmt"
	"net"
	"net/url"
	"time"

	"github.com/BioHazard786/Warpdrop/cli/internal/dns"
	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 64 * 1024
)

// Client manages the WebSocket connection to the signaling server.
type Client struct {
	conn      *websocket.Conn
	serverURL string
	incoming  chan *Message
	outgoing  chan *Message
	done      chan struct{}
	closed    bool
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

// Connect establishes WebSocket connection to the server.
func (c *Client) Connect() error {
	u, err := url.Parse(c.serverURL)
	if err != nil {
		return fmt.Errorf("invalid server URL: %w", err)
	}

	// Create a custom dialer that uses our robust DNS lookup
	dialer := websocket.DefaultDialer
	dialer.NetDial = func(network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}

		// Use our custom DNS lookup with fallback
		resolvedIP, err := dns.Lookup(host)
		if err != nil {
			return nil, fmt.Errorf("dns lookup failed: %w", err)
		}

		// Dial the resolved IP
		return net.Dial(network, net.JoinHostPort(resolvedIP, port))
	}

	conn, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	c.conn = conn

	c.conn.SetReadLimit(maxMessageSize)

	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	go c.readPump()
	go c.writePump()

	return nil
}

// readPump reads messages from the WebSocket connection.
func (c *Client) readPump() {
	defer func() {
		c.conn.Close()
		close(c.incoming)
	}()

	c.conn.SetReadDeadline(time.Now().Add(pongWait))

	for {
		var msg Message
		if err := c.conn.ReadJSON(&msg); err != nil {
			return
		}

		c.incoming <- &msg
	}
}

// writePump writes messages to the WebSocket connection and sends periodic pings.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)

	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message := <-c.outgoing:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteJSON(message); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-c.done:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			c.conn.WriteMessage(websocket.CloseMessage, []byte{})
			return
		}
	}
}

// SendMessage sends a message to the server.
func (c *Client) SendMessage(msg *Message) {
	c.outgoing <- msg
}

// Incoming returns the channel for receiving messages.
func (c *Client) Incoming() <-chan *Message {
	return c.incoming
}

// Close closes the WebSocket connection and cleans up resources.
func (c *Client) Close() {
	if c.closed {
		return
	}
	c.closed = true

	close(c.done)
	close(c.outgoing)
}
