package signaling

// Message represents all WebSocket messages between CLI and server.
type Message struct {
	Type       string `json:"type"`
	Payload    any    `json:"payload,omitempty"`
	RoomID     string `json:"room_id,omitempty"`
	ClientType string `json:"client_type,omitempty"`
}

// Message type constants.
const (
	MessageTypeCreateRoom = "create_room"
	MessageTypeJoinRoom   = "join_room"
	MessageTypeSignal     = "signal"

	MessageTypeRoomCreated = "room_created"
	MessageTypeJoinSuccess = "join_success"
	MessageTypePeerJoined  = "peer_joined"
	MessageTypePeerLeft    = "peer_left"
	MessageTypeError       = "error"
)

// SignalPayload represents the WebRTC signaling data (SDP offer/answer or ICE candidate).
type SignalPayload struct {
	Type         string `json:"type,omitempty"`
	SDP          string `json:"sdp,omitempty"`
	ICECandidate any    `json:"ice_candidate,omitempty"`
}

// ErrorPayload represents error messages from server.
type ErrorPayload struct {
	Error string `json:"error"`
}
