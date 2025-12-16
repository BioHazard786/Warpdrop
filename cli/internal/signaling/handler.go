package signaling

import "encoding/json"

// PeerInfo contains information about the connected peer
type PeerInfo struct {
	ClientType string `json:"client_type"`
}

// Handler routes incoming signaling messages to appropriate channels.
type Handler struct {
	client      *Client
	RoomCreated chan string
	PeerJoined  chan *PeerInfo
	JoinSuccess chan *PeerInfo
	PeerLeft    chan struct{}
	Signal      chan *SignalPayload
	Error       chan string
	closed      bool
}

// NewHandler creates a new message handler.
func NewHandler(client *Client) *Handler {
	return &Handler{
		client:      client,
		RoomCreated: make(chan string, 1),
		PeerJoined:  make(chan *PeerInfo, 1),
		JoinSuccess: make(chan *PeerInfo, 1),
		PeerLeft:    make(chan struct{}, 1),
		Signal:      make(chan *SignalPayload, 32),
		Error:       make(chan string, 1),
	}
}

// Start begins listening to incoming messages and routing them.
func (h *Handler) Start() {
	for msg := range h.client.Incoming() {

		switch msg.Type {

		case MessageTypeRoomCreated:
			h.handleRoomCreated(msg)

		case MessageTypeJoinSuccess:
			h.handleJoinSuccess(msg)

		case MessageTypePeerJoined:
			h.handlePeerJoined(msg)

		case MessageTypePeerLeft:
			h.PeerLeft <- struct{}{}

		case MessageTypeSignal:
			h.handleSignal(msg)

		case MessageTypeError:
			h.handleError(msg)

		default:

		}
	}
}

// handleRoomCreated extracts the room ID and sends it through the channel.
func (h *Handler) handleRoomCreated(msg *Message) {
	h.RoomCreated <- msg.RoomID
}

// handleJoinSuccess is called when we successfully joined a room.
func (h *Handler) handleJoinSuccess(msg *Message) {

	var peerInfo PeerInfo
	if msg.Payload != nil {
		payloadBytes, err := json.Marshal(msg.Payload)
		if err == nil {
			json.Unmarshal(payloadBytes, &peerInfo)
		}
	}

	h.JoinSuccess <- &peerInfo
}

// handlePeerJoined is called when a peer joins our room (sender receives this).
func (h *Handler) handlePeerJoined(msg *Message) {

	var peerInfo PeerInfo
	if msg.Payload != nil {
		payloadBytes, err := json.Marshal(msg.Payload)
		if err == nil {
			json.Unmarshal(payloadBytes, &peerInfo)
		}
	}

	h.PeerJoined <- &peerInfo
}

// handleSignal parses the WebRTC signaling payload and sends it.
func (h *Handler) handleSignal(msg *Message) {
	var payload SignalPayload

	payloadBytes, err := json.Marshal(msg.Payload)
	if err != nil {
		h.Error <- "Failed to parse signal payload"
		return
	}

	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		h.Error <- "Failed to parse signal payload"
		return
	}

	h.Signal <- &payload
}

// handleError parses the error message and sends it through the Error channel.
func (h *Handler) handleError(msg *Message) {
	var errPayload ErrorPayload

	payloadBytes, err := json.Marshal(msg.Payload)
	if err != nil {
		h.Error <- "Unknown error from server"
		return
	}

	if err := json.Unmarshal(payloadBytes, &errPayload); err != nil {
		h.Error <- "Unknown error from server"
		return
	}

	h.Error <- errPayload.Error
}

// Close closes all handler channels.
func (h *Handler) Close() {
	if h.closed {
		return
	}
	h.closed = true

	close(h.RoomCreated)
	close(h.PeerJoined)
	close(h.JoinSuccess)
	close(h.PeerLeft)
	close(h.Signal)
	close(h.Error)
}
