package cmd

import (
	"fmt"

	"github.com/BioHazard786/Warpdrop/cli/internal/config"
	"github.com/BioHazard786/Warpdrop/cli/internal/files"
	"github.com/BioHazard786/Warpdrop/cli/internal/signaling"
	"github.com/BioHazard786/Warpdrop/cli/internal/transfer"
	"github.com/BioHazard786/Warpdrop/cli/internal/ui"
	"github.com/spf13/cobra"
)

var (
	flagDomain   string
	flagSTUN     string
	flagTURN     string
	flagTURNUser string
	flagTURNPass string
	flagRelay    bool
)

var sendCmd = &cobra.Command{
	Use:     "send",
	Aliases: []string{"s"},
	Short:   "Send files to a receiver",
	Long: `Send files directly to a receiver using WebRTC technology.

Examples:
  warpdrop send file1.txt file2.pdf
  warpdrop send --domain custom.example.com file.txt
  warpdrop send --relay file.txt`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("no files specified")
		}
		return sendFiles(args)
	},
}

func sendFiles(filePaths []string) error {
	stopSpinner := ui.RunSpinner("Validating files...")
	defer stopSpinner()
	fileInfos, err := files.ValidateFiles(filePaths)
	if err != nil {
		return err
	}
	stopSpinner()

	displayFileTable(fileInfos)

	cfg, err := LoadConfig(config.Options{
		Domain:     flagDomain,
		STUNServer: flagSTUN,
		TURNServer: flagTURN,
		TURNUser:   flagTURNUser,
		TURNPass:   flagTURNPass,
		ForceRelay: flagRelay,
	})
	if err != nil {
		return err
	}

	fmt.Println()
	stopSpinner = ui.RunConnectionSpinner("Connecting to server...")
	defer stopSpinner()
	ctx, err := NewConnectionContext(cfg)
	if err != nil {
		return err
	}
	defer ctx.Close()
	stopSpinner()

	roomID, err := createRoom(ctx)
	if err != nil {
		return err
	}

	displayRoomInfo(roomID, cfg)

	peerInfo, err := waitForPeer(ctx)
	if err != nil {
		return err
	}
	ctx.PeerInfo = peerInfo

	fileInfoPtrs := prepareFileData(fileInfos)

	session, err := CreateSenderSession(ctx, fileInfoPtrs)
	if err != nil {
		return transfer.NewError("create session", err)
	}

	return RunSenderSession(session, nil)
}

func displayFileTable(fileInfos []files.FileInfo) {
	items := make([]ui.FileTableItem, len(fileInfos))
	for i, f := range fileInfos {
		items[i] = ui.FileTableItem{Index: i + 1, Name: f.Name, Size: f.Size, Type: f.Type}
	}
	fmt.Println()
	ui.RenderFileTable(items)
}

func displayRoomInfo(roomID string, cfg *config.Config) {
	ui.RenderRoomInfo(roomID, cfg.GetRoomLink(roomID))
}

func createRoom(ctx *ConnectionContext) (string, error) {
	ctx.Client.SendMessage(&signaling.Message{
		Type:       signaling.MessageTypeCreateRoom,
		ClientType: "cli",
	})

	select {
	case roomID := <-ctx.Handler.RoomCreated:
		return roomID, nil
	case errMsg := <-ctx.Handler.Error:
		return "", transfer.WrapError("create room", transfer.ErrSignalingError, errMsg)
	}
}

func waitForPeer(ctx *ConnectionContext) (*signaling.PeerInfo, error) {
	fmt.Println()
	stopSpinner := ui.RunWaitingSpinner("Waiting for receiver to join...")
	defer stopSpinner()

	select {
	case peerInfo := <-ctx.Handler.PeerJoined:
		return peerInfo, nil
	case errMsg := <-ctx.Handler.Error:
		return nil, transfer.WrapError("wait for peer", transfer.ErrSignalingError, errMsg)
	}
}

func prepareFileData(fileInfos []files.FileInfo) []*files.FileInfo {
	ptrs := make([]*files.FileInfo, len(fileInfos))

	for i := range fileInfos {
		ptrs[i] = &fileInfos[i]
	}

	return ptrs
}

func init() {
	rootCmd.AddCommand(sendCmd)

	sendCmd.Flags().StringVarP(&flagDomain, "domain", "d", "", "Custom domain")
	sendCmd.Flags().StringVarP(&flagSTUN, "stun", "s", "", "Custom STUN server")
	sendCmd.Flags().StringVarP(&flagTURN, "turn", "t", "", "Custom TURN server")
	sendCmd.Flags().StringVarP(&flagTURNUser, "turn-user", "u", "", "TURN username")
	sendCmd.Flags().StringVarP(&flagTURNPass, "turn-pass", "p", "", "TURN password")
	sendCmd.Flags().BoolVarP(&flagRelay, "relay", "r", false, "Force relay mode")
}
