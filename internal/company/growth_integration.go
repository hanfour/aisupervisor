package company

import (
	"github.com/hanfourmini/aisupervisor/internal/feature"
	"github.com/hanfourmini/aisupervisor/internal/growth"
)

// initGrowth initializes the growth engine and subscribes to company events.
func (m *Manager) initGrowth(features *feature.Manager) {
	if m.growthEngine == nil {
		m.growthEngine = growth.NewEngine()
	}
	m.featureManager = features

	// Load existing skill trees from workers
	m.mu.RLock()
	for id, w := range m.workers {
		if w.SkillTree != nil {
			m.growthEngine.SetSkillTree(id, w.SkillTree)
		}
	}
	m.mu.RUnlock()
}

// processGrowthEvent handles a company event and feeds it to the growth engine.
func (m *Manager) processGrowthEvent(e Event) {
	if m.featureManager == nil || !m.featureManager.IsEnabled("growth_engine") {
		return
	}

	switch e.Type {
	case EventTaskCompleted:
		m.handleTaskCompletedGrowth(e)
	}
}

func (m *Manager) handleTaskCompletedGrowth(e Event) {
	if e.WorkerID == "" {
		return
	}

	m.mu.RLock()
	w, ok := m.workers[e.WorkerID]
	m.mu.RUnlock()
	if !ok || w.SkillTree == nil {
		return
	}

	task, found := m.projectStore.GetTask(e.TaskID)
	if !found {
		return
	}

	info := growth.TaskCompletedInfo{
		TaskType:               string(task.Type),
		Difficulty:             1.0,
		ReviewPassedFirstTime:  task.RejectionCount == 0,
		ReviewAttempts:         task.ReviewCount,
		ConsecutiveCompletions: m.completedTaskCount,
	}

	events := m.growthEngine.ProcessTaskCompleted(e.WorkerID, info)

	// Update worker's skill tree reference
	m.mu.Lock()
	w.SkillTree = m.growthEngine.GetSkillTree(e.WorkerID)
	m.mu.Unlock()
	_ = m.saveWorkers()

	// Emit growth events to frontend
	for _, ge := range events {
		m.emit(Event{
			Type:     EventType("growth_" + string(ge.Type)),
			WorkerID: ge.WorkerID,
			Message:  ge.Message,
		})
	}
}
