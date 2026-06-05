package vad

import "fmt"

// Options are provider settings passed in at build time.
type Options struct {
	Extra map[string]any
}

type Factory func(Options) (Provider, error)

var registry = map[string]Factory{}

func Register(name string, f Factory) { registry[name] = f }
func Build(name string, opts Options) (Provider, error) {
	f, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("vad: unknown provider %q", name)
	}
	return f(opts)
}
