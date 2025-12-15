package multichannel

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
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

	// Create control data channel for communication
	cc, err := createControlChannel(pc)
	if err != nil {
		return nil, fmt.Errorf("failed to create control channel: %w", err)
	}

	// Create data channels for each file
	dataChannels := make([]*SenderFileChannel, len(fileInfos))
	for i, fileInfo := range fileInfos {
		fc, err := createFileChannel(pc, fileInfo, i)
		if err != nil {
			return nil, fmt.Errorf("failed to create file channel: %w", err)
		}
		dataChannels[i] = fc
	}

	peer := &SenderPeer{
		Connection:         pc,
		controlChannel:     cc,
		dataChannels:       dataChannels,
		deviceInfoReceived: make(chan webrtc.DeviceInfoPayload, 1),
		receiverReady:      make(chan struct{}, 1),
		declineReceived:    make(chan struct{}, 1),
		downloadingDone:    make(chan struct{}, 1),
		done:               make(chan struct{}),
	}

	peer.setupHandlers(client)
	peer.SetupControlHandlers()
	peer.SetupFileHandlers()
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
func (p *SenderPeer) setupHandlers(signalingClient *signaling.Client) {
	p.Connection.OnICEConnectionStateChange(func(state pion.ICEConnectionState) {
		if state == pion.ICEConnectionStateFailed || state == pion.ICEConnectionStateClosed {
			select {
			case p.done <- struct{}{}:
			default:
			}
		}
	})

	// Setup ICE candidate handler for trickle ICE
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

// CreateControlChannel creates the control channel
func createControlChannel(pc *pion.PeerConnection) (*pion.DataChannel, error) {
	ordered := true
	maxRetransmits := uint16(5000)

	dc, err := pc.CreateDataChannel("control", &pion.DataChannelInit{
		Ordered:           &ordered,
		MaxPacketLifeTime: &maxRetransmits,
	})
	if err != nil {
		return nil, err
	}
	return dc, nil
}

// SetupControlHandlers sets up control channel event handlers
func (p *SenderPeer) SetupControlHandlers() {
	p.controlChannel.OnOpen(func() {
		p.sendMetadata()
	})

	p.controlChannel.OnMessage(func(msg pion.DataChannelMessage) {
		// Parse message
		var message webrtc.Message
		if err := msgpack.Unmarshal(msg.Data, &message); err != nil {
			fmt.Printf("❌ Failed to parse message: %v\n", err)
			return
		}

		switch message.Type {
		case MessageTypeReadyToReceive:
			p.receiverReady <- struct{}{}

		case MessageTypeDeclineReceive:
			p.declineReceived <- struct{}{}

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
	metadata := make([]webrtc.FileMetadata, len(p.dataChannels))
	for i, fc := range p.dataChannels {
		metadata[i] = webrtc.FileMetadata{
			Name: fc.FileInfo.Name,
			Size: uint64(fc.FileInfo.Size),
			Type: fc.FileInfo.Type,
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

	if err := p.controlChannel.Send(data); err != nil {
		fmt.Printf("❌ Failed to send metadata: %v\n", err)
		return
	}
}

// CreateFileChannel creates a data channel for file transfer
func createFileChannel(pc *pion.PeerConnection, fileInfo *files.FileInfo, index int) (*SenderFileChannel, error) {
	ordered := true
	maxRetransmits := uint16(5000)

	dc, err := pc.CreateDataChannel(fmt.Sprintf("file-transfer-%d", index), &pion.DataChannelInit{
		Ordered:           &ordered,
		MaxPacketLifeTime: &maxRetransmits,
	})

	if err != nil {
		return nil, err
	}

	// Open file
	file, err := os.Open(fileInfo.Path)
	if err != nil {
		return nil, err
	}

	fileChannel := &SenderFileChannel{
		Channel:  dc,
		FileInfo: fileInfo,
		File:     file,
		Packet:   make([]byte, utils.MaxChunkSize), // Allocate max size, actual read size is dynamic
		Index:    index,
	}

	return fileChannel, nil
}

// SetupFileHandlers sets up file data channel event handlers
func (p *SenderPeer) SetupFileHandlers() {
	// Track when channel opens
	for _, fc := range p.dataChannels {
		fc.Channel.OnOpen(func() {
			atomic.AddInt32(&p.channelsReady, 1)
		})
	}
}

// Start establishes the WebRTC connection
func (s *SenderSession) Start() error {
	// Initialize spinner
	stopSpinner := ui.RunConnectionSpinner("Establishing WebRTC connection...")
	defer stopSpinner()

	// Start signal listener for incoming ICE candidates and answer
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

	// Return immediately - ICE candidates will be sent via OnICECandidate handler
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
	// Handle SDP answer
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

// Transfer waits for ready signal and sends all files
func (s *SenderSession) Transfer() error {
	// Wait for receiver to send "ready_to_receive"
	stopSpinner := ui.RunSpinner("Waiting for receiver to accept...")
	defer stopSpinner()

	select {
	case <-s.Peer.receiverReady:
		stopSpinner()
	case <-s.Peer.declineReceived:
		return fmt.Errorf("receiver declined the transfer")
	case <-s.Handler.PeerLeft:
		return fmt.Errorf("peer disconnected")
	case <-s.Handler.Error:
		return fmt.Errorf("signaling server error")
	}

	// Wait for all data channels to be open
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for atomic.LoadInt32(&s.Peer.channelsReady) != int32(len(s.Peer.dataChannels)) {
		select {
		case <-s.Handler.PeerLeft:
			return fmt.Errorf("peer disconnected while opening channels")
		case <-timeout:
			return fmt.Errorf("timeout waiting for data channels to open")
		case <-ticker.C:
			// Continue polling
		}
	}

	s.globalStartTime = time.Now().UnixMilli()

	filesCount := len(s.Peer.dataChannels)

	// Start concurrent transfers with Bubble Tea progress UI
	wg := &sync.WaitGroup{}
	wg.Add(filesCount)
	for _, fileChannel := range s.Peer.dataChannels {
		go s.SendFile(fileChannel, wg)
	}

	// Render progress while transfers are running
	progressDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(progressDone)
	}()

	// Start progress display in background
	s.runProgressLoop(progressDone, filesCount)
	fmt.Println()

	// Wait for receiver to confirm completion
	select {
	case <-s.Peer.downloadingDone:
		// Success
	case <-s.Handler.PeerLeft:
		return fmt.Errorf("peer disconnected during transfer")
	case <-time.After(10 * time.Second):
		fmt.Println(ui.WarningStyle.Render("⚠️  Receiver confirmation timeout (files were sent successfully)"))
	}

	// Calculate stats
	duration := time.Since(time.UnixMilli(s.globalStartTime))
	seconds := duration.Seconds()

	var totalSize int64
	for _, fc := range s.Peer.dataChannels {
		totalSize += fc.FileInfo.Size
	}

	// Display stats
	fmt.Println()
	ui.RenderTransferSummary("📊 Transfer Summary", ui.TransferSummary{
		Status:    "✅ Complete",
		Files:     len(s.Peer.dataChannels),
		TotalSize: utils.FormatSize(totalSize),
		Duration:  utils.FormatTimeDuration(duration),
		Speed:     utils.FormatSpeed(float64(totalSize) / seconds),
	})

	return nil
}

// Helper to run progress loop
func (s *SenderSession) runProgressLoop(done chan struct{}, numFiles int) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	firstPrint := true

	for {
		select {
		case <-done:
			// Final print to ensure 100% state is shown
			if s.ProgressModel != nil {
				if !firstPrint {
					s.clearProgressLines(numFiles)
				}
				fmt.Print(s.ProgressModel.View())
			}
			return
		case <-ticker.C:
			if s.ProgressModel != nil {
				if !firstPrint {
					s.clearProgressLines(numFiles)
				}
				firstPrint = false
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

// SendFile sends a single file through its data channel with dynamic chunking
func (s *SenderSession) SendFile(fc *SenderFileChannel, wg *sync.WaitGroup) error {
	defer wg.Done()
	defer fc.File.Close()

	// Ensure channel is open before starting
	if fc.Channel.ReadyState() != pion.DataChannelStateOpen {
		if s.ProgressModel != nil {
			s.ProgressModel.MarkError(fc.Index, "channel not open")
		}
		return fmt.Errorf("channel not open for %s: %s", fc.FileInfo.Name, fc.Channel.ReadyState())
	}

	// Initialize dynamic chunk size controller
	chunkController := utils.NewChunkSizeController()

	// Set up buffered amount low threshold for backpressure
	fc.Channel.SetBufferedAmountLowThreshold(uint64(LowWaterMark))

	// waitForWindow waits for buffer to drain before sending more data
	waitForWindow := func() error {
		bufferedAmount := fc.Channel.BufferedAmount()
		if bufferedAmount < uint64(HighWaterMark) {
			return nil
		}

		wait := make(chan struct{}, 1)
		fc.Channel.OnBufferedAmountLow(func() {
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
			newBufferedAmount := fc.Channel.BufferedAmount()
			if newBufferedAmount < bufferedAmount {
				// Buffer is draining, continue
				return nil
			}
			return fmt.Errorf("timed out waiting for buffer to drain (buffered: %d bytes)", newBufferedAmount)
		}
	}

	for {
		// Check if channel is still open
		if fc.Channel.ReadyState() != pion.DataChannelStateOpen {
			if s.ProgressModel != nil {
				s.ProgressModel.MarkError(fc.Index, "channel closed")
			}
			return fmt.Errorf("channel closed during transfer")
		}

		// Wait for buffer space
		if err := waitForWindow(); err != nil {
			if s.ProgressModel != nil {
				s.ProgressModel.MarkError(fc.Index, "buffer timeout")
			}
			return err
		}

		// Get current optimal chunk size
		chunkSize := chunkController.GetChunkSize()

		// Buffer has space, proceed with sending
		n, err := fc.File.Read(fc.Packet[:chunkSize])
		if err != nil {
			if err == io.EOF {
				// Wait for all buffered data to be sent (with timeout)
				startDrain := time.Now()
				for fc.Channel.BufferedAmount() > 0 && time.Since(startDrain) < time.Duration(DrainTimeout)*time.Second {
					if fc.Channel.ReadyState() != pion.DataChannelStateOpen {
						if s.ProgressModel != nil {
							s.ProgressModel.MarkComplete(fc.Index)
						}
						return nil
					}
					time.Sleep(50 * time.Millisecond)
				}
				// Mark as complete
				if s.ProgressModel != nil {
					s.ProgressModel.MarkComplete(fc.Index)
				}
				return nil
			}
			if s.ProgressModel != nil {
				s.ProgressModel.MarkError(fc.Index, err.Error())
			}
			return err
		}

		packet := fc.Packet[:n]
		if err := fc.Channel.Send(packet); err != nil {
			if s.ProgressModel != nil {
				s.ProgressModel.MarkError(fc.Index, err.Error())
			}
			return err
		}

		// Record bytes for dynamic chunk sizing
		chunkController.RecordBytesTransferred(int64(len(packet)))

		// Update progress
		atomic.AddInt64(&fc.SentBytes, int64(len(packet)))
		if s.ProgressModel != nil {
			s.ProgressModel.UpdateProgress(fc.Index, atomic.LoadInt64(&fc.SentBytes))
		}
	}
}

// Close closes the session
func (s *SenderSession) Close() error {
	// Close peer first (this closes channels)
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
