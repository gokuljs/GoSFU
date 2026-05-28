package audio

import (
	"context"

	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
)

type Outbound struct {
	enc   *OpusEncoder
	track *webrtc.TrackLocalStaticSample
}

func NewOutbound(track *webrtc.TrackLocalStaticSample) (*Outbound, error) {
	enc, err := NewOpusEncoder(WebrtcSampleRate, ChannelsMono)
	if err != nil {
		return nil, err
	}
	return &Outbound{enc: enc, track: track}, nil
}

// Run reads paced PCM, encodes Opus, writes to WebRTC track.
func (p *Outbound) Run(ctx context.Context, paced <-chan Frame) {
	for {
		select {
		case <-ctx.Done():
			return
		case frame, ok := <-paced:
			if !ok {
				return
			}
			opusPayload, err := p.enc.Encode(frame.Samples)
			if err != nil {
				continue
			}
			if err := p.track.WriteSample(media.Sample{
				Data:     opusPayload,
				Duration: FrameDuration,
			}); err != nil {
				return
			}
		}
	}
}
