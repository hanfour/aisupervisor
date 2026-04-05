package gui

import (
	"fmt"

	"github.com/hanfourmini/aisupervisor/internal/growth"
)

// GetSkillTree returns the skill tree for a worker.
func (c *CompanyApp) GetSkillTree(workerID string) (map[string]interface{}, error) {
	_, ok := c.company.GetWorker(workerID)
	if !ok {
		return nil, fmt.Errorf("worker %s not found", workerID)
	}
	ge := c.company.GetGrowthEngine()
	if ge == nil {
		return nil, fmt.Errorf("growth engine not enabled")
	}
	st := ge.GetTree(workerID)
	branches := make(map[string]interface{})
	for b, node := range st.Branches {
		branches[string(b)] = map[string]interface{}{
			"branch":     string(node.Branch),
			"level":      node.Level,
			"currentExp": node.CurrentEXP,
			"totalExp":   node.TotalEXP,
			"expToNext":  node.EXPToNextLevel(),
			"subSkills":  node.SubSkills,
		}
	}
	return map[string]interface{}{
		"branches":       branches,
		"dominantBranch": string(st.DominantBranch()),
		"averageLevel":   st.AverageLevel(),
		"updatedAt":      st.UpdatedAt,
	}, nil
}

// GetGrowthHistory returns recent growth events for a worker.
// For now returns empty - will be populated when event persistence is added.
func (c *CompanyApp) GetGrowthHistory(workerID string, limit int) ([]map[string]interface{}, error) {
	return []map[string]interface{}{}, nil
}

// SubmitBossFeedback processes boss feedback for a worker.
func (c *CompanyApp) SubmitBossFeedback(workerID, taskID, content string) error {
	feedbackType := growth.ClassifyFeedback(content)
	branches := growth.InferBranches(content)

	var branch growth.SkillBranch
	if len(branches) > 0 {
		branch = branches[0]
	}

	ge := c.company.GetGrowthEngine()
	if ge == nil {
		return nil // growth not enabled
	}

	ge.ProcessFeedback(workerID, growth.FeedbackInfo{
		Type:    string(feedbackType),
		Content: content,
		Branch:  branch,
	})
	return nil
}

// GetFeatureFlags returns all feature flags and their states.
func (c *CompanyApp) GetFeatureFlags() []map[string]interface{} {
	fm := c.company.GetFeatureManager()
	if fm == nil {
		return nil
	}
	flags := fm.List()
	result := make([]map[string]interface{}, len(flags))
	for i, f := range flags {
		result[i] = map[string]interface{}{
			"id":      f.ID,
			"name":    f.Name,
			"enabled": f.Enabled,
		}
	}
	return result
}

// ToggleFeatureFlag toggles a feature flag.
func (c *CompanyApp) ToggleFeatureFlag(id string, enabled bool) error {
	fm := c.company.GetFeatureManager()
	if fm == nil {
		return fmt.Errorf("feature manager not initialized")
	}
	fm.Toggle(id, enabled)
	return nil
}
