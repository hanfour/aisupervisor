package feature

import (
	"os"
	"strings"
	"sync"
)

type Manager struct {
	flags map[string]*FeatureFlag
	mu    sync.RWMutex
}

func NewManager() *Manager {
	fm := &Manager{flags: make(map[string]*FeatureFlag)}
	for _, f := range DefaultFlags {
		cp := f
		fm.flags[f.ID] = &cp
	}
	fm.overrideFromEnv()
	return fm
}

func (fm *Manager) IsEnabled(id string) bool {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	f, ok := fm.flags[id]
	if !ok {
		return false
	}
	return f.Enabled
}

func (fm *Manager) Toggle(id string, enabled bool) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	if f, ok := fm.flags[id]; ok {
		f.Enabled = enabled
	}
}

func (fm *Manager) List() []FeatureFlag {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	result := make([]FeatureFlag, 0, len(fm.flags))
	for _, f := range fm.flags {
		result = append(result, *f)
	}
	return result
}

func (fm *Manager) overrideFromEnv() {
	for id, f := range fm.flags {
		envKey := "AISUPERVISOR_FF_" + strings.ToUpper(strings.ReplaceAll(id, " ", "_"))
		if val := os.Getenv(envKey); val != "" {
			f.Enabled = val == "true" || val == "1"
		}
		if f.EnvVar != "" {
			if val := os.Getenv(f.EnvVar); val != "" {
				f.Enabled = val == "true" || val == "1"
			}
		}
	}
}
