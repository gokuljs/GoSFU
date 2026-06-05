// Package stub streams a canned reply token-by-token to exercise the pipeline.
package stub

import (
	"context"
	"strings"
	"time"

	"github.com/gokuljs/goSfu/plugins/llm"
)

func init() {
	llm.Register("stub", func() (llm.Provider, error) { return New(), nil })
}

type Provider struct{}

func New() *Provider { return &Provider{} }

func (p *Provider) Name() string { return "stub" }

func (p *Provider) StreamCompletion(ctx context.Context, _ []llm.Message) (<-chan llm.Chunk, error) {
	out := make(chan llm.Chunk)
	go func() {
		defer close(out)
		reply := "Hello. I am a stub agent. The pipeline works end to end."
		for _, word := range strings.Fields(reply) {
			select {
			case <-ctx.Done():
				return
			case out <- llm.Chunk{Delta: word + " "}:
				time.Sleep(40 * time.Millisecond) // simulate token latency
			}
		}
		out <- llm.Chunk{Done: true}
	}()
	return out, nil
}
