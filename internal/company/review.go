package company

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/config"
	"github.com/hanfourmini/aisupervisor/internal/gitops"
	"github.com/hanfourmini/aisupervisor/internal/knowledge"
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
	reviewCfg       config.ReviewConfig
}

func newReviewPipeline(mgr *Manager) *ReviewPipeline {
	return &ReviewPipeline{
		mgr:             mgr,
		reviewStartMeta: make(map[string]reviewMeta),
		reviewCfg:       mgr.reviewCfg,
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

	// Take a snapshot of the queue and clear it to avoid races with concurrent DrainQueue calls
	snapshot := make([]ReviewRequest, len(rp.reviewQueue))
	copy(snapshot, rp.reviewQueue)
	rp.reviewQueue = nil
	rp.mu.Unlock()

	var remaining []ReviewRequest
	for _, req := range snapshot {
		// Check manager status under m.mu to get a consistent read
		rp.mgr.mu.RLock()
		managerWorker, ok := rp.mgr.workers[req.ManagerID]
		idle := ok && managerWorker.Status == worker.WorkerIdle
		rp.mgr.mu.RUnlock()

		if !idle {
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

		if err := rp.executeReview(ctx, req, managerWorker, t, p); err != nil {
			remaining = append(remaining, req)
		}
	}

	// Put back any remaining items
	if len(remaining) > 0 {
		rp.mu.Lock()
		rp.reviewQueue = append(rp.reviewQueue, remaining...)
		rp.mu.Unlock()
	}
}

func (rp *ReviewPipeline) executeReview(ctx context.Context, req ReviewRequest, managerWorker *worker.Worker, t *project.Task, p *project.Project) error {
	// Use council pipeline if ChatProvider is available and council is enabled.
	// Copy reviewCfg under lock to avoid race with SetReviewConfig.
	if rp.mgr.chatProvider != nil {
		rp.mu.Lock()
		cfg := rp.reviewCfg
		rp.mu.Unlock()
		if cfg.CouncilEnabled && rp.mgr.council != nil {
			go rp.runCouncilReview(req, t, p, cfg)
		} else {
			go rp.runChatReview(req, t, p, cfg)
		}
		return nil
	}
	return rp.executeReviewTmux(ctx, req, managerWorker, t, p)
}

func (rp *ReviewPipeline) executeReviewTmux(ctx context.Context, req ReviewRequest, managerWorker *worker.Worker, t *project.Task, p *project.Project) error {
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
	now := time.Now()
	t.ReviewStartedAt = &now
	rp.mgr.projectStore.SaveTask(t)
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

// getDiffStats returns diff line count, file count, and diff content for a branch pair.
func getDiffStats(ctx context.Context, repoPath, baseBranch, taskBranch string) (diffLines int, fileCount int, diff string, err error) {
	cmd := exec.CommandContext(ctx, "git", "diff", baseBranch+"..."+taskBranch)
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return 0, 0, "", fmt.Errorf("git diff: %w", err)
	}
	diff = string(out)
	diffLines = strings.Count(diff, "\n")

	cmd2 := exec.CommandContext(ctx, "git", "diff", "--name-only", baseBranch+"..."+taskBranch)
	cmd2.Dir = repoPath
	out2, err2 := cmd2.Output()
	if err2 != nil {
		return diffLines, 0, diff, fmt.Errorf("git diff --name-only: %w", err2)
	}
	names := strings.TrimSpace(string(out2))
	if names == "" {
		fileCount = 0
	} else {
		fileCount = len(strings.Split(names, "\n"))
	}
	return
}

// parseDebateResult extracts a DebateResult from raw output (direct JSON or markdown-wrapped).
func parseDebateResult(output string) (*DebateResult, error) {
	var result DebateResult
	if err := json.Unmarshal([]byte(output), &result); err == nil && result.Status != "" {
		return &result, nil
	}
	extracted := extractChatJSON(output)
	if err := json.Unmarshal([]byte(extracted), &result); err != nil {
		return nil, fmt.Errorf("no valid DebateResult JSON found")
	}
	if result.Status == "" {
		return nil, fmt.Errorf("DebateResult has empty status")
	}
	return &result, nil
}

// reviewConfigWithDefaults applies defaults to a ReviewConfig with zero values.
func reviewConfigWithDefaults(cfg config.ReviewConfig) config.ReviewConfig {
	if cfg.DebateThreshold <= 0 {
		cfg.DebateThreshold = 300
	}
	if cfg.LightMaxLines <= 0 {
		cfg.LightMaxLines = 50
	}
	if cfg.LightMaxFiles <= 0 {
		cfg.LightMaxFiles = 3
	}
	if cfg.FastConverge <= 0 {
		cfg.FastConverge = 5
	}
	if cfg.AnalysisModel == "" {
		cfg.AnalysisModel = "opus"
	}
	if cfg.VoteModel == "" {
		cfg.VoteModel = "haiku"
	}
	if cfg.SynthesisModel == "" {
		cfg.SynthesisModel = "sonnet"
	}
	if cfg.MaxExperts == 0 {
		cfg.MaxExperts = 5
	}
	if cfg.CLIExpertTimeoutS == 0 {
		cfg.CLIExpertTimeoutS = 300
	}
	if cfg.APIExpertTimeoutS == 0 {
		cfg.APIExpertTimeoutS = 60
	}
	if cfg.ConventionDecayDays == 0 {
		cfg.ConventionDecayDays = 30
	}
	if cfg.CarmackFilterScale == "" {
		cfg.CarmackFilterScale = "auto"
	}
	return cfg
}

// runChatReview runs the debate/single-pass review pipeline via ChatProvider.
// cfg is passed by value (copied under lock by caller) to avoid races.
func (rp *ReviewPipeline) runChatReview(req ReviewRequest, t *project.Task, p *project.Project, cfg config.ReviewConfig) {
	cfg = reviewConfigWithDefaults(cfg)

	baseBranch := p.BaseBranch
	if baseBranch == "" {
		baseBranch = "main"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	diffLines, fileCount, diff, err := getDiffStats(ctx, p.RepoPath, baseBranch, t.BranchName)
	if err != nil {
		log.Printf("debate: getDiffStats failed: %v, falling back to tmux review", err)
		rp.fallbackTmuxReview(req, t, p)
		return
	}

	strategy := selectStrategy(diffLines, fileCount, cfg.DebateThreshold, cfg.LightMaxLines, cfg.LightMaxFiles)

	// Graph-based escalation: force debate for cross-community or god-node changes
	if strategy != ReviewDebate && rp.mgr.graphProvider != nil {
		if graph, err := rp.mgr.graphProvider.GetGraph(p.RepoPath); err == nil {
			changedFiles := extractChangedFiles(diff)
			if shouldEscalateReview(changedFiles, graph) {
				log.Printf("debate: escalating task=%s to debate (graph: cross-community or god-node)", t.ID)
				strategy = ReviewDebate
			}
		}
	}

	log.Printf("debate: task=%s strategy=%s (lines=%d files=%d)", t.ID, strategy, diffLines, fileCount)

	// Build PKB context if knowledge injector is available via spawner
	var pkbContext string
	if inj := rp.mgr.spawner.KnowledgeInjector(); inj != nil {
		pkbCtx, _ := inj.BuildContext("", p.ID, knowledge.TierL2RoomRecall)
		pkbContext = pkbCtx
	}

	var result *DebateResult
	switch strategy {
	case ReviewLight:
		result, err = runSinglePassReview(ctx, rp.mgr.chatProvider, diff, pkbContext, cfg.SynthesisModel, rp.mgr.GetLanguage(), ReviewLight)
	case ReviewStandard:
		result, err = runSinglePassReview(ctx, rp.mgr.chatProvider, diff, pkbContext, cfg.AnalysisModel, rp.mgr.GetLanguage(), ReviewStandard)
	case ReviewDebate:
		result, err = runDebate(ctx, rp.mgr.chatProvider, diff, pkbContext, cfg.AnalysisModel, cfg.VoteModel, cfg.SynthesisModel, cfg.FastConverge, rp.mgr.GetLanguage())
	}

	if err != nil {
		log.Printf("debate: review failed: %v, falling back to tmux", err)
		rp.fallbackTmuxReview(req, t, p)
		return
	}

	rp.handleDebateResult(result, req, t, p)
}

// runCouncilReview runs the council-based multi-expert review pipeline.
func (rp *ReviewPipeline) runCouncilReview(req ReviewRequest, t *project.Task, p *project.Project, cfg config.ReviewConfig) {
	cfg = reviewConfigWithDefaults(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	baseBranch := p.BaseBranch
	if baseBranch == "" {
		baseBranch = "main"
	}

	diffLines, fileCount, diff, err := getDiffStats(ctx, p.RepoPath, baseBranch, t.BranchName)
	if err != nil {
		log.Printf("council: getDiffStats failed: %v, falling back to debate", err)
		rp.runChatReview(req, t, p, cfg)
		return
	}

	// Phase 0: automated pre-review checks
	var phase0 *Phase0Report
	if cfg.Phase0Enabled {
		workDir := p.RepoPath
		if t.WorktreePath != "" {
			workDir = t.WorktreePath
		}
		checks := detectChecks(workDir, p.VerifyCmd)
		phase0 = runPhase0Checks(ctx, workDir, checks)
		rp.mgr.emit(Event{
			Type:      EventPhase0Completed,
			ProjectID: p.ID,
			TaskID:    t.ID,
			Message:   phase0.Summary,
		})
	} else {
		phase0 = &Phase0Report{AllGreen: true, Summary: "phase0 disabled"}
	}

	// Build Context Brief
	briefBuilder := &ContextBriefBuilder{
		totalBudget:   6000,
		taskID:        t.ID,
		projectID:     p.ID,
		projectName:   p.Name,
		baseBranch:    baseBranch,
		diffStats:     fmt.Sprintf("%d lines changed, %d files", diffLines, fileCount),
		changedFiles:  extractChangedFiles(diff),
		phase0Summary: phase0.Summary,
	}

	// Inject knowledge context if available
	if inj := rp.mgr.spawner.KnowledgeInjector(); inj != nil {
		archCtx, _ := inj.BuildContext("", p.ID, knowledge.TierL2RoomRecall)
		briefBuilder.architectureCtx = archCtx
	}

	// Inject relevant conventions
	if rp.mgr.conventions != nil {
		seen := map[string]bool{}
		var convParts []string
		for _, f := range extractChangedFiles(diff) {
			for _, c := range rp.mgr.conventions.FindRelevant("", f) {
				key := c.ID
				if !seen[key] {
					seen[key] = true
					convParts = append(convParts, fmt.Sprintf("- [%s] %s (%s)", c.Domain, c.Description, c.FileGlob))
				}
			}
		}
		if len(convParts) > 0 {
			briefBuilder.conventions = strings.Join(convParts, "\n")
		}
	}

	// Inject rejection history
	if len(t.RejectionHistory) > 0 {
		var parts []string
		for _, r := range t.RejectionHistory {
			parts = append(parts, r.Reason)
		}
		briefBuilder.rejectionHistory = truncateOutput(strings.Join(parts, "\n---\n"), 1500)
	}

	brief, err := briefBuilder.Build()
	if err != nil {
		log.Printf("council: brief build failed: %v, falling back to debate", err)
		rp.runChatReview(req, t, p, cfg)
		return
	}

	// Set graph on council engine if available
	if rp.mgr.graphProvider != nil {
		if graph, gErr := rp.mgr.graphProvider.GetGraph(p.RepoPath); gErr == nil {
			rp.mgr.council.graph = graph
		}
	}

	rp.mgr.emit(Event{
		Type:      EventCouncilStarted,
		ProjectID: p.ID,
		TaskID:    t.ID,
		Message:   fmt.Sprintf("council review started for %s (%d lines, %d files)", t.ID, diffLines, fileCount),
	})

	// Run Council
	result, err := rp.mgr.council.RunCouncil(ctx, CouncilRequest{
		Diff:      diff,
		DiffLines: diffLines,
		FileCount: fileCount,
		Brief:     brief,
		Phase0:    phase0,
	})
	if err != nil {
		log.Printf("council: RunCouncil failed: %v, falling back to debate", err)
		rp.runChatReview(req, t, p, cfg)
		return
	}

	rp.mgr.emit(Event{
		Type:      EventCouncilSynthesized,
		ProjectID: p.ID,
		TaskID:    t.ID,
		Message:   fmt.Sprintf("council: %s (%d experts, %d findings)", result.Status, result.ExpertCount, len(result.Findings)),
	})

	rp.handleCouncilResult(result, req, t, p)
}

// handleCouncilResult converts CouncilResult to DebateResult and reuses existing handleDebateResult.
func (rp *ReviewPipeline) handleCouncilResult(result *CouncilResult, req ReviewRequest, t *project.Task, p *project.Project) {
	var comments []Finding
	for _, ef := range result.Findings {
		f := ef.Finding
		f.Source = string(ef.Expert)
		comments = append(comments, f)
	}

	debateResult := &DebateResult{
		Status:   result.Status,
		Summary:  fmt.Sprintf("[%d experts, %d findings] %s", result.ExpertCount, len(result.Findings), result.Summary),
		Comments: comments,
	}

	rp.handleDebateResult(debateResult, req, t, p)

	// Learn from approved reviews
	if result.Status == "APPROVED" && rp.mgr.conventions != nil {
		go rp.learnFromReview(result, t)
	}
}

// learnFromReview extracts potential conventions from approved review findings.
func (rp *ReviewPipeline) learnFromReview(result *CouncilResult, t *project.Task) {
	for _, f := range result.Findings {
		if f.Severity != "MEDIUM" {
			continue
		}
		existing := rp.mgr.conventions.MatchesFinding(f)
		if existing != nil {
			rp.mgr.conventions.Accept(existing.ID)
			rp.mgr.emit(Event{
				Type:    EventConventionAccepted,
				TaskID:  t.ID,
				Message: fmt.Sprintf("convention %s reinforced", existing.ID),
			})
		} else {
			rp.mgr.conventions.Propose(Convention{
				Domain:      f.Expert,
				Pattern:     f.Body,
				Description: f.Body,
				Source:      fmt.Sprintf("review:%s", t.ID),
			})
			rp.mgr.emit(Event{
				Type:    EventConventionProposed,
				TaskID:  t.ID,
				Message: fmt.Sprintf("new convention candidate: %.50s", f.Body),
			})
		}
	}
	if err := rp.mgr.conventions.Save(); err != nil {
		log.Printf("council: conventions save failed: %v", err)
	}
}

// fallbackTmuxReview falls back to the legacy tmux-based review when debate pipeline fails.
func (rp *ReviewPipeline) fallbackTmuxReview(req ReviewRequest, t *project.Task, p *project.Project) {
	rp.mgr.mu.RLock()
	managerWorker, ok := rp.mgr.workers[req.ManagerID]
	rp.mgr.mu.RUnlock()
	if !ok {
		log.Printf("WARN: fallbackTmuxReview: manager %s not found, review for task %s dropped", req.ManagerID, t.ID)
		return
	}
	if err := rp.executeReviewTmux(context.Background(), req, managerWorker, t, p); err != nil {
		log.Printf("ERROR: fallbackTmuxReview failed for task %s: %v", t.ID, err)
	}
}

// handleDebateResult processes the outcome of a debate/single-pass review.
func (rp *ReviewPipeline) handleDebateResult(result *DebateResult, req ReviewRequest, t *project.Task, p *project.Project) {
	approved := result.Status == "APPROVED"

	// Build output string for training/personality
	var output string
	if approved {
		output = "APPROVED\n" + result.Summary
	} else {
		var sb strings.Builder
		sb.WriteString("REJECTED\n")
		sb.WriteString(result.Summary + "\n\n")
		for _, c := range result.Comments {
			sb.WriteString(fmt.Sprintf("[%s] %s:%d — %s\n", c.Severity, c.File, c.Line, c.Body))
		}
		output = sb.String()
	}

	// Emit appropriate event based on verdict
	eventType := EventReviewApproved
	if !approved {
		eventType = EventReviewRejected
	}
	rp.mgr.emit(Event{
		Type:      eventType,
		ProjectID: p.ID,
		TaskID:    t.ID,
		Message:   rp.mgr.msgf("Debate review for task %q: %s (%d findings)", "任務 %q 的辯論審查：%s（%d 個發現）", t.Title, result.Status, len(result.Comments)),
	})

	// Update personality
	if rp.mgr.personalityStore != nil && t.AssigneeID != "" {
		rp.mgr.personalityStore.UpdateProfile(t.AssigneeID, func(profile *personality.CharacterProfile) {
			if approved {
				personality.ApplyEvent(profile, personality.EventReviewApproved)
				personality.ApplySkillEvent(&profile.SkillScores, personality.SkillEventReviewApproved)
				profile.TasksCompleted++
				if profile.TasksCompleted%10 == 0 {
					personality.DecayTowardBaseline(&profile.SkillScores)
				}
			} else {
				personality.ApplyEvent(profile, personality.EventReviewRejected)
				skillEvent := personality.ClassifyRejectionType(output)
				personality.ApplySkillEvent(&profile.SkillScores, skillEvent)
			}
			personality.UpdateAutoMood(profile)
		})
		rp.mgr.emit(Event{
			Type:     EventMoodChanged,
			WorkerID: t.AssigneeID,
			Message:  rp.mgr.msgf("Mood changed after debate review", "辯論審查後心情變化"),
		})
	}

	if approved {
		rp.mgr.projectStore.UpdateTaskStatus(t.ID, project.TaskDone)
		rp.cleanupAfterApproval(t, p.RepoPath, rp.mgr.gitOps)
		// Note: EventReviewApproved already emitted above via eventType.
		promoted, _ := rp.mgr.projectStore.PromoteReady(p.ID)
		for _, pt := range promoted {
			rp.mgr.emit(Event{
				Type:      EventTaskCreated,
				ProjectID: p.ID,
				TaskID:    pt.ID,
				Message:   rp.mgr.msgf("Task %q is now ready", "任務 %q 已就緒", pt.Title),
			})
		}
		if len(promoted) > 0 {
			go rp.mgr.engageIdleManagers(context.Background(), p.ID)
			go rp.mgr.drainReadyQueue(context.Background())
		}
		go rp.mgr.checkProjectCompletion(p.ID)
	} else {
		t.RejectionCount++
		t.RejectionHistory = append(t.RejectionHistory, project.Rejection{
			Stage:         t.Status,
			RejectorID:    "debate-review",
			Reason:        sanitizeForYAML(output),
			ViolationTags: config.ClassifyViolations(output),
			Timestamp:     time.Now(),
		})

		cb := rp.mgr.circuitBreaker
		if cb.CheckBounceLoop(t, "debate-review", t.AssigneeID) || project.ShouldEscalate(t) {
			cb.RecordBounce(t, "debate-review", t.AssigneeID, t.Status, "debate bounce loop")
			cb.Escalate(t, fmt.Sprintf("debate: %d rejections", t.RejectionCount))
			rp.mgr.projectStore.SaveTask(t)
			return
		}
		cb.RecordBounce(t, "debate-review", t.AssigneeID, t.Status, sanitizeForYAML(output))

		rp.mgr.projectStore.UpdateTaskStatus(t.ID, project.TaskRevision)

		basePrompt := t.Prompt
		if idx := strings.Index(basePrompt, "\n\n--- Review Feedback ---\n"); idx != -1 {
			basePrompt = basePrompt[:idx]
		}
		if idx := strings.Index(basePrompt, "\n\n--- 審查回饋 ---\n"); idx != -1 {
			basePrompt = basePrompt[:idx]
		}
		if rp.mgr.GetLanguage() == "en" {
			t.Prompt = fmt.Sprintf("%s\n\n--- Review Feedback (attempt %d) ---\n%s\n\nPlease address the above feedback and resubmit.", basePrompt, t.RejectionCount, output)
		} else {
			t.Prompt = fmt.Sprintf("%s\n\n--- 審查回饋（第 %d 次）---\n%s\n\n請針對以上回饋進行修改後重新提交。", basePrompt, t.RejectionCount, output)
		}
		t.Status = project.TaskReady
		rp.mgr.projectStore.SaveTask(t)

		// Note: EventReviewRejected already emitted above via eventType.

		if t.AssigneeID != "" {
			rp.mgr.mu.RLock()
			eng, ok := rp.mgr.workers[t.AssigneeID]
			rp.mgr.mu.RUnlock()
			if ok && eng.Status == worker.WorkerIdle {
				go func() {
					rp.mgr.AssignTask(context.Background(), eng.ID, t.ID)
				}()
			}
		}
	}
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
	verdict := parseReviewVerdict(output)
	approved := verdict == verdictApproved
	log.Printf("HandleReviewResult: reviewTask=%s originalTask=%s verdict=%d outputLen=%d output_tail=%q",
		reviewTask.ID, originalTask.ID, verdict, len(output), func() string {
			if len(output) > 300 {
				return output[len(output)-300:]
			}
			return output
		}())

	// Inconclusive verdict: request human intervention instead of auto-rejecting
	if verdict == verdictInconclusive {
		rp.mgr.humanGate.createRequest(HumanGateRequest{
			Reason:   "review_inconclusive",
			TaskID:   originalTask.ID,
			WorkerID: managerWorker.ID,
			Message:  rp.mgr.msgf("Review of task %q by %s produced no clear verdict — please review manually", "管理員 %s 對任務 %q 的審查結果不明確，請手動審核", managerWorker.Name, originalTask.Title),
			Blocking: true,
		})
		rp.mgr.emit(Event{
			Type:      EventHumanInterventionRequired,
			ProjectID: p.ID,
			TaskID:    originalTask.ID,
			WorkerID:  managerWorker.ID,
			Message:   rp.mgr.msgf("Review inconclusive for task %q — awaiting human decision", "任務 %q 審查結果不明確，等待人工決定", originalTask.Title),
		})
		return
	}

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
		// Record trajectory: review_approved
		if rp.mgr.spawner != nil {
			if rec := rp.mgr.spawner.TrajectoryRecorder(); rec != nil {
				_ = rec.Record(worker.TrajectoryEntry{
					Timestamp: time.Now(),
					WorkerID:  managerWorker.ID,
					TaskID:    originalTask.ID,
					Event:     worker.TrajectoryEventReviewApproved,
					Details:   fmt.Sprintf("approved by %s", managerWorker.Name),
				})
			}
		}
		rp.cleanupAfterApproval(originalTask, p.RepoPath, rp.mgr.gitOps)

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

		// Engage idle managers and drain ready queue after review approval
		if len(promoted) > 0 {
			go rp.mgr.engageIdleManagers(context.Background(), p.ID)
			go rp.mgr.drainReadyQueue(context.Background())
		}

		// Check if project is fully completed
		go rp.mgr.checkProjectCompletion(p.ID)
	} else {
		// Record rejection
		originalTask.RejectionCount++
		originalTask.RejectionHistory = append(originalTask.RejectionHistory, project.Rejection{
			Stage:         originalTask.Status,
			RejectorID:    managerWorker.ID,
			Reason:        sanitizeForYAML(output),
			ViolationTags: config.ClassifyViolations(output),
			Timestamp:     time.Now(),
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
		// Record trajectory: review_rejected
		if rp.mgr.spawner != nil {
			if rec := rp.mgr.spawner.TrajectoryRecorder(); rec != nil {
				_ = rec.Record(worker.TrajectoryEntry{
					Timestamp: time.Now(),
					WorkerID:  managerWorker.ID,
					TaskID:    originalTask.ID,
					Event:     worker.TrajectoryEventReviewRejected,
					Details:   fmt.Sprintf("rejected by %s (attempt %d)", managerWorker.Name, originalTask.RejectionCount),
				})
			}
		}

		rp.mgr.emit(Event{
			Type:      EventTaskRevision,
			ProjectID: p.ID,
			TaskID:    originalTask.ID,
			Message:   rp.mgr.msgf("Task %q sent back for revision", "任務 %q 已退回修改", originalTask.Title),
		})

		// Update prompt with feedback and re-queue.
		// Strip previous feedback sections to prevent unbounded prompt growth.
		basePrompt := originalTask.Prompt
		if idx := strings.Index(basePrompt, "\n\n--- Review Feedback ---\n"); idx != -1 {
			basePrompt = basePrompt[:idx]
		}
		if idx := strings.Index(basePrompt, "\n\n--- 審查回饋 ---\n"); idx != -1 {
			basePrompt = basePrompt[:idx]
		}
		if rp.mgr.GetLanguage() == "en" {
			originalTask.Prompt = fmt.Sprintf("%s\n\n--- Review Feedback (attempt %d) ---\n%s\n\nPlease address the above feedback and resubmit.", basePrompt, originalTask.RejectionCount, output)
		} else {
			originalTask.Prompt = fmt.Sprintf("%s\n\n--- 審查回饋（第 %d 次）---\n%s\n\n請針對以上回饋進行修改後重新提交。", basePrompt, originalTask.RejectionCount, output)
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

	// Limit length to avoid bloating YAML — keep head + tail for context
	if len(s) > 2000 {
		head := s[:500]
		tail := s[len(s)-1500:]
		s = head + "\n\n[... truncated ...]\n\n" + tail
	}

	return s
}

func (rp *ReviewPipeline) buildReviewPrompt(t *project.Task, p *project.Project) string {
	baseBranch := p.BaseBranch
	if baseBranch == "" {
		baseBranch = "main"
	}

	var sb strings.Builder
	if rp.mgr.GetLanguage() == "en" {
		sb.WriteString("IMPORTANT: Start reviewing IMMEDIATELY. No planning or preparation needed.\n\n")
		sb.WriteString(fmt.Sprintf("Review code on branch %s.\n\n", t.BranchName))
		sb.WriteString(fmt.Sprintf("Task: %s\n", t.Title))
		if t.Description != "" {
			sb.WriteString(fmt.Sprintf("Description: %s\n", t.Description))
		}
		sb.WriteString("\nSteps:\n")
		sb.WriteString(fmt.Sprintf("1. Run `git log %s..%s --oneline` to see commits\n", baseBranch, t.BranchName))
		sb.WriteString(fmt.Sprintf("2. Run `git diff %s...%s` to review all changes\n", baseBranch, t.BranchName))
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
		sb.WriteString(fmt.Sprintf("1. 執行 `git log %s..%s --oneline` 查看提交紀錄\n", baseBranch, t.BranchName))
		sb.WriteString(fmt.Sprintf("2. 執行 `git diff %s...%s` 審查所有變更\n", baseBranch, t.BranchName))
		sb.WriteString("3. 檢查程式碼品質、正確性和測試覆蓋率\n")
		sb.WriteString("4. 在回覆最後務必使用以下其中一個結論：\n")
		sb.WriteString("   APPROVED\n")
		sb.WriteString("   REJECTED: <具體原因和需要修改的內容>\n")
	}
	return sb.String()
}

// reviewVerdict represents the outcome of a review.
type reviewVerdict int

const (
	verdictInconclusive reviewVerdict = iota
	verdictApproved
	verdictRejected
)

// parseReviewVerdict determines the review outcome from manager output.
// Returns verdictInconclusive if neither APPROVED nor REJECTED is found.
func parseReviewVerdict(output string) reviewVerdict {
	lower := strings.ToLower(output)
	// Check last 5000 bytes for the verdict.
	if len(lower) > 5000 {
		lower = lower[len(lower)-5000:]
	}
	hasApproved := strings.Contains(lower, "approved")
	hasRejected := strings.Contains(lower, "rejected")

	if !hasApproved && !hasRejected {
		return verdictInconclusive
	}
	if hasApproved && hasRejected {
		// Both present: last one wins
		if strings.LastIndex(lower, "approved") > strings.LastIndex(lower, "rejected") {
			return verdictApproved
		}
		return verdictRejected
	}
	if hasApproved {
		return verdictApproved
	}
	return verdictRejected
}

// cleanupAfterApproval removes the git worktree after a task is approved.
// If no worktree was used (WorktreePath is empty), this is a no-op.
func (rp *ReviewPipeline) cleanupAfterApproval(t *project.Task, repoPath string, g gitops.GitOps) {
	if t.WorktreePath == "" {
		return
	}
	if g != nil {
		if err := g.CleanupWorktree(repoPath, t.WorktreePath); err != nil {
			log.Printf("WARN: failed to cleanup worktree %s: %v", t.WorktreePath, err)
		}
	}
	t.WorktreePath = ""
}
