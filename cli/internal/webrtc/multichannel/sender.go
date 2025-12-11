package multichannel

import (
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
	"github.com/pion/webrtc/v4"
	"github.com/vmihailenco/msgpack/v5"
)

// NewSenderSession creates a new WebRTC sender session
func NewSenderSession(client *signaling.Client, handler *signaling.Handler, cfg *config.Config, numFiles int, peerInfo *signaling.PeerInfo) (*SenderSession, error) {
	peer, err := NewSenderPeer(cfg, numFiles)
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
func NewSenderPeer(cfg *config.Config, numFiles int) (*SenderPeer, error) {
	pc, err := newPeerConnection(cfg)
	if err != nil {
		return nil, err
	}

	peer := &SenderPeer{
		Connection:      pc,
		dataChannels:    make([]*FileChannel, numFiles),
		readyReceived:   make(chan struct{}, 1),
		declineReceived: make(chan struct{}, 1),
		downloadingDone: make(chan struct{}, 1),
		done:            make(chan struct{}),
	}

	peer.setupHandlers()
	return peer, nil
}

// setupHandlers configures ICE connection state handlers
func (p *SenderPeer) setupHandlers() {
	p.Connection.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		if state == webrtc.ICEConnectionStateFailed {
			p.done <- struct{}{}
		}
	})
}

// Start establishes the WebRTC connection
func (s *SenderSession) Start(fileInfos []*files.FileInfo) error {
	// Initialize spinner
	stopSpinner := ui.RunConnectionSpinner("Establishing WebRTC connection...")

	// Step 1: Create control channel for metadata
	if err := s.Peer.CreateControlChannel(); err != nil {
		stopSpinner()
		return fmt.Errorf("failed to create control channel: %w", err)
	}

	// Setup control channel to send metadata on open
	s.Peer.SetupControlHandlers(fileInfos)

	// Step 2: Create one data channel per file
	for i, fileInfo := range fileInfos {
		if err := s.Peer.CreateFileChannel(fileInfo, i); err != nil {
			stopSpinner()
			return fmt.Errorf("failed to create file channel: %w", err)
		}
	}

	// Step 3: Create and send WebRTC offer
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

	// Step 4: Wait for answer from receiver
	select {
	case signalPayload := <-s.Handler.Signal:
		stopSpinner()
		if err := s.Peer.HandleSignal(signalPayload); err != nil {
			return fmt.Errorf("failed to handle signal: %w", err)
		}

	case errMsg := <-s.Handler.Error:
		stopSpinner()
		return fmt.Errorf("signaling error: %s", errMsg)
	}

	return nil
}

// CreateControlChannel creates the control channel
func (p *SenderPeer) CreateControlChannel() error {
	ordered := true
	maxRetransmits := uint16(5000)

	dc, err := p.Connection.CreateDataChannel("control", &webrtc.DataChannelInit{
		Ordered:           &ordered,
		MaxPacketLifeTime: &maxRetransmits,
	})
	if err != nil {
		return err
	}

	p.controlChannel = dc
	return nil
}

// SetupControlHandlers sets up control channel event handlers
func (p *SenderPeer) SetupControlHandlers(fileInfos []*files.FileInfo) {
	p.controlChannel.OnOpen(func() {
		// Send files metadata immediately (like web app)
		metadata := make([]FileMetadata, len(fileInfos))
		for i, info := range fileInfos {
			metadata[i] = FileMetadata{
				Name: info.Name,
				Size: uint64(info.Size),
				Type: info.Type,
			}
		}

		message := struct {
			Type    string         `msgpack:"type"`
			Payload []FileMetadata `msgpack:"payload"`
		}{
			Type:    "files_metadata",
			Payload: metadata,
		}

		data, err := msgpack.Marshal(message)
		if err != nil {
			return
		}

		_ = p.controlChannel.Send(data)
	})

	p.controlChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		// Parse message
		var message struct {
			Type string `msgpack:"type"`
		}

		if err := msgpack.Unmarshal(msg.Data, &message); err != nil {
			return
		}

		switch message.Type {
		case "ready_to_receive":
			p.readyReceived <- struct{}{}
		case "decline_receive":
			p.declineReceived <- struct{}{}
		case "downloading_done":
			p.downloadingDone <- struct{}{}
		}
	})
}

// CreateFileChannel creates a data channel for file transfer
func (p *SenderPeer) CreateFileChannel(fileInfo *files.FileInfo, index int) error {
	ordered := true
	maxRetransmits := uint16(5000)

	dc, err := p.Connection.CreateDataChannel(fmt.Sprintf("file-transfer-%d", index), &webrtc.DataChannelInit{
		Ordered:           &ordered,
		MaxPacketLifeTime: &maxRetransmits,
	})
	if err != nil {
		return err
	}

	// Open file
	file, err := os.Open(fileInfo.Path)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}

	fileChannel := &FileChannel{
		Channel:  dc,
		FileInfo: fileInfo,
		File:     file,
		Packet:   make([]byte, PacketSize),
		Index:    index,
	}

	// Track when channel opens
	dc.OnOpen(func() {
		atomic.AddInt32(&p.channelsReady, 1)
	})

	p.dataChannels[index] = fileChannel
	return nil
}

// CreateOffer creates WebRTC offer
func (p *SenderPeer) CreateOffer() (*webrtc.SessionDescription, error) {
	offer, err := p.Connection.CreateOffer(nil)
	if err != nil {
		return nil, err
	}

	gatherDone := webrtc.GatheringCompletePromise(p.Connection)

	if err = p.Connection.SetLocalDescription(offer); err != nil {
		return nil, err
	}

	<-gatherDone
	return p.Connection.LocalDescription(), nil
}

// HandleSignal processes incoming signaling messages
func (p *SenderPeer) HandleSignal(payload *signaling.SignalPayload) error {
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
		return p.Connection.SetRemoteDescription(desc)
	}

	return fmt.Errorf("unexpected signal type: %s", desc.Type)
}

// Transfer waits for ready signal and sends all files
func (s *SenderSession) Transfer() error {
	// Wait for receiver to send "ready_to_receive"
	stopSpinner := ui.RunSpinner("Waiting for receiver to accept...")

	select {
	case <-s.Peer.readyReceived:
		stopSpinner()
	case <-s.Peer.declineReceived:
		return fmt.Errorf("receiver declined the transfer")
	case <-s.Handler.PeerLeft:
		return fmt.Errorf("peer disconnected")
	case <-s.Handler.Error:
		return fmt.Errorf("signaling server error")
	}

	// Wait for all data channels to be open
	for atomic.LoadInt32(&s.Peer.channelsReady) != int32(len(s.Peer.dataChannels)) {
		time.Sleep(50 * time.Millisecond)
	}

	s.globalStartTime = time.Now().UnixMilli()

	// Start concurrent transfers with Bubble Tea progress UI
	wg := &sync.WaitGroup{}
	wg.Add(len(s.Peer.dataChannels))

	for _, fileChannel := range s.Peer.dataChannels {
		go s.SendFile(fileChannel, wg)
	}

	// Render progress while transfers are running
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	// Count lines for proper cursor movement
	numFiles := len(s.Peer.dataChannels)
	firstPrint := true

	// Simple progress display loop
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

progressLoop:
	for {
		select {
		case <-done:
			break progressLoop
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

	// Wait for receiver to confirm completion
	select {
	case <-s.Peer.downloadingDone:
	case <-s.Peer.declineReceived:
		return fmt.Errorf("receiver declined during transfer")
	case <-s.Handler.PeerLeft:
		return fmt.Errorf("peer disconnected during transfer")
	case <-time.After(10 * time.Second):
		fmt.Println(ui.WarningStyle.Render("⚠️  Receiver confirmation timeout (files were sent successfully)"))
	}

	// Calculate stats
	currentTime := time.Now().UnixMilli()
	timeDiff := float64(currentTime-s.globalStartTime) / 1000.0

	var totalSize uint64
	for _, fc := range s.Peer.dataChannels {
		totalSize += uint64(fc.FileInfo.Size)
	}

	totalMiB := float64(totalSize) / 1048576.0
	avgSpeed := totalMiB / timeDiff

	// Display final stats using go-pretty table
	fmt.Println()
	ui.RenderTransferSummary("📊 Transfer Summary", ui.TransferSummary{
		Status:    "✅ Complete",
		Files:     len(s.Peer.dataChannels),
		TotalSize: formatSize(int64(totalSize)),
		Duration:  fmt.Sprintf("%.2f seconds", timeDiff),
		Speed:     fmt.Sprintf("%.2f MiB/s", avgSpeed),
	})

	return nil
}

// formatSize formats bytes to human readable string
func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// SendFile sends a single file through its data channel
func (s *SenderSession) SendFile(fc *FileChannel, wg *sync.WaitGroup) error {
	defer wg.Done()
	defer fc.File.Close()

	// Ensure channel is open before starting
	if fc.Channel.ReadyState() != webrtc.DataChannelStateOpen {
		if s.ProgressModel != nil {
			s.ProgressModel.MarkError(fc.Index, "channel not open")
		}
		return fmt.Errorf("channel not open for %s: %s", fc.FileInfo.Name, fc.Channel.ReadyState())
	}

	for {
		// Check if channel is still open
		if fc.Channel.ReadyState() != webrtc.DataChannelStateOpen {
			if s.ProgressModel != nil {
				s.ProgressModel.MarkError(fc.Index, "channel closed")
			}
			return fmt.Errorf("channel closed during transfer")
		}

		// Wait for buffer space with backoff
		if fc.Channel.BufferedAmount() >= BufferThreshold {
			time.Sleep(5 * time.Millisecond)
			continue
		}

		// Buffer has space, proceed with sending
		n, err := fc.File.Read(fc.Packet)
		if err != nil {
			if err == io.EOF {
				// Wait for all buffered data to be sent (with timeout)
				startDrain := time.Now()
				for fc.Channel.BufferedAmount() > 0 && time.Since(startDrain) < 5*time.Second {
					if fc.Channel.ReadyState() != webrtc.DataChannelStateOpen {
						if s.ProgressModel != nil {
							s.ProgressModel.MarkComplete(fc.Index)
						}
						return nil
					}
					time.Sleep(10 * time.Millisecond)
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

		// Update progress
		atomic.AddInt64(&fc.SentBytes, int64(len(packet)))
		if s.ProgressModel != nil {
			s.ProgressModel.UpdateProgress(fc.Index, atomic.LoadInt64(&fc.SentBytes))
		}

		fc.Packet = fc.Packet[:cap(fc.Packet)]
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
