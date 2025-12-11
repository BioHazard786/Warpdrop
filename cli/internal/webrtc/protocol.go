package webrtc

type ProtocolType string

const (
	// MultiChannelProtocol uses one data channel per file for parallel transfers
	MultiChannelProtocol ProtocolType = "multi-channel"

	// SingleChannelProtocol uses one data channel for sequential transfers (web-compatible)
	SingleChannelProtocol ProtocolType = "single-channel"
)

// SelectProtocol determines which protocol to use based on peer capabilities
func SelectProtocol(peerType string) ProtocolType {
	// Check if peer is CLI and supports multi-channel
	if peerType == "cli" {
		return MultiChannelProtocol
	}

	// Default to single-channel for web compatibility
	return SingleChannelProtocol
}
