package webrtc

import "github.com/vmihailenco/msgpack/v5"

// FileMetadata represents a single file's metadata
type FileMetadata struct {
	Name string `msgpack:"name"`
	Size uint64 `msgpack:"size"`
	Type string `msgpack:"type"`
}

// Message represents all WebRTC data channel messages
type Message struct {
	Type    string             `msgpack:"type"`
	Payload msgpack.RawMessage `msgpack:"payload"`
}

// DeviceInfoPayload is sent by receiver with device info
type DeviceInfoPayload struct {
	DeviceName    string `msgpack:"deviceName"`
	DeviceVersion string `msgpack:"deviceVersion"`
}

// ReadyToReceivePayload is sent by receiver to request a file
type ReadyToReceivePayload struct {
	FileName string `msgpack:"fileName"`
	Offset   uint64 `msgpack:"offset"`
}

// ChunkPayload represents a file chunk
type ChunkPayload struct {
	FileName string `msgpack:"fileName"`
	Offset   uint64 `msgpack:"offset"`
	Bytes    []byte `msgpack:"bytes"`
	Final    bool   `msgpack:"final"`
}

// DecodePayload decodes the message payload into the provided struct
func (m Message) DecodePayload(v any) error {
	return msgpack.Unmarshal(m.Payload, v)
}

// NewMessage creates a new Message with the given type and payload
func NewMessage(t string, payload any) (Message, error) {
	b, err := msgpack.Marshal(payload)
	if err != nil {
		return Message{}, err
	}

	return Message{
		Type:    t,
		Payload: b,
	}, nil
}
