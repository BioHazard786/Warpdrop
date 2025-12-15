package multichannel

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
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
	fileNames := make([]string, len(r.Peer.dataChannels))
	fileSizes := make([]int64, len(r.Peer.dataChannels))
	for i, f := range r.Peer.dataChannels {
		fileNames[i] = f.Metadata.Name
		fileSizes[i] = int64(f.Metadata.Size)
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
		metadataReceived: make(chan []webrtc.FileMetadata, 1),
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

// SetupDataHandlers sets up file and control data channel event handlers
func (p *ReceiverPeer) SetupDataHandlers() {
	p.Connection.OnDataChannel(func(dc *pion.DataChannel) {

		if dc.Label() == "control" {
			p.controlChannel = dc
			p.SetupControlHandlers()
			return
		}

		// File transfer channel
		channel := &ReceiverFileChannel{
			Channel:       dc,
			chunkReceived: make(chan []byte, 128),
			Index:         len(p.dataChannels),
		}

		p.dataChannels = append(p.dataChannels, channel)

		dc.OnOpen(func() {
			atomic.AddInt32(&p.channelsReady, 1)
		})

		dc.OnMessage(func(msg pion.DataChannelMessage) {
			// All messages are file data
			channel.chunkReceived <- msg.Data
		})

		dc.OnClose(func() {
			close(channel.chunkReceived) // Close the channel so receiver goroutine can exit
		})
	})
}

// setupControlHandlers configures control data channel event handlers
func (p *ReceiverPeer) SetupControlHandlers() {

	p.controlChannel.OnOpen(func() {
		p.sendDeviceInfo()
	})

	p.controlChannel.OnMessage(func(msg pion.DataChannelMessage) {
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
			p.metadataReceived <- metas

		}
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

	if err := p.controlChannel.Send(data); err != nil {
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
	case fileMetadataList := <-r.Peer.metadataReceived:
		err := r.addMetaData(fileMetadataList)
		if err != nil {
			return fmt.Errorf("failed to add metadata: %w", err)
		}

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

func (r *ReceiverSession) addMetaData(fileMetadataList []webrtc.FileMetadata) error {
	// Wait for all data channels to be open
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for atomic.LoadInt32(&r.Peer.channelsReady) != int32(len(fileMetadataList)) {
		select {
		case <-r.Handler.PeerLeft:
			return fmt.Errorf("peer disconnected while opening channels")
		case <-timeout:
			return fmt.Errorf("timeout waiting for data channels to open")
		case <-ticker.C:
			// Continue polling
		}
	}

	for i, metaData := range fileMetadataList {
		r.Peer.dataChannels[i].Metadata = metaData
	}

	return nil
}

// Transfer waits for receiver consent and receives all files
func (r *ReceiverSession) Transfer() error {

	// Display files to receive in a table
	items := make([]ui.FileTableItem, 0, len(r.Peer.dataChannels))
	for i, fc := range r.Peer.dataChannels {
		items = append(items, ui.FileTableItem{
			Index: i + 1,
			Name:  fc.Metadata.Name,
			Size:  int64(fc.Metadata.Size),
			Type:  fc.Metadata.Type,
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

	err := r.SendReadyToReceive()
	if err != nil {
		return fmt.Errorf("failed to send ready to receive: %w", err)
	}

	r.globalStartTime = time.Now().UnixMilli()

	fmt.Printf("\n%s Receiving files...\n\n", ui.IconReceive)

	filesCount := len(r.Peer.dataChannels)
	var totalSize int64 = 0
	for _, f := range r.Peer.dataChannels {
		totalSize += int64(f.Metadata.Size)
	}

	// Start concurrent transfers with Bubble Tea progress UI
	wg := &sync.WaitGroup{}
	wg.Add(filesCount)

	for _, fileChannel := range r.Peer.dataChannels {
		go r.receiveFile(fileChannel, wg)
	}

	// Render progress while transfers are running
	progressDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(progressDone)
	}()

	// Start progress display in background
	r.runProgressLoop(progressDone, filesCount)
	fmt.Println()

	// Send completion signal
	if err := r.SendDownloadingDone(); err != nil {
		fmt.Println(ui.WarningStyle.Render("⚠️  Failed to send completion signal"))
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

// receiveFile writes data from channel to file (runs in goroutine)
func (r *ReceiverSession) receiveFile(fileChannel *ReceiverFileChannel, wg *sync.WaitGroup) error {
	defer wg.Done()

	// Get unique filename if file already exists
	filename := utils.GetUniqueFilename(fileChannel.Metadata.Name)

	// Create file
	file, err := os.Create(filename)
	if err != nil {
		if r.ProgressModel != nil {
			r.ProgressModel.MarkError(fileChannel.Index, err.Error())
		}
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	var receivedBytes uint64

	for data := range fileChannel.chunkReceived {
		receivedBytes += uint64(len(data))

		if _, err := file.Write(data); err != nil {
			if r.ProgressModel != nil {
				r.ProgressModel.MarkError(fileChannel.Index, err.Error())
			}
			return fmt.Errorf("failed to write file: %w", err)
		}

		// Update progress
		atomic.AddInt64(&fileChannel.ReceivedBytes, int64(len(data)))
		if r.ProgressModel != nil {
			r.ProgressModel.UpdateProgress(fileChannel.Index, int64(receivedBytes))
		}

		// Check if all data received
		if receivedBytes >= fileChannel.Metadata.Size {
			if r.ProgressModel != nil {
				r.ProgressModel.MarkComplete(fileChannel.Index)
			}
			return nil
		}
	}

	// Channel closed before all data received
	if receivedBytes < fileChannel.Metadata.Size {
		if r.ProgressModel != nil {
			r.ProgressModel.MarkError(fileChannel.Index, "channel closed early")
		}
		return fmt.Errorf("channel closed early: received %d/%d bytes for %s", receivedBytes, fileChannel.Metadata.Size, fileChannel.Metadata.Name)
	}

	if r.ProgressModel != nil {
		r.ProgressModel.MarkComplete(fileChannel.Index)
	}
	return nil
}

// Helper to run progress loop
func (r *ReceiverSession) runProgressLoop(done chan struct{}, numFiles int) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	firstPrint := true

	for {
		select {
		case <-done:
			// Final print to ensure 100% state is shown
			if r.ProgressModel != nil {
				if !firstPrint {
					r.clearProgressLines(numFiles)
				}
				fmt.Print(r.ProgressModel.View())
			}
			return
		case <-ticker.C:
			if r.ProgressModel != nil {
				if !firstPrint {
					r.clearProgressLines(numFiles)
				}
				firstPrint = false
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

// SendReadyToReceive sends confirmation to sender (like web app)
func (r *ReceiverSession) SendReadyToReceive() error {
	if r.Peer.controlChannel == nil {
		return fmt.Errorf("control channel not open")
	}

	message := webrtc.Message{Type: MessageTypeReadyToReceive}

	data, err := msgpack.Marshal(message)
	if err != nil {
		return err
	}

	return r.Peer.controlChannel.Send(data)
}

// SendDecline sends a decline message to the sender
func (r *ReceiverSession) SendDecline() error {
	if r.Peer.controlChannel == nil {
		return fmt.Errorf("control channel not open")
	}

	message := webrtc.Message{Type: MessageTypeDeclineReceive}

	data, err := msgpack.Marshal(message)
	if err != nil {
		return err
	}

	return r.Peer.controlChannel.Send(data)
}

// SendDownloadingDone sends completion signal to sender
func (r *ReceiverSession) SendDownloadingDone() error {
	if r.Peer.controlChannel == nil {
		return fmt.Errorf("control channel not open")
	}

	message := webrtc.Message{Type: MessageTypeDownloadingDone}

	data, err := msgpack.Marshal(message)
	if err != nil {
		return err
	}

	return r.Peer.controlChannel.Send(data)
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
	if p.controlChannel != nil {
		p.controlChannel.Close()
	}
	for _, fc := range p.dataChannels {
		if fc != nil && fc.Channel != nil {
			fc.Channel.Close()
		}
	}
	return p.Connection.Close()
}
