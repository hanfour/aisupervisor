package project

import "time"

// ResearchReport holds the structured output of a research task.
type ResearchReport struct {
	ID              string    `yaml:"id" json:"id"`
	TaskID          string    `yaml:"task_id" json:"taskId"`
	ProjectID       string    `yaml:"project_id" json:"projectId"`
	WorkerID        string    `yaml:"worker_id" json:"workerId"`
	Summary         string    `yaml:"summary" json:"summary"`
	KeyFindings     []string  `yaml:"key_findings" json:"keyFindings"`
	Recommendations []string  `yaml:"recommendations" json:"recommendations"`
	References      []string  `yaml:"references" json:"references"`
	RawContent      string    `yaml:"raw_content" json:"rawContent"`
	CreatedAt       time.Time `yaml:"created_at" json:"createdAt"`
}
