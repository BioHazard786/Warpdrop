package main

import (
	"embed"
	"log"
	"net/http"
)

//go:embed install.sh
var installScript embed.FS

func main() {
	// Health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Installer service is healthy."))
	})

	// Serve install.sh
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Only allow GET and HEAD requests
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Serve install.sh for any path
		script, err := installScript.ReadFile("install.sh")
		if err != nil {
			log.Printf("Error reading install.sh: %v", err)
			http.Error(w, "Script not found", http.StatusNotFound)
			return
		}

		// Set appropriate headers
		w.Header().Set("Content-Type", "text/x-sh; charset=utf-8")
		w.Header().Set("Content-Disposition", "inline; filename=\"install.sh\"")
		w.Header().Set("Cache-Control", "public, max-age=3600") // Cache for 1 hour
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Write the script
		_, err = w.Write(script)
		if err != nil {
			log.Printf("Error writing response: %v", err)
		}
	})

	port := ":8080"
	log.Printf("Starting installer service on http://localhost%s", port)
	log.Fatal(http.ListenAndServe(port, nil))
}
