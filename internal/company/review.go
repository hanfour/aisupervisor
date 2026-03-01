package company

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/hanfourmini/aisupervisor/internal/project"
	"github.com/hanfourmini/aisupervisor/internal/worker"
)

// ReviewRequest represents a queued review waiting for a manager.
type ReviewRequest struct {
	TaskID     string
	ProjectID  string
	EngineerID string
	ManagerID  string
}

// ReviewPipeline manages the code review flow between engineers and managers.
type ReviewPipeline struct {
	mu          sync.Mutex
	reviewQueue []ReviewRequest
	mgr         *Manager
}

func newReviewPipeline(mgr *Manager) *ReviewPipeline {
	return &ReviewPipeline{mgr: mgr}
}

// StartReview initiates a manager review for a completed engineer task.
// If the manager is idle, it spawns the review immediately. Otherwise it queues.
func (rp *ReviewPipeline) StartReview(ctx context.Context, engineerWorker *worker.Worker, t *project.Task, p *project.Project) error {
	managerWorker, ok := rp.mgr.GetManager(engineerWorker.ID)
	if !ok {
		// No manager assigned — skip review, go straight to done
		return nil
	}

	req := ReviewRequest{
		TaskID:     t.ID,
		ProjectID:  p.ID,
		EngineerID: engineerWorker.ID,
		ManagerID:  managerWorker.ID,
	}

	rp.mu.Lock()
	if managerWorker.Status != worker.WorkerIdle {
		// Manager busy — queue for later
		rp.reviewQueue = append(rp.reviewQueue, req)
		rp.mu.Unlock()
		return nil
	}
	rp.mu.Unlock()

	return rp.executeReview(ctx, req, managerWorker, t, p)
}

// DrainQueue attempts to process queued reviews with now-idle managers.
func (rp *ReviewPipeline) DrainQueue(ctx context.Context) {
	rp.mu.Lock()
	if len(rp.reviewQueue) == 0 {
		rp.mu.Unlock()
		return
	}

	var remaining []ReviewRequest
	for _, req := range rp.reviewQueue {
		managerWorker, ok := rp.mgr.workers[req.ManagerID]
		if !ok || managerWorker.Status != worker.WorkerIdle {
			remaining = append(remaining, req)
			continue
		}

		t, ok := rp.mgr.projectStore.GetTask(req.TaskID)
		if !ok {
			continue
		}
		p, ok := rp.mgr.projectStore.GetProject(req.ProjectID)
		if !ok {
			continue
		}

		rp.mu.Unlock()
		if err := rp.executeReview(ctx, req, managerWorker, t, p); err != nil {
			remaining = append(remaining, req)
		}
		rp.mu.Lock()
	}
	rp.reviewQueue = remaining
	rp.mu.Unlock()
}

func (rp *ReviewPipeline) executeReview(ctx context.Context, req ReviewRequest, managerWorker *worker.Worker, t *project.Task, p *project.Project) error {
	// Create a review sub-task
	reviewPrompt := buildReviewPrompt(t, p)
	reviewTask := &project.Task{
		ProjectID:    p.ID,
		Title:        fmt.Sprintf("Review: %s", t.Title),
		Description:  fmt.Sprintf("Code review for task %s", t.ID),
		Prompt:       reviewPrompt,
		Status:       project.TaskReady,
		Priority:     t.Priority,
		BranchName:   t.BranchName, // Same branch as the engineer's work
		ReviewerID:   managerWorker.ID,
		ParentTaskID: t.ID,
	}

	if err := rp.mgr.projectStore.SaveTask(reviewTask); err != nil {
		return fmt.Errorf("creating review task: %w", err)
	}

	// Update original task status
	t.ReviewCount++
	t.ReviewerID = managerWorker.ID
	rp.mgr.projectStore.UpdateTaskStatus(t.ID, project.TaskReview)

	rp.mgr.emit(Event{
		Type:      EventReviewStarted,
		ProjectID: p.ID,
		TaskID:    t.ID,
		WorkerID:  managerWorker.ID,
		Message:   fmt.Sprintf("Manager %s reviewing task %q", managerWorker.Name, t.Title),
	})

	// Assign review task to manager
	if err := rp.mgr.AssignTask(ctx, managerWorker.ID, reviewTask.ID); err != nil {
		return fmt.Errorf("assigning review to manager: %w", err)
	}

	return nil
}

// HandleReviewResult processes the outcome of a manager review.
func (rp *ReviewPipeline) HandleReviewResult(managerWorker *worker.Worker, reviewTask *project.Task, p *project.Project, result worker.CompletionResult) {
	if reviewTask.ParentTaskID == "" {
		return
	}

	originalTask, ok := rp.mgr.projectStore.GetTask(reviewTask.ParentTaskID)
	if !ok {
		return
	}

	// Read manager's output to determine verdict
	output := rp.captureManagerOutput(managerWorker)
	approved := parseReviewVerdict(output)

	if approved {
		rp.mgr.projectStore.UpdateTaskStatus(originalTask.ID, project.TaskDone)
		rp.mgr.emit(Event{
			Type:      EventReviewApproved,
			ProjectID: p.ID,
			TaskID:    originalTask.ID,
			WorkerID:  managerWorker.ID,
			Message:   fmt.Sprintf("Task %q approved by %s", originalTask.Title, managerWorker.Name),
		})

		// Promote newly unblocked tasks
		promoted, _ := rp.mgr.projectStore.PromoteReady(p.ID)
		for _, pt := range promoted {
			rp.mgr.emit(Event{
				Type:      EventTaskCreated,
				ProjectID: p.ID,
				TaskID:    pt.ID,
				Message:   fmt.Sprintf("Task %q is now ready (dependencies resolved)", pt.Title),
			})
		}
	} else {
		rp.mgr.projectStore.UpdateTaskStatus(originalTask.ID, project.TaskRevision)
		rp.mgr.emit(Event{
			Type:      EventReviewRejected,
			ProjectID: p.ID,
			TaskID:    originalTask.ID,
			WorkerID:  managerWorker.ID,
			Message:   fmt.Sprintf("Task %q rejected by %s", originalTask.Title, managerWorker.Name),
		})

		// Re-assign to original engineer with feedback
		rp.mgr.emit(Event{
			Type:      EventTaskRevision,
			ProjectID: p.ID,
			TaskID:    originalTask.ID,
			Message:   fmt.Sprintf("Task %q sent back for revision", originalTask.Title),
		})

		// Update prompt with feedback and re-queue
		originalTask.Prompt = fmt.Sprintf("%s\n\n--- Review Feedback ---\n%s\n\nPlease address the above feedback and resubmit.", originalTask.Prompt, output)
		originalTask.Status = project.TaskReady
		rp.mgr.projectStore.SaveTask(originalTask)

		// Auto-assign back to the original engineer if idle
		if originalTask.AssigneeID != "" {
			rp.mgr.mu.RLock()
			eng, ok := rp.mgr.workers[originalTask.AssigneeID]
			rp.mgr.mu.RUnlock()
			if ok && eng.Status == worker.WorkerIdle {
				go func() {
					ctx := context.Background()
					rp.mgr.AssignTask(ctx, eng.ID, originalTask.ID)
				}()
			}
		}
	}
}

func (rp *ReviewPipeline) captureManagerOutput(w *worker.Worker) string {
	if w.TmuxSession == "" {
		return ""
	}
	content, err := rp.mgr.tmuxClient.CapturePane(w.TmuxSession, w.Window, w.Pane, 100)
	if err != nil {
		return ""
	}
	return content
}

func buildReviewPrompt(t *project.Task, p *project.Project) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Review code on branch %s.\n\n", t.BranchName))
	sb.WriteString(fmt.Sprintf("The original task was: %s\n", t.Title))
	if t.Description != "" {
		sb.WriteString(fmt.Sprintf("Description: %s\n", t.Description))
	}
	sb.WriteString("\nPlease:\n")
	sb.WriteString("1. Run `git diff` to review the changes\n")
	sb.WriteString("2. Check code quality, correctness, and test coverage\n")
	sb.WriteString("3. End your response with either:\n")
	sb.WriteString("   - APPROVED: if the code is ready to merge\n")
	sb.WriteString("   - REJECTED: <reason> if changes are needed\n")
	return sb.String()
}

// parseReviewVerdict determines if a review output indicates approval.
func parseReviewVerdict(output string) bool {
	lower := strings.ToLower(output)
	// Check last 500 chars for the verdict
	if len(lower) > 500 {
		lower = lower[len(lower)-500:]
	}
	if strings.Contains(lower, "approved") {
		// Make sure it's not "not approved"
		if strings.Contains(lower, "rejected") {
			// If both present, last one wins
			approvedIdx := strings.LastIndex(lower, "approved")
			rejectedIdx := strings.LastIndex(lower, "rejected")
			return approvedIdx > rejectedIdx
		}
		return true
	}
	return false
}
