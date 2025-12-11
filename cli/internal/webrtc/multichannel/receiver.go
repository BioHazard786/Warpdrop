package multichannel

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/BioHazard786/Warpdrop/cli/internal/config"
	"github.com/BioHazard786/Warpdrop/cli/internal/ui"
	"github.com/pion/webrtc/v4"
	"github.com/vmihailenco/msgpack/v5"
)

// NewReceiverSession creates a receiver-side WebRTC session
func NewReceiverSession(cfg *config.Config) (*ReceiverSession, error) {
	pc, err := newPeerConnection(cfg)
	if err != nil {
		return nil, err
	}

	receiver := &ReceiverSession{
		PeerConnection: pc,
		DataChannels:   make([]*ReceiverChannel, 0),
		FilesMetadata:  make([]interface{}, 0),
		metadataReady:  make(chan struct{}, 1),
		done:           make(chan struct{}),
	}

	receiver.setupHandlers()
	return receiver, nil
}

// SetProgressUI initializes the progress UI
func (r *ReceiverSession) SetProgressUI(fileNames []string, fileSizes []int64) {
	r.ProgressModel = ui.NewProgressModel(fileNames, fileSizes)
}

// setupHandlers configures ICE and data channel handlers
func (r *ReceiverSession) setupHandlers() {
	r.PeerConnection.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		if state == webrtc.ICEConnectionStateFailed {
			r.done <- struct{}{}
		}
	})

	// Handle incoming data channels from sender
	r.PeerConnection.OnDataChannel(func(dc *webrtc.DataChannel) {
		// Check if this is the control channel
		if dc.Label() == "control" {
			r.controlChannel = dc
			r.setupControlChannel(dc)
			return
		}

		// File transfer channel
		channel := &ReceiverChannel{
			Channel:     dc,
			Metadata:    &FileMetadata{},
			MessageChan: make(chan []byte, 128),
			Done:        make(chan struct{}, 1),
			Index:       len(r.DataChannels),
		}

		r.DataChannels = append(r.DataChannels, channel)

		dc.OnMessage(func(msg webrtc.DataChannelMessage) {
			// All messages are file data
			channel.MessageChan <- msg.Data
		})

		dc.OnClose(func() {
			close(channel.MessageChan) // Close the channel so receiver goroutine can exit
			channel.Done <- struct{}{}
		})
	})
}

// setupControlChannel handles control channel messages (like web app)
func (r *ReceiverSession) setupControlChannel(dc *webrtc.DataChannel) {
	dc.OnOpen(func() {
		// Control channel opened
	})

	dc.OnClose(func() {
		// Control channel closed
	})

	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		// Parse the message
		var message struct {
			Type    string         `msgpack:"type"`
			Payload []FileMetadata `msgpack:"payload"`
		}

		if err := msgpack.Unmarshal(msg.Data, &message); err != nil {
			return
		}

		// Handle based on message type
		if message.Type == "files_metadata" {
			r.FilesMetadata = make([]interface{}, len(message.Payload))
			for i, meta := range message.Payload {
				r.FilesMetadata[i] = meta
			}
			r.metadataReady <- struct{}{}
		}
	})
}

// CreateAnswer creates WebRTC answer from offer
func (r *ReceiverSession) CreateAnswer(offer *webrtc.SessionDescription) (*webrtc.SessionDescription, error) {
	if err := r.PeerConnection.SetRemoteDescription(*offer); err != nil {
		return nil, err
	}

	answer, err := r.PeerConnection.CreateAnswer(nil)
	if err != nil {
		return nil, err
	}

	r.gatherDone = webrtc.GatheringCompletePromise(r.PeerConnection)

	if err = r.PeerConnection.SetLocalDescription(answer); err != nil {
		return nil, err
	}

	<-r.gatherDone
	return r.PeerConnection.LocalDescription(), nil
}

// WaitForMetadata blocks until file metadata is received from sender
func (r *ReceiverSession) WaitForMetadata() {
	<-r.metadataReady
}

// SendReadyToReceive sends confirmation to sender (like web app)
func (r *ReceiverSession) SendReadyToReceive() error {
	if r.controlChannel == nil {
		return fmt.Errorf("control channel not open")
	}

	message := struct {
		Type string `msgpack:"type"`
	}{
		Type: "ready_to_receive",
	}

	data, err := msgpack.Marshal(message)
	if err != nil {
		return err
	}

	return r.controlChannel.Send(data)
}

func (r *ReceiverSession) SendDecline() error {
	if r.controlChannel == nil {
		return fmt.Errorf("control channel not open")
	}

	message := struct {
		Type string `msgpack:"type"`
	}{
		Type: "decline_receive",
	}

	data, err := msgpack.Marshal(message)
	if err != nil {
		return err
	}

	return r.controlChannel.Send(data)
}

// ReceiveFile writes data from channel to file (runs in goroutine)
func (r *ReceiverSession) ReceiveFile(fileMetadata FileMetadata, channelIndex int, wg *sync.WaitGroup) error {
	defer wg.Done()

	if channelIndex >= len(r.DataChannels) {
		return fmt.Errorf("channel index out of range")
	}

	channel := r.DataChannels[channelIndex]

	// Get unique filename if file already exists
	filename := GetUniqueFilename(fileMetadata.Name)

	// Create file
	file, err := os.Create(filename)
	if err != nil {
		if r.ProgressModel != nil {
			r.ProgressModel.MarkError(channelIndex, err.Error())
		}
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()
	channel.File = file

	var receivedBytes uint64

	for data := range channel.MessageChan {
		receivedBytes += uint64(len(data))

		if _, err := file.Write(data); err != nil {
			if r.ProgressModel != nil {
				r.ProgressModel.MarkError(channelIndex, err.Error())
			}
			return fmt.Errorf("failed to write file: %w", err)
		}

		// Update progress
		atomic.AddInt64(&channel.ReceivedBytes, int64(len(data)))
		if r.ProgressModel != nil {
			r.ProgressModel.UpdateProgress(channelIndex, int64(receivedBytes))
		}

		// Check if all data received
		if receivedBytes >= fileMetadata.Size {
			if r.ProgressModel != nil {
				r.ProgressModel.MarkComplete(channelIndex)
			}
			return nil
		}
	}

	// Channel closed before all data received
	if receivedBytes < fileMetadata.Size {
		if r.ProgressModel != nil {
			r.ProgressModel.MarkError(channelIndex, "channel closed early")
		}
		return fmt.Errorf("channel closed early: received %d/%d bytes for %s", receivedBytes, fileMetadata.Size, fileMetadata.Name)
	}

	if r.ProgressModel != nil {
		r.ProgressModel.MarkComplete(channelIndex)
	}
	return nil
}

// ReceiveAllFiles starts concurrent file receives
func (r *ReceiverSession) ReceiveAllFiles() error {
	r.globalStartTime = time.Now().UnixMilli()

	// Get metadata as FileMetadata slice
	fileMetadataList := make([]FileMetadata, len(r.FilesMetadata))
	fileNames := make([]string, len(r.FilesMetadata))
	fileSizes := make([]int64, len(r.FilesMetadata))

	for i, meta := range r.FilesMetadata {
		if fm, ok := meta.(FileMetadata); ok {
			fileMetadataList[i] = fm
			fileNames[i] = fm.Name
			fileSizes[i] = int64(fm.Size)
		}
	}

	// Initialize progress UI
	r.SetProgressUI(fileNames, fileSizes)

	wg := &sync.WaitGroup{}
	wg.Add(len(fileMetadataList))

	for i, metadata := range fileMetadataList {
		metadataCopy := metadata
		go func(idx int, meta FileMetadata) {
			if err := r.ReceiveFile(meta, idx, wg); err != nil {
				if r.ProgressModel != nil {
					r.ProgressModel.MarkError(idx, err.Error())
				}
			}
		}(i, metadataCopy)
	}

	// Render progress while transfers are running
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	// Count lines for proper cursor movement
	numFiles := len(fileMetadataList)
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
			if r.ProgressModel != nil {
				if !firstPrint {
					// Move cursor up and clear each line before redrawing
					for i := 0; i < numFiles; i++ {
						fmt.Print("\033[A\033[2K")
					}
				}
				firstPrint = false
				fmt.Print(r.ProgressModel.View())
			}
		}
	}

	// Final update
	if r.ProgressModel != nil {
		if !firstPrint {
			for i := 0; i < numFiles; i++ {
				fmt.Print("\033[A\033[2K")
			}
		}
		fmt.Print(r.ProgressModel.View())
	}
	fmt.Println()

	// Send completion signal
	if err := r.SendDownloadingDone(); err != nil {
		fmt.Println(ui.WarningStyle.Render("⚠️  Failed to send completion signal"))
	}

	// Calculate stats
	currentTime := time.Now().UnixMilli()
	timeDiff := float64(currentTime-r.globalStartTime) / 1000.0

	var totalSize uint64
	for _, metadata := range fileMetadataList {
		totalSize += metadata.Size
	}

	totalMiB := float64(totalSize) / 1048576.0
	avgSpeed := totalMiB / timeDiff

	// Display final stats using go-pretty table
	fmt.Println()
	ui.RenderTransferSummary("📊 Receive Summary", ui.TransferSummary{
		Status:    "✅ Complete",
		Files:     len(fileMetadataList),
		TotalSize: fmt.Sprintf("%.2f MB", totalMiB),
		Duration:  fmt.Sprintf("%.2f seconds", timeDiff),
		Speed:     fmt.Sprintf("%.2f MiB/s", avgSpeed),
	})

	// Small delay to ensure the completion signal is delivered before connection closes
	time.Sleep(200 * time.Millisecond)

	return nil
}

// SendDownloadingDone sends completion signal to sender
func (r *ReceiverSession) SendDownloadingDone() error {
	if r.controlChannel == nil {
		return fmt.Errorf("control channel not open")
	}

	message := struct {
		Type string `msgpack:"type"`
	}{
		Type: "downloading_done",
	}

	data, err := msgpack.Marshal(message)
	if err != nil {
		return err
	}

	return r.controlChannel.Send(data)
}

// Close closes the peer connection
func (r *ReceiverSession) Close() error {
	for _, ch := range r.DataChannels {
		if ch.File != nil {
			ch.File.Close()
		}
		if ch.Channel != nil {
			ch.Channel.Close()
		}
	}
	if r.controlChannel != nil {
		r.controlChannel.Close()
	}
	return r.PeerConnection.Close()
}

// GetUniqueFilename returns a unique filename by appending (1), (2), etc. if file exists
func GetUniqueFilename(filename string) string {
	// If file doesn't exist, return original name
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return filename
	}

	// Extract extension and base name
	ext := filepath.Ext(filename)
	nameWithoutExt := filename[:len(filename)-len(ext)]

	// Try appending (1), (2), (3), etc.
	counter := 1
	for {
		newFilename := fmt.Sprintf("%s (%d)%s", nameWithoutExt, counter, ext)
		if _, err := os.Stat(newFilename); os.IsNotExist(err) {
			return newFilename
		}
		counter++
	}
}
