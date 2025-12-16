package singlechannel

import (
	"fmt"
	"time"

	"github.com/BioHazard786/Warpdrop/cli/internal/config"
	"github.com/BioHazard786/Warpdrop/cli/internal/signaling"
	"github.com/BioHazard786/Warpdrop/cli/internal/transfer"
	"github.com/BioHazard786/Warpdrop/cli/internal/ui"
	"github.com/BioHazard786/Warpdrop/cli/internal/webrtc"
	pion "github.com/pion/webrtc/v4"
	"github.com/vmihailenco/msgpack/v5"
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
	fileNames := make([]string, len(r.peer.filesMetadata))
	fileSizes := make([]int64, len(r.peer.filesMetadata))
	for i, f := range r.peer.filesMetadata {
		fileNames[i] = f.Name
		fileSizes[i] = int64(f.Size)
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
		metadataReceived: make(chan struct{}, 1),
		chunkReceived:    make(chan msgpack.RawMessage, 128),
		done:             make(chan struct{}),
	}

	transfer.SetupICEHandlers(pc, client, peer.done)
	peer.setupDataHandlers()

	return peer, nil
}

func (p *ReceiverPeer) setupDataHandlers() {
	p.connection.OnDataChannel(func(dc *pion.DataChannel) {
		if dc.Label() != "file-transfer" {
			return
		}
		p.dataChannel = dc

		dc.OnOpen(func() {
			transfer.SendDeviceInfo(dc)
		})

		dc.OnMessage(func(msg pion.DataChannelMessage) {
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
				p.filesMetadata = metas
				p.metadataReceived <- struct{}{}

			case transfer.MessageTypeChunk:
				p.chunkReceived <- message.Payload
			}
		})
	})
}

func (r *ReceiverSession) Start() error {
	stopSpinner := ui.RunConnectionSpinner("Establishing WebRTC connection...")
	defer stopSpinner()

	go r.listenForSignals()

	select {
	case <-r.peer.metadataReceived:
		return nil

	case errMsg := <-r.handler.Error:
		return transfer.WrapError("start", transfer.ErrSignalingError, errMsg)

	case <-time.After(time.Duration(transfer.SignalTimeout) * time.Second):
		return transfer.WrapError("start", transfer.ErrTimeout, "waiting for metadata")
	}
}

func (r *ReceiverSession) listenForSignals() {
	for {
		select {
		case sig := <-r.handler.Signal:
			if sig == nil {
				continue
			}
			if err := r.handleSignal(sig); err != nil {
				continue
			}

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

func (r *ReceiverSession) Transfer() error {
	items := transfer.BuildFileTable(r.peer.filesMetadata)
	ui.RenderFileTable(items)

	if !transfer.PromptConsent() {
		transfer.SendSimpleMessage(r.peer.dataChannel, transfer.MessageTypeDeclineReceive)
		return transfer.ErrTransferCancelled
	}

	r.progress.Start()
	fmt.Printf("\n%s Receiving files...\n\n", ui.IconReceive)

	filesCount := len(r.peer.filesMetadata)

	progressDone := make(chan struct{})
	defer close(progressDone)

	go transfer.RunProgressLoop(progressDone, filesCount, r.progress.View, transfer.ClearProgressLines)

	for i, meta := range r.peer.filesMetadata {
		if err := transfer.SendReadyToReceive(r.peer.dataChannel, meta.Name, 0); err != nil {
			return err
		}

		if err := r.receiveFile(meta, i); err != nil {
			return transfer.NewFileError("receive", meta.Name, err)
		}
	}

	transfer.ClearProgressLines(filesCount)
	fmt.Print(r.progress.View())
	fmt.Println()

	transfer.SendSimpleMessage(r.peer.dataChannel, transfer.MessageTypeDownloadingDone)

	transfer.RenderSummary(filesCount, r.progress.TotalSize(), r.progress.Duration())
	return nil
}

func (r *ReceiverSession) receiveFile(meta webrtc.FileMetadata, index int) error {
	writer, err := transfer.NewFileWriter(meta, index, r.options)
	if err != nil {
		return err
	}
	defer writer.Close()

	for {
		select {
		case rawChunk := <-r.peer.chunkReceived:
			var chunk webrtc.ChunkPayload
			if err := msgpack.Unmarshal(rawChunk, &chunk); err != nil {
				return transfer.NewError("decode chunk", err)
			}

			if chunk.FileName != meta.Name {
				return transfer.WrapError("receive", transfer.ErrFilenameMismatch, chunk.FileName)
			}

			if _, err := writer.WriteAt(chunk.Bytes, chunk.Offset); err != nil {
				return err
			}

			r.progress.Update(index, int64(writer.ReceivedBytes))

			if chunk.Final {
				r.progress.Complete(index)
				return nil
			}

		case <-r.handler.PeerLeft:
			return transfer.ErrPeerDisconnected

		case <-time.After(30 * time.Second):
			return transfer.WrapError("receive", transfer.ErrTimeout, "waiting for data")
		}
	}
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
	if p.dataChannel != nil {
		p.dataChannel.Close()
	}
	return p.connection.Close()
}
