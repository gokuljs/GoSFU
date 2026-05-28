package agent

import (
	"context"
	"log/slog"

	"github.com/gokuljs/goSfu/pkg/agent/audio"
	"github.com/pion/webrtc/v4"
)

type Agent struct {
	ctx    context.Context
	cancel context.CancelFunc
	stop   chan struct{}

	track    *webrtc.TrackLocalStaticSample
	inbound  *audio.Inbound
	outbound *audio.Outbound

	playbackIn chan audio.Frame // PCM sources write here (OGG now, TTS later)
	oggPath    string           // path to sample.ogg for Phase 1 outbound sourc
}

func New(ctx context.Context, pc *webrtc.PeerConnection, oggPath string) (*Agent, error) {
	ctx, cancel := context.WithCancel(ctx)

	track, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus},
		"audio", "agent",
	)
	if err != nil {
		cancel()
		return nil, err
	}

	sender, err := pc.AddTrack(track)
	if err != nil {
		cancel()
		return nil, err
	}

	// Must drain RTCP or connection degrades
	go func() {
		buf := make([]byte, 1500)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			if _, _, err := sender.Read(buf); err != nil {
				return
			}
		}
	}()

	inbound, err := audio.NewInbound()
	if err != nil {
		cancel()
		return nil, err
	}

	outbound, err := audio.NewOutbound(track)
	if err != nil {
		cancel()
		return nil, err
	}

	playbackIn := make(chan audio.Frame, 50)

	a := &Agent{
		ctx:        ctx,
		cancel:     cancel,
		stop:       make(chan struct{}),
		track:      track,
		inbound:    inbound,
		outbound:   outbound,
		playbackIn: playbackIn,
		oggPath:    oggPath,
	}

	// stash ogg path on agent for Start — or pass via struct field
	a.oggPath = oggPath
	return a, nil
}

// add field to struct:
// oggPath string

func (a *Agent) HandleInboundTrack(track *webrtc.TrackRemote) {
	go a.inbound.Run(a.ctx, track)
}

func (a *Agent) Start() {
	slog.Info("agent starting audio pipeline")

	pacer := audio.NewFramePacer(a.playbackIn, 10)
	go pacer.Run(a.ctx)
	go a.outbound.Run(a.ctx, pacer.Out())

	// OUTBOUND SOURCE: OGG file → PCM → playbackIn
	go func() {
		src, err := audio.NewOGGSource(a.oggPath)
		if err != nil {
			slog.Error("ogg source init failed", "error", err)
			return
		}
		if err := src.Run(a.ctx, a.playbackIn); err != nil {
			slog.Error("ogg source stopped", "error", err)
		}
	}()
}

func (a *Agent) Stop() {
	a.cancel()
	close(a.stop)
}
