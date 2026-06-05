// Package stub is an energy-based VAD: above an RMS threshold = speech.
// It emits SpeechStart on the rising edge and SpeechEnd after enough silence.
package stub

import (
	"context"

	"github.com/gokuljs/goSfu/pkg/agent/audio"
	"github.com/gokuljs/goSfu/plugins/vad"
)

func init() {
	vad.Register("stub", func(_ vad.Options) (vad.Provider, error) { return New(), nil })
}

type Provider struct {
	threshold     float64
	silenceFrames int // consecutive silent frames before SpeechEnd
	speaking      bool
	silentCount   int
}

func New() *Provider {
	return &Provider{
		threshold:     500, // RMS; tune to your mic
		silenceFrames: 40,  // 40 * 20ms = 800ms of silence = end of turn
	}
}

func (p *Provider) Name() string    { return "stub" }
func (p *Provider) SampleRate() int { return audio.SttSampleRate }

func (p *Provider) Analyze(ctx context.Context, frame audio.Frame) ([]vad.Event, error) {
	loud := audio.RMS(frame.Samples) >= p.threshold
	var events []vad.Event

	switch {
	case loud && !p.speaking:
		p.speaking = true
		p.silentCount = 0
		events = append(events, vad.Event{Type: vad.SpeechStart})
	case loud && p.speaking:
		p.silentCount = 0
	case !loud && p.speaking:
		p.silentCount++
		if p.silentCount >= p.silenceFrames {
			p.speaking = false
			events = append(events, vad.Event{Type: vad.SpeechEnd})
		}
	}
	return events, nil
}

func (p *Provider) Reset()       { p.speaking = false; p.silentCount = 0 }
func (p *Provider) Close() error { return nil }
