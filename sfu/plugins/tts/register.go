package tts

import "fmt"

// Options are provider settings passed in at build time. APIKey may be empty;
// the provider factory may fall back to an environment variable.
type Options struct {
	APIKey string
	Voice  string
	Extra  map[string]any
}

type Factory func(Options) (Provider, error)

var registry = map[string]Factory{}

func Register(name string, f Factory) { registry[name] = f }

func Build(name string, opts Options) (Provider, error) {
	f, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("tts: unknown provider %q", name)
	}
	return f(opts)
}
