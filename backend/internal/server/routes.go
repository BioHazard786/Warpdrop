package server

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"

	"github.com/BioHazard786/Warpdrop/backend/internal/signaling"
)

// Configure the websocket upgrader
var upgrader = websocket.Upgrader{
	ReadBufferSize:  64 * 1024, // 64 KB
	WriteBufferSize: 64 * 1024, // 64 KB

	// We need to check the origin, but for development, we can allow all.
	// In production, you'd check r.Header.Get("Origin") against your frontend's domain
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all connections for now
	},
}

// ServeWs returns an http.HandlerFunc that handles websocket requests.
// It takes the hub as a dependency.
func ServeWs(hub *signaling.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Upgrade the HTTP connection to a WebSocket
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("Failed to upgrade connection:", err)
			return
		}

		// Create a new client
		client := &signaling.Client{
			Hub:    hub,
			Conn:   conn,
			RoomID: "",                                 // Will be set on create/join
			Send:   make(chan *signaling.Message, 256), // Buffered channel for *Message
		}

		// Register the client with the hub
		client.Hub.Register <- client

		// Start the client's read and write pumps in separate goroutines
		// These methods will handle the client's lifecycle
		go client.WritePump()
		go client.ReadPump()
	}
}
