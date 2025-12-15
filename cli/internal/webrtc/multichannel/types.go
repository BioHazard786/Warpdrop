package multichannel

import (
	"os"

	"github.com/BioHazard786/Warpdrop/cli/internal/config"
	"github.com/BioHazard786/Warpdrop/cli/internal/files"
	"github.com/BioHazard786/Warpdrop/cli/internal/signaling"
	"github.com/BioHazard786/Warpdrop/cli/internal/ui"
	"github.com/BioHazard786/Warpdrop/cli/internal/utils"
	"github.com/BioHazard786/Warpdrop/cli/internal/webrtc"
	pion "github.com/pion/webrtc/v4"
)

// Protocol constants - use utils package for dynamic values
var (
	HighWaterMark = utils.HighWaterMark
	LowWaterMark  = utils.LowWaterMark
	SendTimeout   = utils.SendTimeout
	DrainTimeout  = utils.DrainTimeout
	SignalTimeout = utils.SignalTimeout
)

// Message types for multi-channel protocol
const (
	MessageTypeFilesMetadata   = "files_metadata"
	MessageTypeReadyToReceive  = "ready_to_receive"
	MessageTypeDeclineReceive  = "decline_receive"
	MessageTypeDownloadingDone = "downloading_done"
	MessageTypeDeviceInfo      = "device_info"
)

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

// SenderPeer manages WebRTC connection and channels
type SenderPeer struct {
	Connection     *pion.PeerConnection
	controlChannel *pion.DataChannel
	dataChannels   []*SenderFileChannel

	channelsReady int32 // atomic counter for opened channels

	deviceInfoReceived chan webrtc.DeviceInfoPayload
	receiverReady      chan struct{}
	declineReceived    chan struct{} // receiver declined the transfer
	downloadingDone    chan struct{}
	done               chan struct{}
}

// SenderFileChannel wraps a data channel for file transfer
type SenderFileChannel struct {
	Channel   *pion.DataChannel
	FileInfo  *files.FileInfo
	File      *os.File
	Packet    []byte
	Index     int   // file index for progress tracking
	SentBytes int64 // atomic counter for bytes sent
}

// ReceiverSession manages receiving files
type ReceiverSession struct {
	Peer            *ReceiverPeer
	SignalingClient *signaling.Client
	Handler         *signaling.Handler
	Config          *config.Config
	PeerInfo        *signaling.PeerInfo

	globalStartTime int64

	// Bubble Tea progress UI
	ProgressModel *ui.ProgressModel
}

// ReceiverPeer manages the WebRTC transport and parallel Data Channels
type ReceiverPeer struct {
	Connection     *pion.PeerConnection
	controlChannel *pion.DataChannel
	dataChannels   []*ReceiverFileChannel

	channelsReady int32 // atomic counter for opened channels

	metadataReceived chan []webrtc.FileMetadata
	done             chan struct{}
}

// ReceiverFileChannel represents a data channel receiving a file
type ReceiverFileChannel struct {
	Channel       *pion.DataChannel
	Metadata      webrtc.FileMetadata
	chunkReceived chan []byte
	Index         int   // file index for progress tracking
	ReceivedBytes int64 // atomic counter for bytes received
}
