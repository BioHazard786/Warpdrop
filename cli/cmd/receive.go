package cmd

import (
	"fmt"
	"log"
	"net/url"
	"strings"

	"github.com/BioHazard786/Warpdrop/cli/internal/config"
	"github.com/BioHazard786/Warpdrop/cli/internal/signaling"
	"github.com/BioHazard786/Warpdrop/cli/internal/ui"
	"github.com/BioHazard786/Warpdrop/cli/internal/webrtc"
	"github.com/BioHazard786/Warpdrop/cli/internal/webrtc/multichannel"
	"github.com/BioHazard786/Warpdrop/cli/internal/webrtc/singlechannel"
	pionwebrtc "github.com/pion/webrtc/v4"
	"github.com/spf13/cobra"
)

// CLI flags for receive command
var (
	flagReceiverDomain   string
	flagReceiverSTUN     string
	flagReceiverTURN     string
	flagReceiverTURNUser string
	flagReceiverTURNPass string
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
	})
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Step 2: Connect to signaling server
	stopSpinner := ui.RunConnectionSpinner("Establishing WebSocket connection...")

	client := signaling.NewClient(cfg.WebSocketURL)
	if err := client.Connect(); err != nil {
		stopSpinner()
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer client.Close()
	stopSpinner()

	// Step 3: Create message handler
	handler := signaling.NewHandler(client)
	defer handler.Close()

	go handler.Start()

	// Step 4: Send join_room message
	client.SendMessage(&signaling.Message{
		Type:       signaling.MessageTypeJoinRoom,
		RoomID:     roomID,
		ClientType: "cli",
	})

	// Wait for join success
	var peerInfo *signaling.PeerInfo
	select {
	case peerInfo = <-handler.JoinSuccess:
		log.Printf("Peer info: type=%s", peerInfo.ClientType)
	case errMsg := <-handler.Error:
		return fmt.Errorf("join error: %s", errMsg)
	}

	protocol := webrtc.SelectProtocol(peerInfo.ClientType)

	stopSpinner = ui.RunConnectionSpinner("Establishing WebRTC connection...")

	switch protocol {
	case webrtc.MultiChannelProtocol:
		var offer *pionwebrtc.SessionDescription
		select {
		case signalPayload := <-handler.Signal:
			stopSpinner()
			var sdpType pionwebrtc.SDPType
			switch signalPayload.Type {
			case "offer":
				sdpType = pionwebrtc.SDPTypeOffer
			case "answer":
				sdpType = pionwebrtc.SDPTypeAnswer
			default:
				stopSpinner()
				return fmt.Errorf("unexpected signal type: %s", signalPayload.Type)
			}

			offer = &pionwebrtc.SessionDescription{
				Type: sdpType,
				SDP:  signalPayload.SDP,
			}

		case errMsg := <-handler.Error:
			stopSpinner()
			return fmt.Errorf("signaling error: %s", errMsg)
		}

		receiverSession, err := multichannel.NewReceiverSession(cfg, client)
		if err != nil {
			return fmt.Errorf("failed to create receiver session: %w", err)
		}
		defer receiverSession.Close()

		answer, err := receiverSession.CreateAnswer(offer)
		if err != nil {
			return fmt.Errorf("failed to create answer: %w", err)
		}

		client.SendMessage(&signaling.Message{
			Type: signaling.MessageTypeSignal,
			Payload: signaling.SignalPayload{
				Type: answer.Type.String(),
				SDP:  answer.SDP,
			},
		})

		// Start listening for incoming ICE candidates from sender
		go func() {
			for sig := range handler.Signal {
				if sig != nil && sig.ICECandidate != nil {
					_ = receiverSession.HandleICECandidate(sig.ICECandidate)
				}
			}
		}()

		receiverSession.WaitForMetadata()

		// Display files to receive in a table
		items := make([]ui.FileTableItem, 0, len(receiverSession.FilesMetadata))
		for i, meta := range receiverSession.FilesMetadata {
			if fm, ok := meta.(multichannel.FileMetadata); ok {
				items = append(items, ui.FileTableItem{
					Index: i + 1,
					Name:  fm.Name,
					Size:  int64(fm.Size),
					Type:  fm.Type,
				})
			}
		}
		ui.RenderFileTable(items, "📋 Files to receive")

		fmt.Print("\n❓ Do you want to receive these files? [Y/n] ")
		var consent string
		fmt.Scanln(&consent)

		if consent == "n" || consent == "N" {
			if err := receiverSession.SendDecline(); err != nil {
				return fmt.Errorf("failed to send decline signal: %w", err)
			}
			fmt.Println("\n❌ Transfer cancelled by user")
			return nil
		}

		if err := receiverSession.SendReadyToReceive(); err != nil {
			return fmt.Errorf("failed to send ready signal: %w", err)
		}

		if err := receiverSession.ReceiveAllFiles(); err != nil {
			return fmt.Errorf("failed to receive files: %w", err)
		}

	case webrtc.SingleChannelProtocol:
		var offerPayload *signaling.SignalPayload

		for offerPayload == nil {
			select {
			case sig := <-handler.Signal:
				if sig.SDP != "" && sig.Type == "offer" {
					offerPayload = sig
					stopSpinner()
				}
			case errMsg := <-handler.Error:
				stopSpinner()
				return fmt.Errorf("signaling error: %s", errMsg)
			}
		}

		singleSession, err := singlechannel.NewReceiverSession(cfg, client)
		if err != nil {
			return fmt.Errorf("failed to create receiver session: %w", err)
		}
		defer singleSession.Close()

		offer := &pionwebrtc.SessionDescription{Type: pionwebrtc.SDPTypeOffer, SDP: offerPayload.SDP}
		answer, err := singleSession.CreateAnswer(offer)
		if err != nil {
			return fmt.Errorf("failed to create answer: %w", err)
		}

		client.SendMessage(&signaling.Message{
			Type:    signaling.MessageTypeSignal,
			Payload: signaling.SignalPayload{Type: answer.Type.String(), SDP: answer.SDP},
		})

		// Start listening for incoming ICE candidates from sender
		go func() {
			for sig := range handler.Signal {
				if sig != nil && sig.ICECandidate != nil {
					_ = singleSession.HandleICECandidate(sig.ICECandidate)
				}
			}
		}()

		stopMetaSpinner := ui.RunWaitingSpinner("Waiting for file metadata...")

		singleSession.WaitForMetadata()
		stopMetaSpinner()

		// Display files to receive in a table
		items := make([]ui.FileTableItem, len(singleSession.FilesMetadata))
		for i, meta := range singleSession.FilesMetadata {
			items[i] = ui.FileTableItem{
				Index: i + 1,
				Name:  meta.Name,
				Size:  int64(meta.Size),
				Type:  meta.Type,
			}
		}
		ui.RenderFileTable(items, "📋 Files to receive")

		fmt.Print("\n❓ Do you want to receive these files? [Y/n] ")
		var consent string
		fmt.Scanln(&consent)

		if consent == "n" || consent == "N" {
			fmt.Println("\n❌ Transfer cancelled by user")
			return nil
		}

		// Setup progress UI
		fileNames := make([]string, len(singleSession.FilesMetadata))
		fileSizes := make([]int64, len(singleSession.FilesMetadata))
		for i, meta := range singleSession.FilesMetadata {
			fileNames[i] = meta.Name
			fileSizes[i] = int64(meta.Size)
		}
		singleSession.SetProgressUI(fileNames, fileSizes)

		if err := singleSession.ReceiveAllFiles(); err != nil {
			return fmt.Errorf("failed to receive files: %w", err)
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
}
