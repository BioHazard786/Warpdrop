package transfer

import (
	"strings"

	"github.com/BioHazard786/Warpdrop/cli/internal/version"
	"github.com/BioHazard786/Warpdrop/cli/internal/webrtc"
	pion "github.com/pion/webrtc/v4"
	"github.com/vmihailenco/msgpack/v5"
)

func SendMessage(dc *pion.DataChannel, msg webrtc.Message) error {
	if dc == nil {
		return ErrChannelNotOpen
	}
	data, err := msgpack.Marshal(msg)
	if err != nil {
		return NewError("marshal message", err)
	}
	return dc.Send(data)
}

func SendTypedMessage(dc *pion.DataChannel, msgType string, payload any) error {
	if dc == nil {
		return ErrChannelNotOpen
	}
	msg, err := webrtc.NewMessage(msgType, payload)
	if err != nil {
		return NewError("create message", err)
	}
	return SendMessage(dc, msg)
}

func SendDeviceInfo(dc *pion.DataChannel) error {
	return SendTypedMessage(dc, MessageTypeDeviceInfo, webrtc.DeviceInfoPayload{
		DeviceName:    "CLI",
		DeviceVersion: strings.TrimPrefix(version.Version, "v"),
	})
}

func SendReadyToReceive(dc *pion.DataChannel, fileName string, offset uint64) error {
	return SendTypedMessage(dc, MessageTypeReadyToReceive, webrtc.ReadyToReceivePayload{
		FileName: fileName,
		Offset:   offset,
	})
}

func SendSimpleMessage(dc *pion.DataChannel, msgType string) error {
	return SendMessage(dc, webrtc.Message{Type: msgType})
}

func SendFilesMetadata(dc *pion.DataChannel, metadata []webrtc.FileMetadata) error {
	return SendTypedMessage(dc, MessageTypeFilesMetadata, metadata)
}

func ParseMessage(data []byte) (*webrtc.Message, error) {
	var msg webrtc.Message
	if err := msgpack.Unmarshal(data, &msg); err != nil {
		return nil, NewError("parse message", err)
	}
	return &msg, nil
}
