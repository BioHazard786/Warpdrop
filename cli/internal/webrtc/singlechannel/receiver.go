package singlechannel

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/BioHazard786/Warpdrop/cli/internal/config"
	"github.com/BioHazard786/Warpdrop/cli/internal/signaling"
	"github.com/BioHazard786/Warpdrop/cli/internal/ui"
	"github.com/BioHazard786/Warpdrop/cli/internal/utils"
	"github.com/BioHazard786/Warpdrop/cli/internal/version"
	"github.com/BioHazard786/Warpdrop/cli/internal/webrtc"
	pion "github.com/pion/webrtc/v4"
	"github.com/vmihailenco/msgpack/v5"
)

// NewReceiverSession creates a receiver-side WebRTC session
func NewReceiverSession(client *signaling.Client, handler *signaling.Handler, cfg *config.Config, peerInfo *signaling.PeerInfo) (*ReceiverSession, error) {
	peer, err := newReceiverPeer(client, cfg)
	if err != nil {
		return nil, err
	}

	return &ReceiverSession{
		Peer:            peer,
		SignalingClient: client,
		Handler:         handler,
		Config:          cfg,
		PeerInfo:        peerInfo,
	}, nil
}

// SetProgressUI initializes the progress UI
func (r *ReceiverSession) SetProgressUI() {
	// Get file names and sizes for UI
	fileNames := make([]string, len(r.Peer.FilesMetadata))
	fileSizes := make([]int64, len(r.Peer.FilesMetadata))
	for i, f := range r.Peer.FilesMetadata {
		fileNames[i] = f.Name
		fileSizes[i] = int64(f.Size)
	}
	r.ProgressModel = ui.NewProgressModel(fileNames, fileSizes)
}

func newReceiverPeer(client *signaling.Client, cfg *config.Config) (*ReceiverPeer, error) {
	pc, err := newPeerConnection(cfg)
	if err != nil {
		return nil, err
	}

	peer := &ReceiverPeer{
		Connection:       pc,
		metadataReceived: make(chan struct{}, 1),
		chunkReceived:    make(chan msgpack.RawMessage, 128),
		done:             make(chan struct{}),
	}

	peer.SetupHandlers(client)
	peer.SetupDataHandlers()

	return peer, nil
}

// setupHandlers configures ICE connection state handlers
func (p *ReceiverPeer) SetupHandlers(signalingClient *signaling.Client) {
	p.Connection.OnICEConnectionStateChange(func(state pion.ICEConnectionState) {
		if state == pion.ICEConnectionStateFailed || state == pion.ICEConnectionStateClosed {
			select {
			case p.done <- struct{}{}:
			default:
			}
		}
	})

	// Setup ICE candidate handler
	p.Connection.OnICECandidate(func(c *pion.ICECandidate) {
		if c == nil {
			return
		}

		signalingClient.SendMessage(&signaling.Message{
			Type:    signaling.MessageTypeSignal,
			Payload: signaling.SignalPayload{ICECandidate: c.ToJSON()},
		})
	})
}

// setupDataHandlers configures data channel event handlers
func (p *ReceiverPeer) SetupDataHandlers() {
	p.Connection.OnDataChannel(func(dc *pion.DataChannel) {
		if dc.Label() != "file-transfer" {
			return
		}
		p.dataChannel = dc

		dc.OnOpen(func() {
			p.sendDeviceInfo()
		})

		dc.OnMessage(func(msg pion.DataChannelMessage) {
			var message webrtc.Message
			if err := msgpack.Unmarshal(msg.Data, &message); err != nil {
				fmt.Printf("❌ Failed to parse message: %v\n", err)
				return
			}

			switch message.Type {
			case MessageTypeFilesMetadata:
				var metas []webrtc.FileMetadata
				if err := message.DecodePayload(&metas); err != nil {
					fmt.Printf("❌ Failed to decode metadata: %v\n", err)
					return
				}
				p.FilesMetadata = metas
				p.metadataReceived <- struct{}{}

			case MessageTypeChunk:
				p.chunkReceived <- message.Payload
			}
		})
	})
}

// sendDeviceInfo sends device information to sender
func (p *ReceiverPeer) sendDeviceInfo() {
	deviceInfo, err := webrtc.NewMessage(
		MessageTypeDeviceInfo,
		webrtc.DeviceInfoPayload{
			DeviceName:    "CLI",
			DeviceVersion: strings.TrimPrefix(version.Version, "v"),
		},
	)

	if err != nil {
		fmt.Printf("❌ Failed to marshal payload: %v\n", err)
		return
	}

	data, err := msgpack.Marshal(deviceInfo)

	if err != nil {
		fmt.Printf("❌ Failed to marshal metadata: %v\n", err)
		return
	}

	if err := p.dataChannel.Send(data); err != nil {
		fmt.Printf("❌ Failed to send metadata: %v\n", err)
		return
	}
}

// Start establishes the WebRTC connection
func (r *ReceiverSession) Start() error {
	stopSpinner := ui.RunConnectionSpinner("Establishing WebRTC connection...")
	defer stopSpinner()

	// Start signal listener
	go r.listenForSignals()

	// Wait for answer from sender
	select {
	case <-r.Peer.metadataReceived:

	case errMsg := <-r.Handler.Error:
		return fmt.Errorf("signaling error: %s", errMsg)

	case <-time.After(time.Duration(SignalTimeout) * time.Second):
		return fmt.Errorf("timeout waiting for answer")
	}

	return nil
}

// listenForSignals handles incoming signaling messages (ICE candidates and answer)
func (r *ReceiverSession) listenForSignals() {
	for {
		select {
		case sig := <-r.Handler.Signal:
			if sig == nil {
				continue
			}
			if err := r.Peer.HandleSignal(sig, r.SignalingClient); err != nil {
				fmt.Printf("⚠️  Signal handling error: %v\n", err)
			}

		case <-r.Peer.done:
			return
		}
	}
}

// HandleSignal processes incoming signaling messages (SDP and ICE candidates)
func (p *ReceiverPeer) HandleSignal(payload *signaling.SignalPayload, signalingClient *signaling.Client) error {
	if payload.SDP != "" {
		var sdpType pion.SDPType
		switch payload.Type {
		case "offer":
			sdpType = pion.SDPTypeOffer
		case "answer":
			sdpType = pion.SDPTypeAnswer
		default:
			return fmt.Errorf("unexpected signal type: %s", payload.Type)
		}

		desc := pion.SessionDescription{
			Type: sdpType,
			SDP:  payload.SDP,
		}

		answer, err := p.CreateAnswer(&desc)
		if err != nil {
			return fmt.Errorf("failed to create answer: %w", err)
		}

		signalingClient.SendMessage(&signaling.Message{
			Type: signaling.MessageTypeSignal,
			Payload: signaling.SignalPayload{
				Type: answer.Type.String(),
				SDP:  answer.SDP,
			},
		})
	}

	// Handle ICE candidate
	if payload.ICECandidate != nil {
		candidateBytes, _ := json.Marshal(payload.ICECandidate)
		var ice pion.ICECandidateInit
		if err := json.Unmarshal(candidateBytes, &ice); err != nil {
			return fmt.Errorf("failed to parse ICE candidate: %w", err)
		}
		if err := p.Connection.AddICECandidate(ice); err != nil {
			return fmt.Errorf("failed to add ICE candidate: %w", err)
		}
	}

	return nil
}

// CreateAnswer creates WebRTC answer from offer
func (p *ReceiverPeer) CreateAnswer(offer *pion.SessionDescription) (*pion.SessionDescription, error) {
	if err := p.Connection.SetRemoteDescription(*offer); err != nil {
		return nil, err
	}

	answer, err := p.Connection.CreateAnswer(nil)
	if err != nil {
		return nil, err
	}

	if err = p.Connection.SetLocalDescription(answer); err != nil {
		return nil, err
	}

	return p.Connection.LocalDescription(), nil
}

func (r *ReceiverSession) Transfer() error {
	// Display files to receive in a table
	items := make([]ui.FileTableItem, 0, len(r.Peer.FilesMetadata))
	for i, meta := range r.Peer.FilesMetadata {
		items = append(items, ui.FileTableItem{
			Index: i + 1,
			Name:  meta.Name,
			Size:  int64(meta.Size),
			Type:  meta.Type,
		})
	}
	ui.RenderFileTable(items, "📋 Files to receive")

	// Take user consent before receiving files
	fmt.Print("\n❓ Do you want to receive these files? [Y/n] ")
	var consent string
	fmt.Scanln(&consent)

	if consent == "n" || consent == "N" {
		if err := r.SendDecline(); err != nil {
			return fmt.Errorf("failed to send decline signal: %w", err)
		}
		return fmt.Errorf("transfer cancelled by user")
	}

	r.globalStartTime = time.Now().UnixMilli()

	fmt.Printf("\n%s Receiving files...\n\n", ui.IconReceive)

	filesCount := len(r.Peer.FilesMetadata)
	var totalSize int64 = 0
	for _, f := range r.Peer.FilesMetadata {
		totalSize += int64(f.Size)
	}

	// UI Progress Loop (Background)
	progressDone := make(chan struct{})
	defer close(progressDone)

	go r.runProgressLoop(progressDone, filesCount)

	for i, meta := range r.Peer.FilesMetadata {

		// Request the file from Sender
		if err := r.SendReadyToReceive(meta.Name, 0); err != nil {
			return err
		}

		// Wait until the file is fully received
		if err := r.receiveFile(meta, i); err != nil {
			return fmt.Errorf("failed to receive %s: %w", meta.Name, err)
		}

	}

	// Final update - clear progress lines and print final state
	r.clearProgressLines(filesCount)
	if r.ProgressModel != nil {
		fmt.Print(r.ProgressModel.View())
	}
	fmt.Println()

	if err := r.SendDownloadingDone(); err != nil {
		fmt.Printf("⚠️  Failed to send completion signal: %v\n", err)
	}

	// Calculate stats
	duration := time.Since(time.UnixMilli(r.globalStartTime))
	seconds := duration.Seconds()

	// Display final stats
	fmt.Println()
	ui.RenderTransferSummary("📊 Receive Summary", ui.TransferSummary{
		Status:    "✅ Complete",
		Files:     filesCount,
		TotalSize: utils.FormatSize(totalSize),
		Duration:  utils.FormatTimeDuration(duration),
		Speed:     utils.FormatSpeed(float64(totalSize) / seconds),
	})

	return nil
}

func (s *ReceiverSession) receiveFile(meta webrtc.FileMetadata, index int) error {
	// Create the file handle
	filename := utils.GetUniqueFilename(meta.Name)
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close() // Automatically closes when function returns

	var currentOffset uint64 = 0

	// We keep reading from the channel until we get the "Final" chunk
	for {
		select {
		case rawChunk := <-s.Peer.chunkReceived:

			// --- Decode Chunk ---
			var chunk webrtc.ChunkPayload
			if err := msgpack.Unmarshal(rawChunk, &chunk); err != nil {
				return fmt.Errorf("failed to decode metadata %v", err)
			}

			// --- Validation ---
			// Ensure we aren't receiving data for the wrong file
			if chunk.FileName != meta.Name {
				return fmt.Errorf("filename mismatch: expected %s, got %s", meta.Name, chunk.FileName)
			}

			// --- Seek (If needed) ---
			if chunk.Offset != currentOffset {
				if _, err := file.Seek(int64(chunk.Offset), 0); err != nil {
					return fmt.Errorf("seek failed: %w", err)
				}
				currentOffset = chunk.Offset
			}

			// --- Write ---
			n, err := file.Write(chunk.Bytes)
			if err != nil {
				return fmt.Errorf("write failed: %w", err)
			}
			currentOffset += uint64(n)

			// --- Update UI ---
			if s.ProgressModel != nil {
				s.ProgressModel.UpdateProgress(index, int64(currentOffset))
			}

			// --- Termination Condition ---
			if chunk.Final {
				if s.ProgressModel != nil {
					s.ProgressModel.MarkComplete(index)
				}
				return nil
			}

		case <-s.Handler.PeerLeft:
			return fmt.Errorf("sender disconnected")

		case <-time.After(30 * time.Second):
			return fmt.Errorf("timeout waiting for data")
		}
	}

}

// SendReadyToReceive requests a specific file from the sender
func (r *ReceiverSession) SendReadyToReceive(fileName string, offset uint64) error {
	if r.Peer.dataChannel == nil {
		return fmt.Errorf("data channel not open")
	}

	message, err := webrtc.NewMessage(
		MessageTypeReadyToReceive,
		webrtc.ReadyToReceivePayload{
			FileName: fileName,
			Offset:   offset,
		},
	)
	if err != nil {
		return err
	}

	data, err := msgpack.Marshal(message)
	if err != nil {
		return err
	}
	return r.Peer.dataChannel.Send(data)
}

// SendDownloadingDone sends completion signal to sender
func (r *ReceiverSession) SendDownloadingDone() error {
	if r.Peer.dataChannel == nil {
		return fmt.Errorf("data channel not open")
	}

	message := webrtc.Message{Type: MessageTypeDownloadingDone}
	data, err := msgpack.Marshal(message)
	if err != nil {
		return err
	}
	return r.Peer.dataChannel.Send(data)
}

// SendDecline sends a decline message to the sender
func (r *ReceiverSession) SendDecline() error {
	if r.Peer.dataChannel == nil {
		return fmt.Errorf("control channel not open")
	}

	message := webrtc.Message{Type: MessageTypeDeclineReceive}
	data, err := msgpack.Marshal(message)
	if err != nil {
		return err
	}

	return r.Peer.dataChannel.Send(data)
}

// Helper to keep the main function clean
func (r *ReceiverSession) runProgressLoop(done chan struct{}, numFiles int) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	firstPrint := true

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			if r.ProgressModel != nil {
				if !firstPrint {
					r.clearProgressLines(numFiles)
				}
				firstPrint = false
				// Note: Ensure View() is concurrent-safe if modified by SendFile
				fmt.Print(r.ProgressModel.View())
			}
		}
	}
}

// Helper to clear lines
func (r *ReceiverSession) clearProgressLines(count int) {
	for range count {
		fmt.Print("\033[A\033[2K")
	}
}

// Close closes the session
func (r *ReceiverSession) Close() error {
	// Close peer first (this closes channels)
	if r.Peer != nil {
		if err := r.Peer.Close(); err != nil {
			fmt.Printf("⚠️  Error closing peer: %v\n", err)
		}
		r.Peer = nil
	}

	// Small delay to allow final messages to be sent
	time.Sleep(100 * time.Millisecond)

	// Close signaling last
	if r.SignalingClient != nil {
		r.SignalingClient.Close()
		r.SignalingClient = nil
	}
	if r.Handler != nil {
		r.Handler.Close()
		r.Handler = nil
	}
	return nil
}

func (p *ReceiverPeer) Close() error {
	if p.dataChannel != nil {
		p.dataChannel.Close()
	}
	return p.Connection.Close()
}
