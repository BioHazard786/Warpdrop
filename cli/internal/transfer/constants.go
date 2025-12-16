package transfer

import (
	"github.com/BioHazard786/Warpdrop/cli/internal/utils"
)

const (
	MessageTypeFilesMetadata   = "files_metadata"
	MessageTypeDeviceInfo      = "device_info"
	MessageTypeReadyToReceive  = "ready_to_receive"
	MessageTypeChunk           = "chunk"
	MessageTypeDownloadingDone = "downloading_done"
	MessageTypeDeclineReceive  = "decline_receive"
)

var (
	HighWaterMark = utils.HighWaterMark
	LowWaterMark  = utils.LowWaterMark
	SendTimeout   = utils.SendTimeout
	DrainTimeout  = utils.DrainTimeout
	SignalTimeout = utils.SignalTimeout
)

type TransferOptions struct {
	OutputDir string
	ZipMode   bool
}
