package singlechannel

import (
	"github.com/BioHazard786/Warpdrop/cli/internal/config"
	"github.com/BioHazard786/Warpdrop/cli/internal/files"
	"github.com/BioHazard786/Warpdrop/cli/internal/signaling"
	"github.com/BioHazard786/Warpdrop/cli/internal/ui"
	"github.com/BioHazard786/Warpdrop/cli/internal/utils"
	"github.com/BioHazard786/Warpdrop/cli/internal/webrtc"
	pion "github.com/pion/webrtc/v4"
	"github.com/vmihailenco/msgpack/v5"
)

// Protocol constants - use utils package for dynamic values
var (
	HighWaterMark = utils.HighWaterMark
	LowWaterMark  = utils.LowWaterMark
	SendTimeout   = utils.SendTimeout
	DrainTimeout  = utils.DrainTimeout
	SignalTimeout = utils.SignalTimeout
)

// Message types for single-channel protocol
const (
	MessageTypeFilesMetadata   = "files_metadata"
	MessageTypeDeviceInfo      = "device_info"
	MessageTypeReadyToReceive  = "ready_to_receive"
	MessageTypeChunk           = "chunk"
	MessageTypeDownloadingDone = "downloading_done"
	MessageTypeDeclineReceive  = "decline_receive"
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

// SenderPeer manages WebRTC connection and the single data channel
type SenderPeer struct {
	Connection  *pion.PeerConnection
	dataChannel *pion.DataChannel
	files       []*files.FileInfo

	deviceInfoReceived chan webrtc.DeviceInfoPayload
	receiverReady      chan webrtc.ReadyToReceivePayload
	declineReceived    chan struct{} // receiver declined the transfer
	downloadingDone    chan struct{}
	done               chan struct{}
}

type ReceiverPeer struct {
	Connection  *pion.PeerConnection
	dataChannel *pion.DataChannel

	FilesMetadata []webrtc.FileMetadata

	metadataReceived chan struct{}
	chunkReceived    chan msgpack.RawMessage
	done             chan struct{}
}

// ReceiverSession manages receiving files using single channel
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
