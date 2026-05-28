package audio

import (
	"context"
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

	flush := func() {
		// SampleBuffer.Push with empty input returns nothing;
		// we manually emit from acc.pending via Push on next chunk.
	}

	for {
		select {
		case <-ctx.Done():
			return

		case frame, ok := <-p.in:
			if !ok {
				return
			}
			if frame.SampleRate == SttSampleRate {
				frame = Upsample16kTo48k(frame)
			}
			for _, f := range acc.Push(frame.Samples) {
				select {
				case p.out <- f:
				case <-ctx.Done():
					return
				}
			}
			_ = flush

		case <-ticker.C:
			// Underrun: emit silence so browser clock stays smooth
			if len(acc.pending) >= SamplesPerFrame48k {
				for _, f := range acc.Push(nil) {
					select {
					case p.out <- f:
					case <-ctx.Done():
						return
					}
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
