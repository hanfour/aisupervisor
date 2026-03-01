package role

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// Manager manages multiple roles and coordinates their evaluation.
type Manager struct {
	mu    sync.RWMutex
	roles []Role
}

// NewManager creates a role manager with the given roles.
func NewManager(roles ...Role) *Manager {
	return &Manager{roles: roles}
}

// Add adds a role to the manager.
func (m *Manager) Add(r Role) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.roles = append(m.roles, r)
}

// Remove removes a role by ID.
func (m *Manager) Remove(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, r := range m.roles {
		if r.ID() == id {
			m.roles = append(m.roles[:i], m.roles[i+1:]...)
			return
		}
	}
}

// List returns all roles.
func (m *Manager) List() []Role {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]Role, len(m.roles))
	copy(result, m.roles)
	return result
}

// Get returns a role by ID.
func (m *Manager) Get(id string) (Role, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, r := range m.roles {
		if r.ID() == id {
			return r, true
		}
	}
	return nil, false
}

// EvaluateReactive runs all reactive/hybrid roles on the observation.
// Returns the intervention from the highest-priority role that wants to act.
func (m *Manager) EvaluateReactive(ctx context.Context, obs Observation) (*Intervention, error) {
	m.mu.RLock()
	candidates := make([]Role, 0)
	for _, r := range m.roles {
		mode := r.Mode()
		if (mode == ModeReactive || mode == ModeHybrid) && r.ShouldEvaluate(obs) {
			candidates = append(candidates, r)
		}
	}
	m.mu.RUnlock()

	if len(candidates) == 0 {
		return nil, nil
	}

	// Sort by priority descending (higher priority = more important)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Priority() > candidates[j].Priority()
	})

	// Evaluate the highest-priority role
	for _, r := range candidates {
		intervention, err := r.Evaluate(ctx, obs)
		if err != nil {
			return nil, fmt.Errorf("role %s: %w", r.ID(), err)
		}
		if intervention != nil && intervention.Type != InterventionNone {
			return intervention, nil
		}
	}

	return nil, nil
}

// EvaluateReactiveFiltered runs only the specified roles on the observation.
// Returns the intervention from the highest-priority role that wants to act.
func (m *Manager) EvaluateReactiveFiltered(ctx context.Context, obs Observation, roles []Role) (*Intervention, error) {
	candidates := make([]Role, 0)
	for _, r := range roles {
		mode := r.Mode()
		if (mode == ModeReactive || mode == ModeHybrid) && r.ShouldEvaluate(obs) {
			candidates = append(candidates, r)
		}
	}

	if len(candidates) == 0 {
		return nil, nil
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Priority() > candidates[j].Priority()
	})

	for _, r := range candidates {
		intervention, err := r.Evaluate(ctx, obs)
		if err != nil {
			return nil, fmt.Errorf("role %s: %w", r.ID(), err)
		}
		if intervention != nil && intervention.Type != InterventionNone {
			return intervention, nil
		}
	}

	return nil, nil
}

// EvaluateProactive runs all proactive/hybrid roles on the observation.
// Returns all interventions from roles that want to act.
func (m *Manager) EvaluateProactive(ctx context.Context, obs Observation) ([]*Intervention, error) {
	m.mu.RLock()
	candidates := make([]Role, 0)
	for _, r := range m.roles {
		mode := r.Mode()
		if (mode == ModeProactive || mode == ModeHybrid) && r.ShouldEvaluate(obs) {
			candidates = append(candidates, r)
		}
	}
	m.mu.RUnlock()

	var interventions []*Intervention
	for _, r := range candidates {
		intervention, err := r.Evaluate(ctx, obs)
		if err != nil {
			return nil, fmt.Errorf("role %s: %w", r.ID(), err)
		}
		if intervention != nil && intervention.Type != InterventionNone {
			interventions = append(interventions, intervention)
		}
	}

	// Sort by priority descending
	sort.Slice(interventions, func(i, j int) bool {
		return interventions[i].Priority > interventions[j].Priority
	})

	return interventions, nil
}
