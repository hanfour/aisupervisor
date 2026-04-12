package company

import (
	"github.com/hanfourmini/aisupervisor/internal/feature"
	"github.com/hanfourmini/aisupervisor/internal/growth"
)

// GetGrowthEngine returns the growth engine.
func (m *Manager) GetGrowthEngine() *growth.Engine {
	return m.growthEngine
}

// SetGrowthEngine sets the growth engine.
func (m *Manager) SetGrowthEngine(ge *growth.Engine) {
	m.growthEngine = ge
}

// GetFeatureManager returns the feature manager.
func (m *Manager) GetFeatureManager() *feature.Manager {
	return m.featureManager
}

// SetFeatureManager sets the feature manager.
func (m *Manager) SetFeatureManager(fm *feature.Manager) {
	m.featureManager = fm
}
