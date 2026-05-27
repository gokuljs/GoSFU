package agent

import (
	"context"
	"log/slog"
	"sync"

	"github.com/gokuljs/goSfu/pkg/agent/audio"
	"github.com/pion/webrtc/v4"
)

type Agent struct {
	ctx       context.Context
	track     *webrtc.TrackLocalStaticSample
	audioPath string
	stopOnce  sync.Once
	stop      chan struct{}
}

func New(ctx context.Context, pc *webrtc.PeerConnection, audioPath string) (*Agent, error) {
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

	// Must drain RTCP or connection degrades over time
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

	return &Agent{
		ctx:       ctx,
		track:     track,
		audioPath: audioPath,
		stop:      make(chan struct{}),
	}, nil
}

func (a *Agent) Start() {
	slog.Info("agent audio starting", "path", a.audioPath)
	go func() {
		if err := audio.PlayOGG(a.ctx, a.audioPath, a.track, a.stop); err != nil {
			slog.Error("agent audio stopped", "error", err, "path", a.audioPath)
		}
	}()
}

func (a *Agent) Stop() {
	a.stopOnce.Do(func() { close(a.stop) })
}
