package config

import (
	"fmt"
	"os"
)

// Default configuration values (production)
const (
	DefaultDomain   = "warpdrop.qzz.io"
	DefaultSTUN     = "stun:stun.l.google.com:19302"
	DefaultTURN     = "turn:warpdrop.qzz.io" // Optional, empty by default
	DefaultTURNUser = "warpdrop"
	DefaultTURNPass = "warpdrop-secret"
)

// Config holds application configuration
type Config struct {
	// Domain is the backend server domain
	Domain string

	// WebSocketURL is constructed from domain
	WebSocketURL string

	// ICE servers for WebRTC
	STUNServer string
	TURNServer string
	TURNUser   string
	TURNPass   string
}

// Options for loading config with CLI flag overrides
type Options struct {
	Domain     string
	STUNServer string
	TURNServer string
	TURNUser   string
	TURNPass   string
}

// Load reads configuration with the following priority:
// 1. CLI flags (passed via Options) - highest priority
// 2. Environment variables
// 3. Hardcoded defaults - lowest priority
func Load(opts Options) (*Config, error) {
	// Load domain: CLI flag > env > default
	domain := opts.Domain
	if domain == "" {
		domain = os.Getenv("DOMAIN")
	}
	if domain == "" {
		domain = DefaultDomain
	}

	// Load STUN server: CLI flag > env > default
	stunServer := opts.STUNServer
	if stunServer == "" {
		stunServer = os.Getenv("STUN_SERVER")
	}
	if stunServer == "" {
		stunServer = DefaultSTUN
	}

	// Load TURN server: CLI flag > env > default
	turnServer := opts.TURNServer
	if turnServer == "" {
		turnServer = os.Getenv("TURN_SERVER")
	}
	if turnServer == "" {
		turnServer = DefaultTURN
	}

	// Load TURN credentials: CLI flag > env > default
	turnUser := opts.TURNUser
	if turnUser == "" {
		turnUser = os.Getenv("TURN_USERNAME")
	}
	if turnUser == "" {
		turnUser = DefaultTURNUser
	}

	turnPass := opts.TURNPass
	if turnPass == "" {
		turnPass = os.Getenv("TURN_PASSWORD")
	}
	if turnPass == "" {
		turnPass = DefaultTURNPass
	}

	// Construct WebSocket URL
	wsURL := fmt.Sprintf("wss://%s/ws", domain)

	return &Config{
		Domain:       domain,
		WebSocketURL: wsURL,
		STUNServer:   stunServer,
		TURNServer:   turnServer,
		TURNUser:     turnUser,
		TURNPass:     turnPass,
	}, nil
}

// GetRoomLink returns the webapp URL for a room ID
func (c *Config) GetRoomLink(roomID string) string {
	return fmt.Sprintf("https://%s/r/%s", c.Domain, roomID)
}

// GetSTUNServers returns STUN server URLs as strings
func (c *Config) GetSTUNServers() []string {
	return []string{c.STUNServer}
}

// GetTURNServers returns TURN server URLs if configured
func (c *Config) GetTURNServers() []string {
	if c.TURNServer == "" {
		return nil
	}
	return []string{
		fmt.Sprintf("%s:3478?transport=udp", c.TURNServer),
		fmt.Sprintf("%s:3478?transport=tcp", c.TURNServer),
		fmt.Sprintf("turns:%s:5349?transport=tcp", c.TURNServer),
	}
}

// GetTURNCredentials returns TURN username and password
func (c *Config) GetTURNCredentials() (string, string) {
	return c.TURNUser, c.TURNPass
}
