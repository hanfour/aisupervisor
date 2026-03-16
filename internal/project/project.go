package project

import "time"

type ProjectStatus string

const (
	ProjectActive    ProjectStatus = "active"
	ProjectCompleted ProjectStatus = "completed"
	ProjectArchived  ProjectStatus = "archived"
)

type ProjectPhase string

const (
	PhasePRD         ProjectPhase = "prd"
	PhaseDevelopment ProjectPhase = "development"
	PhaseCompleted   ProjectPhase = "completed"
)

type Project struct {
	ID          string        `yaml:"id" json:"id"`
	Name        string        `yaml:"name" json:"name"`
	Description string        `yaml:"description" json:"description"`
	RepoPath    string        `yaml:"repo_path" json:"repoPath"`
	BaseBranch  string        `yaml:"base_branch" json:"baseBranch"`
	Goals       []string      `yaml:"goals,omitempty" json:"goals,omitempty"`
	Phase       ProjectPhase  `yaml:"phase,omitempty" json:"phase,omitempty"`
	Status      ProjectStatus `yaml:"status" json:"status"`
	CreatedAt   time.Time     `yaml:"created_at" json:"createdAt"`
	UpdatedAt   time.Time     `yaml:"updated_at" json:"updatedAt"`

	// VerifyCmd is a shell command run after each task completes to verify quality.
	// If set, all tasks in this project will be verified before being marked done.
	// Example: "go vet ./... && go test ./..."
	VerifyCmd string `yaml:"verify_cmd,omitempty" json:"verifyCmd,omitempty"`
	// MaxIterations is the project-level default for how many self-improve cycles
	// a worker can attempt before accepting the result (default 3).
	MaxIterations int `yaml:"max_iterations,omitempty" json:"maxIterations,omitempty"`
}
