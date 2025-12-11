package cmd

import (
	"fmt"

	"github.com/BioHazard786/Warpdrop/cli/internal/config"
	"github.com/BioHazard786/Warpdrop/cli/internal/files"
	"github.com/BioHazard786/Warpdrop/cli/internal/signaling"
	"github.com/BioHazard786/Warpdrop/cli/internal/ui"
	"github.com/BioHazard786/Warpdrop/cli/internal/webrtc"
	"github.com/BioHazard786/Warpdrop/cli/internal/webrtc/multichannel"
	"github.com/BioHazard786/Warpdrop/cli/internal/webrtc/singlechannel"
	"github.com/spf13/cobra"
)

// CLI flags for send command
var (
	flagDomain   string
	flagSTUN     string
	flagTURN     string
	flagTURNUser string
	flagTURNPass string
)

// sendCmd represents the send command
var sendCmd = &cobra.Command{
	Use:     "send",
	Aliases: []string{"s"},
	Short:   "Send files to a receiver",
	Long: `The send command allows you to send files directly to a receiver using WebRTC technology.

Examples:
  # Send files using default settings
  warpdrop send file1.txt file2.pdf

  # Send files using custom domain
  warpdrop send --domain custom.example.com file.txt

  # Send files using custom STUN server
  warpdrop send --stun stun:stun.custom.com:19302 file.txt

  # Send files using custom TURN server
  warpdrop send --turn turn:turn.example.com --turn-user myuser --turn-pass mypass file.txt`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("no files specified to send")
		}
		return sendFiles(args)
	},
}

// sendFiles handles the actual file sending logic
func sendFiles(filePaths []string) error {
	// Step 1: Validate files exist and are readable
	stopFileSpinner := ui.RunSpinner("Validating files...")

	fileInfos, err := files.ValidateFiles(filePaths)
	if err != nil {
		fmt.Errorf("file validation error: %w", err)
		return err
	}
	stopFileSpinner()

	// Display files to be sent in a beautiful table
	items := make([]ui.FileTableItem, len(fileInfos))
	for i, file := range fileInfos {
		items[i] = ui.FileTableItem{
			Index: i + 1,
			Name:  file.Name,
			Size:  file.Size,
			Type:  file.Type,
		}
	}

	ui.RenderFileTable(items, "📋 Files to Send")
	fmt.Println()

	// Step 2: Load config with CLI flag overrides
	cfg, err := config.Load(config.Options{
		Domain:     flagDomain,
		STUNServer: flagSTUN,
		TURNServer: flagTURN,
		TURNUser:   flagTURNUser,
		TURNPass:   flagTURNPass,
	})
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Step 3: Connect to signaling server
	stopSpinner := ui.RunConnectionSpinner("Establishing WebSocket connection...")

	client := signaling.NewClient(cfg.WebSocketURL)
	if err := client.Connect(); err != nil {
		stopSpinner()
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer client.Close()
	stopSpinner()

	// Step 4: Create a message handler
	handler := signaling.NewHandler(client)
	defer handler.Close()

	// Step 5: Start the handler in a goroutine (background)
	go handler.Start()

	// Step 6: Send create_room message with client info
	client.SendMessage(&signaling.Message{
		Type:       signaling.MessageTypeCreateRoom,
		ClientType: "cli",
	})

	// Step 7: Wait for room creation
	var roomID string
	select {
	case roomID = <-handler.RoomCreated:
		// Display room info with beautiful box
		roomInfo := ui.NewRoomInfo(roomID, cfg.GetRoomLink(roomID))
		fmt.Println()
		fmt.Println(roomInfo.View())
		fmt.Println()

	case errMsg := <-handler.Error:
		return fmt.Errorf("server error: %s", errMsg)
	}

	// Step 8: Wait for peer to join
	spin := ui.NewWaitingSpinner("Waiting for receiver to join...")
	spin.Start()

	var peerInfo *signaling.PeerInfo
	select {
	case peerInfo = <-handler.PeerJoined:
		spin.Success(fmt.Sprintf("Peer joined (type: %s)", peerInfo.ClientType))

	case errMsg := <-handler.Error:
		spin.Stop()
		return fmt.Errorf("server error while waiting for peer: %s", errMsg)
	}

	// Step 9: Create WebRTC session and start transfer
	protocol := webrtc.SelectProtocol(peerInfo.ClientType)

	// Convert []FileInfo to []*FileInfo
	fileInfoPtrs := make([]*files.FileInfo, len(fileInfos))
	for i := range fileInfos {
		fileInfoPtrs[i] = &fileInfos[i]
	}

	// Get file names and sizes for UI
	fileNames := make([]string, len(fileInfos))
	fileSizes := make([]int64, len(fileInfos))
	for i, f := range fileInfos {
		fileNames[i] = f.Name
		fileSizes[i] = f.Size
	}

	switch protocol {
	case webrtc.MultiChannelProtocol:
		session, err := multichannel.NewSenderSession(client, handler, cfg, len(fileInfos), peerInfo)
		if err != nil {
			return fmt.Errorf("failed to create WebRTC session: %w", err)
		}
		defer session.Close()

		// Set progress callback for UI updates
		session.SetProgressUI(fileNames, fileSizes)

		if err := session.Start(fileInfoPtrs); err != nil {
			return fmt.Errorf("failed to start WebRTC connection: %w", err)
		}

		if err := session.Transfer(); err != nil {
			return fmt.Errorf("failed to transfer files: %w", err)
		}

	case webrtc.SingleChannelProtocol:
		session, err := singlechannel.NewSenderSession(client, handler, cfg, peerInfo)
		if err != nil {
			return fmt.Errorf("failed to create WebRTC session: %w", err)
		}
		defer session.Close()

		// Set progress callback for UI updates
		session.SetProgressUI(fileNames, fileSizes)

		if err := session.Start(fileInfoPtrs); err != nil {
			return fmt.Errorf("failed to start WebRTC connection: %w", err)
		}

		if err := session.Transfer(); err != nil {
			return fmt.Errorf("failed to transfer files: %w", err)
		}

	default:
		return fmt.Errorf("unsupported protocol: %s", protocol)
	}

	return nil
}

// Add sendCmd to rootCmd
func init() {
	rootCmd.AddCommand(sendCmd)

	// Define CLI flags for send command
	sendCmd.Flags().StringVarP(&flagDomain, "domain", "d", "", "Custom domain (default: warpdrop.qzz.io)")
	sendCmd.Flags().StringVarP(&flagSTUN, "stun", "s", "", "Custom STUN server (default: stun:stun.l.google.com:19302)")
	sendCmd.Flags().StringVarP(&flagTURN, "turn", "t", "", "Custom TURN server (optional)")
	sendCmd.Flags().StringVarP(&flagTURNUser, "turn-user", "u", "", "TURN server username (default: warpdrop)")
	sendCmd.Flags().StringVarP(&flagTURNPass, "turn-pass", "p", "", "TURN server password (default: warpdrop-secret)")
}
