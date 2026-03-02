package worker

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/config"
	"github.com/hanfourmini/aisupervisor/internal/gitops"
	"github.com/hanfourmini/aisupervisor/internal/project"
	"github.com/hanfourmini/aisupervisor/internal/session"
	"github.com/hanfourmini/aisupervisor/internal/supervisor"
	"github.com/hanfourmini/aisupervisor/internal/tmux"
)

// TierSpawnConfig holds resolved spawn parameters for a worker tier.
type TierSpawnConfig struct {
	CLITool    string
	CLIArgs    string
	ReadyCheck *regexp.Regexp
}

type Spawner struct {
	tmuxClient    tmux.TmuxClient
	gitOps        gitops.GitOps
	sup           *supervisor.Supervisor
	sessionMgr    *session.Manager
	tierConfigs   map[WorkerTier]TierSpawnConfig
	skillProfiles map[string]config.SkillProfile
}

func NewSpawner(
	tmuxClient tmux.TmuxClient,
	gitOps gitops.GitOps,
	sup *supervisor.Supervisor,
	sessionMgr *session.Manager,
) *Spawner {
	return &Spawner{
		tmuxClient:    tmuxClient,
		gitOps:        gitOps,
		sup:           sup,
		sessionMgr:    sessionMgr,
		tierConfigs:   make(map[WorkerTier]TierSpawnConfig),
		skillProfiles: make(map[string]config.SkillProfile),
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

// LoadSkillProfiles populates skill profile configurations from config.
func (s *Spawner) LoadSkillProfiles(profiles []config.SkillProfile) {
	for _, sp := range profiles {
		s.skillProfiles[sp.ID] = sp
	}
}

// buildSkillArgs converts a worker's skill profile into CLI flags.
func (s *Spawner) buildSkillArgs(w *Worker) string {
	sp, ok := s.skillProfiles[w.SkillProfile]
	if !ok {
		return ""
	}
	var parts []string
	if sp.SystemPrompt != "" {
		parts = append(parts, "--append-system-prompt", shellEscape(sp.SystemPrompt))
	}
	if len(sp.AllowedTools) > 0 {
		// Each tool is a separate argument: --allowedTools "Bash" "Edit" "Read"
		parts = append(parts, "--allowedTools")
		for _, tool := range sp.AllowedTools {
			parts = append(parts, shellEscape(tool))
		}
	}
	if len(sp.DisallowedTools) > 0 {
		parts = append(parts, "--disallowedTools")
		for _, tool := range sp.DisallowedTools {
			parts = append(parts, shellEscape(tool))
		}
	}
	if sp.Model != "" {
		parts = append(parts, "--model", sp.Model)
	}
	if sp.PermissionMode != "" {
		parts = append(parts, "--permission-mode", sp.PermissionMode)
	}
	if sp.ExtraCLIArgs != "" {
		parts = append(parts, sp.ExtraCLIArgs)
	}
	return strings.Join(parts, " ")
}

// SpawnForTask creates a tmux session, sets up the git branch, launches Claude Code,
// sends the task prompt, and wires the session into the existing supervisor pipeline.
func (s *Spawner) SpawnForTask(ctx context.Context, w *Worker, t *project.Task, p *project.Project) error {
	tmuxName := fmt.Sprintf("aiworker-%s", w.ID)

	// 1. Create git branch if it doesn't exist
	if t.BranchName != "" {
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

	// 2. Create tmux session
	if err := s.tmuxClient.CreateSession(tmuxName); err != nil {
		return fmt.Errorf("creating tmux session: %w", err)
	}

	// 3. cd to repo path
	s.tmuxClient.SendKeys(tmuxName, 0, 0, fmt.Sprintf("cd %s", shellEscape(p.RepoPath))+" Enter")
	time.Sleep(500 * time.Millisecond)

	// 4. Checkout task branch
	if t.BranchName != "" {
		s.tmuxClient.SendKeys(tmuxName, 0, 0, fmt.Sprintf("git checkout %s", t.BranchName)+" Enter")
		time.Sleep(1 * time.Second)
	}

	// 5. Launch CLI tool (claude or aider)
	cliTool, cliArgs, readyRe := s.resolveCLI(w)
	if cliArgs != "" {
		s.tmuxClient.SendKeys(tmuxName, 0, 0, fmt.Sprintf("%s %s", cliTool, cliArgs)+" Enter")
	} else {
		s.tmuxClient.SendKeys(tmuxName, 0, 0, cliTool+" Enter")
	}

	// 6. Wait for CLI to be ready
	if err := s.waitForReady(ctx, tmuxName, 30*time.Second, readyRe); err != nil {
		s.tmuxClient.KillSession(tmuxName)
		return fmt.Errorf("waiting for %s ready: %w", cliTool, err)
	}

	// 7. Send task prompt
	prompt := buildPromptForTier(t, p, w.EffectiveTier())
	s.tmuxClient.SendLiteralKeys(tmuxName, 0, 0, prompt)
	s.tmuxClient.SendKeys(tmuxName, 0, 0, "Enter")

	// 8. Update worker state
	w.TmuxSession = tmuxName
	w.Window = 0
	w.Pane = 0
	w.Status = WorkerWorking
	w.CurrentTaskID = t.ID

	// 9. Create MonitoredSession and register with supervisor
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

	// 10. Wire into supervisor monitoring pipeline
	if s.sup != nil {
		go s.sup.Monitor(ctx, ms)
	}

	return nil
}

// Cleanup kills the tmux session for a worker.
func (s *Spawner) Cleanup(w *Worker) error {
	tmuxName := fmt.Sprintf("aiworker-%s", w.ID)
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

// waitForReady polls the pane content until the CLI shows its prompt indicator.
// If readyRe is provided, it is used instead of default Claude Code detection.
func (s *Spawner) waitForReady(ctx context.Context, tmuxSession string, timeout time.Duration, readyRe *regexp.Regexp) error {
	deadline := time.After(timeout)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			return fmt.Errorf("timeout waiting for CLI ready")
		case <-ticker.C:
			content, err := s.tmuxClient.CapturePane(tmuxSession, 0, 0, 10)
			if err != nil {
				continue
			}
			lines := strings.Split(content, "\n")
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if readyRe != nil {
					if readyRe.MatchString(trimmed) {
						return nil
					}
				} else {
					// Default Claude Code detection
					if trimmed == ">" || strings.HasPrefix(trimmed, "> ") || strings.Contains(line, "What can I help") {
						return nil
					}
				}
			}
		}
	}
}

// buildPromptForTier formats the task prompt based on the worker tier.
func buildPromptForTier(t *project.Task, p *project.Project, tier WorkerTier) string {
	var sb strings.Builder
	sb.WriteString(t.Prompt)
	if t.BranchName != "" {
		sb.WriteString(fmt.Sprintf("\n\nYou are working on branch: %s", t.BranchName))
	}
	// Aider uses a simpler prompt format
	if tier == TierEngineer {
		return sb.String()
	}
	return sb.String()
}

func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
