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

func NewWebrtc(pc *webrtc.PeerConnection, roomID string) (*WebRTC, error) {
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
	inbound, err := audio.NewInbound(roomID)
	if err != nil {
		return nil, err
	}
	outbound, err := audio.NewOutbound(track)
	if err != nil {
		return nil, err
	}
	// Small channel: the pacer's bounded jitter buffer is the real backpressure
	// ceiling, so Send blocks once the pacer is full instead of dropping audio.
	playback := make(chan audio.Frame, 8)
	return &WebRTC{
		pc:       pc,
		track:    track,
		sender:   sender,
		inbound:  inbound,
		outbound: outbound,
		pacer:    audio.NewFramePacer(playback, 8, roomID),
		playback: playback,
	}, nil
}

func (t *WebRTC) HandleRemoteTrack(ctx context.Context, track *webrtc.TrackRemote) {
	go t.inbound.Run(ctx, track)
}
func (t *WebRTC) Inbound() <-chan audio.Frame { return t.inbound.Frames() }
func (t *WebRTC) Send(ctx context.Context, frame audio.Frame) error {
	select {
	case t.playback <- frame:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (t *WebRTC) WaitForPlayout(ctx context.Context) error { return t.pacer.WaitForDrain(ctx) }
func (t *WebRTC) ClearPlayout()                            { t.pacer.Clear() }
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
