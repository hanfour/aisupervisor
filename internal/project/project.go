package project

import "time"

type ProjectStatus string

const (
	ProjectActive    ProjectStatus = "active"
	ProjectCompleted ProjectStatus = "completed"
	ProjectArchived  ProjectStatus = "archived"
)

type Project struct {
	ID          string        `yaml:"id" json:"id"`
	Name        string        `yaml:"name" json:"name"`
	Description string        `yaml:"description" json:"description"`
	RepoPath    string        `yaml:"repo_path" json:"repoPath"`
	BaseBranch  string        `yaml:"base_branch" json:"baseBranch"`
	Goals       []string      `yaml:"goals,omitempty" json:"goals,omitempty"`
	Status      ProjectStatus `yaml:"status" json:"status"`
	CreatedAt   time.Time     `yaml:"created_at" json:"createdAt"`
	UpdatedAt   time.Time     `yaml:"updated_at" json:"updatedAt"`
}
