package company

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hanfourmini/aisupervisor/internal/gitops"
	"github.com/hanfourmini/aisupervisor/internal/project"
)

func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runCmd(t, dir, "git", "init")
	runCmd(t, dir, "git", "config", "user.email", "test@test.com")
	runCmd(t, dir, "git", "config", "user.name", "Test")
	f := filepath.Join(dir, "README.md")
	os.WriteFile(f, []byte("# test"), 0o644)
	runCmd(t, dir, "git", "add", ".")
	runCmd(t, dir, "git", "commit", "-m", "initial")
	return dir
}

func runCmd(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, string(out))
	}
}

func TestCleanupAfterApproval_CleansWorktree(t *testing.T) {
	repo := initTestRepo(t)
	g := gitops.New()

	baseBranch, _ := g.CurrentBranch(repo)
	g.CreateBranch(repo, "feature/wt-review", baseBranch)

	wtPath := t.TempDir()
	g.CreateWorktree(repo, wtPath, "feature/wt-review")

	rp := &ReviewPipeline{}

	task := &project.Task{
		ID:           "task-1",
		WorktreePath: wtPath,
	}

	rp.cleanupAfterApproval(task, repo, g)

	if task.WorktreePath != "" {
		t.Error("WorktreePath should be cleared after cleanup")
	}
}

func TestCleanupAfterApproval_NoOpWhenNoWorktree(t *testing.T) {
	rp := &ReviewPipeline{}
	task := &project.Task{
		ID: "task-2",
		// No WorktreePath set
	}

	// Should not panic or error
	rp.cleanupAfterApproval(task, "/nonexistent", nil)

	if task.WorktreePath != "" {
		t.Error("WorktreePath should remain empty")
	}
}
