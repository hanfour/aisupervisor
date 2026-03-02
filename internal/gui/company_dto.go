package gui

import (
	"time"

	"github.com/hanfourmini/aisupervisor/internal/company"
	"github.com/hanfourmini/aisupervisor/internal/project"
	"github.com/hanfourmini/aisupervisor/internal/worker"
)

type ProjectDTO struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	RepoPath    string   `json:"repoPath"`
	BaseBranch  string   `json:"baseBranch"`
	Goals       []string `json:"goals"`
	Status      string   `json:"status"`
	CreatedAt   string   `json:"createdAt"`
	UpdatedAt   string   `json:"updatedAt"`
}

func ProjectToDTO(p *project.Project) ProjectDTO {
	return ProjectDTO{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		RepoPath:    p.RepoPath,
		BaseBranch:  p.BaseBranch,
		Goals:       p.Goals,
		Status:      string(p.Status),
		CreatedAt:   p.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   p.UpdatedAt.Format(time.RFC3339),
	}
}

type TaskDTO struct {
	ID           string   `json:"id"`
	ProjectID    string   `json:"projectId"`
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	Prompt       string   `json:"prompt"`
	Status       string   `json:"status"`
	Priority     int      `json:"priority"`
	BranchName   string   `json:"branchName"`
	AssigneeID   string   `json:"assigneeId"`
	DependsOn    []string `json:"dependsOn"`
	Milestone    string   `json:"milestone"`
	ReviewerID   string   `json:"reviewerId,omitempty"`
	ParentTaskID string   `json:"parentTaskId,omitempty"`
	ReviewCount  int      `json:"reviewCount,omitempty"`
	CreatedAt    string   `json:"createdAt"`
	StartedAt    string   `json:"startedAt,omitempty"`
	CompletedAt  string   `json:"completedAt,omitempty"`
}

func TaskToDTO(t *project.Task) TaskDTO {
	dto := TaskDTO{
		ID:           t.ID,
		ProjectID:    t.ProjectID,
		Title:        t.Title,
		Description:  t.Description,
		Prompt:       t.Prompt,
		Status:       string(t.Status),
		Priority:     t.Priority,
		BranchName:   t.BranchName,
		AssigneeID:   t.AssigneeID,
		DependsOn:    t.DependsOn,
		Milestone:    t.Milestone,
		ReviewerID:   t.ReviewerID,
		ParentTaskID: t.ParentTaskID,
		ReviewCount:  t.ReviewCount,
		CreatedAt:    t.CreatedAt.Format(time.RFC3339),
	}
	if t.StartedAt != nil {
		dto.StartedAt = t.StartedAt.Format(time.RFC3339)
	}
	if t.CompletedAt != nil {
		dto.CompletedAt = t.CompletedAt.Format(time.RFC3339)
	}
	return dto
}

type WorkerDTO struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Avatar        string `json:"avatar"`
	Status        string `json:"status"`
	Tier          string `json:"tier"`
	BackendID     string `json:"backendId,omitempty"`
	ParentID      string `json:"parentId,omitempty"`
	ModelVersion  string `json:"modelVersion,omitempty"`
	CLITool       string `json:"cliTool,omitempty"`
	CurrentTaskID string `json:"currentTaskId"`
	TmuxSession   string `json:"tmuxSession"`
	SessionID     string `json:"sessionId"`
	CreatedAt     string `json:"createdAt"`
}

func WorkerToDTO(w *worker.Worker) WorkerDTO {
	return WorkerDTO{
		ID:            w.ID,
		Name:          w.Name,
		Avatar:        w.Avatar,
		Status:        string(w.Status),
		Tier:          string(w.EffectiveTier()),
		BackendID:     w.BackendID,
		ParentID:      w.ParentID,
		ModelVersion:  w.ModelVersion,
		CLITool:       w.CLITool,
		CurrentTaskID: w.CurrentTaskID,
		TmuxSession:   w.TmuxSession,
		SessionID:     w.SessionID,
		CreatedAt:     w.CreatedAt.Format(time.RFC3339),
	}
}

type CompanyProgressDTO struct {
	Total      int     `json:"total"`
	Done       int     `json:"done"`
	InProgress int     `json:"inProgress"`
	Failed     int     `json:"failed"`
	Percent    float64 `json:"percent"`
}

func ProgressToDTO(p company.ProgressDTO) CompanyProgressDTO {
	return CompanyProgressDTO{
		Total:      p.Total,
		Done:       p.Done,
		InProgress: p.InProgress,
		Failed:     p.Failed,
		Percent:    p.Percent,
	}
}

type CompanyEventDTO struct {
	Type      string `json:"type"`
	ProjectID string `json:"projectId,omitempty"`
	TaskID    string `json:"taskId,omitempty"`
	WorkerID  string `json:"workerId,omitempty"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

type ReviewRequestDTO struct {
	TaskID     string `json:"taskId"`
	ProjectID  string `json:"projectId"`
	EngineerID string `json:"engineerId"`
	ManagerID  string `json:"managerId"`
	CreatedAt  string `json:"createdAt"`
}

func ReviewRequestToDTO(r company.ReviewRequest) ReviewRequestDTO {
	return ReviewRequestDTO{
		TaskID:     r.TaskID,
		ProjectID:  r.ProjectID,
		EngineerID: r.EngineerID,
		ManagerID:  r.ManagerID,
		CreatedAt:  r.CreatedAt.Format(time.RFC3339),
	}
}

type TrainingStatsDTO struct {
	TotalPairs   int     `json:"totalPairs"`
	Accepted     int     `json:"accepted"`
	Rejected     int     `json:"rejected"`
	ApprovalRate float64 `json:"approvalRate"`
}

func CompanyEventToDTO(e company.Event) CompanyEventDTO {
	return CompanyEventDTO{
		Type:      string(e.Type),
		ProjectID: e.ProjectID,
		TaskID:    e.TaskID,
		WorkerID:  e.WorkerID,
		Message:   e.Message,
		Timestamp: e.Timestamp.Format(time.RFC3339),
	}
}
