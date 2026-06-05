package stt

import "fmt"

// Options are provider settings passed in at build time. APIKey may be empty;
// the provider factory may fall back to an environment variable.
type Options struct {
	APIKey string
	Model  string
	Extra  map[string]any
}

// Factory builds a Provider from explicit options.
type Factory func(Options) (Provider, error)

var registry = map[string]Factory{}

// Register is called from a provider package's init(). The driver pattern
// (like database/sql): importing the package registers the name.
func Register(name string, f Factory) {
	registry[name] = f
}

func Build(name string, opts Options) (Provider, error) {
	f, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("stt: unknown provider %q (did you blank-import it?)", name)
	}
	return f(opts)
}
