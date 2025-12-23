package singlechannel

import (
	"fmt"
	"io"
	"os"
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
	fileNames := make([]string, len(s.peer.files))
	fileSizes := make([]int64, len(s.peer.files))
	for i, f := range s.peer.files {
		fileNames[i] = f.Name
		fileSizes[i] = int64(f.Size)
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

	dc, err := transfer.CreateDataChannel(pc, "file-transfer")
	if err != nil {
		pc.Close()
		return nil, err
	}

	peer := &SenderPeer{
		connection:         pc,
		dataChannel:        dc,
		files:              fileInfos,
		deviceInfoReceived: make(chan webrtc.DeviceInfoPayload, 1),
		receiverReady:      make(chan webrtc.ReadyToReceivePayload, 1),
		declineReceived:    make(chan struct{}, 1),
		downloadingDone:    make(chan struct{}, 1),
		done:               make(chan struct{}),
	}

	transfer.SetupICEHandlers(pc, client, peer.done)
	peer.setupDataHandlers()
	return peer, nil
}

func (p *SenderPeer) setupDataHandlers() {
	p.dataChannel.OnOpen(func() {
		p.sendMetadata()
	})

	p.dataChannel.OnMessage(func(msg pion.DataChannelMessage) {
		message, err := transfer.ParseMessage(msg.Data)
		if err != nil {
			return
		}

		switch message.Type {
		case transfer.MessageTypeReadyToReceive:
			var ready webrtc.ReadyToReceivePayload
			if err := message.DecodePayload(&ready); err != nil {
				return
			}
			p.receiverReady <- ready

		case transfer.MessageTypeDownloadingDone:
			p.downloadingDone <- struct{}{}

		case transfer.MessageTypeDeclineReceive:
			p.declineReceived <- struct{}{}

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
	metadata := make([]webrtc.FileMetadata, len(p.files))
	for i, info := range p.files {
		metadata[i] = webrtc.FileMetadata{
			Name: info.Name,
			Size: uint64(info.Size),
			Type: info.Type,
		}
	}
	transfer.SendFilesMetadata(p.dataChannel, metadata)
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
		case sig, ok := <-s.handler.Signal:
			if !ok {
				return
			}
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

	filesCount := len(s.peer.files)
	fileByName := make(map[string]*files.FileInfo, filesCount)
	fileIndexByName := make(map[string]int, filesCount)

	var totalSize int64
	for i, f := range s.peer.files {
		fileByName[f.Name] = f
		fileIndexByName[f.Name] = i
		totalSize += f.Size
	}

	var readyPayload webrtc.ReadyToReceivePayload

	select {
	case readyPayload = <-s.peer.receiverReady:
		stopSpinner()
	case <-s.peer.declineReceived:
		return transfer.ErrTransferDeclined
	case <-s.handler.PeerLeft:
		return transfer.ErrPeerDisconnected
	case <-s.handler.Error:
		return transfer.ErrSignalingError
	}

	fmt.Printf("\n%s Sending files...\n\n", ui.IconSend)

	s.progress.Start()

	errChan := make(chan error, 1)

	go func() {
		defer s.progress.Program.Quit()

		for i := range filesCount {
			if i > 0 {
				select {
				case readyPayload = <-s.peer.receiverReady:
				case <-s.peer.declineReceived:
					errChan <- transfer.ErrTransferDeclined
					return
				case <-s.handler.PeerLeft:
					errChan <- transfer.ErrPeerDisconnected
					return
				case <-s.handler.Error:
					errChan <- transfer.ErrSignalingError
					return
				}
			}

			fileInfo, ok := fileByName[readyPayload.FileName]
			if !ok {
				errChan <- transfer.WrapError("transfer", transfer.ErrInvalidFile, readyPayload.FileName)
				return
			}

			fileIndex := fileIndexByName[readyPayload.FileName]
			if err := s.sendFile(fileInfo, readyPayload.Offset, fileIndex); err != nil {
				errChan <- err
				return
			}
		}

		select {
		case <-s.peer.downloadingDone:
		case <-s.handler.PeerLeft:
			errChan <- transfer.ErrPeerDisconnected
			return
		case <-time.After(10 * time.Second):
			// We don't fail the transfer here, just log warning after UI cleans up
		}

		errChan <- nil
	}()

	// Block until UI is done
	if err := s.progress.Run(); err != nil {
		return err
	}

	// Check if there was an error during transfer
	transferErr := <-errChan
	if transferErr != nil {
		return transferErr
	}

	transfer.RenderSummary(filesCount, totalSize, s.progress.Duration())
	return nil
}

func (s *SenderSession) sendFile(fileInfo *files.FileInfo, startOffset uint64, fileIndex int) error {
	file, err := os.Open(fileInfo.Path)
	if err != nil {
		return transfer.NewFileError("open", fileInfo.Name, err)
	}
	defer file.Close()

	if _, err := file.Seek(int64(startOffset), io.SeekStart); err != nil {
		return transfer.NewFileError("seek", fileInfo.Name, err)
	}

	sender := transfer.NewSingleChannelFileSender(s.peer.dataChannel, fileInfo.Name, fileInfo.Size)

	return sender.SendChunks(
		file,
		startOffset,
		func(offset uint64) { s.progress.Update(fileIndex, int64(offset)) },
		func() { s.progress.Complete(fileIndex) },
		func(msg string) { s.progress.Error(fileIndex, msg) },
	)
}

func (s *SenderSession) Close() error {
	if s.peer != nil {
		s.peer.close()
	}
	time.Sleep(100 * time.Millisecond)

	if s.signalingClient != nil {
		s.signalingClient.Close()
	}
	if s.handler != nil {
		s.handler.Close()
	}
	return nil
}

func (p *SenderPeer) close() error {
	if p.dataChannel != nil {
		p.dataChannel.Close()
	}
	return p.connection.Close()
}
