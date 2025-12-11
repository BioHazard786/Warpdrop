package singlechannel

import (
	"os"

	"github.com/BioHazard786/Warpdrop/cli/internal/config"
	"github.com/BioHazard786/Warpdrop/cli/internal/files"
	"github.com/BioHazard786/Warpdrop/cli/internal/signaling"
	"github.com/BioHazard786/Warpdrop/cli/internal/ui"
	"github.com/pion/webrtc/v4"
)

// Protocol constants
const (
	PacketSize    = 60 * 1024       // 60KB chunks (matches webapp)
	HighWaterMark = 2 * 1024 * 1024 // 2MB backpressure threshold
	LowWaterMark  = 512 * 1024      // 512KB resume threshold
	SendTimeout   = 20              // seconds
	SignalTimeout = 30              // seconds
)

// Message types for single channel protocol
const (
	MessageTypeFilesMetadata   = "files_metadata"
	MessageTypeDeviceInfo      = "device_info"
	MessageTypeReadyToReceive  = "ready_to_receive"
	MessageTypeChunk           = "chunk"
	MessageTypeDownloadingDone = "downloading_done"
)

// FileMetadata represents a single file's metadata
type FileMetadata struct {
	Name string `msgpack:"name"`
	Size uint64 `msgpack:"size"`
	Type string `msgpack:"type"`
}

// Message represents all WebRTC data channel messages
type Message struct {
	Type    string `msgpack:"type"`
	Payload any    `msgpack:"payload"`
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

// SenderSession manages the entire WebRTC file transfer session
type SenderSession struct {
	Peer            *SenderPeer
	SignalingClient *signaling.Client
	Handler         *signaling.Handler
	Config          *config.Config
	PeerInfo        *signaling.PeerInfo

	globalStartTime int64

	// Bubble Tea progress UI
	ProgressModel *ui.ProgressModel
	fileNames     []string
	fileSizes     []int64
}

// SenderPeer manages WebRTC connection and the single data channel
type SenderPeer struct {
	Connection  *webrtc.PeerConnection
	DataChannel *webrtc.DataChannel
	Files       []*files.FileInfo

	answerReceived chan struct{}
	readyReceived  chan ReadyToReceivePayload
	downloadDone   chan struct{}
	signalDone     chan struct{}
}

// ReceiverSession manages receiving files using single channel
type ReceiverSession struct {
	PeerConnection *webrtc.PeerConnection
	DataChannel    *webrtc.DataChannel

	FilesMetadata []FileMetadata
	metadataReady chan struct{}
	done          chan struct{}
	gatherDone    <-chan struct{}

	// Current file state
	currentFile   *os.File
	currentMeta   *FileMetadata
	currentOffset uint64
	currentIndex  int

	globalStartTime int64

	// Bubble Tea progress UI
	ProgressModel *ui.ProgressModel
}
