package multichannel

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/BioHazard786/Warpdrop/cli/internal/config"
	"github.com/BioHazard786/Warpdrop/cli/internal/signaling"
	"github.com/BioHazard786/Warpdrop/cli/internal/transfer"
	"github.com/BioHazard786/Warpdrop/cli/internal/ui"
	"github.com/BioHazard786/Warpdrop/cli/internal/webrtc"
	pion "github.com/pion/webrtc/v4"
)

func NewReceiverSession(client *signaling.Client, handler *signaling.Handler, cfg *config.Config, peerInfo *signaling.PeerInfo) (*ReceiverSession, error) {
	peer, err := newReceiverPeer(client, cfg)
	if err != nil {
		return nil, err
	}

	return &ReceiverSession{
		peer:            peer,
		signalingClient: client,
		handler:         handler,
		config:          cfg,
		peerInfo:        peerInfo,
	}, nil
}

func (r *ReceiverSession) SetProgressUI() {
	fileNames := make([]string, len(r.peer.fileChannels))
	fileSizes := make([]int64, len(r.peer.fileChannels))
	for i, fc := range r.peer.fileChannels {
		fileNames[i] = fc.Metadata.Name
		fileSizes[i] = int64(fc.Metadata.Size)
	}
	r.progress = transfer.NewProgressTracker(fileNames, fileSizes)
}

func (r *ReceiverSession) SetOptions(opts *transfer.TransferOptions) {
	r.options = opts
}

func newReceiverPeer(client *signaling.Client, cfg *config.Config) (*ReceiverPeer, error) {
	pc, err := transfer.NewPeerConnection(cfg)
	if err != nil {
		return nil, err
	}

	peer := &ReceiverPeer{
		connection:       pc,
		metadataReceived: make(chan []webrtc.FileMetadata, 1),
		done:             make(chan struct{}),
	}

	transfer.SetupICEHandlers(pc, client, peer.done)
	peer.setupDataHandlers()

	return peer, nil
}

func (p *ReceiverPeer) setupDataHandlers() {
	p.connection.OnDataChannel(func(dc *pion.DataChannel) {
		if dc.Label() == "control" {
			p.controlChannel = dc
			p.setupControlHandlers()
			return
		}

		channel := &ReceiverFileChannel{
			Channel:       dc,
			chunkReceived: make(chan []byte, 128),
			Index:         len(p.fileChannels),
		}
		p.fileChannels = append(p.fileChannels, channel)

		dc.OnOpen(func() {
			atomic.AddInt32(&p.channelsReady, 1)
		})

		dc.OnMessage(func(msg pion.DataChannelMessage) {
			channel.chunkReceived <- msg.Data
		})

		dc.OnClose(func() {
			close(channel.chunkReceived)
		})
	})
}

func (p *ReceiverPeer) setupControlHandlers() {
	p.controlChannel.OnOpen(func() {
		transfer.SendDeviceInfo(p.controlChannel)
	})

	p.controlChannel.OnMessage(func(msg pion.DataChannelMessage) {
		message, err := transfer.ParseMessage(msg.Data)
		if err != nil {
			return
		}

		switch message.Type {
		case transfer.MessageTypeFilesMetadata:
			var metas []webrtc.FileMetadata
			if err := message.DecodePayload(&metas); err != nil {
				return
			}
			p.metadataReceived <- metas
		}
	})
}

func (r *ReceiverSession) Start() error {
	stopSpinner := ui.RunConnectionSpinner("Establishing WebRTC connection...")
	defer stopSpinner()

	go r.listenForSignals()

	select {
	case fileMetadataList := <-r.peer.metadataReceived:
		if err := r.addMetadata(fileMetadataList); err != nil {
			return err
		}

	case errMsg := <-r.handler.Error:
		return transfer.WrapError("start", transfer.ErrSignalingError, errMsg)

	case <-time.After(time.Duration(transfer.SignalTimeout) * time.Second):
		return transfer.WrapError("start", transfer.ErrTimeout, "waiting for metadata")
	}

	return nil
}

func (r *ReceiverSession) listenForSignals() {
	for {
		select {
		case sig := <-r.handler.Signal:
			if sig == nil {
				continue
			}
			r.handleSignal(sig)

		case <-r.peer.done:
			return
		}
	}
}

func (r *ReceiverSession) handleSignal(payload *signaling.SignalPayload) error {
	if payload.SDP != "" {
		var sdpType pion.SDPType
		switch payload.Type {
		case "offer":
			sdpType = pion.SDPTypeOffer
		case "answer":
			sdpType = pion.SDPTypeAnswer
		default:
			return transfer.WrapError("handle signal", transfer.ErrUnexpectedSignal, payload.Type)
		}

		desc := pion.SessionDescription{Type: sdpType, SDP: payload.SDP}
		answer, err := transfer.CreateAnswer(r.peer.connection, &desc)
		if err != nil {
			return err
		}

		r.signalingClient.SendMessage(&signaling.Message{
			Type: signaling.MessageTypeSignal,
			Payload: signaling.SignalPayload{
				Type: answer.Type.String(),
				SDP:  answer.SDP,
			},
		})
	}

	return transfer.HandleICECandidate(r.peer.connection, payload)
}

func (r *ReceiverSession) addMetadata(fileMetadataList []webrtc.FileMetadata) error {
	if err := transfer.WaitForChannels(&r.peer.channelsReady, len(fileMetadataList), r.handler.PeerLeft); err != nil {
		return err
	}

	for i, metaData := range fileMetadataList {
		r.peer.fileChannels[i].Metadata = metaData
	}

	return nil
}

func (r *ReceiverSession) Transfer() error {
	items := transfer.BuildFileTable(r.buildMetadataList())
	ui.RenderFileTable(items)

	if !transfer.PromptConsent() {
		transfer.SendSimpleMessage(r.peer.controlChannel, transfer.MessageTypeDeclineReceive)
		return transfer.ErrTransferCancelled
	}

	transfer.SendSimpleMessage(r.peer.controlChannel, transfer.MessageTypeReadyToReceive)

	r.progress.Start()
	fmt.Printf("\n%s Receiving files...\n\n", ui.IconReceive)

	filesCount := len(r.peer.fileChannels)

	wg := &sync.WaitGroup{}
	wg.Add(filesCount)

	for _, fc := range r.peer.fileChannels {
		go r.receiveFile(fc, wg)
	}

	progressDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(progressDone)
	}()

	transfer.RunProgressLoop(progressDone, filesCount, r.progress.View, transfer.ClearProgressLines)
	fmt.Println()

	transfer.SendSimpleMessage(r.peer.controlChannel, transfer.MessageTypeDownloadingDone)

	transfer.RenderSummary(filesCount, r.progress.TotalSize(), r.progress.Duration())
	return nil
}

func (r *ReceiverSession) buildMetadataList() []webrtc.FileMetadata {
	metas := make([]webrtc.FileMetadata, len(r.peer.fileChannels))
	for i, fc := range r.peer.fileChannels {
		metas[i] = fc.Metadata
	}
	return metas
}

func (r *ReceiverSession) receiveFile(fc *ReceiverFileChannel, wg *sync.WaitGroup) error {
	defer wg.Done()

	writer, err := transfer.NewFileWriter(fc.Metadata, fc.Index, r.options)
	if err != nil {
		r.progress.Error(fc.Index, err.Error())
		return err
	}
	defer writer.Close()

	for data := range fc.chunkReceived {
		if _, err := writer.Write(data); err != nil {
			r.progress.Error(fc.Index, err.Error())
			return err
		}

		atomic.StoreInt64(&fc.ReceivedBytes, int64(writer.ReceivedBytes))
		r.progress.Update(fc.Index, int64(writer.ReceivedBytes))

		if writer.IsComplete() {
			r.progress.Complete(fc.Index)
			return nil
		}
	}

	if !writer.IsComplete() {
		r.progress.Error(fc.Index, "channel closed early")
		return transfer.WrapError("receive", transfer.ErrChannelClosed, fc.Metadata.Name)
	}

	r.progress.Complete(fc.Index)
	return nil
}

func (r *ReceiverSession) Close() error {
	if r.peer != nil {
		r.peer.close()
		r.peer = nil
	}
	time.Sleep(100 * time.Millisecond)

	if r.signalingClient != nil {
		r.signalingClient.Close()
		r.signalingClient = nil
	}
	if r.handler != nil {
		r.handler.Close()
		r.handler = nil
	}
	return nil
}

func (p *ReceiverPeer) close() error {
	if p.controlChannel != nil {
		p.controlChannel.Close()
	}
	for _, fc := range p.fileChannels {
		if fc != nil && fc.Channel != nil {
			fc.Channel.Close()
		}
	}
	return p.connection.Close()
}
