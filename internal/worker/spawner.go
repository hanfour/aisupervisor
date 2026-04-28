package worker

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/agent"
	"github.com/hanfourmini/aisupervisor/internal/config"
	"github.com/hanfourmini/aisupervisor/internal/gitops"
	"github.com/hanfourmini/aisupervisor/internal/growth"
	"github.com/hanfourmini/aisupervisor/internal/knowledge"
	"github.com/hanfourmini/aisupervisor/internal/project"
	"github.com/hanfourmini/aisupervisor/internal/session"
	"github.com/hanfourmini/aisupervisor/internal/supervisor"
	"github.com/hanfourmini/aisupervisor/internal/tmux"

	"github.com/hanfourmini/aisupervisor/internal/personality"
)

// TierSpawnConfig holds resolved spawn parameters for a worker tier.
type TierSpawnConfig struct {
	CLITool    string
	CLIArgs    string
	ReadyCheck *regexp.Regexp
}

type Spawner struct {
	tmuxClient         tmux.TmuxClient
	gitOps             gitops.GitOps
	sup                *supervisor.Supervisor
	sessionMgr         *session.Manager
	tierConfigs        map[WorkerTier]TierSpawnConfig
	skillProfiles      map[string]config.SkillProfile
	skillOverrides     map[string]config.SkillProfileOverride // keyed by workerID
	projectStore       projectStoreReader
	language           string // "en" or "zh-TW"
	personalityStore   *personality.Store
	knowledgeInjector  *knowledge.Injector
	useWorktrees       bool // Enable git worktree isolation per task
	trajectoryRecorder *TrajectoryRecorder
	pendingMessagesFn  func(workerID string) string
	runtimeRegistry    *agent.RuntimeRegistry // Phase 3: pluggable agent backends
}

// projectStoreReader is the subset of project.Store needed by Spawner.
type projectStoreReader interface {
	GetTask(id string) (*project.Task, bool)
}

func NewSpawner(
	tmuxClient tmux.TmuxClient,
	gitOps gitops.GitOps,
	sup *supervisor.Supervisor,
	sessionMgr *session.Manager,
) *Spawner {
	return &Spawner{
		tmuxClient:     tmuxClient,
		gitOps:         gitOps,
		sup:            sup,
		sessionMgr:     sessionMgr,
		tierConfigs:    make(map[WorkerTier]TierSpawnConfig),
		skillProfiles:  make(map[string]config.SkillProfile),
		skillOverrides: make(map[string]config.SkillProfileOverride),
	}
}

// SetTrajectoryRecorder sets the trajectory recorder for this spawner.
func (s *Spawner) SetTrajectoryRecorder(rec *TrajectoryRecorder) {
	s.trajectoryRecorder = rec
}

// TrajectoryRecorder returns the trajectory recorder, if set.
func (s *Spawner) TrajectoryRecorder() *TrajectoryRecorder {
	return s.trajectoryRecorder
}

// recordTrajectory is a convenience method that records a trajectory entry if the
// recorder is configured. Logs but does not propagate errors.
func (s *Spawner) recordTrajectory(entry TrajectoryEntry) {
	if s.trajectoryRecorder == nil {
		return
	}
	if err := s.trajectoryRecorder.Record(entry); err != nil {
		log.Printf("WARNING: trajectory record failed: %v", err)
	}
}

// LoadTierConfigs populates spawn configurations from config.
func (s *Spawner) LoadTierConfigs(tiers []config.WorkerTierConfig) {
	for _, tc := range tiers {
		tier := WorkerTier(tc.Tier)
		sc := TierSpawnConfig{
			CLITool: tc.CLITool,
			CLIArgs: tc.CLIArgs,
		}
		if tc.ReadyCheck != "" {
			if re, err := regexp.Compile(tc.ReadyCheck); err == nil {
				sc.ReadyCheck = re
			}
		}
		s.tierConfigs[tier] = sc
	}
}

// SetLanguage sets the prompt language ("en" or "zh-TW").
func (s *Spawner) SetLanguage(lang string) {
	s.language = lang
}

// SetProjectStore sets the project store for dependency context lookups.
func (s *Spawner) SetProjectStore(ps projectStoreReader) {
	s.projectStore = ps
}

// SetUseWorktrees enables git worktree isolation per task.
func (s *Spawner) SetUseWorktrees(enabled bool) {
	s.useWorktrees = enabled
}

func (s *Spawner) worktreePath(repoPath, taskID string) string {
	return filepath.Join(repoPath, ".worktrees", taskID)
}

// buildKarpathyOverlay generates targeted behavioral guidelines based on rejection history.
// Returns empty string if no violation tags are present.
func buildKarpathyOverlay(t *project.Task, lang string) string {
	if len(t.RejectionHistory) == 0 {
		return ""
	}

	seen := make(map[string]bool)
	for _, r := range t.RejectionHistory {
		for _, tag := range r.ViolationTags {
			seen[tag] = true
		}
	}
	if len(seen) == 0 {
		return ""
	}

	guidelines := config.KarpathyGuidelines()
	var sb strings.Builder

	if lang == "zh-TW" {
		sb.WriteString("--- 行為準則（根據先前審查回饋）---\n")
	} else {
		sb.WriteString("--- Behavioral Guidelines (from prior review feedback) ---\n")
	}

	for tag := range seen {
		if g, ok := guidelines[tag]; ok {
			sb.WriteString(g)
			sb.WriteString("\n\n")
		}
	}

	if lang == "zh-TW" {
		sb.WriteString("--- 準則結束 ---\n\n")
	} else {
		sb.WriteString("--- End Guidelines ---\n\n")
	}

	return sb.String()
}

// MaxDelegationDepth is the maximum nesting level for delegated tasks.
const MaxDelegationDepth = 2

// shouldIncludeDelegation returns true if the task's depth allows further delegation.
func shouldIncludeDelegation(t *project.Task) bool {
	return t.DelegationDepth < MaxDelegationDepth
}

const maxGraphReportLen = 4000

// readGraphReport reads the Graphify GRAPH_REPORT.md if it exists in the repo.
// Returns formatted section string, or empty string if not found.
func readGraphReport(repoPath string) string {
	reportPath := filepath.Join(repoPath, "graphify-out", "GRAPH_REPORT.md")
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return ""
	}
	content := string(data)
	if len(content) > maxGraphReportLen {
		content = content[:maxGraphReportLen] + "\n... (truncated)"
	}
	var sb strings.Builder
	sb.WriteString("--- Project Architecture (Knowledge Graph) ---\n")
	sb.WriteString(content)
	sb.WriteString("\n--- End Architecture ---\n\n")
	return sb.String()
}

// SetPersonalityStore sets the personality store for skill score lookups.
func (s *Spawner) SetPersonalityStore(ps *personality.Store) {
	s.personalityStore = ps
}

// LoadSkillProfiles populates skill profile configurations from config.
func (s *Spawner) LoadSkillProfiles(profiles []config.SkillProfile) {
	for _, sp := range profiles {
		s.skillProfiles[sp.ID] = sp
	}
}

// SetKnowledgeInjector sets the knowledge injector for prompt enrichment.
func (s *Spawner) SetKnowledgeInjector(inj *knowledge.Injector) {
	s.knowledgeInjector = inj
}

// KnowledgeInjector returns the knowledge injector (may be nil).
func (s *Spawner) KnowledgeInjector() *knowledge.Injector {
	return s.knowledgeInjector
}

// SetPendingMessages registers a callback that returns formatted pending
// messages for a given worker ID. This is used to inject mailbox messages
// into the worker prompt at spawn time.
func (s *Spawner) SetPendingMessages(fn func(workerID string) string) {
	s.pendingMessagesFn = fn
}

// SetRuntimeRegistry wires a runtime registry for plugin-based spawning.
// When set, spawnForTaskInner will prefer the matching runtime over the legacy
// inline CLI logic.
func (s *Spawner) SetRuntimeRegistry(r *agent.RuntimeRegistry) {
	s.runtimeRegistry = r
}

// LoadSkillOverrides populates per-worker skill profile overrides from config.
func (s *Spawner) LoadSkillOverrides(overrides map[string]config.SkillProfileOverride) {
	for k, v := range overrides {
		s.skillOverrides[k] = v
	}
}

// buildSkillArgs converts a worker's skill profile into CLI flags.
// Per-worker overrides (from retro results) are applied on top of the base profile.
// If the worker has a skill tree, growth-based config takes precedence.
func (s *Spawner) buildSkillArgs(w *Worker) string {
	if w.SkillTree != nil {
		return s.buildGrowthSkillArgs(w)
	}
	sp, ok := s.skillProfiles[w.SkillProfile]
	if !ok {
		if w.SkillProfile != "" {
			log.Printf("WARNING: skill profile %q not found for worker %s (%s), spawning without restrictions", w.SkillProfile, w.ID, w.Name)
		}
		return ""
	}

	// Apply per-worker overrides
	override, hasOverride := s.skillOverrides[w.ID]

	systemPrompt := sp.SystemPrompt
	if hasOverride && override.ExtraPrompt != "" {
		systemPrompt += "\n\n" + override.ExtraPrompt
	}

	model := sp.Model
	if hasOverride && override.ModelOverride != "" {
		model = override.ModelOverride
	}

	// Build allowed tools with override additions/removals
	allowedTools := append([]string{}, sp.AllowedTools...)
	if hasOverride {
		for _, tool := range override.AddTools {
			allowedTools = append(allowedTools, tool)
		}
		if len(override.RemoveTools) > 0 {
			removeSet := make(map[string]bool, len(override.RemoveTools))
			for _, t := range override.RemoveTools {
				removeSet[t] = true
			}
			filtered := allowedTools[:0]
			for _, t := range allowedTools {
				if !removeSet[t] {
					filtered = append(filtered, t)
				}
			}
			allowedTools = filtered
		}
	}

	var parts []string
	if systemPrompt != "" {
		parts = append(parts, "--append-system-prompt", shellEscape(systemPrompt))
	}
	if len(allowedTools) > 0 {
		// Each tool is a separate argument: --allowedTools "Bash" "Edit" "Read"
		parts = append(parts, "--allowedTools")
		for _, tool := range allowedTools {
			parts = append(parts, shellEscape(tool))
		}
	}
	// Merge profile-specific and global autonomous disallowed tools.
	// Global tools (Skill, EnterPlanMode, ExitPlanMode) prevent infinite loops
	// caused by interactive skills overriding worker instructions.
	disallowed := mergeDisallowedTools(sp.DisallowedTools, config.AutonomousDisallowedTools())
	if len(disallowed) > 0 {
		parts = append(parts, "--disallowedTools")
		for _, tool := range disallowed {
			parts = append(parts, shellEscape(tool))
		}
	}
	if model != "" {
		parts = append(parts, "--model", model)
	}
	if sp.PermissionMode != "" {
		if sp.PermissionMode == "bypassPermissions" {
			// Use --dangerously-skip-permissions to avoid the interactive confirmation prompt
			parts = append(parts, "--dangerously-skip-permissions")
		} else {
			parts = append(parts, "--permission-mode", sp.PermissionMode)
		}
	}
	if sp.ExtraCLIArgs != "" {
		parts = append(parts, sp.ExtraCLIArgs)
	}
	return strings.Join(parts, " ")
}

// buildGrowthSkillArgs derives CLI flags from the worker's skill tree level.
func (s *Spawner) buildGrowthSkillArgs(w *Worker) string {
	dominant := w.SkillTree.DominantBranch()
	cfg := growth.EffectiveConfig(w.SkillTree, dominant)

	// Use ais-agent flags when CLITool is ais-agent
	if cfg.CLITool == "ais-agent" {
		return s.buildAISAgentArgs(w, cfg)
	}

	// Default: Claude Code CLI flags
	var parts []string

	if cfg.Model != "" {
		parts = append(parts, "--model", cfg.Model)
	}

	switch cfg.PermissionMode {
	case "bypassPermissions":
		parts = append(parts, "--dangerously-skip-permissions")
	case "acceptEdits":
		parts = append(parts, "--permission-mode", "acceptEdits")
	case "plan":
		parts = append(parts, "--permission-mode", "plan")
	}

	if len(cfg.AllowedTools) > 0 {
		parts = append(parts, "--allowedTools")
		for _, tool := range cfg.AllowedTools {
			parts = append(parts, shellEscape(tool))
		}
	}

	// Always block interactive skill tools for autonomous workers.
	// Use mergeDisallowedTools for consistency with buildSkillArgs and to
	// support future growth configs that may carry their own disallowed tools.
	disallowed := mergeDisallowedTools(nil, config.AutonomousDisallowedTools())
	if len(disallowed) > 0 {
		parts = append(parts, "--disallowedTools")
		for _, tool := range disallowed {
			parts = append(parts, shellEscape(tool))
		}
	}

	if cfg.ExtraPrompt != "" {
		parts = append(parts, "--append-system-prompt", shellEscape(cfg.ExtraPrompt))
	}

	if w.SkillProfile != "" {
		if sp, ok := s.skillProfiles[w.SkillProfile]; ok && sp.SystemPrompt != "" {
			parts = append(parts, "--append-system-prompt", shellEscape(sp.SystemPrompt))
		}
	}

	return strings.Join(parts, " ")
}

// buildAISAgentArgs builds CLI flags for the ais-agent runtime.
// Note: ais-agent does not load .claude/ skills or SessionStart hooks,
// so it is not susceptible to the interactive skill loop. No disallowed
// tools are needed here. If ais-agent gains skill support in the future,
// add autonomousDisallowedTools enforcement here as well.
func (s *Spawner) buildAISAgentArgs(w *Worker, cfg growth.LevelConfig) string {
	var parts []string

	if cfg.Provider != "" {
		parts = append(parts, "--provider", cfg.Provider)
	}
	if cfg.Model != "" {
		parts = append(parts, "--model", cfg.Model)
	}
	if cfg.PermissionMode != "" {
		parts = append(parts, "--permission-mode", cfg.PermissionMode)
	}
	if len(cfg.AllowedTools) > 0 {
		parts = append(parts, "--allowed-tools", strings.Join(cfg.AllowedTools, ","))
	}
	if cfg.MaxTokenBudget > 0 {
		parts = append(parts, "--max-tokens", fmt.Sprintf("%d", cfg.MaxTokenBudget))
	}
	if cfg.ExtraPrompt != "" {
		parts = append(parts, "--append-system-prompt", shellEscape(cfg.ExtraPrompt))
	}
	if w.SkillProfile != "" {
		if sp, ok := s.skillProfiles[w.SkillProfile]; ok && sp.SystemPrompt != "" {
			parts = append(parts, "--append-system-prompt", shellEscape(sp.SystemPrompt))
		}
	}

	return strings.Join(parts, " ")
}

// SpawnForTask creates a tmux session, sets up the git branch, launches Claude Code,
// sends the task prompt, and wires the session into the existing supervisor pipeline.
// Retries up to 3 times with exponential backoff on failure.
func (s *Spawner) SpawnForTask(ctx context.Context, w *Worker, t *project.Task, p *project.Project) error {
	backoffs := []time.Duration{0, 2 * time.Second, 5 * time.Second, 10 * time.Second}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			// Check tmux server health before retry
			if err := s.checkTmuxServer(); err != nil {
				log.Printf("WARNING: tmux server unhealthy before retry %d: %v", attempt, err)
				time.Sleep(backoffs[attempt])
				continue
			}
			time.Sleep(backoffs[attempt])
			log.Printf("SpawnForTask: retry %d for worker %s task %s", attempt, w.ID, t.ID)
		}
		if err := s.spawnForTaskInner(ctx, w, t, p); err != nil {
			lastErr = err
			action := ClassifyError(err)
			log.Printf("SpawnForTask: error classified as %s: %v", action, err)
			switch action {
			case ActionAbandon:
				return fmt.Errorf("spawn abandoned (unrecoverable): %w", err)
			case ActionCompress:
				log.Printf("SpawnForTask: context too long for worker %s, cannot compress at spawn", w.ID)
				return fmt.Errorf("spawn failed (context too long): %w", err)
			default:
				continue
			}
		}
		return nil
	}
	return lastErr
}

// checkTmuxServer verifies the tmux server is running.
func (s *Spawner) checkTmuxServer() error {
	// Try listing sessions — if tmux server is down this will fail
	_, err := s.tmuxClient.ListSessions()
	return err
}

// spawnForTaskInner is the actual spawn implementation.
func (s *Spawner) spawnForTaskInner(ctx context.Context, w *Worker, t *project.Task, p *project.Project) error {
	tmuxName := fmt.Sprintf("aiworker-%s", w.ID)

	isNonCodeTask := t.Type == project.TaskTypeResearch ||
		t.Type == project.TaskTypePRD || t.Type == project.TaskTypeDesign ||
		t.Type == project.TaskTypeAdmin || t.Type == project.TaskTypeHR
	// Training tasks use git branches like code tasks

	// 1. Create git branch if it doesn't exist (skip for non-code tasks)
	if !isNonCodeTask && t.BranchName != "" {
		exists, err := s.gitOps.BranchExists(p.RepoPath, t.BranchName)
		if err != nil {
			return fmt.Errorf("checking branch: %w", err)
		}
		if !exists {
			if err := s.gitOps.CreateBranch(p.RepoPath, t.BranchName, p.BaseBranch); err != nil {
				return fmt.Errorf("creating branch %s: %w", t.BranchName, err)
			}
		}
	}

	// 2. Set up working directory (worktree or repo root)
	var workDir string
	if s.useWorktrees && !isNonCodeTask && t.BranchName != "" {
		wtPath := s.worktreePath(p.RepoPath, t.ID)
		if err := s.gitOps.CreateWorktree(p.RepoPath, wtPath, t.BranchName); err != nil {
			return fmt.Errorf("create worktree: %w", err)
		}
		workDir = wtPath
		t.WorktreePath = wtPath
	} else {
		workDir = p.RepoPath
	}

	// 2b. Phase 3: delegate to AgentRuntime plugin when a registry is wired.
	// This path replaces the legacy inline tmux/CLI bootstrap below.
	if s.runtimeRegistry != nil {
		if rt, ok := s.runtimeRegistry.Get(s.resolveRuntimeName(w)); ok && rt != nil {
			return s.spawnViaRuntime(ctx, rt, w, t, p, workDir)
		}
	}

	// 3. Create tmux session (kill stale session if it exists)
	if exists, _ := s.tmuxClient.HasSession(tmuxName); exists {
		s.tmuxClient.KillSession(tmuxName)
	}
	if err := s.tmuxClient.CreateSession(tmuxName); err != nil {
		// Cleanup worktree if session creation fails
		if t.WorktreePath != "" {
			if wtErr := s.gitOps.CleanupWorktree(p.RepoPath, t.WorktreePath); wtErr != nil {
				log.Printf("WARNING: failed to cleanup worktree after session failure: %v", wtErr)
			}
			t.WorktreePath = ""
		}
		return fmt.Errorf("creating tmux session: %w", err)
	}

	// 4. cd to working directory
	s.tmuxClient.SendKeys(tmuxName, 0, 0, fmt.Sprintf("cd %s", shellEscape(workDir))+" Enter")
	s.waitForPaneContent(ctx, tmuxName, isShellPromptReady, 5*time.Second)

	// 5. Checkout task branch (only when NOT using worktrees — worktree is already on correct branch)
	if !s.useWorktrees && !isNonCodeTask && t.BranchName != "" {
		s.tmuxClient.SendKeys(tmuxName, 0, 0, fmt.Sprintf("git checkout %s", t.BranchName)+" Enter")
		s.waitForPaneContent(ctx, tmuxName, isShellPromptReady, 5*time.Second)
	}

	// 6. Launch CLI tool (claude or aider)
	// Unset CLAUDECODE to avoid "nested session" detection when the supervisor
	// itself is running inside a Claude Code session (e.g. during development).
	s.tmuxClient.SendKeys(tmuxName, 0, 0, "unset CLAUDECODE"+" Enter")
	s.waitForPaneContent(ctx, tmuxName, isShellPromptReady, 3*time.Second)

	cliTool, cliArgs, readyRe := s.resolveCLI(w)

	// For autonomous tasks (PRD, design), force bypass permissions so the CLI
	// can run bash commands without interactive confirmation prompts.
	if isNonCodeTask && !strings.Contains(cliArgs, "--dangerously-skip-permissions") {
		cliArgs = strings.TrimSpace(cliArgs + " --dangerously-skip-permissions")
	}

	if cliArgs != "" {
		s.tmuxClient.SendKeys(tmuxName, 0, 0, fmt.Sprintf("%s %s", cliTool, cliArgs)+" Enter")
	} else {
		s.tmuxClient.SendKeys(tmuxName, 0, 0, cliTool+" Enter")
	}

	// 7. Wait for CLI to be ready
	if err := s.waitForReady(ctx, tmuxName, 120*time.Second, readyRe); err != nil {
		// Don't kill session on failure to allow debugging
		log.Printf("WARNING: CLI ready timeout for %s in tmux session %s", cliTool, tmuxName)
		return fmt.Errorf("waiting for %s ready: %w", cliTool, err)
	}

	// 8. Send task prompt
	deps := s.resolveDeps(t)
	prompt := s.buildPromptForTier(t, p, w.EffectiveTier(), deps)

	// Append skill score guidance from personality profile
	if s.personalityStore != nil {
		if profile := s.personalityStore.GetProfile(w.ID); profile != nil {
			skillPrompt := personality.GenerateSkillPrompt(profile.SkillScores)
			if skillPrompt != "" {
				prompt += skillPrompt
			}
		}
	}

	s.tmuxClient.SendLiteralKeys(tmuxName, 0, 0, prompt)
	// Wait for the CLI to finish rendering the pasted text before pressing Enter.
	// Scale delay with prompt length to handle large multi-line prompts.
	delay := promptRenderDelay(len(prompt))
	time.Sleep(delay)
	s.tmuxClient.SendKeys(tmuxName, 0, 0, "Enter")

	// Record trajectory: spawn (before prompt)
	s.recordTrajectory(TrajectoryEntry{
		Timestamp: time.Now(),
		WorkerID:  w.ID,
		TaskID:    t.ID,
		Event:     TrajectoryEventSpawn,
		Details:   fmt.Sprintf("spawned %s in tmux session %s", cliTool, tmuxName),
	})

	// Record trajectory: prompt_sent
	s.recordTrajectory(TrajectoryEntry{
		Timestamp: time.Now(),
		WorkerID:  w.ID,
		TaskID:    t.ID,
		Event:     TrajectoryEventPromptSent,
		Details:   fmt.Sprintf("prompt sent (%d chars)", len(prompt)),
	})

	// 9. Update worker state
	w.TmuxSession = tmuxName
	w.Window = 0
	w.Pane = 0
	w.Status = WorkerWorking
	w.CurrentTaskID = t.ID

	// 10. Create MonitoredSession and register with supervisor
	toolType := "claude_code"
	if cliTool == "aider" {
		toolType = "aider"
	}
	ms := &session.MonitoredSession{
		ID:          fmt.Sprintf("worker-%s", w.ID),
		Name:        fmt.Sprintf("Worker: %s", w.Name),
		TmuxSession: tmuxName,
		Window:      0,
		Pane:        0,
		ToolType:    toolType,
		TaskGoal:    t.Title,
		ProjectDir:  p.RepoPath,
		Status:      session.StatusActive,
	}
	w.SessionID = ms.ID

	if s.sessionMgr != nil {
		s.sessionMgr.Add(ms)
	}

	// 11. Wire into supervisor monitoring pipeline
	if s.sup != nil {
		go s.sup.Monitor(ctx, ms)
	}

	return nil
}

// promptRenderDelay returns how long to wait after SendLiteralKeys before pressing Enter.
// Scales with prompt length: 1s base + 500ms per 2000 chars, capped at 5s.
func promptRenderDelay(promptLen int) time.Duration {
	base := 1 * time.Second
	extra := time.Duration(promptLen/2000) * 500 * time.Millisecond
	total := base + extra
	if total > 5*time.Second {
		total = 5 * time.Second
	}
	return total
}

// compressRejectionHistory builds a text summary of a task's rejection history.
// If total Reason text across all rejections fits within maxLen, all reasons are
// returned verbatim. Otherwise, only the last 2 rejections are kept in full and
// older ones are compressed to a one-line summary with violation tags.
func compressRejectionHistory(t *project.Task, maxLen int) string {
	if len(t.RejectionHistory) == 0 {
		return ""
	}

	// Calculate total length of all reason text
	totalLen := 0
	for _, r := range t.RejectionHistory {
		totalLen += len(r.Reason)
	}

	var sb strings.Builder

	// If under limit or 2 or fewer rejections, return all in full
	if totalLen <= maxLen || len(t.RejectionHistory) <= 2 {
		for i, r := range t.RejectionHistory {
			if i > 0 {
				sb.WriteString("\n\n")
			}
			sb.WriteString(fmt.Sprintf("Rejection %d", i+1))
			if len(r.ViolationTags) > 0 {
				sb.WriteString(fmt.Sprintf(" [%s]", strings.Join(r.ViolationTags, ", ")))
			}
			sb.WriteString(":\n")
			sb.WriteString(r.Reason)
		}
		return sb.String()
	}

	// Over limit with >2 rejections: compress old ones, keep last 2 in full
	oldCount := len(t.RejectionHistory) - 2
	allTags := make(map[string]bool)
	for _, r := range t.RejectionHistory[:oldCount] {
		for _, tag := range r.ViolationTags {
			allTags[tag] = true
		}
	}

	tagList := make([]string, 0, len(allTags))
	for tag := range allTags {
		tagList = append(tagList, tag)
	}
	// Sort for deterministic output
	sort.Strings(tagList)

	sb.WriteString(fmt.Sprintf("Previously rejected %d times for: [%s]", oldCount, strings.Join(tagList, ", ")))

	// Append last 2 in full
	recent := t.RejectionHistory[oldCount:]
	for i, r := range recent {
		sb.WriteString("\n\n")
		sb.WriteString(fmt.Sprintf("Rejection %d", oldCount+i+1))
		if len(r.ViolationTags) > 0 {
			sb.WriteString(fmt.Sprintf(" [%s]", strings.Join(r.ViolationTags, ", ")))
		}
		sb.WriteString(":\n")
		sb.WriteString(r.Reason)
	}

	return sb.String()
}

// SendPromptToExisting sends a prompt to an already-running tmux session.
// Used for resume after pause, where the session is still alive.
func (s *Spawner) SendPromptToExisting(w *Worker, prompt string) error {
	if w.TmuxSession == "" {
		return fmt.Errorf("worker has no tmux session")
	}
	has, err := s.tmuxClient.HasSession(w.TmuxSession)
	if err != nil || !has {
		return fmt.Errorf("tmux session %s not found", w.TmuxSession)
	}
	s.tmuxClient.SendLiteralKeys(w.TmuxSession, w.Window, w.Pane, prompt)
	time.Sleep(1 * time.Second)
	s.tmuxClient.SendKeys(w.TmuxSession, w.Window, w.Pane, "Enter")
	return nil
}

// Cleanup kills the tmux session for a worker.
//
// Plugin runtimes generate random session names (e.g. "claude-<nanos>-<hex>")
// and store them in w.TmuxSession, so prefer that when set. Fall back to the
// legacy "aiworker-<id>" scheme for workers spawned via the inline path.
func (s *Spawner) Cleanup(w *Worker) error {
	tmuxName := w.TmuxSession
	if tmuxName == "" {
		tmuxName = fmt.Sprintf("aiworker-%s", w.ID)
	}
	has, err := s.tmuxClient.HasSession(tmuxName)
	if err != nil {
		return err
	}
	if has {
		return s.tmuxClient.KillSession(tmuxName)
	}
	return nil
}

// resolveCLI determines the CLI command, args, and ready regex for a worker.
func (s *Spawner) resolveCLI(w *Worker) (cliTool, cliArgs string, readyRe *regexp.Regexp) {
	cliTool = "claude"
	if w.CLITool != "" {
		cliTool = w.CLITool
	}

	// Growth system can override CLITool based on skill level
	if w.SkillTree != nil {
		dominant := w.SkillTree.DominantBranch()
		cfg := growth.EffectiveConfig(w.SkillTree, dominant)
		if cfg.CLITool != "" {
			cliTool = cfg.CLITool
		}
	}

	if tc, ok := s.tierConfigs[w.EffectiveTier()]; ok {
		if tc.CLITool != "" {
			cliTool = tc.CLITool
		}
		cliArgs = tc.CLIArgs
		readyRe = tc.ReadyCheck
	}

	// Append skill profile flags
	if w.SkillProfile != "" {
		skillArgs := s.buildSkillArgs(w)
		if skillArgs != "" {
			if cliArgs != "" {
				cliArgs = cliArgs + " " + skillArgs
			} else {
				cliArgs = skillArgs
			}
		}
	}

	return cliTool, cliArgs, readyRe
}

// waitForPaneContent polls the pane every 200ms until checkFn returns true or timeout.
func (s *Spawner) waitForPaneContent(ctx context.Context, tmuxSession string, checkFn func(string) bool, timeout time.Duration) {
	deadline := time.After(timeout)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-deadline:
			return
		case <-ticker.C:
			content, err := s.tmuxClient.CapturePane(tmuxSession, 0, 0, 5)
			if err != nil {
				continue
			}
			if checkFn(content) {
				return
			}
		}
	}
}

// isShellPromptReady checks if a shell prompt ($, %, #) is visible.
func isShellPromptReady(content string) bool {
	lines := strings.Split(content, "\n")
	for i := len(lines) - 1; i >= 0 && i >= len(lines)-3; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}
		if strings.HasSuffix(trimmed, "$") || strings.HasSuffix(trimmed, "%") || strings.HasSuffix(trimmed, "#") {
			return true
		}
	}
	return false
}

// waitForReady polls the pane content until the CLI shows its prompt indicator.
// If readyRe is provided, it is used instead of default Claude Code detection.
// It also auto-accepts the --dangerously-skip-permissions confirmation dialog.
func (s *Spawner) waitForReady(ctx context.Context, tmuxSession string, timeout time.Duration, readyRe *regexp.Regexp) error {
	deadline := time.After(timeout)
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()

	pollCount := 0
	bypassAccepted := false
	bypassAcceptedAt := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			return fmt.Errorf("timeout waiting for CLI ready (polled %d times)", pollCount)
		case <-ticker.C:
			pollCount++
			content, err := s.tmuxClient.CapturePane(tmuxSession, 0, 0, 10)
			if err != nil {
				continue
			}

			// Auto-accept CLI confirmation dialogs.
			// There are two types:
			// 1. Trust folder dialog: "Yes, I trust this folder" is option 1 (top) → just Enter
			// 2. Skip-permissions dialog: "No, exit" is on top, "Yes, I accept" is below → Down + Enter
			if !bypassAccepted && (strings.Contains(content, "No, exit") || strings.Contains(content, "I accept")) {
				if strings.Contains(content, "I trust this folder") {
					// Trust folder dialog: "Yes, I trust" is already selected (option 1)
					log.Printf("waitForReady[%s] trust folder dialog detected, auto-accepting", tmuxSession)
					s.tmuxClient.SendKeys(tmuxSession, 0, 0, "Enter")
				} else {
					// Skip-permissions dialog: need Down to select "Yes, I accept"
					log.Printf("waitForReady[%s] bypass permissions dialog detected, auto-accepting", tmuxSession)
					s.tmuxClient.SendKeys(tmuxSession, 0, 0, "Down")
					time.Sleep(300 * time.Millisecond)
					s.tmuxClient.SendKeys(tmuxSession, 0, 0, "Enter")
				}
				bypassAccepted = true
				bypassAcceptedAt = pollCount
				continue
			}

			// Skip ready detection while the bypass confirmation dialog is showing
			// or shortly after accepting it (give CLI time to initialize).
			if strings.Contains(content, "No, exit") || strings.Contains(content, "I accept") {
				if !bypassAccepted {
					continue // will be handled by the auto-accept block above on next poll
				}
				// Already accepted but dialog text still in pane buffer — wait for it to clear
				if pollCount-bypassAcceptedAt < 10 {
					continue
				}
			}

			lines := strings.Split(content, "\n")
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if readyRe != nil {
					if readyRe.MatchString(trimmed) {
						return nil
					}
				} else {
					// Default Claude Code detection.
					// Claude Code v2+ uses "❯" (U+276F) as its prompt character.
					if trimmed == ">" || strings.HasPrefix(trimmed, "> ") ||
						trimmed == "\u276f" || strings.HasPrefix(trimmed, "\u276f ") ||
						strings.Contains(line, "What can I help") ||
						strings.Contains(line, "Welcome back") {
						log.Printf("waitForReady[%s] CLI ready", tmuxSession)
						return nil
					}
				}
			}
		}
	}
}

// buildPromptForTier formats the task prompt based on the worker tier and language.
func (s *Spawner) buildPromptForTier(t *project.Task, p *project.Project, tier WorkerTier, deps []depContext) string {
	prompt := s.buildPromptForTierInner(t, p, tier, deps)
	// Append knowledge context if injector is available
	if s.knowledgeInjector != nil && t.AssigneeID != "" {
		tier := knowledge.TierL2RoomRecall
		if t.Type == project.TaskTypePRD {
			tier = knowledge.TierL3DeepSearch
		}
		knowledgeCtx, err := s.knowledgeInjector.BuildContext(t.AssigneeID, t.ProjectID, tier)
		if err == nil && knowledgeCtx != "" {
			prompt += "\n\n" + knowledgeCtx
		}
	}

	// Inject pending mailbox messages for this worker
	if s.pendingMessagesFn != nil && t.AssigneeID != "" {
		if msgs := s.pendingMessagesFn(t.AssigneeID); msgs != "" {
			prompt += "\n\n## Pending Messages\n\n" + msgs + "\n\nReply with REPLY:{messageID}:{your response} if needed.\n"
		}
	}

	return prompt
}

func (s *Spawner) buildPromptForTierInner(t *project.Task, p *project.Project, tier WorkerTier, deps []depContext) string {
	if t.Type == project.TaskTypeResearch {
		return s.buildResearchPrompt(t, deps)
	}

	if t.Type == project.TaskTypeTraining {
		return s.buildTrainingPrompt(t, p)
	}

	if t.Type == project.TaskTypeAdmin {
		return s.buildAdminPrompt(t, deps)
	}
	if t.Type == project.TaskTypeHR {
		return s.buildHRPrompt(t, deps)
	}

	// PRD and design tasks use their pre-built prompt directly
	if t.Type == project.TaskTypePRD || t.Type == project.TaskTypeDesign {
		prompt := t.Prompt
		lang := s.language
		if lang == "" {
			lang = "zh-TW"
		}
		if t.Type == project.TaskTypeDesign {
			if lang == "en" {
				prompt += "\n\nOutput design documents to the docs/design/ directory. Create the directory if it doesn't exist.\nWhen done, commit your changes. The system will detect completion automatically once your turn finishes — do NOT type /stop."
			} else {
				prompt += "\n\n將設計文件輸出到 docs/design/ 目錄。如果目錄不存在，請建立它。\n完成時，提交變更。回合結束時系統會自動偵測完成，**不要**輸入 /stop。"
			}
		}
		// Add verification instructions for PRD/design tasks too
		verifyCmd := t.VerifyCmd
		if verifyCmd == "" && p != nil {
			verifyCmd = p.VerifyCmd
		}
		if verifyCmd != "" {
			if lang == "en" {
				prompt += fmt.Sprintf("\n\n--- Self-Test ---\nBefore committing, run: %s\nIf it fails, fix the issues and try again. Iterate until it passes.", verifyCmd)
			} else {
				prompt += fmt.Sprintf("\n\n--- 自我測試 ---\n提交前請執行：%s\n如果失敗，請修正問題並重試。反覆修正直到通過。", verifyCmd)
			}
		}
		return prompt
	}

	lang := s.language
	if lang == "" {
		lang = "zh-TW"
	}

	// Inject compressed rejection history if the task has been rejected before
	rejectionContext := compressRejectionHistory(t, 3000)

	var sb strings.Builder

	if lang == "en" {
		if overlay := buildKarpathyOverlay(t, "en"); overlay != "" {
			sb.WriteString(overlay)
		}
		if graphReport := readGraphReport(p.RepoPath); graphReport != "" {
			sb.WriteString(graphReport)
		}
		sb.WriteString("IMPORTANT: Start writing code IMMEDIATELY. Do NOT create planning documents, design docs, or architecture files. Write code directly.\n\n")
		sb.WriteString(fmt.Sprintf("Project: %s\n", p.Name))
		if p.Description != "" {
			sb.WriteString(fmt.Sprintf("Description: %s\n\n", p.Description))
		}
		sb.WriteString(t.Prompt)
		if rejectionContext != "" {
			sb.WriteString("\n\n--- Previous Rejection History ---\n")
			sb.WriteString(rejectionContext)
			sb.WriteString("\n--- End Rejection History ---\n")
		}
		if t.BranchName != "" {
			sb.WriteString(fmt.Sprintf("\n\nYou are working on branch: %s", t.BranchName))
		}
		if len(deps) > 0 {
			sb.WriteString("\n\n--- Completed Dependencies ---\n")
			for _, d := range deps {
				sb.WriteString(fmt.Sprintf("- %s (branch: %s)\n", d.Title, d.Branch))
			}
			sb.WriteString("You may reference or build on the code from these branches.\n")
		}
		// Self-iteration instructions when VerifyCmd is available
		verifyCmd := t.VerifyCmd
		if verifyCmd == "" {
			verifyCmd = p.VerifyCmd
		}
		if verifyCmd != "" {
			sb.WriteString("\n\n--- Self-Test Loop ---\n")
			sb.WriteString("BEFORE committing, you MUST run this verification command and iterate until it passes:\n")
			sb.WriteString(fmt.Sprintf("  %s\n\n", verifyCmd))
			sb.WriteString("Follow this loop:\n")
			sb.WriteString("1. Make your changes\n")
			sb.WriteString("2. Run the verification command above\n")
			sb.WriteString("3. If it PASSES → commit\n")
			sb.WriteString("4. If it FAILS → read the output, fix the issues, go back to step 2\n")
			sb.WriteString("5. Repeat up to 5 times. If still failing, commit your best attempt\n")
			sb.WriteString("\nDo NOT skip verification. Do NOT commit code that fails the check.\n")
		}

		sb.WriteString("\n\n--- When Done ---\n")
		sb.WriteString("Commit all changes with a descriptive message. The system will detect completion automatically once your turn finishes — do NOT type /stop.\n")
		if (tier == TierManager || tier == TierConsultant) && shouldIncludeDelegation(t) {
			sb.WriteString("\n--- Delegation ---\n")
			sb.WriteString("If you need to delegate sub-tasks to subordinates, output EXACTLY this JSON format:\n")
			sb.WriteString(`{"delegate": [{"title": "Short task title", "prompt": "Detailed step-by-step instructions for the assignee", "priority": 1}]}`)
			sb.WriteString("\nFields:\n")
			sb.WriteString("- title (required): concise task name\n")
			sb.WriteString("- prompt (required): full instructions the assignee will receive\n")
			sb.WriteString("- priority (required): 1 = highest, 2 = normal, 3 = low\n")
			sb.WriteString("Only output this JSON block when you have concrete work to delegate. Do NOT delegate if you can complete the task yourself.\n")
		}
	} else {
		if overlay := buildKarpathyOverlay(t, "zh-TW"); overlay != "" {
			sb.WriteString(overlay)
		}
		if graphReport := readGraphReport(p.RepoPath); graphReport != "" {
			sb.WriteString(graphReport)
		}
		sb.WriteString("重要：請立即開始寫程式碼。不要建立規劃文件、設計文件或架構文件。直接寫程式碼。\n\n")
		sb.WriteString(fmt.Sprintf("專案：%s\n", p.Name))
		if p.Description != "" {
			sb.WriteString(fmt.Sprintf("描述：%s\n\n", p.Description))
		}
		sb.WriteString(t.Prompt)
		if rejectionContext != "" {
			sb.WriteString("\n\n--- 過往退回記錄 ---\n")
			sb.WriteString(rejectionContext)
			sb.WriteString("\n--- 退回記錄結束 ---\n")
		}
		if t.BranchName != "" {
			sb.WriteString(fmt.Sprintf("\n\n你正在分支上工作：%s", t.BranchName))
		}
		if len(deps) > 0 {
			sb.WriteString("\n\n--- 已完成的依賴項目 ---\n")
			for _, d := range deps {
				sb.WriteString(fmt.Sprintf("- %s（分支：%s）\n", d.Title, d.Branch))
			}
			sb.WriteString("你可以參考或基於這些分支的程式碼進行開發。\n")
		}

		// Self-iteration instructions when VerifyCmd is available
		verifyCmd := t.VerifyCmd
		if verifyCmd == "" {
			verifyCmd = p.VerifyCmd
		}
		if verifyCmd != "" {
			sb.WriteString("\n\n--- 自我測試迴圈 ---\n")
			sb.WriteString("提交前，你必須執行以下驗證指令並反覆修正直到通過：\n")
			sb.WriteString(fmt.Sprintf("  %s\n\n", verifyCmd))
			sb.WriteString("按照以下流程操作：\n")
			sb.WriteString("1. 完成修改\n")
			sb.WriteString("2. 執行上面的驗證指令\n")
			sb.WriteString("3. 如果通過 → 提交\n")
			sb.WriteString("4. 如果失敗 → 閱讀輸出，修正問題，回到步驟 2\n")
			sb.WriteString("5. 最多重複 5 次。如果仍然失敗，提交你最好的版本\n")
			sb.WriteString("\n不要跳過驗證。不要提交無法通過檢查的程式碼。\n")
		}

		sb.WriteString("\n\n--- 完成時 ---\n")
		sb.WriteString("用描述性訊息提交所有變更。回合結束時系統會自動偵測完成，**不要**輸入 /stop。\n")
		if (tier == TierManager || tier == TierConsultant) && shouldIncludeDelegation(t) {
			sb.WriteString("\n--- 委派 ---\n")
			sb.WriteString("當你需要將工作委派給下屬時，請嚴格按照以下 JSON 格式輸出：\n")
			sb.WriteString(`{"delegate": [{"title": "簡短任務標題", "prompt": "給被指派者的詳細步驟指示", "priority": 1}]}`)
			sb.WriteString("\n欄位說明：\n")
			sb.WriteString("- title（必填）：簡潔的任務名稱\n")
			sb.WriteString("- prompt（必填）：被指派者將收到的完整指示\n")
			sb.WriteString("- priority（必填）：1 = 最高、2 = 一般、3 = 低\n")
			sb.WriteString("只有在你有具體工作需要委派時才輸出此 JSON。如果你自己能完成任務，請不要委派。\n")
		}
	}

	return sb.String()
}

// buildTrainingPrompt creates a prompt for agentic training iterations.
func (s *Spawner) buildTrainingPrompt(t *project.Task, p *project.Project) string {
	lang := s.language
	if lang == "" {
		lang = "zh-TW"
	}

	cfg := t.TrainingConfig
	if cfg == nil {
		// Fallback: use the task prompt directly
		return t.Prompt
	}

	var sb strings.Builder

	// Build score trend string from history
	scoreTrend := ""
	if len(cfg.ScoreHistory) > 0 {
		parts := make([]string, len(cfg.ScoreHistory))
		for i, s := range cfg.ScoreHistory {
			parts[i] = fmt.Sprintf("%.4f", s)
		}
		scoreTrend = strings.Join(parts, " → ")
	}

	if lang == "en" {
		sb.WriteString(fmt.Sprintf("TRAINING ITERATION %d/%d\n\n", cfg.CurrentIter+1, cfg.MaxIterations))
		sb.WriteString(fmt.Sprintf("Project: %s\n", p.Name))
		sb.WriteString(fmt.Sprintf("Goal: %s\n\n", t.Prompt))

		if cfg.CurrentIter > 0 {
			sb.WriteString("--- Previous Iteration Results ---\n")
			sb.WriteString(fmt.Sprintf("Best Score So Far: %.4f\n", cfg.BestScore))
			if scoreTrend != "" {
				sb.WriteString(fmt.Sprintf("Score Trend: %s\n", scoreTrend))
			}
			if cfg.LastTestOutput != "" {
				sb.WriteString(fmt.Sprintf("Last Test Output:\n%s\n", cfg.LastTestOutput))
			}
			sb.WriteString("\n")
		}

		sb.WriteString("--- Instructions ---\n")
		sb.WriteString("1. Analyze the current code and test results\n")
		sb.WriteString("2. Make improvements to achieve a higher score\n")
		sb.WriteString(fmt.Sprintf("3. Target score: >= %.4f\n", cfg.PassThreshold))
		sb.WriteString(fmt.Sprintf("4. Test command: %s\n", cfg.TestCmd))
		if cfg.BenchmarkCmd != "" {
			sb.WriteString(fmt.Sprintf("5. Benchmark command: %s\n", cfg.BenchmarkCmd))
			sb.WriteString("6. When done with your changes, commit. The system will detect completion automatically once your turn finishes — do NOT type /stop.\n")
		} else {
			sb.WriteString("5. When done with your changes, commit. The system will detect completion automatically once your turn finishes — do NOT type /stop.\n")
		}
		sb.WriteString("\nIMPORTANT: Focus on making targeted improvements. Do NOT rewrite everything.\n")
		if scoreTrend != "" {
			sb.WriteString("Look at the score trend above — if scores are flat, try a fundamentally different approach.\n")
		}
		if t.BranchName != "" {
			sb.WriteString(fmt.Sprintf("\nYou are working on branch: %s\n", t.BranchName))
		}
	} else {
		sb.WriteString(fmt.Sprintf("訓練迭代 第 %d/%d 輪\n\n", cfg.CurrentIter+1, cfg.MaxIterations))
		sb.WriteString(fmt.Sprintf("專案：%s\n", p.Name))
		sb.WriteString(fmt.Sprintf("目標：%s\n\n", t.Prompt))

		if cfg.CurrentIter > 0 {
			sb.WriteString("--- 上一輪迭代結果 ---\n")
			sb.WriteString(fmt.Sprintf("目前最佳分數：%.4f\n", cfg.BestScore))
			if scoreTrend != "" {
				sb.WriteString(fmt.Sprintf("分數趨勢：%s\n", scoreTrend))
			}
			if cfg.LastTestOutput != "" {
				sb.WriteString(fmt.Sprintf("上一輪測試輸出：\n%s\n", cfg.LastTestOutput))
			}
			sb.WriteString("\n")
		}

		sb.WriteString("--- 操作指引 ---\n")
		sb.WriteString("1. 分析目前程式碼和測試結果\n")
		sb.WriteString("2. 進行改進以獲得更高分數\n")
		sb.WriteString(fmt.Sprintf("3. 目標分數：>= %.4f\n", cfg.PassThreshold))
		sb.WriteString(fmt.Sprintf("4. 測試指令：%s\n", cfg.TestCmd))
		if cfg.BenchmarkCmd != "" {
			sb.WriteString(fmt.Sprintf("5. 基準測試指令：%s\n", cfg.BenchmarkCmd))
			sb.WriteString("6. 完成修改後，提交變更。回合結束時系統會自動偵測完成，**不要**輸入 /stop。\n")
		} else {
			sb.WriteString("5. 完成修改後，提交變更。回合結束時系統會自動偵測完成，**不要**輸入 /stop。\n")
		}
		sb.WriteString("\n重要：專注於針對性改進，不要重寫所有程式碼。\n")
		if scoreTrend != "" {
			sb.WriteString("觀察上方分數趨勢 — 如果分數持平，嘗試根本不同的方法。\n")
		}
		if t.BranchName != "" {
			sb.WriteString(fmt.Sprintf("\n你正在分支上工作：%s\n", t.BranchName))
		}
	}

	return sb.String()
}

// buildResearchPrompt creates a prompt for research tasks that instructs the
// worker to investigate a topic and output a structured JSON report.
func (s *Spawner) buildResearchPrompt(t *project.Task, deps []depContext) string {
	lang := s.language
	if lang == "" {
		lang = "zh-TW"
	}

	var sb strings.Builder

	if lang == "en" {
		sb.WriteString("You are a professional researcher. Please conduct an in-depth investigation on the following topic:\n\n")
		sb.WriteString(t.Prompt)
		if len(deps) > 0 {
			sb.WriteString("\n\n--- Related Prior Research ---\n")
			for _, d := range deps {
				sb.WriteString(fmt.Sprintf("- %s\n", d.Title))
			}
		}
		sb.WriteString("\n\n--- Output Format ---\n")
		sb.WriteString("After completing your research, output the following JSON report (must be valid JSON):\n")
		sb.WriteString(`{"summary": "Research summary (under 200 words)", "keyFindings": ["Finding 1", ...], "recommendations": ["Recommendation 1", ...], "references": ["Reference 1", ...], "rawContent": "Full research content in markdown"}`)
		sb.WriteString("\n\n--- When Done ---\n")
		sb.WriteString("After outputting the JSON above, your turn naturally ends — the system will detect completion automatically. Do NOT type /stop.\n")
	} else {
		sb.WriteString("你是一位專業研究員。請針對以下主題進行深入調查研究：\n\n")
		sb.WriteString(t.Prompt)
		if len(deps) > 0 {
			sb.WriteString("\n\n--- 相關前置研究 ---\n")
			for _, d := range deps {
				sb.WriteString(fmt.Sprintf("- %s\n", d.Title))
			}
		}
		sb.WriteString("\n\n--- 輸出格式 ---\n")
		sb.WriteString("研究完成後，請輸出以下 JSON 格式的報告（必須是合法 JSON）：\n")
		sb.WriteString(`{"summary": "研究摘要 (200字以內)", "keyFindings": ["發現1", "發現2", ...], "recommendations": ["建議1", ...], "references": ["參考資料1", ...], "rawContent": "完整研究內容 markdown"}`)
		sb.WriteString("\n\n--- When Done ---\n")
		sb.WriteString("將上述 JSON 輸出後，回合自然結束 — 系統會自動偵測完成。**不要**輸入 /stop。\n")
	}

	return sb.String()
}

// buildAdminPrompt creates a prompt for administrative document tasks.
func (s *Spawner) buildAdminPrompt(t *project.Task, deps []depContext) string {
	lang := s.language
	if lang == "" {
		lang = "zh-TW"
	}

	var sb strings.Builder

	if lang == "en" {
		sb.WriteString("You are an administrative assistant. Generate the requested document.\n\n")
		sb.WriteString("IMPORTANT: You are working autonomously without a human operator.\n")
		sb.WriteString("- Do NOT ask questions or use AskUserQuestion\n")
		sb.WriteString("- Do NOT invoke brainstorming, writing-plans, or any interactive skills\n")
		sb.WriteString("- Do NOT use EnterPlanMode or ExitPlanMode\n\n")
		sb.WriteString(t.Prompt)
		if len(deps) > 0 {
			sb.WriteString("\n\n--- Related Documents ---\n")
			for _, d := range deps {
				sb.WriteString(fmt.Sprintf("- %s\n", d.Title))
			}
		}
		sb.WriteString("\n\n--- Instructions ---\n")
		sb.WriteString("1. Check docs/templates/ for a matching Markdown template\n")
		sb.WriteString("2. If found, read the template and fill in fields from the task description\n")
		sb.WriteString("3. If not found, create a clean Markdown document from scratch\n")
		sb.WriteString("4. Output the completed document to docs/output/ (create directory if needed)\n")
		sb.WriteString("5. Commit your changes with a descriptive message. The system will detect completion automatically once your turn finishes — do NOT type /stop.\n")
	} else {
		sb.WriteString("你是一位行政助理。請產生所要求的文件。\n\n")
		sb.WriteString("重要：你正在自主工作，沒有人類操作員。\n")
		sb.WriteString("- 不要詢問問題、不要使用 AskUserQuestion\n")
		sb.WriteString("- 不要使用 brainstorming、writing-plans 或其他互動式技能\n")
		sb.WriteString("- 不要使用 EnterPlanMode 或 ExitPlanMode\n\n")
		sb.WriteString(t.Prompt)
		if len(deps) > 0 {
			sb.WriteString("\n\n--- 相關文件 ---\n")
			for _, d := range deps {
				sb.WriteString(fmt.Sprintf("- %s\n", d.Title))
			}
		}
		sb.WriteString("\n\n--- 操作指引 ---\n")
		sb.WriteString("1. 檢查 docs/templates/ 中是否有相關的 Markdown 模板\n")
		sb.WriteString("2. 如果有模板，讀取並根據任務描述填入欄位\n")
		sb.WriteString("3. 如果沒有模板，用 Markdown 從頭建立乾淨的文件\n")
		sb.WriteString("4. 將完成的文件輸出到 docs/output/（如目錄不存在請建立）\n")
		sb.WriteString("5. 用描述性訊息提交變更。回合結束時系統會自動偵測完成，**不要**輸入 /stop。\n")
	}

	return sb.String()
}

// buildHRPrompt creates a prompt for HR/recruitment tasks.
func (s *Spawner) buildHRPrompt(t *project.Task, deps []depContext) string {
	lang := s.language
	if lang == "" {
		lang = "zh-TW"
	}

	var sb strings.Builder

	if lang == "en" {
		sb.WriteString("You are an HR specialist. Find matching skill profiles for the given job description.\n\n")
		sb.WriteString("IMPORTANT: You are working autonomously without a human operator.\n")
		sb.WriteString("- Do NOT ask questions or use AskUserQuestion\n")
		sb.WriteString("- Do NOT invoke brainstorming, writing-plans, or any interactive skills\n")
		sb.WriteString("- Do NOT use EnterPlanMode or ExitPlanMode\n\n")
		sb.WriteString(t.Prompt)
		if len(deps) > 0 {
			sb.WriteString("\n\n--- Related Context ---\n")
			for _, d := range deps {
				sb.WriteString(fmt.Sprintf("- %s\n", d.Title))
			}
		}
		sb.WriteString("\n\n--- SkillsMP API Reference ---\n")
		sb.WriteString("- Search: https://skillsmp.com/api/skills/search?q=<query>\n")
		sb.WriteString("- AI Search: https://skillsmp.com/api/skills/ai-search?q=<query>\n")
		sb.WriteString("- Raw SKILL.md: https://raw.githubusercontent.com/<owner>/<repo>/main/SKILL.md\n")
		sb.WriteString("\n--- Instructions ---\n")
		sb.WriteString("1. Analyze the job description and identify required skills\n")
		sb.WriteString("2. Search SkillsMP for matching profiles using WebFetch\n")
		sb.WriteString("3. Produce a recruitment report in Markdown with: candidate profiles, match scores, recommendations\n")
		sb.WriteString("4. Output the report to docs/hr/ (create directory if needed)\n")
		sb.WriteString("5. Commit your changes with a descriptive message. The system will detect completion automatically once your turn finishes — do NOT type /stop.\n")
	} else {
		sb.WriteString("你是一位人資專員。請根據職缺描述搜尋匹配的技能配置。\n\n")
		sb.WriteString("重要：你正在自主工作，沒有人類操作員。\n")
		sb.WriteString("- 不要詢問問題、不要使用 AskUserQuestion\n")
		sb.WriteString("- 不要使用 brainstorming、writing-plans 或其他互動式技能\n")
		sb.WriteString("- 不要使用 EnterPlanMode 或 ExitPlanMode\n\n")
		sb.WriteString(t.Prompt)
		if len(deps) > 0 {
			sb.WriteString("\n\n--- 相關背景 ---\n")
			for _, d := range deps {
				sb.WriteString(fmt.Sprintf("- %s\n", d.Title))
			}
		}
		sb.WriteString("\n\n--- SkillsMP API 參考 ---\n")
		sb.WriteString("- 搜尋：https://skillsmp.com/api/skills/search?q=<query>\n")
		sb.WriteString("- AI 搜尋：https://skillsmp.com/api/skills/ai-search?q=<query>\n")
		sb.WriteString("- 原始 SKILL.md：https://raw.githubusercontent.com/<owner>/<repo>/main/SKILL.md\n")
		sb.WriteString("\n--- 操作指引 ---\n")
		sb.WriteString("1. 分析職缺描述，識別所需技能\n")
		sb.WriteString("2. 使用 WebFetch 在 SkillsMP 搜尋匹配的配置\n")
		sb.WriteString("3. 用 Markdown 產生招募報告：候選配置、匹配分數、建議\n")
		sb.WriteString("4. 將報告輸出到 docs/hr/（如目錄不存在請建立）\n")
		sb.WriteString("5. 用描述性訊息提交變更。回合結束時系統會自動偵測完成，**不要**輸入 /stop。\n")
	}

	return sb.String()
}

// depContext holds resolved dependency info for prompt building.
type depContext struct {
	Title  string
	Branch string
}

// resolveDeps looks up completed dependency tasks and returns their context.
func (s *Spawner) resolveDeps(t *project.Task) []depContext {
	if s.projectStore == nil || len(t.DependsOn) == 0 {
		return nil
	}
	var deps []depContext
	for _, depID := range t.DependsOn {
		if dt, ok := s.projectStore.GetTask(depID); ok {
			deps = append(deps, depContext{Title: dt.Title, Branch: dt.BranchName})
		}
	}
	return deps
}

// resolveRuntimeName picks which runtime plugin to use for a worker.
//
// Resolution order — applied in sequence, each overwriting the previous
// so the LAST matching source wins:
//   1. Seed: "claude"
//   2. w.CLITool (if non-empty)
//   3. Growth config from w.SkillTree (if set and non-empty)
//   4. Tier config (if found and non-empty)
//
// Effective precedence (highest first): tier > growth > worker > default.
// This mirrors resolveCLI so legacy and plugin paths agree on CLI choice.
func (s *Spawner) resolveRuntimeName(w *Worker) string {
	name := "claude"
	if w != nil && w.CLITool != "" {
		name = w.CLITool
	}
	if w != nil && w.SkillTree != nil {
		gcfg := growth.EffectiveConfig(w.SkillTree, w.SkillTree.DominantBranch())
		if gcfg.CLITool != "" {
			name = gcfg.CLITool
		}
	}
	if w != nil {
		if tc, ok := s.tierConfigs[w.EffectiveTier()]; ok && tc.CLITool != "" {
			name = tc.CLITool
		}
	}
	return name
}

// buildSpawnConfig assembles an agent.SpawnConfig from the spawner's current
// view of the worker's skill profile, growth config, and tier config. The
// returned SpawnConfig carries enough information for any AgentRuntime plugin
// to build its own CLI command without reaching back into spawner internals.
func (s *Spawner) buildSpawnConfig(w *Worker, p *project.Project, workDir string, branch string) agent.SpawnConfig {
	cfg := agent.SpawnConfig{
		WorkDir: workDir,
		Branch:  branch,
		EnvVars: make(map[string]string),
	}

	// Start with the base skill profile (if any).
	if w != nil && w.SkillProfile != "" {
		if sp, ok := s.skillProfiles[w.SkillProfile]; ok {
			cfg.SystemPrompt = sp.SystemPrompt
			cfg.AllowedTools = append([]string(nil), sp.AllowedTools...)
			cfg.DisallowedTools = append([]string(nil), sp.DisallowedTools...)
			cfg.Model = sp.Model
			cfg.PermissionMode = sp.PermissionMode
			cfg.ExtraCLIArgs = sp.ExtraCLIArgs
		}
	}

	// Growth/skill-tree can override model/permissions/tools and surface
	// ais-agent-only knobs via EnvVars.
	if w != nil && w.SkillTree != nil {
		gcfg := growth.EffectiveConfig(w.SkillTree, w.SkillTree.DominantBranch())
		if gcfg.Model != "" {
			cfg.Model = gcfg.Model
		}
		if gcfg.PermissionMode != "" {
			cfg.PermissionMode = gcfg.PermissionMode
		}
		if len(gcfg.AllowedTools) > 0 {
			cfg.AllowedTools = append([]string(nil), gcfg.AllowedTools...)
		}
		if gcfg.ExtraPrompt != "" {
			if cfg.SystemPrompt != "" {
				cfg.SystemPrompt += "\n\n" + gcfg.ExtraPrompt
			} else {
				cfg.SystemPrompt = gcfg.ExtraPrompt
			}
		}
		// ais-agent surfaces Provider and MaxTokenBudget via EnvVars so the
		// shared SpawnConfig struct can stay backend-agnostic (see
		// internal/agent/aisagent/runtime.go docs).
		if gcfg.Provider != "" {
			cfg.EnvVars["AIS_PROVIDER"] = gcfg.Provider
		}
		if gcfg.MaxTokenBudget > 0 {
			cfg.EnvVars["AIS_MAX_TOKENS"] = strconv.Itoa(gcfg.MaxTokenBudget)
		}
	}

	// Apply per-worker skill overrides on top of the profile and growth merge.
	// This mirrors the legacy buildSkillArgs logic so that Phase-2 retro
	// overrides (ExtraPrompt, ModelOverride, AddTools, RemoveTools) continue
	// to flow through when the plugin-runtime path is active.
	if w != nil {
		if override, ok := s.skillOverrides[w.ID]; ok {
			if override.ExtraPrompt != "" {
				if cfg.SystemPrompt != "" {
					cfg.SystemPrompt += "\n\n" + override.ExtraPrompt
				} else {
					cfg.SystemPrompt = override.ExtraPrompt
				}
			}
			if override.ModelOverride != "" {
				cfg.Model = override.ModelOverride
			}
			if len(override.AddTools) > 0 {
				cfg.AllowedTools = append(cfg.AllowedTools, override.AddTools...)
			}
			if len(override.RemoveTools) > 0 {
				removeSet := make(map[string]bool, len(override.RemoveTools))
				for _, t := range override.RemoveTools {
					removeSet[t] = true
				}
				filtered := cfg.AllowedTools[:0]
				for _, t := range cfg.AllowedTools {
					if !removeSet[t] {
						filtered = append(filtered, t)
					}
				}
				cfg.AllowedTools = filtered
			}
		}
	}

	// Enforce autonomous DisallowedTools safety net. Runtimes that don't load
	// .claude/ skills (e.g. ais-agent) may simply forward these flags without
	// ill effect.
	cfg.DisallowedTools = mergeDisallowedTools(cfg.DisallowedTools, config.AutonomousDisallowedTools())

	return cfg
}

// spawnViaRuntime uses an AgentRuntime plugin for the CLI lifecycle. Git setup
// and workdir selection happen in the caller (spawnForTaskInner); this function
// owns Spawn → DetectReady → SendPrompt → MonitoredSession creation.
//
// Runtime fallback: if rt.DetectReady fails and rt is NOT already "claude"
// AND ctx is still alive, this function cleans up the failed session and
// retries once with the "claude" runtime, using a SANITIZED SpawnConfig
// (cfg.Model is cleared because runtime-specific model names like "llama3"
// from ais-agent's ollama provider are rejected by claude). If the claude
// fallback also fails, the returned error wraps ErrRuntimeFallbackExhausted
// so the outer SpawnForTask retry loop abandons the task instead of
// re-triggering the same failure chain (ClassifyError routes the sentinel
// to ActionAbandon).
func (s *Spawner) spawnViaRuntime(
	ctx context.Context, rt agent.AgentRuntime,
	w *Worker, t *project.Task, p *project.Project, workDir string,
) error {
	cfg := s.buildSpawnConfig(w, p, workDir, t.BranchName)

	// For autonomous / non-code tasks (PRD, design, research, admin, HR), force
	// bypass permissions so the CLI can run bash without interactive prompts.
	isNonCodeTask := t.Type == project.TaskTypeResearch ||
		t.Type == project.TaskTypePRD || t.Type == project.TaskTypeDesign ||
		t.Type == project.TaskTypeAdmin || t.Type == project.TaskTypeHR
	if isNonCodeTask && cfg.PermissionMode != "bypassPermissions" {
		cfg.PermissionMode = "bypassPermissions"
	}

	// When worktrees are disabled, the legacy inline path runs `git checkout`
	// inside the tmux pane so the worker lands on its task branch. Plugin
	// runtimes only `cd $WorkDir`, so we must check out the branch here before
	// the runtime spawns — otherwise code tasks run on whatever branch happens
	// to be checked out in RepoPath.
	if !s.useWorktrees && !isNonCodeTask && t.BranchName != "" {
		if s.gitOps != nil {
			if err := s.gitOps.Checkout(p.RepoPath, t.BranchName); err != nil {
				return fmt.Errorf("runtime %s checkout %s: %w", rt.Name(), t.BranchName, err)
			}
		} else {
			log.Printf("WARNING: spawnViaRuntime[%s] no gitOps configured — worker will run on current branch of %s, not %s",
				rt.Name(), p.RepoPath, t.BranchName)
		}
	}

	return s.spawnViaRuntimeWithConfig(ctx, rt, cfg, w, t, p, workDir)
}

// sanitizeForClaudeFallback returns a copy of cfg with fields cleared or
// normalized that would have been runtime-specific to whatever non-claude
// plugin was tried first.
//
// Cleared fields:
//   - Model — value like "llama3" or "gpt-4o-mini" from ais-agent's growth
//     config makes claude idle forever printing "Run /model to pick a
//     different model". Empty Model lets claude pick its built-in default
//     (currently the latest sonnet variant).
//   - ExtraCLIArgs — today this is SkillProfile-sourced (spawner.go
//     buildSpawnConfig) and the bundled SkillProfiles are claude-shaped, so
//     in practice it's safe. But the field is structurally a runtime-
//     specific verbatim flag string ("--max-tokens 50000" for ais-agent
//     would error claude), so clearing it on fallback is defensive: when
//     claude is being used as a recovery substitute, we want the safest
//     possible invocation, not whatever flags suited the failed runtime.
//
// Normalized fields:
//   - PermissionMode — forced to "bypassPermissions". The growth config
//     for low-level workers (growth/config_mapper.go level 1) sets
//     PermissionMode to "plan", which makes claude print proposed actions
//     and wait indefinitely for human approval — incompatible with this
//     project's autonomous-worker model where every prompt instructs the
//     CLI to "work without a human operator". When claude is the fallback
//     of last resort, the autonomous contract still applies, so we
//     override the growth-supplied mode to ensure the worker can actually
//     execute.
//
// Pass-through fields (intentionally NOT cleared because they're either
// runtime-agnostic or claude ignores irrelevant entries):
//   - AllowedTools, DisallowedTools, SystemPrompt — runtime agnostic.
//     Claude needs the same safety guarantees as the original.
//   - EnvVars — claude reads no AIS_* keys; harmless leftovers stay.
//   - WorkDir, Branch — same task target.
//
// The shallow copy via `out := cfg` shares the underlying AllowedTools /
// DisallowedTools slices and EnvVars map with the input. The claude
// runtime treats these as read-only (see claudecode/runtime.go
// buildCLICommand), so callers can rely on the input cfg not being
// mutated by sanitize-then-Spawn. Future runtime authors must keep this
// read-only invariant.
func sanitizeForClaudeFallback(cfg agent.SpawnConfig) agent.SpawnConfig {
	out := cfg
	out.Model = ""
	out.ExtraCLIArgs = ""
	out.PermissionMode = "bypassPermissions"
	return out
}

// spawnViaRuntimeWithConfig is the inner half of spawnViaRuntime that takes
// a pre-built SpawnConfig. Splitting it lets the fallback path supply a
// sanitized cfg (different Model) without re-running buildSpawnConfig and
// without recursion shenanigans.
func (s *Spawner) spawnViaRuntimeWithConfig(
	ctx context.Context, rt agent.AgentRuntime, cfg agent.SpawnConfig,
	w *Worker, t *project.Task, p *project.Project, workDir string,
) error {
	// Early compatibility check. Validate is a fast pure inspection of cfg —
	// when it fails we know the CLI would crash at startup (e.g. ais-agent
	// v0.1.0 given --provider ollama) and we should skip straight to the
	// claude fallback rather than spending ~120s waiting out DetectReady.
	if verr := rt.Validate(cfg); verr != nil {
		return s.tryClaudeFallback(ctx, rt, cfg, w, t, p, workDir, verr, "validate")
	}

	runtimeSess, err := rt.Spawn(ctx, cfg)
	if err != nil {
		return fmt.Errorf("runtime %s spawn: %w", rt.Name(), err)
	}

	if err := rt.DetectReady(ctx, runtimeSess, 120*time.Second); err != nil {
		_ = rt.Cleanup(runtimeSess)
		return s.tryClaudeFallback(ctx, rt, cfg, w, t, p, workDir, err, "ready")
	}

	// Build prompt (same logic as legacy path).
	deps := s.resolveDeps(t)
	prompt := s.buildPromptForTier(t, p, w.EffectiveTier(), deps)
	if s.personalityStore != nil {
		if profile := s.personalityStore.GetProfile(w.ID); profile != nil {
			skillPrompt := personality.GenerateSkillPrompt(profile.SkillScores)
			if skillPrompt != "" {
				prompt += skillPrompt
			}
		}
	}

	if err := rt.SendPrompt(runtimeSess, prompt); err != nil {
		_ = rt.Cleanup(runtimeSess)
		return fmt.Errorf("runtime %s send prompt: %w", rt.Name(), err)
	}

	// Trajectory recording (same events/content as legacy).
	s.recordTrajectory(TrajectoryEntry{
		Timestamp: time.Now(),
		WorkerID:  w.ID,
		TaskID:    t.ID,
		Event:     TrajectoryEventSpawn,
		Details:   fmt.Sprintf("spawned %s via runtime in tmux session %s", rt.Name(), runtimeSess.TmuxSession),
	})
	s.recordTrajectory(TrajectoryEntry{
		Timestamp: time.Now(),
		WorkerID:  w.ID,
		TaskID:    t.ID,
		Event:     TrajectoryEventPromptSent,
		Details:   fmt.Sprintf("prompt sent (%d chars)", len(prompt)),
	})

	// Update worker state.
	w.TmuxSession = runtimeSess.TmuxSession
	w.Window = runtimeSess.Window
	w.Pane = runtimeSess.Pane
	w.Status = WorkerWorking
	w.CurrentTaskID = t.ID

	// MonitoredSession setup: runtime owns its own tool-type tag so the
	// mapping lives next to the plugin, not in a central switch that drifts
	// as new plugins land.
	toolType := rt.MonitoredSessionType()
	if toolType == "" {
		toolType = "claude_code"
	}
	ms := &session.MonitoredSession{
		ID:          fmt.Sprintf("worker-%s", w.ID),
		Name:        fmt.Sprintf("Worker: %s", w.Name),
		TmuxSession: runtimeSess.TmuxSession,
		Window:      runtimeSess.Window,
		Pane:        runtimeSess.Pane,
		ToolType:    toolType,
		TaskGoal:    t.Title,
		ProjectDir:  p.RepoPath,
		Status:      session.StatusActive,
	}
	w.SessionID = ms.ID
	if s.sessionMgr != nil {
		s.sessionMgr.Add(ms)
	}
	if s.sup != nil {
		go s.sup.Monitor(ctx, ms)
	}

	return nil
}

// tryClaudeFallback retries the spawn via the "claude" runtime when a
// non-claude runtime fails either at Validate (fast-fail) or DetectReady
// (~120s timeout). Both failure modes share the same recovery path:
// switch to claude with a sanitized cfg so the worker isn't stuck in a
// retry loop on a permanently misconfigured plugin.
//
// Preconditions for fallback (all must hold):
//   - rt is NOT "claude" (avoid infinite recursion).
//   - ctx has not been cancelled / expired — a context error means the
//     caller wants to stop, not switch runtimes.
//   - runtimeRegistry is wired and has a "claude" runtime registered.
//
// stage is a short verb describing where in the lifecycle the failure
// happened ("validate", "ready"), used in log + error messages so the
// trail tells operators which path triggered the fallback.
//
// If claude itself fails, the returned error wraps
// ErrRuntimeFallbackExhausted so ClassifyError returns ActionAbandon and
// SpawnForTask's 3x retry loop short-circuits.
func (s *Spawner) tryClaudeFallback(
	ctx context.Context, rt agent.AgentRuntime, cfg agent.SpawnConfig,
	w *Worker, t *project.Task, p *project.Project, workDir string,
	cause error, stage string,
) error {
	if rt.Name() == "claude" || s.runtimeRegistry == nil || ctx.Err() != nil {
		return fmt.Errorf("runtime %s %s: %w", rt.Name(), stage, cause)
	}
	fallback, ok := s.runtimeRegistry.Get("claude")
	if !ok || fallback == nil {
		return fmt.Errorf("runtime %s %s: %w", rt.Name(), stage, cause)
	}
	fallbackCfg := sanitizeForClaudeFallback(cfg)
	log.Printf("spawnViaRuntime: fallback for worker %s task %s — runtime %s failed at %s (%v), retrying with claude (model %q → default)",
		w.ID, t.ID, rt.Name(), stage, cause, cfg.Model)
	if ferr := s.spawnViaRuntimeWithConfig(ctx, fallback, fallbackCfg, w, t, p, workDir); ferr != nil {
		return fmt.Errorf("%w: runtime %s %s (%v); claude fallback also failed (%v)",
			ErrRuntimeFallbackExhausted, rt.Name(), stage, cause, ferr)
	}
	return nil
}

func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// mergeDisallowedTools combines profile-specific and global disallowed tools,
// deduplicating entries.
func mergeDisallowedTools(profile, global []string) []string {
	seen := make(map[string]bool, len(profile)+len(global))
	var result []string
	for _, t := range profile {
		if !seen[t] {
			seen[t] = true
			result = append(result, t)
		}
	}
	for _, t := range global {
		if !seen[t] {
			seen[t] = true
			result = append(result, t)
		}
	}
	return result
}
