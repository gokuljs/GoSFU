package llm

import "fmt"

type Factory func() (Provider, error)

var registry = map[string]Factory{}

func Register(name string, f Factory) { registry[name] = f }

func Build(name string) (Provider, error) {
	f, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("llm: unknown provider %q", name)
	}
	return f()
}
