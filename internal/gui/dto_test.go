package gui

import (
	"testing"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/company"
	"github.com/hanfourmini/aisupervisor/internal/project"
	"github.com/hanfourmini/aisupervisor/internal/worker"
)

func TestProjectToDTO(t *testing.T) {
	now := time.Now()
	p := &project.Project{
		ID:          "proj-1",
		Name:        "Test",
		Description: "desc",
		RepoPath:    "/tmp/repo",
		BaseBranch:  "main",
		Goals:       []string{"goal1"},
		Status:      project.ProjectActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	dto := ProjectToDTO(p)
	if dto.ID != "proj-1" {
		t.Errorf("ID mismatch: %s", dto.ID)
	}
	if dto.Name != "Test" {
		t.Errorf("Name mismatch: %s", dto.Name)
	}
	if dto.Status != string(project.ProjectActive) {
		t.Errorf("Status mismatch: %s", dto.Status)
	}
	if len(dto.Goals) != 1 {
		t.Errorf("Goals count: %d", len(dto.Goals))
	}
}

func TestTaskToDTO(t *testing.T) {
	now := time.Now()
	task := &project.Task{
		ID:          "t-1",
		ProjectID:   "proj-1",
		Title:       "Fix bug",
		Status:      project.TaskReady,
		Priority:    5,
		BranchName:  "ai/proj/t1",
		CreatedAt:   now,
		ReviewerID:  "mgr-1",
		ReviewCount: 2,
	}
	dto := TaskToDTO(task)
	if dto.ID != "t-1" {
		t.Errorf("ID mismatch")
	}
	if dto.ReviewerID != "mgr-1" {
		t.Errorf("ReviewerID mismatch: %s", dto.ReviewerID)
	}
	if dto.ReviewCount != 2 {
		t.Errorf("ReviewCount mismatch: %d", dto.ReviewCount)
	}
	if dto.StartedAt != "" {
		t.Error("StartedAt should be empty for nil")
	}
}

func TestWorkerToDTO(t *testing.T) {
	w := &worker.Worker{
		ID:           "w-1",
		Name:         "Dev",
		Avatar:       "robot",
		Status:       worker.WorkerIdle,
		Tier:         worker.TierManager,
		BackendID:    "claude-sonnet",
		ModelVersion: "v2",
		CLITool:      "claude",
		CreatedAt:    time.Now(),
	}
	dto := WorkerToDTO(w)
	if dto.Tier != "manager" {
		t.Errorf("Tier mismatch: %s", dto.Tier)
	}
	if dto.ModelVersion != "v2" {
		t.Errorf("ModelVersion mismatch: %s", dto.ModelVersion)
	}
	if dto.CLITool != "claude" {
		t.Errorf("CLITool mismatch: %s", dto.CLITool)
	}
}

func TestCompanyEventToDTO(t *testing.T) {
	e := company.Event{
		Type:      company.EventTaskCompleted,
		ProjectID: "proj-1",
		TaskID:    "t-1",
		WorkerID:  "w-1",
		Message:   "done",
		Timestamp: time.Now(),
	}
	dto := CompanyEventToDTO(e)
	if dto.Type != "task_completed" {
		t.Errorf("Type mismatch: %s", dto.Type)
	}
	if dto.Timestamp == "" {
		t.Error("Timestamp should be set")
	}
}

func TestReviewRequestToDTO(t *testing.T) {
	r := company.ReviewRequest{
		TaskID:     "t-1",
		ProjectID:  "proj-1",
		EngineerID: "eng-1",
		ManagerID:  "mgr-1",
		CreatedAt:  time.Now(),
	}
	dto := ReviewRequestToDTO(r)
	if dto.CreatedAt == "" {
		t.Error("CreatedAt should be set")
	}
	if dto.EngineerID != "eng-1" {
		t.Errorf("EngineerID mismatch: %s", dto.EngineerID)
	}
}

func TestProgressToDTO(t *testing.T) {
	p := company.ProgressDTO{
		Total:      10,
		Done:       3,
		InProgress: 2,
		Failed:     1,
		Percent:    30.0,
	}
	dto := ProgressToDTO(p)
	if dto.Total != 10 || dto.Done != 3 || dto.Percent != 30.0 {
		t.Errorf("ProgressToDTO mismatch: %+v", dto)
	}
}
