package signaling

// Message represents all WebSocket messages between CLI and server.
type Message struct {
	Type       string `json:"type"`                  // Message type (e.g., "create_room")
	Payload    any    `json:"payload,omitempty"`     // Payload as any for proper JSON encoding
	RoomID     string `json:"room_id,omitempty"`     // Room ID for join/create operations
	ClientType string `json:"client_type,omitempty"` // "cli" or "web"
}

// Message type constants
const (
	// Client to Server (C2S) messages
	MessageTypeCreateRoom = "create_room"
	MessageTypeJoinRoom   = "join_room"
	MessageTypeSignal     = "signal"

	// Server to Client (S2C) messages
	MessageTypeRoomCreated = "room_created"
	MessageTypeJoinSuccess = "join_success"
	MessageTypePeerJoined  = "peer_joined"
	MessageTypePeerLeft    = "peer_left"
	MessageTypeError       = "error"
)

// SignalPayload represents the WebRTC signaling data (SDP offer/answer or ICE candidate)
type SignalPayload struct {
	Type         string         `json:"type,omitempty"`          // "offer" or "answer"
	SDP          string         `json:"sdp,omitempty"`           // Session Description Protocol
	ICECandidate map[string]any `json:"ice_candidate,omitempty"` // ICE candidate data
}

// ErrorPayload represents error messages from server
type ErrorPayload struct {
	Error string `json:"error"`
}
