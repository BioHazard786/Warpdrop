package cmd

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BioHazard786/Warpdrop/cli/internal/config"
	"github.com/BioHazard786/Warpdrop/cli/internal/signaling"
	"github.com/BioHazard786/Warpdrop/cli/internal/transfer"
	"github.com/BioHazard786/Warpdrop/cli/internal/ui"
	"github.com/BioHazard786/Warpdrop/cli/internal/utils"
	"github.com/spf13/cobra"
)

var (
	flagReceiverDomain   string
	flagReceiverSTUN     string
	flagReceiverTURN     string
	flagReceiverTURNUser string
	flagReceiverTURNPass string
	flagReceiverRelay    bool
	flagReceiverZip      bool
	flagReceiverDir      string
)

var receiveCmd = &cobra.Command{
	Use:     "receive <room-id|url>",
	Aliases: []string{"r"},
	Short:   "Receive files from a sender",
	Long: `Receive files directly from a sender using WebRTC technology.

Examples:
  warpdrop receive ABC123
  warpdrop receive https://warpdrop.qzz.io/r/ABC123
  warpdrop receive ABC123 --relay`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		roomID, err := parseRoomInput(args[0])
		if err != nil {
			return err
		}
		return receiveFiles(roomID)
	},
}

func receiveFiles(roomID string) error {
	cfg, err := LoadConfig(config.Options{
		Domain:     flagReceiverDomain,
		STUNServer: flagReceiverSTUN,
		TURNServer: flagReceiverTURN,
		TURNUser:   flagReceiverTURNUser,
		TURNPass:   flagReceiverTURNPass,
		ForceRelay: flagReceiverRelay,
	})
	if err != nil {
		return err
	}

	fmt.Println()
	stopSpinner := ui.RunConnectionSpinner("Connecting to server...")
	ctx, err := NewConnectionContext(cfg)
	if err != nil {
		return err
	}
	defer ctx.Close()
	stopSpinner()

	peerInfo, err := joinRoom(ctx, roomID)
	if err != nil {
		return err
	}
	ctx.PeerInfo = peerInfo

	session, err := CreateReceiverSession(ctx)
	if err != nil {
		return transfer.NewError("create session", err)
	}

	opts, tempDir, cleanup, err := prepareTransferOptions(flagReceiverZip, flagReceiverDir)
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}

	if err := RunReceiverSession(session, opts); err != nil {
		return err
	}

	return finalizeTransfer(flagReceiverZip, flagReceiverDir, tempDir)
}

func prepareTransferOptions(zipMode bool, outputDir string) (*transfer.TransferOptions, string, func(), error) {
	opts := &transfer.TransferOptions{
		ZipMode:   zipMode,
		OutputDir: outputDir,
	}

	var tempDir string
	var cleanup func()

	if zipMode {
		var err error
		tempDir, err = os.MkdirTemp("", "warpdrop-receive-*")
		if err != nil {
			return nil, "", nil, transfer.NewError("create temp dir", err)
		}
		opts.OutputDir = tempDir
		cleanup = func() {
			os.RemoveAll(tempDir)
		}
	}

	return opts, tempDir, cleanup, nil
}

func finalizeTransfer(zipMode bool, outputDir, tempDir string) error {
	if !zipMode {
		return nil
	}

	zipName := fmt.Sprintf("warpdrop-download-%d.zip", time.Now().UnixMilli())
	if outputDir != "" {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return transfer.NewError("create output dir", err)
		}
		zipName = filepath.Join(outputDir, zipName)
	}

	fmt.Println()
	s := ui.NewWaitingSpinner("Zipping files...")
	s.Start()
	if err := utils.ZipDirectory(tempDir, zipName); err != nil {
		s.Stop()
		return transfer.NewError("zip files", err)
	}
	s.Success(fmt.Sprintf("Files zipped to %s", zipName))

	return nil
}

func joinRoom(ctx *ConnectionContext, roomID string) (*signaling.PeerInfo, error) {
	ctx.Client.SendMessage(&signaling.Message{
		Type:       signaling.MessageTypeJoinRoom,
		RoomID:     roomID,
		ClientType: "cli",
	})

	select {
	case peerInfo := <-ctx.Handler.JoinSuccess:
		return peerInfo, nil
	case errMsg := <-ctx.Handler.Error:
		return nil, transfer.WrapError("join room", transfer.ErrSignalingError, errMsg)
	}
}

func parseRoomInput(input string) (string, error) {
	if input == "" {
		return "", fmt.Errorf("room ID cannot be empty")
	}

	if strings.Contains(input, "://") || strings.Contains(input, ".") {
		roomID, err := extractRoomIDFromURL(input)
		if err != nil {
			return "", err
		}
		ui.PrintSuccessf("Extracted room ID: %s", roomID)
		return roomID, nil
	}

	return input, nil
}

func extractRoomIDFromURL(urlStr string) (string, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", transfer.NewError("parse URL", err)
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

func init() {
	rootCmd.AddCommand(receiveCmd)

	receiveCmd.Flags().StringVar(&flagReceiverDomain, "domain", "", "Custom domain")
	receiveCmd.Flags().StringVarP(&flagReceiverSTUN, "stun", "s", "", "Custom STUN server")
	receiveCmd.Flags().StringVarP(&flagReceiverTURN, "turn", "t", "", "Custom TURN server")
	receiveCmd.Flags().StringVar(&flagReceiverTURNUser, "turn-user", "", "TURN username")
	receiveCmd.Flags().StringVar(&flagReceiverTURNPass, "turn-pass", "", "TURN password")
	receiveCmd.Flags().BoolVarP(&flagReceiverRelay, "relay", "r", false, "Force relay mode")
	receiveCmd.Flags().BoolVarP(&flagReceiverZip, "zip", "z", false, "Zip received files")
	receiveCmd.Flags().StringVarP(&flagReceiverDir, "dir", "d", "", "Directory to save received files")
}
