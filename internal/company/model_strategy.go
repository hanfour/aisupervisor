package company

import (
	"github.com/hanfourmini/aisupervisor/internal/project"
	"github.com/hanfourmini/aisupervisor/internal/worker"
)

// ModelStrategy resolves which AI model to use for a given worker + task combination.
// Priority: Worker.ModelVersion > TaskType override > SkillProfile > Tier default
type ModelStrategy struct {
	tierDefaults     map[worker.WorkerTier]string
	profileOverrides map[string]string
	taskTypeOverrides map[project.TaskType]string
}

// NewModelStrategy creates a ModelStrategy with sensible defaults.
func NewModelStrategy() *ModelStrategy {
	return &ModelStrategy{
		tierDefaults: map[worker.WorkerTier]string{
			worker.TierConsultant: "opus",
			worker.TierManager:   "opus",
			worker.TierEngineer:  "sonnet",
		},
		profileOverrides: map[string]string{
			"architect": "opus",
			"coder":     "sonnet",
			"hacker":    "sonnet",
			"designer":  "sonnet",
			"analyst":   "sonnet",
			"devops":    "sonnet",
		},
		taskTypeOverrides: map[project.TaskType]string{
			project.TaskTypeResearch: "opus",
		},
	}
}

// SetTierDefault sets the default model for a tier.
func (ms *ModelStrategy) SetTierDefault(tier worker.WorkerTier, model string) {
	ms.tierDefaults[tier] = model
}

// SetProfileOverride sets the model override for a skill profile.
func (ms *ModelStrategy) SetProfileOverride(profile, model string) {
	ms.profileOverrides[profile] = model
}

// SetTaskTypeOverride sets the model override for a task type.
func (ms *ModelStrategy) SetTaskTypeOverride(taskType project.TaskType, model string) {
	ms.taskTypeOverrides[taskType] = model
}

// ResolveModel determines the model to use based on the priority chain:
// Worker.ModelVersion > TaskType override > SkillProfile > Tier default
func (ms *ModelStrategy) ResolveModel(w *worker.Worker, t *project.Task) string {
	// Highest priority: explicit worker model
	if w.ModelVersion != "" {
		return w.ModelVersion
	}

	// TaskType override
	if t != nil {
		if model, ok := ms.taskTypeOverrides[t.Type]; ok {
			return model
		}
	}

	// SkillProfile override
	if w.SkillProfile != "" {
		if model, ok := ms.profileOverrides[w.SkillProfile]; ok {
			return model
		}
	}

	// Tier default
	if model, ok := ms.tierDefaults[w.EffectiveTier()]; ok {
		return model
	}

	return "sonnet"
}
