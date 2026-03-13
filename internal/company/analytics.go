package company

import (
	"time"

	"github.com/hanfourmini/aisupervisor/internal/project"
	"github.com/hanfourmini/aisupervisor/internal/worker"
)

// PerformanceSnapshot captures a point-in-time performance metric for a worker.
type PerformanceSnapshot struct {
	WorkerID       string    `json:"workerId"`
	Date           time.Time `json:"date"`
	TasksCompleted int       `json:"tasksCompleted"`
	TasksFailed    int       `json:"tasksFailed"`
	ApprovalRate   float64   `json:"approvalRate"`
	TokensUsed     int64     `json:"tokensUsed"`
}

// CompanyOverview provides high-level metrics for the board dashboard.
type CompanyOverview struct {
	TotalObjectives    int     `json:"totalObjectives"`
	ActiveObjectives   int     `json:"activeObjectives"`
	TotalProjects      int     `json:"totalProjects"`
	ActiveProjects     int     `json:"activeProjects"`
	CompletedProjects  int     `json:"completedProjects"`
	TotalTasks         int     `json:"totalTasks"`
	CompletedTasks     int     `json:"completedTasks"`
	FailedTasks        int     `json:"failedTasks"`
	TotalWorkers       int     `json:"totalWorkers"`
	ActiveWorkers      int     `json:"activeWorkers"`
	OverallApproval    float64 `json:"overallApprovalRate"`
	MonthlyTokensUsed  int64   `json:"monthlyTokensUsed"`
	MonthlyTokenBudget int64   `json:"monthlyTokenBudget"`
}

// recordAnalyticsSnapshot appends a performance snapshot for a worker after task completion.
// Must be called with m.mu held.
func (m *Manager) recordAnalyticsSnapshot(workerID string, success bool) {
	snap := PerformanceSnapshot{
		WorkerID: workerID,
		Date:     time.Now(),
	}

	// Count worker's completed and failed tasks
	for _, t := range m.projectStore.ListTasks("") {
		if t.AssigneeID != workerID {
			continue
		}
		if t.Status == project.TaskDone || t.Status == project.TaskDeployed {
			snap.TasksCompleted++
			snap.TokensUsed += t.TokensConsumed
		} else if t.Status == project.TaskFailed {
			snap.TasksFailed++
		}
	}

	total := snap.TasksCompleted + snap.TasksFailed
	if total > 0 {
		snap.ApprovalRate = float64(snap.TasksCompleted) / float64(total) * 100
	}

	m.analytics = append(m.analytics, snap)

	// Cap analytics history to last 1000 entries
	if len(m.analytics) > 1000 {
		m.analytics = m.analytics[len(m.analytics)-1000:]
	}
}

// GetPerformanceHistory returns analytics snapshots, optionally filtered by worker ID.
func (m *Manager) GetPerformanceHistory(workerID string) []PerformanceSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if workerID == "" {
		out := make([]PerformanceSnapshot, len(m.analytics))
		copy(out, m.analytics)
		return out
	}

	var filtered []PerformanceSnapshot
	for _, s := range m.analytics {
		if s.WorkerID == workerID {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

// GetCompanyOverview computes a high-level summary of company metrics.
func (m *Manager) GetCompanyOverview() CompanyOverview {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ov := CompanyOverview{}

	// Objectives
	for _, obj := range m.objectives {
		ov.TotalObjectives++
		if obj.Status == ObjectiveActive {
			ov.ActiveObjectives++
		}
	}

	// Projects
	for _, p := range m.projectStore.ListProjects() {
		ov.TotalProjects++
		if p.Status == project.ProjectActive {
			ov.ActiveProjects++
		} else if p.Status == project.ProjectCompleted {
			ov.CompletedProjects++
		}
	}

	// Tasks
	totalReviewed := 0
	approved := 0
	for _, t := range m.projectStore.ListTasks("") {
		ov.TotalTasks++
		switch t.Status {
		case project.TaskDone, project.TaskDeployed:
			ov.CompletedTasks++
			if t.ReviewCount > 0 {
				totalReviewed++
				approved++
			}
		case project.TaskFailed:
			ov.FailedTasks++
			if t.ReviewCount > 0 {
				totalReviewed++
			}
		}
	}
	if totalReviewed > 0 {
		ov.OverallApproval = float64(approved) / float64(totalReviewed) * 100
	}

	// Workers
	for _, w := range m.workers {
		ov.TotalWorkers++
		if w.Status == worker.WorkerWorking {
			ov.ActiveWorkers++
		}
	}

	// Budget (inline to avoid lock re-entry)
	month := time.Now().Format("2006-01")
	for _, b := range m.budgets {
		if b.Month == month {
			ov.MonthlyTokensUsed = b.TokensUsed
			ov.MonthlyTokenBudget = b.TokenBudget
			break
		}
	}

	return ov
}
