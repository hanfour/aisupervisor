package gitops

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func initTempRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run(t, dir, "git", "init")
	run(t, dir, "git", "config", "user.email", "test@test.com")
	run(t, dir, "git", "config", "user.name", "Test")
	// Create initial commit so HEAD exists
	f := filepath.Join(dir, "README.md")
	os.WriteFile(f, []byte("# test"), 0o644)
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "initial")
	return dir
}

func run(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, string(out))
	}
}

func TestCurrentBranch(t *testing.T) {
	repo := initTempRepo(t)
	g := New()

	branch, err := g.CurrentBranch(repo)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	// Could be "main" or "master" depending on git config
	if branch != "main" && branch != "master" {
		t.Fatalf("unexpected branch: %s", branch)
	}
}

func TestCreateBranchAndExists(t *testing.T) {
	repo := initTempRepo(t)
	g := New()

	baseBranch, _ := g.CurrentBranch(repo)

	exists, err := g.BranchExists(repo, "feature/test")
	if err != nil {
		t.Fatalf("BranchExists: %v", err)
	}
	if exists {
		t.Fatal("branch should not exist yet")
	}

	if err := g.CreateBranch(repo, "feature/test", baseBranch); err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}

	exists, err = g.BranchExists(repo, "feature/test")
	if err != nil {
		t.Fatalf("BranchExists after create: %v", err)
	}
	if !exists {
		t.Fatal("branch should exist after creation")
	}
}

func TestLatestCommit(t *testing.T) {
	repo := initTempRepo(t)
	g := New()

	baseBranch, _ := g.CurrentBranch(repo)
	info, err := g.LatestCommit(repo, baseBranch)
	if err != nil {
		t.Fatalf("LatestCommit: %v", err)
	}
	if info.Hash == "" {
		t.Fatal("expected non-empty hash")
	}
	if info.Message != "initial" {
		t.Fatalf("expected message 'initial', got %q", info.Message)
	}
	if info.Author != "Test" {
		t.Fatalf("expected author 'Test', got %q", info.Author)
	}
}

func TestHasUncommitted(t *testing.T) {
	repo := initTempRepo(t)
	g := New()

	has, err := g.HasUncommitted(repo)
	if err != nil {
		t.Fatalf("HasUncommitted: %v", err)
	}
	if has {
		t.Fatal("should not have uncommitted changes")
	}

	// Create uncommitted file
	os.WriteFile(filepath.Join(repo, "new.txt"), []byte("hello"), 0o644)

	has, err = g.HasUncommitted(repo)
	if err != nil {
		t.Fatalf("HasUncommitted after change: %v", err)
	}
	if !has {
		t.Fatal("should have uncommitted changes")
	}
}

func TestDiffBranch(t *testing.T) {
	repo := initTempRepo(t)
	g := New()

	baseBranch, _ := g.CurrentBranch(repo)

	// Create a feature branch with a new file
	run(t, repo, "git", "branch", "feature/diff-test", baseBranch)
	run(t, repo, "git", "checkout", "feature/diff-test")
	os.WriteFile(filepath.Join(repo, "new-file.go"), []byte("package main\n"), 0o644)
	run(t, repo, "git", "add", ".")
	run(t, repo, "git", "commit", "-m", "add new file")
	run(t, repo, "git", "checkout", baseBranch)

	diff, err := g.DiffBranch(repo, "feature/diff-test")
	if err != nil {
		t.Fatalf("DiffBranch: %v", err)
	}
	if diff == "" {
		t.Fatal("expected non-empty diff")
	}
	if !strings.Contains(diff, "new-file.go") {
		t.Fatalf("diff should mention new-file.go, got: %s", diff)
	}
}

func TestLatestCommitHash(t *testing.T) {
	repo := initTempRepo(t)
	g := New()

	baseBranch, _ := g.CurrentBranch(repo)

	hash, err := g.LatestCommitHash(repo, baseBranch)
	if err != nil {
		t.Fatalf("LatestCommitHash: %v", err)
	}
	if len(hash) < 40 {
		t.Fatalf("expected full SHA hash, got %q", hash)
	}

	// Verify it matches LatestCommit
	info, _ := g.LatestCommit(repo, baseBranch)
	if hash != info.Hash {
		t.Fatalf("LatestCommitHash %q != LatestCommit.Hash %q", hash, info.Hash)
	}
}

func TestBranchName(t *testing.T) {
	name := BranchName("p123", "t456", "add-login")
	expected := "ai/p123/t456-add-login"
	if name != expected {
		t.Fatalf("expected %q, got %q", expected, name)
	}
}
