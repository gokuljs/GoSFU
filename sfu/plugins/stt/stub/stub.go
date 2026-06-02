// Package stub is a no-API STT engine for development. It "transcribes"
// by emitting a fixed final result after it has seen enough speech frames.
package stub

import (
	"context"

	"github.com/gokuljs/goSfu/pkg/agent/audio"
	"github.com/gokuljs/goSfu/plugins/stt"
)

func init() {
	stt.Register("stub", func() (stt.Provider, error) { return New(), nil })
}

type Provider struct{}

func New() *Provider { return &Provider{} }

func (p *Provider) Name() string    { return "stub" }
func (p *Provider) SampleRate() int { return audio.SttSampleRate } // 16000

func (p *Provider) NewSession(ctx context.Context) (stt.Session, error) {
	s := &session{results: make(chan stt.Result, 4)}
	return s, nil
}

type session struct {
	results chan stt.Result
	frames  int
	emitted bool
}

// SendFrame counts frames; after ~1s of audio it emits one final transcript.
func (s *session) SendFrame(ctx context.Context, frame audio.Frame) error {
	s.frames++
	if !s.emitted && s.frames >= 50 { // 50 * 20ms = 1s
		s.emitted = true
		select {
		case s.results <- stt.Result{Text: "hello from stub stt", IsFinal: true, Confidence: 1}:
		case <-ctx.Done():
		}
	}
	return nil
}

func (s *session) Results() <-chan stt.Result { return s.results }

func (s *session) Close() error {
	close(s.results)
	return nil
}
