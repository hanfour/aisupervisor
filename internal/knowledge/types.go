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
}
