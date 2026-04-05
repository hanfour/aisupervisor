// Package feature provides feature flag management.
package feature

// Flag represents a single feature flag.
type Flag struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

// Manager manages feature flags.
type Manager struct {
	flags map[string]*Flag
}

// NewManager creates a new feature manager with default flags.
func NewManager() *Manager {
	m := &Manager{
		flags: make(map[string]*Flag),
	}
	// Default flags
	m.flags["growth_system"] = &Flag{ID: "growth_system", Name: "Growth System", Enabled: true}
	m.flags["skill_radar"] = &Flag{ID: "skill_radar", Name: "Skill Radar Chart", Enabled: true}
	m.flags["boss_feedback"] = &Flag{ID: "boss_feedback", Name: "Boss Feedback", Enabled: true}
	m.flags["visual_evolution"] = &Flag{ID: "visual_evolution", Name: "Visual Evolution", Enabled: true}
	return m
}

// IsEnabled checks if a flag is enabled.
func (m *Manager) IsEnabled(id string) bool {
	if f, ok := m.flags[id]; ok {
		return f.Enabled
	}
	return false
}

// Toggle sets a flag's enabled state.
func (m *Manager) Toggle(id string, enabled bool) {
	if f, ok := m.flags[id]; ok {
		f.Enabled = enabled
	}
}

// List returns all flags.
func (m *Manager) List() []Flag {
	result := make([]Flag, 0, len(m.flags))
	for _, f := range m.flags {
		result = append(result, *f)
	}
	return result
}
