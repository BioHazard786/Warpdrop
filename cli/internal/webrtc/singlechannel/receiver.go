package singlechannel

import (
	"fmt"
	"os"
	"path/filepath"
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
		FilesMetadata:  make([]FileMetadata, 0),
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

	// Handle incoming data channel from sender
	r.PeerConnection.OnDataChannel(func(dc *webrtc.DataChannel) {
		if dc.Label() == "file-transfer" {
			r.DataChannel = dc
			r.setupDataChannelHandlers()
		}
	})
}

// setupDataChannelHandlers configures data channel event handlers
func (r *ReceiverSession) setupDataChannelHandlers() {
	r.DataChannel.OnOpen(func() {
		fmt.Println("📡 Data channel opened!")
		// Send device info for UI parity
		r.sendDeviceInfo()
	})

	r.DataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		var message Message
		if err := msgpack.Unmarshal(msg.Data, &message); err != nil {
			fmt.Printf("❌ Failed to parse message: %v\n", err)
			return
		}

		switch message.Type {
		case MessageTypeFilesMetadata:
			r.handleFilesMetadata(message.Payload)

		case MessageTypeChunk:
			r.handleChunk(message.Payload)
		}
	})
}

// sendDeviceInfo sends device information to sender
func (r *ReceiverSession) sendDeviceInfo() {
	deviceInfo := Message{
		Type: MessageTypeDeviceInfo,
		Payload: DeviceInfoPayload{
			DeviceName:    "CLI",
			DeviceVersion: "0.0.1",
		},
	}
	data, err := msgpack.Marshal(deviceInfo)
	if err == nil {
		_ = r.DataChannel.Send(data)
	}
}

// handleFilesMetadata processes incoming file metadata
func (r *ReceiverSession) handleFilesMetadata(payload any) {
	payloadBytes, _ := msgpack.Marshal(payload)
	var metas []FileMetadata
	if err := msgpack.Unmarshal(payloadBytes, &metas); err != nil {
		fmt.Printf("❌ Failed to decode metadata: %v\n", err)
		return
	}
	r.FilesMetadata = metas
	r.metadataReady <- struct{}{}
}

// handleChunk processes an incoming file chunk
func (r *ReceiverSession) handleChunk(payload any) {
	payloadBytes, _ := msgpack.Marshal(payload)
	var chunk ChunkPayload
	if err := msgpack.Unmarshal(payloadBytes, &chunk); err != nil {
		fmt.Printf("❌ Failed to decode chunk: %v\n", err)
		return
	}

	if r.currentFile == nil || r.currentMeta == nil {
		fmt.Printf("❌ No active file to write to\n")
		return
	}

	if chunk.FileName != r.currentMeta.Name {
		fmt.Printf("❌ Unexpected file %s (expecting %s)\n", chunk.FileName, r.currentMeta.Name)
		return
	}

	if chunk.Offset != r.currentOffset {
		if _, err := r.currentFile.Seek(int64(chunk.Offset), 0); err != nil {
			fmt.Printf("❌ Failed to seek: %v\n", err)
			return
		}
		r.currentOffset = chunk.Offset
	}

	if _, err := r.currentFile.Write(chunk.Bytes); err != nil {
		fmt.Printf("❌ Failed to write chunk: %v\n", err)
		return
	}

	r.currentOffset += uint64(len(chunk.Bytes))
	if r.ProgressModel != nil {
		r.ProgressModel.UpdateProgress(r.currentIndex, int64(r.currentOffset))
	}

	if chunk.Final {
		if r.ProgressModel != nil {
			r.ProgressModel.MarkComplete(r.currentIndex)
		}
		r.currentFile.Close()
		r.currentFile = nil
		r.currentMeta = nil
		r.currentOffset = 0
		r.done <- struct{}{}
	}
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

// SendReadyToReceive requests a specific file from the sender
func (r *ReceiverSession) SendReadyToReceive(fileName string, offset uint64) error {
	if r.DataChannel == nil {
		return fmt.Errorf("data channel not open")
	}

	message := Message{
		Type: MessageTypeReadyToReceive,
		Payload: ReadyToReceivePayload{
			FileName: fileName,
			Offset:   offset,
		},
	}
	data, err := msgpack.Marshal(message)
	if err != nil {
		return err
	}
	return r.DataChannel.Send(data)
}

// SendDownloadingDone sends completion signal to sender
func (r *ReceiverSession) SendDownloadingDone() error {
	if r.DataChannel == nil {
		return fmt.Errorf("data channel not open")
	}

	message := Message{Type: MessageTypeDownloadingDone}
	data, err := msgpack.Marshal(message)
	if err != nil {
		return err
	}
	return r.DataChannel.Send(data)
}

// beginFile creates and prepares a file for receiving
func (r *ReceiverSession) beginFile(meta FileMetadata) error {
	filename := GetUniqueFilename(meta.Name)
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	r.currentFile = file
	r.currentMeta = &meta
	r.currentOffset = 0
	return nil
}

// ReceiveAllFiles downloads files sequentially
func (r *ReceiverSession) ReceiveAllFiles() error {
	r.globalStartTime = time.Now().UnixMilli()

	fmt.Printf("\n%s Receiving files...\n\n", ui.IconReceive)

	// Count lines for proper cursor movement
	numFiles := len(r.FilesMetadata)
	firstPrint := true

	// Start progress display in background
	progressDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-progressDone:
				return
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
	}()

	for idx, meta := range r.FilesMetadata {
		r.currentIndex = idx
		if err := r.beginFile(meta); err != nil {
			close(progressDone)
			return err
		}

		if err := r.SendReadyToReceive(meta.Name, 0); err != nil {
			close(progressDone)
			return fmt.Errorf("failed to send ready message: %w", err)
		}

		// Wait until current file finishes
		<-r.done

		if idx == len(r.FilesMetadata)-1 {
			break
		}
	}

	// Stop progress display
	close(progressDone)

	// Final update - clear progress lines and print final state
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
		fmt.Printf("⚠️  Failed to send completion signal: %v\n", err)
	}

	// Calculate stats
	currentTime := time.Now().UnixMilli()
	timeDiff := float64(currentTime-r.globalStartTime) / 1000.0

	var totalSize uint64
	for _, f := range r.FilesMetadata {
		totalSize += f.Size
	}

	totalMiB := float64(totalSize) / 1048576.0
	avgSpeed := totalMiB / timeDiff

	// Display final stats using go-pretty table
	fmt.Println()
	ui.RenderTransferSummary("📊 Receive Summary", ui.TransferSummary{
		Status:    "✅ Complete",
		Files:     len(r.FilesMetadata),
		TotalSize: fmt.Sprintf("%.2f MB", totalMiB),
		Duration:  fmt.Sprintf("%.2f seconds", timeDiff),
		Speed:     fmt.Sprintf("%.2f MiB/s", avgSpeed),
	})

	return nil
}

// Close closes the peer connection
func (r *ReceiverSession) Close() error {
	if r.currentFile != nil {
		r.currentFile.Close()
	}
	if r.DataChannel != nil {
		r.DataChannel.Close()
	}
	return r.PeerConnection.Close()
}

// GetUniqueFilename returns a unique filename by appending (1), (2), etc. if file exists
func GetUniqueFilename(filename string) string {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return filename
	}

	ext := filepath.Ext(filename)
	nameWithoutExt := filename[:len(filename)-len(ext)]

	counter := 1
	for {
		newFilename := fmt.Sprintf("%s (%d)%s", nameWithoutExt, counter, ext)
		if _, err := os.Stat(newFilename); os.IsNotExist(err) {
			return newFilename
		}
		counter++
	}
}
