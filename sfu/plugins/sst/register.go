package stt

import "fmt"

// Factory builds a Provider. It reads its own env (API keys etc.).
type Factory func() (Provider, error)

var registry = map[string]Factory{}

// Register is called from a provider package's init(). The driver pattern
// (like database/sql): importing the package registers the name.
func Register(name string, f Factory) {
	registry[name] = f
}

func Build(name string) (Provider, error) {
	f, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("stt: unknown provider %q (did you blank-import it?)", name)
	}
	return f()
}
