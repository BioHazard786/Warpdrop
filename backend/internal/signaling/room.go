package signaling

// Room represents a single room where two peers (sender and receiver) can connect.
type Room struct {
	// ID is the unique identifier for the room.
	ID string

	// Sender is the client who initiated the room (Peer A).
	Sender *Client

	// Receiver is the client who joined the room (Peer B).
	Receiver *Client
}
