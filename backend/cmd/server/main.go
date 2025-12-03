package main

import (
	"log"
	"net/http"

	"github.com/BioHazard786/WarpDrop/internal/server"
	"github.com/BioHazard786/WarpDrop/internal/signaling"
)

// Health Check endpoint
func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	// Simple check to make sure the health endpoint only responds to "/health"
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Signaling server is healthy."))
}

func main() {

	// 1. Create the Hub
	hub := signaling.NewHub()

	// 2. Run the Hub in a separate goroutine
	// This starts the hub's main event loop (the 'select' statement)
	go hub.Run()

	// 3. Register our handlers
	http.HandleFunc("/", healthCheckHandler)

	// Get the ServeWs handler function (which includes the hub as a dependency)
	// and register it for the "/ws" route
	http.HandleFunc("/ws", server.ServeWs(hub))

	// 4. Start the server
	port := ":8080"
	log.Printf("Starting signaling server on http://localhost%s", port)

	log.Fatal(http.ListenAndServe(port, nil))
}
