package ai

import "fmt"

type Registry struct {
	backends map[string]Backend
}

func NewRegistry() *Registry {
	return &Registry{backends: make(map[string]Backend)}
}

func (r *Registry) Register(b Backend) {
	r.backends[b.Name()] = b
}

func (r *Registry) Get(name string) (Backend, error) {
	b, ok := r.backends[name]
	if !ok {
		return nil, fmt.Errorf("backend %q not found", name)
	}
	return b, nil
}

func (r *Registry) List() []string {
	names := make([]string, 0, len(r.backends))
	for name := range r.backends {
		names = append(names, name)
	}
	return names
}
