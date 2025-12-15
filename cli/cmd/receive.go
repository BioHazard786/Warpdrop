package cmd

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/BioHazard786/Warpdrop/cli/internal/config"
	"github.com/BioHazard786/Warpdrop/cli/internal/signaling"
	"github.com/BioHazard786/Warpdrop/cli/internal/ui"
	"github.com/BioHazard786/Warpdrop/cli/internal/webrtc"
	"github.com/BioHazard786/Warpdrop/cli/internal/webrtc/multichannel"
	"github.com/BioHazard786/Warpdrop/cli/internal/webrtc/singlechannel"
	"github.com/spf13/cobra"
)

// CLI flags for receive command
var (
	flagReceiverDomain   string
	flagReceiverSTUN     string
	flagReceiverTURN     string
	flagReceiverTURNUser string
	flagReceiverTURNPass string
	flagReceiverRelay    bool
)

// receiveCmd represents the receive command
var receiveCmd = &cobra.Command{
	Use:     "receive <room-id|url>",
	Aliases: []string{"r"},
	Short:   "Receive files from a sender",
	Long: `The receive command allows you to receive files directly from a sender using WebRTC technology.

Examples:
  # Receive files from room ID
  warpdrop receive ABC123

  # Receive from webapp URL
  warpdrop receive https://warpdrop.qzz.io/r/ABC123

  # Receive with custom STUN server
  warpdrop receive ABC123 --stun stun:stun.custom.com:19302`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		roomID, err := parseRoomInput(args[0])
		if err != nil {
			return err
		}
		return receiveFiles(roomID)
	},
}

// receiveFiles handles the actual file receiving logic
func receiveFiles(roomID string) error {
	// Step 1: Load config
	cfg, err := config.Load(config.Options{
		Domain:     flagReceiverDomain,
		STUNServer: flagReceiverSTUN,
		TURNServer: flagReceiverTURN,
		TURNUser:   flagReceiverTURNUser,
		TURNPass:   flagReceiverTURNPass,
		ForceRelay: flagReceiverRelay,
	})
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cfg.ForceRelay && cfg.GetTURNServers() == nil {
		return fmt.Errorf("cannot force relay mode without TURN server configured")
	}

	// Step 2: Connect to signaling server
	stopSpinner := ui.RunConnectionSpinner("Establishing WebSocket connection...")
	defer stopSpinner()

	client := signaling.NewClient(cfg.WebSocketURL)
	if err := client.Connect(); err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer client.Close()
	stopSpinner()

	// Step 3: Create message handler
	handler := signaling.NewHandler(client)
	defer handler.Close()

	// Step 4: Start the handler in a goroutine (background)
	go handler.Start()

	// Step 5: Send join_room message
	client.SendMessage(&signaling.Message{
		Type:       signaling.MessageTypeJoinRoom,
		RoomID:     roomID,
		ClientType: "cli",
	})

	// Step 6: Wait for peer to join
	var peerInfo *signaling.PeerInfo
	select {
	case peerInfo = <-handler.JoinSuccess:
		fmt.Printf("Peer joined (type: %s)\n", peerInfo.ClientType)

	case errMsg := <-handler.Error:
		return fmt.Errorf("server error: %s", errMsg)
	}

	// Step 7: Create WebRTC session and start transfer
	protocol := webrtc.SelectProtocol(peerInfo.ClientType)

	switch protocol {
	case webrtc.MultiChannelProtocol:
		session, err := multichannel.NewReceiverSession(client, handler, cfg, peerInfo)
		if err != nil {
			return fmt.Errorf("failed to create WebRTC session: %w", err)
		}
		defer session.Close()

		if err := session.Start(); err != nil {
			return fmt.Errorf("failed to start WebRTC connection: %w", err)
		}

		// Set progress callback for UI updates
		session.SetProgressUI()

		if err := session.Transfer(); err != nil {
			return fmt.Errorf("failed to transfer files: %w", err)
		}

	case webrtc.SingleChannelProtocol:
		session, err := singlechannel.NewReceiverSession(client, handler, cfg, peerInfo)
		if err != nil {
			return fmt.Errorf("failed to create WebRTC session: %w", err)
		}
		defer session.Close()

		if err := session.Start(); err != nil {
			return fmt.Errorf("failed to start WebRTC connection: %w", err)
		}

		// Set progress callback for UI updates
		session.SetProgressUI()

		if err := session.Transfer(); err != nil {
			return fmt.Errorf("failed to transfer files: %w", err)
		}

	default:
		return fmt.Errorf("unsupported protocol: %s", protocol)
	}

	return nil
}

// parseRoomInput parses room ID or URL from command argument
func parseRoomInput(input string) (string, error) {
	if input == "" {
		return "", fmt.Errorf("room ID or URL cannot be empty")
	}

	if strings.Contains(input, "://") || strings.Contains(input, ".") {
		roomID, err := extractRoomIDFromURL(input)
		if err != nil {
			return "", err
		}
		fmt.Printf("✅ Extracted room ID: %s\n", roomID)
		return roomID, nil
	}

	return input, nil
}

// extractRoomIDFromURL extracts room ID from webapp URL
func extractRoomIDFromURL(urlStr string) (string, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	path := strings.TrimSuffix(parsedURL.Path, "/")
	parts := strings.Split(path, "/")

	for i, part := range parts {
		if part == "r" && i+1 < len(parts) && parts[i+1] != "" {
			return parts[i+1], nil
		}
	}

	return "", fmt.Errorf("could not extract room ID from URL: %s", urlStr)
}

// Add receiveCmd to rootCmd
func init() {
	rootCmd.AddCommand(receiveCmd)

	receiveCmd.Flags().StringVarP(&flagReceiverDomain, "domain", "d", "", "Custom domain (default: warpdrop.qzz.io)")
	receiveCmd.Flags().StringVarP(&flagReceiverSTUN, "stun", "s", "", "Custom STUN server (default: stun:stun.l.google.com:19302)")
	receiveCmd.Flags().StringVarP(&flagReceiverTURN, "turn", "t", "", "Custom TURN server (optional)")
	receiveCmd.Flags().StringVarP(&flagReceiverTURNUser, "turn-user", "u", "", "TURN server username")
	receiveCmd.Flags().StringVarP(&flagReceiverTURNPass, "turn-pass", "p", "", "TURN server password")
	receiveCmd.Flags().BoolVarP(&flagReceiverRelay, "relay", "r", false, "Force relay mode (use when behind restrictive networks)")
}
