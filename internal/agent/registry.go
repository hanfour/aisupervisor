package agent

import "sync"

// RuntimeRegistry holds the set of AgentRuntime implementations known to the
// application. Registration order is preserved so callers can rely on
// deterministic listing and a stable "default" backend.
//
// All methods are safe for concurrent use.
type RuntimeRegistry struct {
	mu       sync.RWMutex
	runtimes map[string]AgentRuntime
	order    []string
}

// NewRuntimeRegistry returns an empty RuntimeRegistry ready for use.
func NewRuntimeRegistry() *RuntimeRegistry {
	return &RuntimeRegistry{
		runtimes: make(map[string]AgentRuntime),
		order:    nil,
	}
}

// Register adds rt to the registry keyed by rt.Name(). If a runtime with the
// same name is already registered, the stored instance is overwritten but the
// original insertion position is preserved (the name is not moved to the end
// of the list and no duplicate entry is created).
func (r *RuntimeRegistry) Register(rt AgentRuntime) {
	name := rt.Name()

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.runtimes[name]; !exists {
		r.order = append(r.order, name)
	}
	r.runtimes[name] = rt
}

// Get returns the runtime registered under name. The second return value is
// false when no such runtime exists.
func (r *RuntimeRegistry) Get(name string) (AgentRuntime, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rt, ok := r.runtimes[name]
	return rt, ok
}

// List returns the names of all registered runtimes in the order they were
// first registered. The returned slice is a copy and safe to mutate.
func (r *RuntimeRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]string, len(r.order))
	copy(out, r.order)
	return out
}

// Default returns the first registered runtime, or nil if the registry is
// empty.
func (r *RuntimeRegistry) Default() AgentRuntime {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.order) == 0 {
		return nil
	}
	return r.runtimes[r.order[0]]
}
