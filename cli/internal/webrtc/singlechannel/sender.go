package singlechannel

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/BioHazard786/Warpdrop/cli/internal/config"
	"github.com/BioHazard786/Warpdrop/cli/internal/files"
	"github.com/BioHazard786/Warpdrop/cli/internal/signaling"
	"github.com/BioHazard786/Warpdrop/cli/internal/ui"
	"github.com/BioHazard786/Warpdrop/cli/internal/utils"
	"github.com/BioHazard786/Warpdrop/cli/internal/webrtc"
	pion "github.com/pion/webrtc/v4"
	"github.com/vmihailenco/msgpack/v5"
)

// NewSenderSession creates a new WebRTC sender session
func NewSenderSession(client *signaling.Client, handler *signaling.Handler, cfg *config.Config, fileInfos []*files.FileInfo, peerInfo *signaling.PeerInfo) (*SenderSession, error) {
	peer, err := NewSenderPeer(client, cfg, fileInfos)
	if err != nil {
		return nil, err
	}

	return &SenderSession{
		Peer:            peer,
		SignalingClient: client,
		Handler:         handler,
		Config:          cfg,
		PeerInfo:        peerInfo,
	}, nil
}

// SetProgressUI initializes the progress UI
func (s *SenderSession) SetProgressUI(fileNames []string, fileSizes []int64) {
	s.fileNames = fileNames
	s.fileSizes = fileSizes
	s.ProgressModel = ui.NewProgressModel(fileNames, fileSizes)
}

// NewSenderPeer creates a new WebRTC peer
func NewSenderPeer(client *signaling.Client, cfg *config.Config, fileInfos []*files.FileInfo) (*SenderPeer, error) {
	pc, err := newPeerConnection(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create peer connection: %w", err)
	}

	// Create data channel for file transfer
	dc, err := createDataChannel(pc)
	if err != nil {
		return nil, fmt.Errorf("failed to create data channel: %w", err)
	}

	peer := &SenderPeer{
		Connection:         pc,
		dataChannel:        dc,
		files:              fileInfos,
		deviceInfoReceived: make(chan webrtc.DeviceInfoPayload, 1),
		receiverReady:      make(chan webrtc.ReadyToReceivePayload, 1),
		downloadingDone:    make(chan struct{}, 1),
		done:               make(chan struct{}),
	}

	peer.SetupHandlers(client)
	peer.SetupDataHandlers()
	return peer, nil
}

// newPeerConnection centralizes ICE server configuration
func newPeerConnection(cfg *config.Config) (*pion.PeerConnection, error) {
	iceServers := []pion.ICEServer{{URLs: cfg.GetSTUNServers()}}

	turnServers := cfg.GetTURNServers()

	if turnServers != nil {
		username, password := cfg.GetTURNCredentials()
		iceServers = append(iceServers, pion.ICEServer{
			URLs:       turnServers,
			Username:   username,
			Credential: password,
		})
	}

	// Determine ICE transport policy
	// ForceRelay uses only TURN servers (useful behind restrictive networks)
	// Otherwise use All to try direct P2P first, fall back to TURN if needed
	policy := pion.ICETransportPolicyAll
	if turnServers != nil && (cfg.ForceRelay || utils.ShouldForceRelay()) {
		policy = pion.ICETransportPolicyRelay
	}

	return pion.NewPeerConnection(pion.Configuration{
		ICEServers:         iceServers,
		ICETransportPolicy: policy,
	})
}

// setupHandlers configures ICE connection state handlers
func (p *SenderPeer) SetupHandlers(signalingClient *signaling.Client) {
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

// CreateDataChannel creates the data channel
func createDataChannel(pc *pion.PeerConnection) (*pion.DataChannel, error) {
	ordered := true
	maxRetransmits := uint16(5000)

	dc, err := pc.CreateDataChannel("file-transfer", &pion.DataChannelInit{
		Ordered:           &ordered,
		MaxPacketLifeTime: &maxRetransmits,
	})
	if err != nil {
		return nil, err
	}
	return dc, nil
}

// SetupDataHandlers sets up data channel event handlers
func (p *SenderPeer) SetupDataHandlers() {
	p.dataChannel.OnOpen(func() {
		p.sendMetadata()
	})

	p.dataChannel.OnMessage(func(msg pion.DataChannelMessage) {
		var message webrtc.Message
		if err := msgpack.Unmarshal(msg.Data, &message); err != nil {
			fmt.Printf("❌ Failed to parse message: %v\n", err)
			return
		}

		switch message.Type {
		case MessageTypeReadyToReceive:
			var ready webrtc.ReadyToReceivePayload
			if err := message.DecodePayload(&ready); err != nil {
				fmt.Printf("❌ Failed to decode metadata: %v\n", err)
				return
			}
			p.receiverReady <- ready

		case MessageTypeDownloadingDone:
			p.downloadingDone <- struct{}{}

		case MessageTypeDeviceInfo:
			var deviceInfo webrtc.DeviceInfoPayload
			if err := message.DecodePayload(&deviceInfo); err != nil {
				fmt.Printf("❌ Failed to decode metadata: %v\n", err)
				return
			}
			p.deviceInfoReceived <- deviceInfo
		}
	})
}

// sendMetadata sends file metadata to the receiver
func (p *SenderPeer) sendMetadata() {
	metadata := make([]webrtc.FileMetadata, len(p.files))
	for i, info := range p.files {
		metadata[i] = webrtc.FileMetadata{
			Name: info.Name,
			Size: uint64(info.Size),
			Type: info.Type,
		}
	}

	message, err := webrtc.NewMessage(MessageTypeFilesMetadata, metadata)
	if err != nil {
		fmt.Printf("❌ Failed to marshal payload: %v\n", err)
		return
	}
	data, err := msgpack.Marshal(message)
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
func (s *SenderSession) Start() error {
	// Initialize spinner
	stopSpinner := ui.RunConnectionSpinner("Establishing WebRTC connection...")
	defer stopSpinner()

	// Start signal listener
	go s.listenForSignals()

	// Create and send WebRTC offer
	offer, err := s.Peer.CreateOffer()
	if err != nil {
		return fmt.Errorf("failed to create offer: %w", err)
	}

	s.SignalingClient.SendMessage(&signaling.Message{
		Type: signaling.MessageTypeSignal,
		Payload: signaling.SignalPayload{
			Type: offer.Type.String(),
			SDP:  offer.SDP,
		},
	})

	// Wait for answer from receiver
	select {
	case deviceInfo := <-s.Peer.deviceInfoReceived:
		stopSpinner()
		fmt.Printf("🖥️  Receiver device: %s v%s\n", deviceInfo.DeviceName, deviceInfo.DeviceVersion)

	case errMsg := <-s.Handler.Error:
		return fmt.Errorf("signaling error: %s", errMsg)

	case <-time.After(time.Duration(SignalTimeout) * time.Second):
		return fmt.Errorf("timeout waiting for answer")
	}

	return nil
}

// CreateOffer creates WebRTC offer with trickle ICE (doesn't wait for gathering)
func (p *SenderPeer) CreateOffer() (*pion.SessionDescription, error) {
	offer, err := p.Connection.CreateOffer(nil)
	if err != nil {
		return nil, err
	}

	if err = p.Connection.SetLocalDescription(offer); err != nil {
		return nil, err
	}

	return p.Connection.LocalDescription(), nil
}

// listenForSignals handles incoming signaling messages (ICE candidates and answer)
func (s *SenderSession) listenForSignals() {
	for {
		select {
		case sig := <-s.Handler.Signal:
			if sig == nil {
				continue
			}
			if err := s.Peer.HandleSignal(sig); err != nil {
				fmt.Printf("⚠️  Signal handling error: %v\n", err)
			}

		case <-s.Peer.done:
			return
		}
	}
}

// HandleSignal processes incoming signaling messages (SDP and ICE candidates)
func (p *SenderPeer) HandleSignal(payload *signaling.SignalPayload) error {
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

		if desc.Type == pion.SDPTypeAnswer {
			return p.Connection.SetRemoteDescription(desc)
		}
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

// Transfer streams all files sequentially using the single channel
func (s *SenderSession) Transfer() error {
	// Wait for receiver to send "ready_to_receive"
	stopSpinner := ui.RunSpinner("Waiting for receiver to accept...")
	defer stopSpinner()

	// 1. Optimize Map: Pre-allocate size to avoid resizing
	filesCount := len(s.Peer.files)
	fileByName := make(map[string]*files.FileInfo, filesCount)
	fileIndexByName := make(map[string]int, filesCount)

	var totalSize int64
	for i, f := range s.Peer.files {
		fileByName[f.Name] = f
		fileIndexByName[f.Name] = i
		totalSize += f.Size
	}

	var readyPayload webrtc.ReadyToReceivePayload

	select {
	case readyPayload = <-s.Peer.receiverReady:
		stopSpinner()
	case <-s.Peer.declineReceived:
		return fmt.Errorf("receiver declined the transfer")
	case <-s.Handler.PeerLeft:
		return fmt.Errorf("peer disconnected")
	case <-s.Handler.Error:
		return fmt.Errorf("signaling server error")
	}

	s.globalStartTime = time.Now().UnixMilli()

	// UI Progress Ticker - Setup
	progressDone := make(chan struct{})
	defer close(progressDone) // GUARANTEE: UI routine dies when function exits

	// Start progress display in background
	go s.runProgressLoop(progressDone, filesCount)
	// Process first file
	fileInfo, ok := fileByName[readyPayload.FileName]
	if !ok {
		return fmt.Errorf("receiver requested unknown file: %s", readyPayload.FileName)
	}
	fileIndex := fileIndexByName[readyPayload.FileName]

	if err := s.SendFile(fileInfo, readyPayload.Offset, fileIndex); err != nil {
		return err
	}

	// Process remaining files
	for i := 1; i < filesCount; i++ {

		// Wait for ready signal OR errors
		select {
		case readyPayload = <-s.Peer.receiverReady:
		case <-s.Peer.declineReceived:
			return fmt.Errorf("receiver declined the transfer")
		case <-s.Handler.PeerLeft:
			return fmt.Errorf("peer disconnected")
		case <-s.Handler.Error:
			return fmt.Errorf("signaling server error")
		}

		// Validate file existence
		fileInfo, ok := fileByName[readyPayload.FileName]
		if !ok {
			return fmt.Errorf("receiver requested unknown file: %s", readyPayload.FileName)
		}

		fileIndex := fileIndexByName[readyPayload.FileName]

		// Perform Transfer
		if err := s.SendFile(fileInfo, readyPayload.Offset, fileIndex); err != nil {
			return err
		}
	}

	// Clear progress lines one last time to prepare for summary
	s.clearProgressLines(filesCount)
	if s.ProgressModel != nil {
		fmt.Print(s.ProgressModel.View())
	}
	fmt.Println()

	// Wait for receiver confirmation
	select {
	case <-s.Peer.downloadingDone:
		// Success
	case <-s.Handler.PeerLeft:
		return fmt.Errorf("peer disconnected during transfer")
	case <-time.After(10 * time.Second):
		fmt.Println(ui.WarningStyle.Render("⚠️  Receiver confirmation timeout (files were sent successfully)"))
	}

	// 3. Stats Calculation
	duration := time.Since(time.UnixMilli(s.globalStartTime))
	seconds := duration.Seconds()

	// Display final stats
	fmt.Println()
	ui.RenderTransferSummary("📊 Transfer Summary", ui.TransferSummary{
		Status:    "✅ Complete",
		Files:     filesCount,
		TotalSize: utils.FormatSize(totalSize),
		Duration:  utils.FormatTimeDuration(duration),
		Speed:     utils.FormatSpeed(float64(totalSize) / seconds),
	})

	return nil
}

// Helper to keep the main function clean
func (s *SenderSession) runProgressLoop(done chan struct{}, numFiles int) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	firstPrint := true

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			if s.ProgressModel != nil {
				if !firstPrint {
					s.clearProgressLines(numFiles)
				}
				firstPrint = false
				// Note: Ensure View() is concurrent-safe if modified by SendFile
				fmt.Print(s.ProgressModel.View())
			}
		}
	}
}

// Helper to clear lines
func (s *SenderSession) clearProgressLines(count int) {
	for range count {
		fmt.Print("\033[A\033[2K")
	}
}

// SendFile sends a single file through the data channel with dynamic chunking
func (s *SenderSession) SendFile(fileInfo *files.FileInfo, startOffset uint64, fileIndex int) error {
	file, err := os.Open(fileInfo.Path)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	if _, err := file.Seek(int64(startOffset), io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek file: %w", err)
	}

	// Ensure channel is open before starting
	if s.Peer.dataChannel.ReadyState() != pion.DataChannelStateOpen {
		if s.ProgressModel != nil {
			s.ProgressModel.MarkError(fileIndex, "channel not open")
		}
		return fmt.Errorf("%s data channel not open: %s", s.Peer.dataChannel.Label(), s.Peer.dataChannel.ReadyState())
	}

	// Initialize dynamic chunk size controller
	chunkController := utils.NewChunkSizeController()

	// Allocate buffer with maximum possible size, we'll use slices for actual reads
	buffer := make([]byte, utils.MaxChunkSize)
	offset := startOffset

	// Set up buffered amount low threshold for backpressure
	s.Peer.dataChannel.SetBufferedAmountLowThreshold(uint64(LowWaterMark))

	// waitForWindow waits for buffer to drain before sending more data
	// Uses adaptive waiting with backoff for slow connections
	waitForWindow := func() error {
		bufferedAmount := s.Peer.dataChannel.BufferedAmount()
		if bufferedAmount < uint64(HighWaterMark) {
			return nil
		}

		wait := make(chan struct{}, 1)
		s.Peer.dataChannel.OnBufferedAmountLow(func() {
			select {
			case wait <- struct{}{}:
			default:
			}
		})

		// Use a longer timeout for slow connections
		timeout := time.Duration(SendTimeout) * time.Second

		select {
		case <-wait:
			return nil
		case <-time.After(timeout):
			// Check if we made any progress (buffer decreased)
			newBufferedAmount := s.Peer.dataChannel.BufferedAmount()
			if newBufferedAmount < bufferedAmount {
				// Buffer is draining, continue waiting
				return nil
			}
			return fmt.Errorf("timed out waiting for buffer to drain (buffered: %d bytes)", newBufferedAmount)
		}
	}

	for {
		// Check if channel is still open
		if s.Peer.dataChannel.ReadyState() != pion.DataChannelStateOpen {
			if s.ProgressModel != nil {
				s.ProgressModel.MarkError(fileIndex, "channel closed")
			}
			return fmt.Errorf("channel closed during transfer")
		}

		// Wait for buffer space
		if err := waitForWindow(); err != nil {
			if s.ProgressModel != nil {
				s.ProgressModel.MarkError(fileIndex, "buffer timeout")
			}
			return err
		}

		// Get current optimal chunk size
		chunkSize := chunkController.GetChunkSize()

		n, err := file.Read(buffer[:chunkSize])

		if err != nil {
			if err == io.EOF {
				// Wait for all buffered data to be sent (with timeout)
				startDrain := time.Now()
				for s.Peer.dataChannel.BufferedAmount() > 0 && time.Since(startDrain) < time.Duration(DrainTimeout)*time.Second {
					if s.Peer.dataChannel.ReadyState() != pion.DataChannelStateOpen {
						if s.ProgressModel != nil {
							s.ProgressModel.MarkComplete(fileIndex)
						}
						return nil
					}
					time.Sleep(50 * time.Millisecond)
				}
				// Mark as complete
				if s.ProgressModel != nil {
					s.ProgressModel.MarkComplete(fileIndex)
				}
				return nil
			}
			if s.ProgressModel != nil {
				s.ProgressModel.MarkError(fileIndex, err.Error())
			}
			return err
		}

		final := offset+uint64(n) >= uint64(fileInfo.Size)
		message, err := webrtc.NewMessage(MessageTypeChunk, webrtc.ChunkPayload{FileName: fileInfo.Name,
			Offset: offset,
			Bytes:  buffer[:n],
			Final:  final})

		if err != nil {
			if s.ProgressModel != nil {
				s.ProgressModel.MarkError(fileIndex, err.Error())
			}
			return err
		}

		data, err := msgpack.Marshal(message)
		if err != nil {
			if s.ProgressModel != nil {
				s.ProgressModel.MarkError(fileIndex, err.Error())
			}
			return err
		}

		if err := s.Peer.dataChannel.Send(data); err != nil {
			if s.ProgressModel != nil {
				s.ProgressModel.MarkError(fileIndex, err.Error())
			}
			return err
		}

		offset += uint64(n)

		// Record bytes for dynamic chunk sizing
		chunkController.RecordBytesTransferred(int64(n))

		if s.ProgressModel != nil {
			s.ProgressModel.UpdateProgress(fileIndex, int64(offset))
		}
	}

}

// Close closes the session
func (s *SenderSession) Close() error {
	// Close peer first
	if s.Peer != nil {
		if err := s.Peer.Close(); err != nil {
			fmt.Printf("⚠️  Error closing peer: %v\n", err)
		}
		s.Peer = nil
	}

	// Small delay to allow final messages to be sent
	time.Sleep(100 * time.Millisecond)

	// Close signaling last
	if s.SignalingClient != nil {
		s.SignalingClient.Close()
		s.SignalingClient = nil
	}
	if s.Handler != nil {
		s.Handler.Close()
		s.Handler = nil
	}
	return nil
}

// Close closes the peer
func (p *SenderPeer) Close() error {
	if p.dataChannel != nil {
		p.dataChannel.Close()
	}
	return p.Connection.Close()
}
