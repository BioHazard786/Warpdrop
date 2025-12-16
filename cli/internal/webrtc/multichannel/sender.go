package multichannel

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/BioHazard786/Warpdrop/cli/internal/config"
	"github.com/BioHazard786/Warpdrop/cli/internal/files"
	"github.com/BioHazard786/Warpdrop/cli/internal/signaling"
	"github.com/BioHazard786/Warpdrop/cli/internal/transfer"
	"github.com/BioHazard786/Warpdrop/cli/internal/ui"
	"github.com/BioHazard786/Warpdrop/cli/internal/webrtc"
	pion "github.com/pion/webrtc/v4"
)

func NewSenderSession(client *signaling.Client, handler *signaling.Handler, cfg *config.Config, fileInfos []*files.FileInfo, peerInfo *signaling.PeerInfo) (*SenderSession, error) {
	peer, err := newSenderPeer(client, cfg, fileInfos)
	if err != nil {
		return nil, err
	}

	return &SenderSession{
		peer:            peer,
		signalingClient: client,
		handler:         handler,
		config:          cfg,
		peerInfo:        peerInfo,
	}, nil
}

func (s *SenderSession) SetProgressUI() {
	fileNames := make([]string, len(s.peer.fileChannels))
	fileSizes := make([]int64, len(s.peer.fileChannels))
	for i, f := range s.peer.fileChannels {
		fileNames[i] = f.FileInfo.Name
		fileSizes[i] = int64(f.FileInfo.Size)
	}
	s.progress = transfer.NewProgressTracker(fileNames, fileSizes)
}

func (s *SenderSession) SetOptions(opts *transfer.TransferOptions) {
	s.options = opts
}

func newSenderPeer(client *signaling.Client, cfg *config.Config, fileInfos []*files.FileInfo) (*SenderPeer, error) {
	pc, err := transfer.NewPeerConnection(cfg)
	if err != nil {
		return nil, err
	}

	cc, err := transfer.CreateDataChannel(pc, "control")
	if err != nil {
		pc.Close()
		return nil, err
	}

	fileChannels := make([]*SenderFileChannel, len(fileInfos))
	for i, fileInfo := range fileInfos {
		fc, err := createFileChannel(pc, fileInfo, i)
		if err != nil {
			pc.Close()
			return nil, err
		}
		fileChannels[i] = fc
	}

	peer := &SenderPeer{
		connection:         pc,
		controlChannel:     cc,
		fileChannels:       fileChannels,
		deviceInfoReceived: make(chan webrtc.DeviceInfoPayload, 1),
		receiverReady:      make(chan struct{}, 1),
		declineReceived:    make(chan struct{}, 1),
		downloadingDone:    make(chan struct{}, 1),
		done:               make(chan struct{}),
	}

	transfer.SetupICEHandlers(pc, client, peer.done)
	peer.setupControlHandlers()
	peer.setupFileHandlers()
	return peer, nil
}

func createFileChannel(pc *pion.PeerConnection, fileInfo *files.FileInfo, index int) (*SenderFileChannel, error) {
	dc, err := transfer.CreateDataChannel(pc, fmt.Sprintf("file-transfer-%d", index))
	if err != nil {
		return nil, err
	}

	file, err := os.Open(fileInfo.Path)
	if err != nil {
		return nil, transfer.NewFileError("open", fileInfo.Name, err)
	}

	return &SenderFileChannel{
		Channel:  dc,
		FileInfo: fileInfo,
		File:     file,
		Index:    index,
	}, nil
}

func (p *SenderPeer) setupControlHandlers() {
	p.controlChannel.OnOpen(func() {
		p.sendMetadata()
	})

	p.controlChannel.OnMessage(func(msg pion.DataChannelMessage) {
		message, err := transfer.ParseMessage(msg.Data)
		if err != nil {
			return
		}

		switch message.Type {
		case transfer.MessageTypeReadyToReceive:
			p.receiverReady <- struct{}{}

		case transfer.MessageTypeDeclineReceive:
			p.declineReceived <- struct{}{}

		case transfer.MessageTypeDownloadingDone:
			p.downloadingDone <- struct{}{}

		case transfer.MessageTypeDeviceInfo:
			var deviceInfo webrtc.DeviceInfoPayload
			if err := message.DecodePayload(&deviceInfo); err != nil {
				return
			}
			p.deviceInfoReceived <- deviceInfo
		}
	})
}

func (p *SenderPeer) sendMetadata() {
	metadata := make([]webrtc.FileMetadata, len(p.fileChannels))
	for i, fc := range p.fileChannels {
		metadata[i] = webrtc.FileMetadata{
			Name: fc.FileInfo.Name,
			Size: uint64(fc.FileInfo.Size),
			Type: fc.FileInfo.Type,
		}
	}
	transfer.SendFilesMetadata(p.controlChannel, metadata)
}

func (p *SenderPeer) setupFileHandlers() {
	for _, fc := range p.fileChannels {
		fc.Channel.OnOpen(func() {
			atomic.AddInt32(&p.channelsReady, 1)
		})
	}
}

func (s *SenderSession) Start() error {
	stopSpinner := ui.RunConnectionSpinner("Establishing WebRTC connection...")
	defer stopSpinner()

	go s.listenForSignals()

	offer, err := transfer.CreateOffer(s.peer.connection)
	if err != nil {
		return err
	}

	s.signalingClient.SendMessage(&signaling.Message{
		Type: signaling.MessageTypeSignal,
		Payload: signaling.SignalPayload{
			Type: offer.Type.String(),
			SDP:  offer.SDP,
		},
	})

	select {
	case deviceInfo := <-s.peer.deviceInfoReceived:
		stopSpinner()
		fmt.Printf("🖥️  Receiver device: %s v%s\n", deviceInfo.DeviceName, deviceInfo.DeviceVersion)

	case errMsg := <-s.handler.Error:
		return transfer.WrapError("start", transfer.ErrSignalingError, errMsg)

	case <-time.After(time.Duration(transfer.SignalTimeout) * time.Second):
		return transfer.WrapError("start", transfer.ErrTimeout, "waiting for answer")
	}

	return nil
}

func (s *SenderSession) listenForSignals() {
	for {
		select {
		case sig := <-s.handler.Signal:
			if sig == nil {
				continue
			}
			transfer.HandleSDPSignal(s.peer.connection, sig)
			transfer.HandleICECandidate(s.peer.connection, sig)

		case <-s.peer.done:
			return
		}
	}
}

func (s *SenderSession) Transfer() error {
	stopSpinner := ui.RunSpinner("Waiting for receiver to accept...")
	defer stopSpinner()

	select {
	case <-s.peer.receiverReady:
		stopSpinner()
	case <-s.peer.declineReceived:
		return transfer.ErrTransferDeclined
	case <-s.handler.PeerLeft:
		return transfer.ErrPeerDisconnected
	case <-s.handler.Error:
		return transfer.ErrSignalingError
	}

	if err := transfer.WaitForChannels(&s.peer.channelsReady, len(s.peer.fileChannels), s.handler.PeerLeft); err != nil {
		return err
	}

	s.progress.Start()
	filesCount := len(s.peer.fileChannels)

	wg := &sync.WaitGroup{}
	wg.Add(filesCount)
	for _, fc := range s.peer.fileChannels {
		go s.sendFile(fc, wg)
	}

	progressDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(progressDone)
	}()

	transfer.RunProgressLoop(progressDone, filesCount, s.progress.View, transfer.ClearProgressLines)
	fmt.Println()

	select {
	case <-s.peer.downloadingDone:
	case <-s.handler.PeerLeft:
		return transfer.ErrPeerDisconnected
	case <-time.After(10 * time.Second):
		fmt.Println(ui.WarningStyle.Render("⚠️  Receiver confirmation timeout (files were sent successfully)"))
	}

	var totalSize int64
	for _, fc := range s.peer.fileChannels {
		totalSize += fc.FileInfo.Size
	}

	transfer.RenderSummary(filesCount, totalSize, s.progress.Duration())
	return nil
}

func (s *SenderSession) sendFile(fc *SenderFileChannel, wg *sync.WaitGroup) error {
	defer wg.Done()
	defer fc.File.Close()

	sender := transfer.NewMultiChannelFileSender(fc.Channel)

	return sender.SendChunks(
		fc.File,
		func(sentBytes int64) {
			atomic.StoreInt64(&fc.SentBytes, sentBytes)
			s.progress.Update(fc.Index, sentBytes)
		},
		func() { s.progress.Complete(fc.Index) },
		func(msg string) { s.progress.Error(fc.Index, msg) },
	)
}

func (s *SenderSession) Close() error {
	if s.peer != nil {
		s.peer.close()
		s.peer = nil
	}
	time.Sleep(100 * time.Millisecond)

	if s.signalingClient != nil {
		s.signalingClient.Close()
		s.signalingClient = nil
	}
	if s.handler != nil {
		s.handler.Close()
		s.handler = nil
	}
	return nil
}

func (p *SenderPeer) close() error {
	if p.controlChannel != nil {
		p.controlChannel.Close()
	}
	for _, fc := range p.fileChannels {
		if fc != nil {
			if fc.Channel != nil {
				fc.Channel.Close()
			}
			if fc.File != nil {
				fc.File.Close()
			}
		}
	}
	return p.connection.Close()
}
