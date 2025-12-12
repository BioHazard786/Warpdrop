package singlechannel

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/BioHazard786/Warpdrop/cli/internal/config"
	"github.com/BioHazard786/Warpdrop/cli/internal/files"
	"github.com/BioHazard786/Warpdrop/cli/internal/signaling"
	"github.com/BioHazard786/Warpdrop/cli/internal/ui"
	"github.com/pion/webrtc/v4"
	"github.com/vmihailenco/msgpack/v5"
)

// NewSenderSession creates a new WebRTC sender session
func NewSenderSession(client *signaling.Client, handler *signaling.Handler, cfg *config.Config, peerInfo *signaling.PeerInfo) (*SenderSession, error) {
	peer, err := NewSenderPeer(cfg)
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

// NewSenderPeer creates a new WebRTC peer for single channel transfers
func NewSenderPeer(cfg *config.Config) (*SenderPeer, error) {
	pc, err := newPeerConnection(cfg)
	if err != nil {
		return nil, err
	}

	peer := &SenderPeer{
		Connection:     pc,
		answerReceived: make(chan struct{}, 1),
		readyReceived:  make(chan ReadyToReceivePayload, 1),
		downloadDone:   make(chan struct{}, 1),
		signalDone:     make(chan struct{}),
	}

	peer.setupHandlers()
	return peer, nil
}

// setupHandlers configures ICE connection state handlers
func (p *SenderPeer) setupHandlers() {
	p.Connection.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		// ICE state changes handled silently
	})
}

// Start establishes the WebRTC connection
func (s *SenderSession) Start(fileInfos []*files.FileInfo) error {
	// Initialize spinner
	stopSpinner := ui.RunConnectionSpinner("Establishing WebRTC connection...")
	s.Peer.Files = fileInfos

	// Create the data channel
	if err := s.Peer.CreateDataChannel(); err != nil {
		stopSpinner()
		return fmt.Errorf("failed to create data channel: %w", err)
	}

	// Setup data channel handlers
	s.Peer.SetupDataChannelHandlers()

	// Setup ICE candidate handler
	s.Peer.Connection.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			stopSpinner()
			return
		}
		candidateBytes, _ := json.Marshal(c.ToJSON())
		var candidateMap map[string]any
		_ = json.Unmarshal(candidateBytes, &candidateMap)
		s.SignalingClient.SendMessage(&signaling.Message{
			Type:    signaling.MessageTypeSignal,
			Payload: signaling.SignalPayload{ICECandidate: candidateMap},
		})
	})

	// Start signal listener
	go s.listenForSignals(stopSpinner)

	// Create and send WebRTC offer
	offer, err := s.Peer.CreateOffer()
	if err != nil {
		stopSpinner()
		return fmt.Errorf("failed to create offer: %w", err)
	}

	s.SignalingClient.SendMessage(&signaling.Message{
		Type: signaling.MessageTypeSignal,
		Payload: signaling.SignalPayload{
			Type: offer.Type.String(),
			SDP:  offer.SDP,
		},
	})

	stopSpinner = ui.RunSpinner("Waiting for answer from receiver...")

	// Wait for answer from receiver
	select {
	case <-s.Peer.answerReceived:
		stopSpinner()

	case errMsg := <-s.Handler.Error:
		stopSpinner()
		return fmt.Errorf("signaling error: %s", errMsg)

	case <-time.After(time.Duration(SignalTimeout) * time.Second):
		stopSpinner()
		return errors.New("timeout waiting for answer")
	}

	return nil
}

// CreateDataChannel creates the single data channel for file transfer
func (p *SenderPeer) CreateDataChannel() error {
	dc, err := p.Connection.CreateDataChannel("file-transfer", nil)
	if err != nil {
		return err
	}
	p.DataChannel = dc
	return nil
}

// SetupDataChannelHandlers sets up data channel event handlers
func (p *SenderPeer) SetupDataChannelHandlers() {
	p.DataChannel.OnOpen(func() {
		p.sendMetadata()
	})

	p.DataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		var message Message
		if err := msgpack.Unmarshal(msg.Data, &message); err != nil {
			fmt.Printf("❌ Failed to parse message: %v\n", err)
			return
		}

		switch message.Type {
		case MessageTypeReadyToReceive:
			payloadBytes, _ := json.Marshal(message.Payload)
			var ready ReadyToReceivePayload
			_ = json.Unmarshal(payloadBytes, &ready)
			p.readyReceived <- ready

		case MessageTypeDownloadingDone:
			p.downloadDone <- struct{}{}

		case MessageTypeDeviceInfo:
			fmt.Printf("🖥️  Receiver device: %+v\n", message.Payload)
		}
	})
}

// sendMetadata sends file metadata to the receiver
func (p *SenderPeer) sendMetadata() {
	metadata := make([]FileMetadata, len(p.Files))
	for i, info := range p.Files {
		metadata[i] = FileMetadata{
			Name: info.Name,
			Size: uint64(info.Size),
			Type: info.Type,
		}
	}

	message := Message{Type: MessageTypeFilesMetadata, Payload: metadata}
	data, err := msgpack.Marshal(message)
	if err != nil {
		fmt.Printf("❌ Failed to marshal metadata: %v\n", err)
		return
	}

	if err := p.DataChannel.Send(data); err != nil {
		fmt.Printf("❌ Failed to send metadata: %v\n", err)
		return
	}
}

// CreateOffer creates WebRTC offer with trickle ICE (doesn't wait for gathering)
func (p *SenderPeer) CreateOffer() (*webrtc.SessionDescription, error) {
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

// HandleSignal processes incoming signaling messages
func (p *SenderPeer) HandleSignal(payload *signaling.SignalPayload) error {
	if payload.SDP != "" {
		var sdpType webrtc.SDPType
		switch payload.Type {
		case "offer":
			sdpType = webrtc.SDPTypeOffer
		case "answer":
			sdpType = webrtc.SDPTypeAnswer
		default:
			return fmt.Errorf("unexpected signal type: %s", payload.Type)
		}

		desc := webrtc.SessionDescription{
			Type: sdpType,
			SDP:  payload.SDP,
		}

		if desc.Type == webrtc.SDPTypeAnswer {
			if err := p.Connection.SetRemoteDescription(desc); err != nil {
				return err
			}
			p.answerReceived <- struct{}{}
		}
	}

	if payload.ICECandidate != nil {
		candidateBytes, _ := json.Marshal(payload.ICECandidate)
		var ice webrtc.ICECandidateInit
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
	// Wait for receiver to be ready
	// Map for quick lookup
	fileByName := make(map[string]*files.FileInfo)
	fileIndexByName := make(map[string]int)
	for i, f := range s.Peer.Files {
		fileByName[f.Name] = f
		fileIndexByName[f.Name] = i
	}

	// Wait for first ready signal
	readyPayload := <-s.Peer.readyReceived

	s.globalStartTime = time.Now().UnixMilli()

	fmt.Printf("\n%s Transferring files...\n\n", ui.IconSend)

	// Count lines for proper cursor movement
	numFiles := len(s.Peer.Files)
	firstPrint := true

	// Start progress display in background
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if s.ProgressModel != nil {
					if !firstPrint {
						// Move cursor up and clear each line before redrawing
						for i := 0; i < numFiles; i++ {
							fmt.Print("\033[A\033[2K")
						}
					}
					firstPrint = false
					fmt.Print(s.ProgressModel.View())
				}
			}
		}
	}()

	// Process first file
	fileInfo, ok := fileByName[readyPayload.FileName]
	if !ok {
		close(done)
		return fmt.Errorf("receiver requested unknown file: %s", readyPayload.FileName)
	}
	fileIndex := fileIndexByName[readyPayload.FileName]

	if err := s.SendFile(fileInfo, readyPayload.Offset, fileIndex); err != nil {
		close(done)
		return err
	}
	if s.ProgressModel != nil {
		s.ProgressModel.MarkComplete(fileIndex)
	}

	// Process remaining files
	for i := 1; i < len(s.Peer.Files); i++ {
		readyPayload = <-s.Peer.readyReceived

		fileInfo, ok = fileByName[readyPayload.FileName]
		if !ok {
			close(done)
			return fmt.Errorf("receiver requested unknown file: %s", readyPayload.FileName)
		}

		fileIndex = fileIndexByName[readyPayload.FileName]

		if err := s.SendFile(fileInfo, readyPayload.Offset, fileIndex); err != nil {
			close(done)
			return err
		}

		// Mark file as complete
		if s.ProgressModel != nil {
			s.ProgressModel.MarkComplete(fileIndex)
		}
	}

	// Stop progress display
	close(done)

	// Final update - clear progress lines and print final state
	if s.ProgressModel != nil {
		if !firstPrint {
			for i := 0; i < numFiles; i++ {
				fmt.Print("\033[A\033[2K")
			}
		}
		fmt.Print(s.ProgressModel.View())
	}
	fmt.Println()

	// Wait for receiver confirmation
	select {
	case <-s.Peer.downloadDone:
	case <-time.After(10 * time.Second):
		fmt.Println(ui.WarningStyle.Render("⚠️  Receiver confirmation timeout (files were sent successfully)"))
	}

	// Calculate stats
	currentTime := time.Now().UnixMilli()
	timeDiff := float64(currentTime-s.globalStartTime) / 1000.0

	var totalSize int64
	for _, f := range s.Peer.Files {
		totalSize += f.Size
	}

	totalMiB := float64(totalSize) / 1048576.0
	avgSpeed := totalMiB / timeDiff

	// Display final stats using go-pretty table
	fmt.Println()
	ui.RenderTransferSummary("📊 Transfer Summary", ui.TransferSummary{
		Status:    "✅ Complete",
		Files:     len(s.Peer.Files),
		TotalSize: files.FormatSize(totalSize),
		Duration:  fmt.Sprintf("%.2f seconds", timeDiff),
		Speed:     fmt.Sprintf("%.2f MiB/s", avgSpeed),
	})

	return nil
}

// SendFile sends a single file through the data channel
func (s *SenderSession) SendFile(fileInfo *files.FileInfo, startOffset uint64, fileIndex int) error {
	file, err := os.Open(fileInfo.Path)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	if _, err := file.Seek(int64(startOffset), io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek file: %w", err)
	}

	buffer := make([]byte, PacketSize)
	offset := startOffset

	s.Peer.DataChannel.SetBufferedAmountLowThreshold(LowWaterMark)

	waitForWindow := func() error {
		if s.Peer.DataChannel.BufferedAmount() < HighWaterMark {
			return nil
		}

		wait := make(chan struct{}, 1)
		s.Peer.DataChannel.OnBufferedAmountLow(func() {
			select {
			case wait <- struct{}{}:
			default:
			}
		})

		select {
		case <-wait:
			return nil
		case <-time.After(time.Duration(SendTimeout) * time.Second):
			return errors.New("timed out waiting for buffered amount to drain")
		}
	}

	for {
		n, readErr := file.Read(buffer)
		if n > 0 {
			final := offset+uint64(n) >= uint64(fileInfo.Size)
			payload := Message{
				Type: MessageTypeChunk,
				Payload: ChunkPayload{
					FileName: fileInfo.Name,
					Offset:   offset,
					Bytes:    buffer[:n],
					Final:    final,
				},
			}

			data, err := msgpack.Marshal(payload)
			if err != nil {
				return fmt.Errorf("failed to marshal chunk: %w", err)
			}

			if err := waitForWindow(); err != nil {
				return err
			}

			if err := s.Peer.DataChannel.Send(data); err != nil {
				return fmt.Errorf("failed to send chunk: %w", err)
			}

			offset += uint64(n)
			if s.ProgressModel != nil {
				s.ProgressModel.UpdateProgress(fileIndex, int64(offset))
			}
		}

		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("read error: %w", readErr)
		}
	}

	return nil
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
	close(p.signalDone)
	if p.DataChannel != nil {
		p.DataChannel.Close()
	}
	return p.Connection.Close()
}

// listenForSignals handles incoming signaling messages
func (s *SenderSession) listenForSignals(stopSpinner func()) {
	for {
		select {
		case sig := <-s.Handler.Signal:
			if sig == nil {
				continue
			}
			stopSpinner()
			if err := s.Peer.HandleSignal(sig); err != nil {
				fmt.Printf("⚠️  Signal handling error: %v\n", err)
			}

		case <-s.Peer.signalDone:
			stopSpinner()
			return
		}
	}
}

// newPeerConnection centralizes ICE server configuration
func newPeerConnection(cfg *config.Config) (*webrtc.PeerConnection, error) {
	iceServers := []webrtc.ICEServer{{URLs: cfg.GetSTUNServers()}}

	if turnServers := cfg.GetTURNServers(); turnServers != nil {
		username, password := cfg.GetTURNCredentials()
		iceServers = append(iceServers, webrtc.ICEServer{
			URLs:       turnServers,
			Username:   username,
			Credential: password,
		})
	}

	return webrtc.NewPeerConnection(webrtc.Configuration{ICEServers: iceServers})
}
