package audio

import (
	"context"
	"log/slog"
	"time"
)

// FramePacer turns bursty PCM into a steady 20ms clock for WebRTC.
type FramePacer struct {
	in  <-chan Frame
	out chan Frame
}

func NewFramePacer(in <-chan Frame, buf int) *FramePacer {
	return &FramePacer{
		in:  in,
		out: make(chan Frame, buf),
	}
}

func (p *FramePacer) Out() <-chan Frame { return p.out }

func (p *FramePacer) Run(ctx context.Context) {
	ticker := time.NewTicker(FrameDuration)
	defer ticker.Stop()

	acc := NewSampleBuffer(WebrtcSampleRate)

	for {
		select {
		case <-ctx.Done():
			return

		case frame, ok := <-p.in:
			if !ok {
				return
			}
			if frame.SampleRate != WebrtcSampleRate && frame.SampleRate != 0 {
				slog.Warn("pacer got non-48k frame; upstream should have resampled", "rate", frame.SampleRate)
				continue
			}
			acc.pending = append(acc.pending, frame.Samples...)

		case <-ticker.C:
			if len(acc.pending) >= SamplesPerFrame48k {
				f := Frame{
					Samples:    make([]int16, SamplesPerFrame48k),
					SampleRate: WebrtcSampleRate,
				}
				copy(f.Samples, acc.pending[:SamplesPerFrame48k])
				acc.pending = acc.pending[SamplesPerFrame48k:]
				select {
				case p.out <- f:
				case <-ctx.Done():
					return
				}
			} else {
				select {
				case p.out <- NewSilentFrame48k():
				case <-ctx.Done():
					return
				}
			}
		}
	}
}
