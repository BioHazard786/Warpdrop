package transfer

import (
	"errors"
	"fmt"

	"github.com/BioHazard786/Warpdrop/cli/internal/ui"
)

var (
	ErrPeerDisconnected  = errors.New("peer disconnected")
	ErrSignalingError    = errors.New("signaling server error")
	ErrTimeout           = errors.New("timeout")
	ErrChannelClosed     = errors.New("channel closed")
	ErrChannelNotOpen    = errors.New("channel not open")
	ErrTransferDeclined  = errors.New("receiver declined the transfer")
	ErrTransferCancelled = errors.New("transfer cancelled by user")
	ErrBufferTimeout     = errors.New("buffer drain timeout")
	ErrInvalidFile       = errors.New("invalid file")
	ErrFilenameMismatch  = errors.New("filename mismatch")
	ErrUnexpectedSignal  = errors.New("unexpected signal type")
	ErrMetadataFailed    = errors.New("failed to process metadata")
	ErrConnectionFailed  = errors.New("connection failed")
	ErrChannelsNotReady  = errors.New("channels not ready")
)

type TransferError struct {
	Op      string
	File    string
	Err     error
	Details string
}

func (e *TransferError) Error() string {
	if e.File != "" {
		return fmt.Sprintf("%s %s: %v", e.Op, e.File, e.Err)
	}
	if e.Details != "" {
		return fmt.Sprintf("%s: %v (%s)", e.Op, e.Err, e.Details)
	}
	return fmt.Sprintf("%s: %v", e.Op, e.Err)
}

func (e *TransferError) Unwrap() error {
	return e.Err
}

func (e *TransferError) Print() {
	ui.PrintError(e.Error())
}

func NewError(op string, err error) *TransferError {
	return &TransferError{Op: op, Err: err}
}

func NewFileError(op, file string, err error) *TransferError {
	return &TransferError{Op: op, File: file, Err: err}
}

func WrapError(op string, err error, details string) *TransferError {
	return &TransferError{Op: op, Err: err, Details: details}
}

func PrintErr(err error) {
	ui.PrintError(err.Error())
}
