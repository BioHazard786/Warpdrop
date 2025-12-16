package multichannel

import (
	"os"

	"github.com/BioHazard786/Warpdrop/cli/internal/config"
	"github.com/BioHazard786/Warpdrop/cli/internal/files"
	"github.com/BioHazard786/Warpdrop/cli/internal/signaling"
	"github.com/BioHazard786/Warpdrop/cli/internal/transfer"
	"github.com/BioHazard786/Warpdrop/cli/internal/webrtc"
	pion "github.com/pion/webrtc/v4"
)

type SenderSession struct {
	peer            *SenderPeer
	signalingClient *signaling.Client
	handler         *signaling.Handler
	config          *config.Config
	peerInfo        *signaling.PeerInfo
	progress        *transfer.ProgressTracker
	options         *transfer.TransferOptions
}

type SenderPeer struct {
	connection         *pion.PeerConnection
	controlChannel     *pion.DataChannel
	fileChannels       []*SenderFileChannel
	channelsReady      int32
	deviceInfoReceived chan webrtc.DeviceInfoPayload
	receiverReady      chan struct{}
	declineReceived    chan struct{}
	downloadingDone    chan struct{}
	done               chan struct{}
}

type SenderFileChannel struct {
	Channel   *pion.DataChannel
	FileInfo  *files.FileInfo
	File      *os.File
	Index     int
	SentBytes int64
}

type ReceiverSession struct {
	peer            *ReceiverPeer
	signalingClient *signaling.Client
	handler         *signaling.Handler
	config          *config.Config
	peerInfo        *signaling.PeerInfo
	progress        *transfer.ProgressTracker
	options         *transfer.TransferOptions
}

type ReceiverPeer struct {
	connection       *pion.PeerConnection
	controlChannel   *pion.DataChannel
	fileChannels     []*ReceiverFileChannel
	channelsReady    int32
	metadataReceived chan []webrtc.FileMetadata
	done             chan struct{}
}

type ReceiverFileChannel struct {
	Channel       *pion.DataChannel
	Metadata      webrtc.FileMetadata
	chunkReceived chan []byte
	Index         int
	ReceivedBytes int64
}
