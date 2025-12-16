package singlechannel

import (
	"os"

	"github.com/BioHazard786/Warpdrop/cli/internal/config"
	"github.com/BioHazard786/Warpdrop/cli/internal/files"
	"github.com/BioHazard786/Warpdrop/cli/internal/signaling"
	"github.com/BioHazard786/Warpdrop/cli/internal/transfer"
	"github.com/BioHazard786/Warpdrop/cli/internal/webrtc"
	pion "github.com/pion/webrtc/v4"
	"github.com/vmihailenco/msgpack/v5"
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
	dataChannel        *pion.DataChannel
	files              []*files.FileInfo
	deviceInfoReceived chan webrtc.DeviceInfoPayload
	receiverReady      chan webrtc.ReadyToReceivePayload
	declineReceived    chan struct{}
	downloadingDone    chan struct{}
	done               chan struct{}
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
	dataChannel      *pion.DataChannel
	filesMetadata    []webrtc.FileMetadata
	metadataReceived chan struct{}
	chunkReceived    chan msgpack.RawMessage
	done             chan struct{}
}

type FileContext struct {
	Info  *files.FileInfo
	File  *os.File
	Index int
}
