package multichannel

import (
	"os"

	"github.com/BioHazard786/Warpdrop/cli/internal/files"
	"github.com/BioHazard786/Warpdrop/cli/internal/signaling"
	"github.com/BioHazard786/Warpdrop/cli/internal/ui"
	"github.com/pion/webrtc/v4"
)

const (
	PacketSize      = 16 * 1024  // 16KB chunks
	BufferThreshold = 512 * 1024 // 512KB threshold
)

// FileMetadata represents file metadata for multi-channel transfers
type FileMetadata struct {
	Name string `msgpack:"name"`
	Size uint64 `msgpack:"size"`
	Type string `msgpack:"type"`
}

// SenderSession manages the entire WebRTC file transfer session
type SenderSession struct {
	Peer            *SenderPeer
	SignalingClient *signaling.Client
	Handler         *signaling.Handler
	Config          any // config.Config
	PeerInfo        *signaling.PeerInfo

	globalStartTime int64

	// Bubble Tea progress UI
	ProgressModel *ui.ProgressModel
	fileNames     []string
	fileSizes     []int64
}

// SenderPeer manages WebRTC connection and channels
type SenderPeer struct {
	Connection      *webrtc.PeerConnection
	controlChannel  *webrtc.DataChannel
	dataChannels    []*FileChannel
	channelsReady   int32 // atomic counter for opened channels
	readyReceived   chan struct{}
	declineReceived chan struct{} // receiver declined the transfer
	downloadingDone chan struct{}
	done            chan struct{}
}

// FileChannel wraps a data channel for file transfer
type FileChannel struct {
	Channel   *webrtc.DataChannel
	FileInfo  *files.FileInfo
	File      *os.File
	Packet    []byte
	Index     int   // file index for progress tracking
	SentBytes int64 // atomic counter for bytes sent
}

// ReceiverSession manages receiving files
type ReceiverSession struct {
	PeerConnection  *webrtc.PeerConnection
	SignalingClient *signaling.Client // For sending ICE candidates

	controlChannel *webrtc.DataChannel // For metadata and control
	DataChannels   []*ReceiverChannel  // One channel per file

	FilesMetadata   []any // FileMetadata slice
	metadataReady   chan struct{}
	done            chan struct{}
	globalStartTime int64

	// Bubble Tea progress UI
	ProgressModel *ui.ProgressModel
}

// ReceiverChannel represents a data channel receiving a file
type ReceiverChannel struct {
	Channel       *webrtc.DataChannel
	Metadata      any // FileMetadata
	File          *os.File
	MessageChan   chan []byte
	Done          chan struct{}
	Index         int   // file index for progress tracking
	ReceivedBytes int64 // atomic counter for bytes received
}
