package worker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/gitops"
	"github.com/hanfourmini/aisupervisor/internal/project"
	"github.com/hanfourmini/aisupervisor/internal/session"
	"github.com/hanfourmini/aisupervisor/internal/supervisor"
	"github.com/hanfourmini/aisupervisor/internal/tmux"
)

type Spawner struct {
	tmuxClient tmux.TmuxClient
	gitOps     gitops.GitOps
	sup        *supervisor.Supervisor
	sessionMgr *session.Manager
}

func NewSpawner(
	tmuxClient tmux.TmuxClient,
	gitOps gitops.GitOps,
	sup *supervisor.Supervisor,
	sessionMgr *session.Manager,
) *Spawner {
	return &Spawner{
		tmuxClient: tmuxClient,
		gitOps:     gitOps,
		sup:        sup,
		sessionMgr: sessionMgr,
	}
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

	// 5. Launch Claude Code
	s.tmuxClient.SendKeys(tmuxName, 0, 0, "claude Enter")

	// 6. Wait for Claude Code to be ready
	if err := s.waitForReady(ctx, tmuxName, 30*time.Second); err != nil {
		// Cleanup on failure
		s.tmuxClient.KillSession(tmuxName)
		return fmt.Errorf("waiting for Claude Code: %w", err)
	}

	// 7. Send task prompt
	prompt := buildPrompt(t, p)
	s.tmuxClient.SendLiteralKeys(tmuxName, 0, 0, prompt)
	s.tmuxClient.SendKeys(tmuxName, 0, 0, "Enter")

	// 8. Update worker state
	w.TmuxSession = tmuxName
	w.Window = 0
	w.Pane = 0
	w.Status = WorkerWorking
	w.CurrentTaskID = t.ID

	// 9. Create MonitoredSession and register with supervisor
	ms := &session.MonitoredSession{
		ID:          fmt.Sprintf("worker-%s", w.ID),
		Name:        fmt.Sprintf("Worker: %s", w.Name),
		TmuxSession: tmuxName,
		Window:      0,
		Pane:        0,
		ToolType:    "claude_code",
		TaskGoal:    t.Title,
		ProjectDir:  p.RepoPath,
		Status:      session.StatusActive,
	}
	w.SessionID = ms.ID

	if s.sessionMgr != nil {
		s.sessionMgr.Add(ms)
	}

	// 10. Wire into supervisor monitoring pipeline
	go s.sup.Monitor(ctx, ms)

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

// waitForReady polls the pane content until Claude Code shows its prompt indicator.
func (s *Spawner) waitForReady(ctx context.Context, tmuxSession string, timeout time.Duration) error {
	deadline := time.After(timeout)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			return fmt.Errorf("timeout waiting for Claude Code ready")
		case <-ticker.C:
			content, err := s.tmuxClient.CapturePane(tmuxSession, 0, 0, 10)
			if err != nil {
				continue
			}
			// Claude Code shows ">" as its prompt when ready
			lines := strings.Split(content, "\n")
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if trimmed == ">" || strings.HasPrefix(trimmed, "> ") || strings.Contains(line, "What can I help") {
					return nil
				}
			}
		}
	}
}

func buildPrompt(t *project.Task, p *project.Project) string {
	var sb strings.Builder
	sb.WriteString(t.Prompt)
	if t.BranchName != "" {
		sb.WriteString(fmt.Sprintf("\n\nYou are working on branch: %s", t.BranchName))
	}
	return sb.String()
}

func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
