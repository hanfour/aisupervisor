package gitops

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateWorktree(t *testing.T) {
	repo := initTempRepo(t)
	g := New()

	// Create a branch first
	baseBranch, err := g.CurrentBranch(repo)
	if err != nil {
		t.Fatal(err)
	}
	err = g.CreateBranch(repo, "feature/test-wt", baseBranch)
	if err != nil {
		t.Fatal(err)
	}

	wtPath := filepath.Join(t.TempDir(), "worktree-test")
	err = g.CreateWorktree(repo, wtPath, "feature/test-wt")
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}

	// Verify worktree directory exists
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Fatal("worktree directory does not exist")
	}

	// Verify correct branch in worktree
	branch, err := g.CurrentBranch(wtPath)
	if err != nil {
		t.Fatalf("CurrentBranch in worktree: %v", err)
	}
	if branch != "feature/test-wt" {
		t.Errorf("expected branch feature/test-wt, got %s", branch)
	}
}

func TestListWorktrees(t *testing.T) {
	repo := initTempRepo(t)
	g := New()

	baseBranch, _ := g.CurrentBranch(repo)
	g.CreateBranch(repo, "feature/wt-list", baseBranch)

	wtPath := filepath.Join(t.TempDir(), "worktree-list")
	g.CreateWorktree(repo, wtPath, "feature/wt-list")

	infos, err := g.ListWorktrees(repo)
	if err != nil {
		t.Fatalf("ListWorktrees: %v", err)
	}
	// Main repo + 1 worktree = at least 2
	if len(infos) < 2 {
		t.Errorf("expected at least 2 worktrees, got %d", len(infos))
	}

	found := false
	for _, info := range infos {
		if info.Branch == "feature/wt-list" {
			found = true
		}
	}
	if !found {
		t.Error("worktree with branch feature/wt-list not found")
	}
}

func TestCleanupWorktree(t *testing.T) {
	repo := initTempRepo(t)
	g := New()

	baseBranch, _ := g.CurrentBranch(repo)
	g.CreateBranch(repo, "feature/wt-cleanup", baseBranch)

	wtPath := filepath.Join(t.TempDir(), "worktree-cleanup")
	g.CreateWorktree(repo, wtPath, "feature/wt-cleanup")

	err := g.CleanupWorktree(repo, wtPath)
	if err != nil {
		t.Fatalf("CleanupWorktree: %v", err)
	}

	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Error("worktree directory should be removed")
	}
}

func TestWorktreeIsolation(t *testing.T) {
	repo := initTempRepo(t)
	g := New()

	baseBranch, _ := g.CurrentBranch(repo)
	g.CreateBranch(repo, "feature/wt-iso-a", baseBranch)
	g.CreateBranch(repo, "feature/wt-iso-b", baseBranch)

	wtA := filepath.Join(t.TempDir(), "worktree-a")
	wtB := filepath.Join(t.TempDir(), "worktree-b")
	g.CreateWorktree(repo, wtA, "feature/wt-iso-a")
	g.CreateWorktree(repo, wtB, "feature/wt-iso-b")

	// Create file in worktree A
	os.WriteFile(filepath.Join(wtA, "only-in-a.txt"), []byte("hello"), 0644)

	// Verify file does NOT exist in worktree B
	if _, err := os.Stat(filepath.Join(wtB, "only-in-a.txt")); !os.IsNotExist(err) {
		t.Error("file created in worktree A should not appear in worktree B")
	}
}
