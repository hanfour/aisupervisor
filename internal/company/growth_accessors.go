package company

import (
	"github.com/hanfourmini/aisupervisor/internal/feature"
	"github.com/hanfourmini/aisupervisor/internal/growth"
)

// GetGrowthEngine returns the growth engine.
func (m *Manager) GetGrowthEngine() *growth.Engine {
	if m.growthEngine == nil {
		return nil
	}
	if ge, ok := m.growthEngine.(*growth.Engine); ok {
		return ge
	}
	return nil
}

// SetGrowthEngine sets the growth engine.
func (m *Manager) SetGrowthEngine(ge *growth.Engine) {
	m.growthEngine = ge
}

// GetFeatureManager returns the feature manager.
func (m *Manager) GetFeatureManager() *feature.Manager {
	if m.featureManager == nil {
		return nil
	}
	if fm, ok := m.featureManager.(*feature.Manager); ok {
		return fm
	}
	return nil
}

// SetFeatureManager sets the feature manager.
func (m *Manager) SetFeatureManager(fm *feature.Manager) {
	m.featureManager = fm
}
