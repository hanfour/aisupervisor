package knowledge

import "time"

type KnowledgeType string

const (
	KnowledgeArchitecture KnowledgeType = "architecture"
	KnowledgeDecision     KnowledgeType = "decision"
	KnowledgeGotcha       KnowledgeType = "gotcha"
	KnowledgeTaskSummary  KnowledgeType = "task_summary"
	KnowledgeFeedback     KnowledgeType = "feedback"
	KnowledgeLessonLearnt KnowledgeType = "lesson_learnt"
)

type KnowledgeTier int

const (
	TierL0Identity   KnowledgeTier = 0 // ~50 tokens: name, type, tech stack
	TierL1Essential  KnowledgeTier = 1 // ~200 tokens: conventions, patterns, decisions
	TierL2RoomRecall KnowledgeTier = 2 // ~500 tokens: architecture, module summaries
	TierL3DeepSearch KnowledgeTier = 3 // ~800+ tokens: full API surface, all modules
)

// TierForType returns the minimum tier that includes a given knowledge type.
func TierForType(kt KnowledgeType) KnowledgeTier {
	switch kt {
	case KnowledgeDecision, KnowledgeFeedback:
		return TierL1Essential
	case KnowledgeArchitecture, KnowledgeGotcha, KnowledgeTaskSummary:
		return TierL2RoomRecall
	case KnowledgeLessonLearnt:
		return TierL3DeepSearch
	default:
		return TierL0Identity
	}
}

type Entry struct {
	ID          string        `yaml:"id" json:"id"`
	ProjectID   string        `yaml:"project_id" json:"projectId"`
	WorkerID    string        `yaml:"worker_id,omitempty" json:"workerId,omitempty"`
	TaskID      string        `yaml:"task_id,omitempty" json:"taskId,omitempty"`
	Type        KnowledgeType `yaml:"type" json:"type"`
	Summary     string        `yaml:"summary" json:"summary"`
	FullContent string        `yaml:"full_content,omitempty" json:"fullContent,omitempty"`
	Files       []string      `yaml:"files,omitempty" json:"files,omitempty"`
	Relevance   float64       `yaml:"relevance" json:"relevance"`
	CreatedAt   time.Time     `yaml:"created_at" json:"createdAt"`
	AccessCount int           `yaml:"access_count" json:"accessCount"`
	Tier        KnowledgeTier `yaml:"tier" json:"tier"`
}
