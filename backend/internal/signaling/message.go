package signaling

import "encoding/json"

// Message defines the structure for all C2S (Client to Server)
// and S2C (Server to Client) websocket messages.
type Message struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
	RoomID  string          `json:"room_id,omitempty"`

	// client is the client that sent the message.
	// It's used internally by the Hub and not sent over JSON.
	client *Client `json:"-"`
}
