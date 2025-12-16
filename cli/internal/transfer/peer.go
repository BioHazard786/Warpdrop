package transfer

import (
	"encoding/json"
	"time"

	"github.com/BioHazard786/Warpdrop/cli/internal/config"
	"github.com/BioHazard786/Warpdrop/cli/internal/signaling"
	"github.com/BioHazard786/Warpdrop/cli/internal/utils"
	pion "github.com/pion/webrtc/v4"
)

func NewPeerConnection(cfg *config.Config) (*pion.PeerConnection, error) {
	iceServers := []pion.ICEServer{{URLs: cfg.GetSTUNServers()}}

	turnServers := cfg.GetTURNServers()
	if turnServers != nil {
		username, password := cfg.GetTURNCredentials()
		iceServers = append(iceServers, pion.ICEServer{
			URLs:       turnServers,
			Username:   username,
			Credential: password,
		})
	}

	policy := pion.ICETransportPolicyAll
	if turnServers != nil && (cfg.ForceRelay || utils.ShouldForceRelay()) {
		policy = pion.ICETransportPolicyRelay
	}

	pc, err := pion.NewPeerConnection(pion.Configuration{
		ICEServers:         iceServers,
		ICETransportPolicy: policy,
	})
	if err != nil {
		return nil, NewError("create peer connection", err)
	}
	return pc, nil
}

func SetupICEHandlers(pc *pion.PeerConnection, client *signaling.Client, done chan struct{}) {
	pc.OnICEConnectionStateChange(func(state pion.ICEConnectionState) {
		if state == pion.ICEConnectionStateFailed || state == pion.ICEConnectionStateClosed {
			select {
			case done <- struct{}{}:
			default:
			}
		}
	})

	pc.OnICECandidate(func(c *pion.ICECandidate) {
		if c == nil {
			return
		}
		client.SendMessage(&signaling.Message{
			Type:    signaling.MessageTypeSignal,
			Payload: signaling.SignalPayload{ICECandidate: c.ToJSON()},
		})
	})
}

func CreateDataChannel(pc *pion.PeerConnection, label string) (*pion.DataChannel, error) {
	ordered := true
	maxRetransmits := uint16(5000)

	dc, err := pc.CreateDataChannel(label, &pion.DataChannelInit{
		Ordered:           &ordered,
		MaxPacketLifeTime: &maxRetransmits,
	})
	if err != nil {
		return nil, NewError("create data channel", err)
	}
	return dc, nil
}

func CreateOffer(pc *pion.PeerConnection) (*pion.SessionDescription, error) {
	offer, err := pc.CreateOffer(nil)
	if err != nil {
		return nil, NewError("create offer", err)
	}

	if err = pc.SetLocalDescription(offer); err != nil {
		return nil, NewError("set local description", err)
	}

	return pc.LocalDescription(), nil
}

func CreateAnswer(pc *pion.PeerConnection, offer *pion.SessionDescription) (*pion.SessionDescription, error) {
	if err := pc.SetRemoteDescription(*offer); err != nil {
		return nil, NewError("set remote description", err)
	}

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		return nil, NewError("create answer", err)
	}

	if err = pc.SetLocalDescription(answer); err != nil {
		return nil, NewError("set local description", err)
	}

	return pc.LocalDescription(), nil
}

func HandleSDPSignal(pc *pion.PeerConnection, payload *signaling.SignalPayload) error {
	if payload.SDP == "" {
		return nil
	}

	var sdpType pion.SDPType
	switch payload.Type {
	case "offer":
		sdpType = pion.SDPTypeOffer
	case "answer":
		sdpType = pion.SDPTypeAnswer
	default:
		return WrapError("handle signal", ErrUnexpectedSignal, payload.Type)
	}

	desc := pion.SessionDescription{Type: sdpType, SDP: payload.SDP}
	if desc.Type == pion.SDPTypeAnswer {
		return pc.SetRemoteDescription(desc)
	}
	return nil
}

func HandleICECandidate(pc *pion.PeerConnection, payload *signaling.SignalPayload) error {
	if payload.ICECandidate == nil {
		return nil
	}

	candidateBytes, _ := json.Marshal(payload.ICECandidate)
	var ice pion.ICECandidateInit
	if err := json.Unmarshal(candidateBytes, &ice); err != nil {
		return NewError("parse ICE candidate", err)
	}
	if err := pc.AddICECandidate(ice); err != nil {
		return NewError("add ICE candidate", err)
	}
	return nil
}

func WaitForChannels(channelsReady *int32, expected int, peerLeft <-chan struct{}) error {
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-peerLeft:
			return ErrPeerDisconnected
		case <-timeout:
			return WrapError("wait channels", ErrTimeout, "channels not ready")
		case <-ticker.C:
			if int(*channelsReady) == expected {
				return nil
			}
		}
	}
}
