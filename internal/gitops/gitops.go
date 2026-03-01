package gitops

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type CommitInfo struct {
	Hash    string
	Message string
	Author  string
	Date    time.Time
}

type GitOps interface {
	CurrentBranch(repoPath string) (string, error)
	BranchExists(repoPath, branch string) (bool, error)
	CreateBranch(repoPath, branch, baseBranch string) error
	LatestCommit(repoPath, branch string) (CommitInfo, error)
	HasUncommitted(repoPath string) (bool, error)
}

type gitOps struct{}

func New() GitOps {
	return &gitOps{}
}

func (g *gitOps) CurrentBranch(repoPath string) (string, error) {
	out, err := g.run(repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (g *gitOps) BranchExists(repoPath, branch string) (bool, error) {
	_, err := g.run(repoPath, "rev-parse", "--verify", "refs/heads/"+branch)
	if err == nil {
		return true, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 128 {
		return false, nil
	}
	return false, nil
}

func (g *gitOps) CreateBranch(repoPath, branch, baseBranch string) error {
	_, err := g.run(repoPath, "branch", branch, baseBranch)
	return err
}

func (g *gitOps) LatestCommit(repoPath, branch string) (CommitInfo, error) {
	out, err := g.run(repoPath, "log", "-1", "--format=%H%n%s%n%an%n%aI", branch)
	if err != nil {
		return CommitInfo{}, err
	}
	lines := strings.SplitN(strings.TrimSpace(out), "\n", 4)
	if len(lines) < 4 {
		return CommitInfo{}, fmt.Errorf("unexpected git log output")
	}
	date, _ := time.Parse(time.RFC3339, lines[3])
	return CommitInfo{
		Hash:    lines[0],
		Message: lines[1],
		Author:  lines[2],
		Date:    date,
	}, nil
}

func (g *gitOps) HasUncommitted(repoPath string) (bool, error) {
	out, err := g.run(repoPath, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

func (g *gitOps) run(repoPath string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", repoPath}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", args[0], err, string(out))
	}
	return string(out), nil
}

// BranchName generates a standardized branch name for a task.
func BranchName(projectID, taskID, slug string) string {
	return fmt.Sprintf("ai/%s/%s-%s", projectID, taskID, slug)
}
