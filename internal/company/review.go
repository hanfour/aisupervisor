package company

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/personality"
	"github.com/hanfourmini/aisupervisor/internal/project"
	"github.com/hanfourmini/aisupervisor/internal/training"
	"github.com/hanfourmini/aisupervisor/internal/worker"
)

// ReviewRequest represents a queued review waiting for a manager.
type ReviewRequest struct {
	TaskID     string
	ProjectID  string
	EngineerID string
	ManagerID  string
	CreatedAt  time.Time
}

// reviewMeta tracks per-review metadata for training data capture.
type reviewMeta struct {
	StartTime      time.Time
	EngineerTmux   string
	EngineerWindow int
	EngineerPane   int
}

// ReviewPipeline manages the code review flow between engineers and managers.
type ReviewPipeline struct {
	mu              sync.Mutex
	reviewQueue     []ReviewRequest
	mgr             *Manager
	reviewStartMeta map[string]reviewMeta // keyed by original task ID
}

func newReviewPipeline(mgr *Manager) *ReviewPipeline {
	return &ReviewPipeline{
		mgr:             mgr,
		reviewStartMeta: make(map[string]reviewMeta),
	}
}

// PendingReviews returns a copy of the current review queue.
func (rp *ReviewPipeline) PendingReviews() []ReviewRequest {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	out := make([]ReviewRequest, len(rp.reviewQueue))
	copy(out, rp.reviewQueue)
	return out
}

// StartReview initiates a manager review for a completed engineer task.
// If the manager is idle, it spawns the review immediately. Otherwise it queues.
func (rp *ReviewPipeline) StartReview(ctx context.Context, engineerWorker *worker.Worker, t *project.Task, p *project.Project) error {
	managerWorker, ok := rp.mgr.GetManager(engineerWorker.ID)
	if !ok {
		// No manager assigned — skip review, go straight to done
		return nil
	}

	// Capture engineer pane state before review begins (for training data)
	rp.mu.Lock()
	rp.reviewStartMeta[t.ID] = reviewMeta{
		StartTime:      time.Now(),
		EngineerTmux:   engineerWorker.TmuxSession,
		EngineerWindow: engineerWorker.Window,
		EngineerPane:   engineerWorker.Pane,
	}
	rp.mu.Unlock()

	req := ReviewRequest{
		TaskID:     t.ID,
		ProjectID:  p.ID,
		EngineerID: engineerWorker.ID,
		ManagerID:  managerWorker.ID,
		CreatedAt:  time.Now(),
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
	if err := rp.mgr.projectStore.UpdateTaskStatus(t.ID, project.TaskReview); err != nil {
		return fmt.Errorf("updating task status to review: %w", err)
	}

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

	// Capture training data via collector
	rp.captureTrainingData(originalTask, managerWorker, p, output, approved)

	// Update personality mood based on review outcome
	if rp.mgr.personalityStore != nil {
		engineerID := originalTask.AssigneeID
		rp.mgr.personalityStore.UpdateProfile(engineerID, func(profile *personality.CharacterProfile) {
			if approved {
				personality.ApplyEvent(profile, personality.EventReviewApproved)
			} else {
				personality.ApplyEvent(profile, personality.EventReviewRejected)
			}
			personality.UpdateAutoMood(profile)
		})
		rp.mgr.emit(Event{
			Type:     EventMoodChanged,
			WorkerID: engineerID,
			Message:  fmt.Sprintf("Mood changed for %s after review", engineerID),
		})
	}

	if approved {
		_ = rp.mgr.projectStore.UpdateTaskStatus(originalTask.ID, project.TaskDone)
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

		// Engage idle managers after review approval
		if len(promoted) > 0 {
			go rp.mgr.engageIdleManagers(context.Background(), p.ID)
		}
	} else {
		_ = rp.mgr.projectStore.UpdateTaskStatus(originalTask.ID, project.TaskRevision)
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

// captureTrainingData collects review pair data for model fine-tuning.
func (rp *ReviewPipeline) captureTrainingData(originalTask *project.Task, managerWorker *worker.Worker, p *project.Project, managerOutput string, approved bool) {
	if rp.mgr.collector == nil {
		return
	}

	verdict := training.VerdictRejected
	if approved {
		verdict = training.VerdictAccepted
	}

	// Retrieve start metadata
	rp.mu.Lock()
	meta, hasMeta := rp.reviewStartMeta[originalTask.ID]
	delete(rp.reviewStartMeta, originalTask.ID)
	rp.mu.Unlock()

	// Look up engineer worker for model info
	var engineerModel, managerModel string
	rp.mgr.mu.RLock()
	if eng, ok := rp.mgr.workers[originalTask.AssigneeID]; ok {
		engineerModel = eng.ModelVersion
		if engineerModel == "" {
			engineerModel = eng.BackendID
		}
	}
	managerModel = managerWorker.ModelVersion
	if managerModel == "" {
		managerModel = managerWorker.BackendID
	}
	rp.mgr.mu.RUnlock()

	input := training.CaptureReviewInput{
		TaskID:        originalTask.ID,
		ProjectID:     p.ID,
		RepoPath:      p.RepoPath,
		BranchName:    originalTask.BranchName,
		EngineerID:    originalTask.AssigneeID,
		ManagerID:     managerWorker.ID,
		EngineerModel: engineerModel,
		ManagerModel:  managerModel,
		Prompt:        originalTask.Prompt,
		ManagerTmux:   managerWorker.TmuxSession,
		ManagerWindow: managerWorker.Window,
		ManagerPane:   managerWorker.Pane,
		Verdict:       verdict,
		Feedback:      managerOutput,
	}

	if hasMeta {
		input.StartTime = meta.StartTime
		input.EngineerTmux = meta.EngineerTmux
		input.EngineerWindow = meta.EngineerWindow
		input.EngineerPane = meta.EngineerPane
	}

	// Capture asynchronously to avoid blocking the review flow
	go func() {
		if err := rp.mgr.collector.CaptureReview(input); err == nil {
			rp.mgr.emit(Event{
				Type:      EventTrainingCaptured,
				ProjectID: p.ID,
				TaskID:    originalTask.ID,
				Message:   fmt.Sprintf("Training data captured for task %q (verdict: %s)", originalTask.Title, verdict),
			})

			// Check auto-trigger for fine-tuning
			if rp.mgr.finetuneRunner != nil {
				if shouldTrigger, _ := rp.mgr.finetuneRunner.CheckAutoTrigger(rp.mgr.finetuneCfg); shouldTrigger {
					if job, err := rp.mgr.finetuneRunner.Launch(rp.mgr.finetuneCfg); err == nil {
						rp.mgr.emit(Event{
							Type:    EventFinetuneStarted,
							Message: fmt.Sprintf("Auto-triggered fine-tune job %s (%d pairs threshold)", job.ID, rp.mgr.finetuneCfg.AutoTrigger),
						})
					}
				}
			}
		}
	}()
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
	sb.WriteString("IMPORTANT: Start reviewing IMMEDIATELY. No planning or preparation needed.\n\n")
	sb.WriteString(fmt.Sprintf("Review code on branch %s.\n\n", t.BranchName))
	sb.WriteString(fmt.Sprintf("Task: %s\n", t.Title))
	if t.Description != "" {
		sb.WriteString(fmt.Sprintf("Description: %s\n", t.Description))
	}
	sb.WriteString("\nSteps:\n")
	sb.WriteString(fmt.Sprintf("1. Run `git log main..%s --oneline` to see commits\n", t.BranchName))
	sb.WriteString(fmt.Sprintf("2. Run `git diff main...%s` to review all changes\n", t.BranchName))
	sb.WriteString("3. Check code quality, correctness, and test coverage\n")
	sb.WriteString("4. End your response with EXACTLY one of:\n")
	sb.WriteString("   APPROVED\n")
	sb.WriteString("   REJECTED: <specific reason and required changes>\n")
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
