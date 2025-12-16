package cmd

import (
	"fmt"

	"github.com/BioHazard786/Warpdrop/cli/internal/config"
	"github.com/BioHazard786/Warpdrop/cli/internal/files"
	"github.com/BioHazard786/Warpdrop/cli/internal/signaling"
	"github.com/BioHazard786/Warpdrop/cli/internal/transfer"
	"github.com/BioHazard786/Warpdrop/cli/internal/webrtc"
	"github.com/BioHazard786/Warpdrop/cli/internal/webrtc/multichannel"
	"github.com/BioHazard786/Warpdrop/cli/internal/webrtc/singlechannel"
)

type SenderSession interface {
	SetProgressUI()
	SetOptions(opts *transfer.TransferOptions)
	Start() error
	Transfer() error
	Close() error
}

type ReceiverSession interface {
	SetProgressUI()
	SetOptions(opts *transfer.TransferOptions)
	Start() error
	Transfer() error
	Close() error
}

type ConnectionContext struct {
	Client   *signaling.Client
	Handler  *signaling.Handler
	Config   *config.Config
	PeerInfo *signaling.PeerInfo
}

func NewConnectionContext(cfg *config.Config) (*ConnectionContext, error) {
	client := signaling.NewClient(cfg.WebSocketURL)
	if err := client.Connect(); err != nil {
		return nil, transfer.NewError("connect to server", err)
	}

	handler := signaling.NewHandler(client)
	go handler.Start()

	return &ConnectionContext{
		Client:  client,
		Handler: handler,
		Config:  cfg,
	}, nil
}

func (c *ConnectionContext) Close() {
	if c.Handler != nil {
		c.Handler.Close()
	}
	if c.Client != nil {
		c.Client.Close()
	}
}

func LoadConfig(opts config.Options) (*config.Config, error) {
	cfg, err := config.Load(opts)
	if err != nil {
		return nil, transfer.NewError("load config", err)
	}

	if cfg.ForceRelay && cfg.GetTURNServers() == nil {
		return nil, fmt.Errorf("cannot force relay mode without TURN server configured")
	}

	return cfg, nil
}

func CreateSenderSession(ctx *ConnectionContext, fileInfos []*files.FileInfo) (SenderSession, error) {
	protocol := webrtc.SelectProtocol(ctx.PeerInfo.ClientType)

	switch protocol {
	case webrtc.MultiChannelProtocol:
		return multichannel.NewSenderSession(ctx.Client, ctx.Handler, ctx.Config, fileInfos, ctx.PeerInfo)
	case webrtc.SingleChannelProtocol:
		return singlechannel.NewSenderSession(ctx.Client, ctx.Handler, ctx.Config, fileInfos, ctx.PeerInfo)
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", protocol)
	}
}

func CreateReceiverSession(ctx *ConnectionContext) (ReceiverSession, error) {
	protocol := webrtc.SelectProtocol(ctx.PeerInfo.ClientType)

	switch protocol {
	case webrtc.MultiChannelProtocol:
		return multichannel.NewReceiverSession(ctx.Client, ctx.Handler, ctx.Config, ctx.PeerInfo)
	case webrtc.SingleChannelProtocol:
		return singlechannel.NewReceiverSession(ctx.Client, ctx.Handler, ctx.Config, ctx.PeerInfo)
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", protocol)
	}
}

func RunSenderSession(session SenderSession, opts *transfer.TransferOptions) error {
	defer session.Close()

	session.SetProgressUI()
	if opts != nil {
		session.SetOptions(opts)
	}

	if err := session.Start(); err != nil {
		return transfer.NewError("start connection", err)
	}

	if err := session.Transfer(); err != nil {
		return transfer.NewError("transfer files", err)
	}

	return nil
}

func RunReceiverSession(session ReceiverSession, opts *transfer.TransferOptions) error {
	defer session.Close()

	if err := session.Start(); err != nil {
		return transfer.NewError("start connection", err)
	}

	session.SetProgressUI()
	if opts != nil {
		session.SetOptions(opts)
	}

	if err := session.Transfer(); err != nil {
		return transfer.NewError("receive files", err)
	}

	return nil
}
