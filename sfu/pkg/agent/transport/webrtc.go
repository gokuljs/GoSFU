package transport

import (
	"context"

	"github.com/gokuljs/goSfu/pkg/agent/audio"
	"github.com/pion/webrtc/v4"
)

type WebRTC struct {
	pc       *webrtc.PeerConnection
	track    *webrtc.TrackLocalStaticSample
	sender   *webrtc.RTPSender
	inbound  *audio.Inbound
	outbound *audio.Outbound
	pacer    *audio.FramePacer
	playback chan audio.Frame
}

func NewWebrtc(pc *webrtc.PeerConnection) (*WebRTC, error) {
	track, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus},
		"audio", "agent",
	)
	if err != nil {
		return nil, err
	}
	sender, err := pc.AddTrack(track)
	if err != nil {
		return nil, err
	}
	inbound, err := audio.NewInbound()
	if err != nil {
		return nil, err
	}
	outbound, err := audio.NewOutbound(track)
	if err != nil {
		return nil, err
	}
	playback := make(chan audio.Frame, 50)
	return &WebRTC{
		pc:       pc,
		track:    track,
		sender:   sender,
		inbound:  inbound,
		outbound: outbound,
		pacer:    audio.NewFramePacer(playback, 10),
		playback: playback,
	}, nil
}

func (t *WebRTC) HandleRemoteTrack(ctx context.Context, track *webrtc.TrackRemote) {
	go t.inbound.Run(ctx, track)
}
func (t *WebRTC) Inbound() <-chan audio.Frame { return t.inbound.Frames() }
func (t *WebRTC) Send(frame audio.Frame) error {
	select {
	case t.playback <- frame:
	default: // playout buffer full — drop to avoid backpressure on caller
	}
	return nil
}
func (t *WebRTC) Start(ctx context.Context) error {
	go t.drainRTCP(ctx)
	go t.pacer.Run(ctx)
	go t.outbound.Run(ctx, t.pacer.Out())
	return nil
}
func (t *WebRTC) drainRTCP(ctx context.Context) {
	buf := make([]byte, 1500)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if _, _, err := t.sender.Read(buf); err != nil {
			return
		}
	}
}
func (t *WebRTC) Close() error { return nil }
