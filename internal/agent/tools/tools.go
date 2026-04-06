package tools

import (
	"context"
	"encoding/json"
)

// Tool defines the interface for an agent tool that can be invoked by the AI.
type Tool interface {
	Name() string
	Description() string
	Parameters() json.RawMessage
	Execute(ctx context.Context, args json.RawMessage) (string, error)
}

// Registry holds registered tools and supports allow-list filtering.
type Registry struct {
	tools   map[string]Tool
	allowed map[string]bool
}

// NewRegistry creates a new empty tool registry.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register adds a tool to the registry.
func (r *Registry) Register(t Tool) {
	r.tools[t.Name()] = t
}

// Get returns a tool by name, respecting the allow-list if set.
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	if !ok {
		return nil, false
	}
	if r.allowed != nil && !r.allowed[name] {
		return nil, false
	}
	return t, true
}

// List returns all tools in the registry, respecting the allow-list if set.
func (r *Registry) List() []Tool {
	var result []Tool
	for _, t := range r.tools {
		if r.allowed != nil && !r.allowed[t.Name()] {
			continue
		}
		result = append(result, t)
	}
	return result
}

// SetAllowed restricts which tools can be retrieved. Pass nil or empty to clear.
func (r *Registry) SetAllowed(names []string) {
	if len(names) == 0 {
		r.allowed = nil
		return
	}
	r.allowed = make(map[string]bool, len(names))
	for _, n := range names {
		r.allowed[n] = true
	}
}
