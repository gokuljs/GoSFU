package llm

import "fmt"

// Options are non-secret LLM settings passed in at build time. APIKey may be
// empty; the provider factory may fall back to an environment variable.
//
// Extra holds provider-specific fields not yet modeled as typed options (e.g.
// max_tokens, top_p). Each provider decides how to map them to its API.
type Options struct {
	APIKey      string
	Model       string
	BaseURL     string
	Temperature *float64
	Extra       map[string]any
}

type Factory func(Options) (Provider, error)

var registry = map[string]Factory{}

func Register(name string, f Factory) { registry[name] = f }

func Build(name string, opts Options) (Provider, error) {
	f, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("llm: unknown provider %q", name)
	}
	return f(opts)
}
