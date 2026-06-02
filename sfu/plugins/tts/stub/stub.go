// Package stub returns short silence so the outbound path can be exercised
// without a real TTS vendor. Native rate is 24000 to prove resampling works.
package stub

import (
	"context"

	"github.com/gokuljs/goSfu/plugins/tts"
)

func init() {
	tts.Register("stub", func() (tts.Provider, error) { return New(), nil })
}

type Provider struct{}

func New() *Provider { return &Provider{} }

func (p *Provider) Name() string    { return "stub" }
func (p *Provider) SampleRate() int { return 24000 }

func (p *Provider) Synthesize(ctx context.Context, text string) (<-chan tts.Chunk, error) {
	out := make(chan tts.Chunk, 1)
	go func() {
		defer close(out)
		// ~300ms of silence at 24kHz (0.3 * 24000 = 7200 samples)
		out <- tts.Chunk{Samples: make([]int16, 7200)}
		out <- tts.Chunk{Done: true}
	}()
	return out, nil
}
