package growth

import "time"

type GrowthEventType string

const (
	GrowthEXPGained      GrowthEventType = "exp_gained"
	GrowthLevelUp        GrowthEventType = "level_up"
	GrowthFeedback       GrowthEventType = "feedback"
	GrowthAchievement    GrowthEventType = "achievement"
	GrowthPairSuggested  GrowthEventType = "pair_suggested"
	GrowthKnowledgeAdded GrowthEventType = "knowledge_added"
)

type GrowthEvent struct {
	Type      GrowthEventType `json:"type"`
	WorkerID  string          `json:"workerId"`
	Branch    SkillBranch     `json:"branch,omitempty"`
	Amount    int             `json:"amount,omitempty"`
	NewLevel  int             `json:"newLevel,omitempty"`
	Message   string          `json:"message"`
	Timestamp time.Time       `json:"timestamp"`
}
