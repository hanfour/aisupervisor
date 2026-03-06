package company

import (
	"context"
	"fmt"
	"log"
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
	reviewPrompt := rp.buildReviewPrompt(t, p)
	reviewTask := &project.Task{
		ProjectID:    p.ID,
		Title:        rp.mgr.msgf("Review: %s", "審查：%s", t.Title),
		Description:  rp.mgr.msgf("Code review for task %s", "程式碼審查任務 %s", t.ID),
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
		Message:   rp.mgr.msgf("Manager %s reviewing task %q", "管理員 %s 正在審查任務 %q", managerWorker.Name, t.Title),
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
	log.Printf("HandleReviewResult: reviewTask=%s originalTask=%s approved=%v outputLen=%d output_tail=%q",
		reviewTask.ID, originalTask.ID, approved, len(output), func() string {
			if len(output) > 300 {
				return output[len(output)-300:]
			}
			return output
		}())

	// Capture training data via collector
	rp.captureTrainingData(originalTask, managerWorker, p, output, approved)

	// Update personality mood and skill scores based on review outcome
	if rp.mgr.personalityStore != nil {
		engineerID := originalTask.AssigneeID
		rp.mgr.personalityStore.UpdateProfile(engineerID, func(profile *personality.CharacterProfile) {
			if approved {
				personality.ApplyEvent(profile, personality.EventReviewApproved)
				personality.ApplySkillEvent(&profile.SkillScores, personality.SkillEventReviewApproved)
				profile.TasksCompleted++
				// Decay skill scores every 10 tasks
				if profile.TasksCompleted%10 == 0 {
					personality.DecayTowardBaseline(&profile.SkillScores)
				}
			} else {
				personality.ApplyEvent(profile, personality.EventReviewRejected)
				// Classify rejection feedback and apply specific skill penalty
				skillEvent := personality.ClassifyRejectionType(output)
				personality.ApplySkillEvent(&profile.SkillScores, skillEvent)
			}
			personality.UpdateAutoMood(profile)
		})
		rp.mgr.emit(Event{
			Type:     EventMoodChanged,
			WorkerID: engineerID,
			Message:  rp.mgr.msgf("Mood changed for %s after review", "%s 審查後心情變化", engineerID),
		})
	}

	if approved {
		_ = rp.mgr.projectStore.UpdateTaskStatus(originalTask.ID, project.TaskDone)
		rp.mgr.emit(Event{
			Type:      EventReviewApproved,
			ProjectID: p.ID,
			TaskID:    originalTask.ID,
			WorkerID:  managerWorker.ID,
			Message:   rp.mgr.msgf("Task %q approved by %s", "任務 %q 已由 %s 核准", originalTask.Title, managerWorker.Name),
		})

		// Promote newly unblocked tasks
		promoted, _ := rp.mgr.projectStore.PromoteReady(p.ID)
		for _, pt := range promoted {
			rp.mgr.emit(Event{
				Type:      EventTaskCreated,
				ProjectID: p.ID,
				TaskID:    pt.ID,
				Message:   rp.mgr.msgf("Task %q is now ready (dependencies resolved)", "任務 %q 已就緒（依賴已解決）", pt.Title),
			})
		}

		// Engage idle managers after review approval
		if len(promoted) > 0 {
			go rp.mgr.engageIdleManagers(context.Background(), p.ID)
		}

		// Check if project is fully completed
		go rp.mgr.checkProjectCompletion(p.ID)
	} else {
		// Record rejection
		originalTask.RejectionCount++
		originalTask.RejectionHistory = append(originalTask.RejectionHistory, project.Rejection{
			Stage:      originalTask.Status,
			RejectorID: managerWorker.ID,
			Reason:     sanitizeForYAML(output),
			Timestamp:  time.Now(),
		})

		// Check circuit breaker before re-queuing
		cb := rp.mgr.circuitBreaker
		if cb.CheckBounceLoop(originalTask, managerWorker.ID, originalTask.AssigneeID) || project.ShouldEscalate(originalTask) {
			cb.RecordBounce(originalTask, managerWorker.ID, originalTask.AssigneeID, originalTask.Status, "bounce loop detected")
			cb.Escalate(originalTask, fmt.Sprintf("bounce loop: %d rejections, %d bounces", originalTask.RejectionCount, len(originalTask.BounceHistory)))
			rp.mgr.projectStore.SaveTask(originalTask)
			return
		}

		cb.RecordBounce(originalTask, managerWorker.ID, originalTask.AssigneeID, originalTask.Status, sanitizeForYAML(output))

		_ = rp.mgr.projectStore.UpdateTaskStatus(originalTask.ID, project.TaskRevision)
		rp.mgr.emit(Event{
			Type:      EventReviewRejected,
			ProjectID: p.ID,
			TaskID:    originalTask.ID,
			WorkerID:  managerWorker.ID,
			Message:   rp.mgr.msgf("Task %q rejected by %s (%d/%d)", "任務 %q 已由 %s 退回（%d/%d）", originalTask.Title, managerWorker.Name, originalTask.RejectionCount, project.MaxRejectionsBeforeEscalation),
		})

		rp.mgr.emit(Event{
			Type:      EventTaskRevision,
			ProjectID: p.ID,
			TaskID:    originalTask.ID,
			Message:   rp.mgr.msgf("Task %q sent back for revision", "任務 %q 已退回修改", originalTask.Title),
		})

		// Update prompt with feedback and re-queue
		if rp.mgr.GetLanguage() == "en" {
			originalTask.Prompt = fmt.Sprintf("%s\n\n--- Review Feedback ---\n%s\n\nPlease address the above feedback and resubmit.", originalTask.Prompt, output)
		} else {
			originalTask.Prompt = fmt.Sprintf("%s\n\n--- 審查回饋 ---\n%s\n\n請針對以上回饋進行修改後重新提交。", originalTask.Prompt, output)
		}
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
				Message:   rp.mgr.msgf("Training data captured for task %q (verdict: %s)", "已擷取任務 %q 的訓練資料（結果：%s）", originalTask.Title, verdict),
			})

			// Check auto-trigger for fine-tuning
			if rp.mgr.finetuneRunner != nil {
				if shouldTrigger, _ := rp.mgr.finetuneRunner.CheckAutoTrigger(rp.mgr.finetuneCfg); shouldTrigger {
					if job, err := rp.mgr.finetuneRunner.Launch(rp.mgr.finetuneCfg); err == nil {
						rp.mgr.emit(Event{
							Type:    EventFinetuneStarted,
							Message: rp.mgr.msgf("Auto-triggered fine-tune job %s (%d pairs threshold)", "已自動觸發微調任務 %s（%d 對閾值）", job.ID, rp.mgr.finetuneCfg.AutoTrigger),
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
	content, err := rp.mgr.tmuxClient.CapturePane(w.TmuxSession, w.Window, w.Pane, 500)
	if err != nil {
		return ""
	}
	return content
}

// sanitizeForYAML cleans tmux output so it can be safely stored in YAML.
// Removes box-drawing characters, excessive whitespace, and non-printable chars
// that can break YAML block scalars.
func sanitizeForYAML(s string) string {
	// Replace common box-drawing characters with dashes
	replacer := strings.NewReplacer(
		"─", "-", "━", "-", "│", "|", "┃", "|",
		"┌", "+", "┐", "+", "└", "+", "┘", "+",
		"├", "+", "┤", "+", "┬", "+", "┴", "+", "┼", "+",
		"╔", "+", "╗", "+", "╚", "+", "╝", "+",
		"║", "|", "═", "=",
		"❯", ">",
	)
	s = replacer.Replace(s)

	// Collapse runs of 3+ dashes/equals to just 3
	for _, ch := range []string{"-", "="} {
		long := strings.Repeat(ch, 4)
		short := strings.Repeat(ch, 3)
		for strings.Contains(s, long) {
			s = strings.ReplaceAll(s, long, short)
		}
	}

	// Limit length to avoid bloating YAML
	if len(s) > 2000 {
		s = s[len(s)-2000:]
	}

	return s
}

func (rp *ReviewPipeline) buildReviewPrompt(t *project.Task, p *project.Project) string {
	var sb strings.Builder
	if rp.mgr.GetLanguage() == "en" {
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
	} else {
		sb.WriteString("重要：請立即開始審查。不需要規劃或準備。\n\n")
		sb.WriteString(fmt.Sprintf("審查分支 %s 上的程式碼。\n\n", t.BranchName))
		sb.WriteString(fmt.Sprintf("任務：%s\n", t.Title))
		if t.Description != "" {
			sb.WriteString(fmt.Sprintf("描述：%s\n", t.Description))
		}
		sb.WriteString("\n步驟：\n")
		sb.WriteString(fmt.Sprintf("1. 執行 `git log main..%s --oneline` 查看提交紀錄\n", t.BranchName))
		sb.WriteString(fmt.Sprintf("2. 執行 `git diff main...%s` 審查所有變更\n", t.BranchName))
		sb.WriteString("3. 檢查程式碼品質、正確性和測試覆蓋率\n")
		sb.WriteString("4. 在回覆最後務必使用以下其中一個結論：\n")
		sb.WriteString("   APPROVED\n")
		sb.WriteString("   REJECTED: <具體原因和需要修改的內容>\n")
	}
	return sb.String()
}

// parseReviewVerdict determines if a review output indicates approval.
func parseReviewVerdict(output string) bool {
	lower := strings.ToLower(output)
	// Check last 5000 bytes for the verdict.
	// Must be large enough to account for UTF-8 multi-byte chars
	// (CJK = 3 bytes each, box-drawing ─ = 3 bytes each)
	// and scrollback content from Claude Code's verbose output.
	if len(lower) > 5000 {
		lower = lower[len(lower)-5000:]
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
