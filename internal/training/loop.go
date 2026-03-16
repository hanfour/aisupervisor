package training

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/hanfourmini/aisupervisor/internal/project"
)

// TrainingLoop drives the code iteration cycle: run tests, compare scores,
// commit improvements or rollback regressions.
type TrainingLoop struct{}

// IterationResult captures the outcome of a single training iteration.
type IterationResult struct {
	Iteration  int
	CommitHash string
	TestOutput string
	Score      float64
	Improved   bool
	RolledBack bool
}

// EvaluateAndDecide runs the test command, parses the score, and either
// commits the changes (if improved) or rolls back (if not).
func (l *TrainingLoop) EvaluateAndDecide(repoPath, branchName string, cfg *project.TrainingTaskConfig) (*IterationResult, error) {
	if cfg.TestCmd == "" {
		return nil, fmt.Errorf("test_cmd is empty")
	}

	// Run test command
	cmd := exec.Command("bash", "-c", cfg.TestCmd)
	cmd.Dir = repoPath
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("running test cmd: %w", err)
		}
	}

	output := stdout.String() + "\n" + stderr.String()
	score := ParseScore(output, exitCode)

	result := &IterationResult{
		Iteration:  cfg.CurrentIter + 1,
		TestOutput: truncateOutput(output, 4000),
		Score:      score,
	}

	// On first iteration (BestScore==0 and CurrentIter==0), use >= so that
	// even a score of 0 creates a baseline commit. Subsequent iterations use >.
	improved := score > cfg.BestScore
	if cfg.CurrentIter == 0 && cfg.BestScore == 0 {
		improved = score >= cfg.BestScore
	}

	if improved {
		// Improved — commit changes
		result.Improved = true

		commitMsg := fmt.Sprintf("training: iteration %d, score %.4f", result.Iteration, score)
		addCmd := exec.Command("git", "add", "-A")
		addCmd.Dir = repoPath
		if out, err := addCmd.CombinedOutput(); err != nil {
			log.Printf("training loop: git add failed: %s %v", string(out), err)
		}

		commitCmd := exec.Command("git", "commit", "-m", commitMsg, "--allow-empty")
		commitCmd.Dir = repoPath
		if out, err := commitCmd.CombinedOutput(); err != nil {
			log.Printf("training loop: git commit failed: %s %v", string(out), err)
		}

		// Get the new commit hash
		hashCmd := exec.Command("git", "rev-parse", "HEAD")
		hashCmd.Dir = repoPath
		if hashOut, err := hashCmd.Output(); err == nil {
			result.CommitHash = strings.TrimSpace(string(hashOut))
		}
	} else {
		// No improvement — rollback
		result.RolledBack = true

		resetCmd := exec.Command("git", "checkout", "--", ".")
		resetCmd.Dir = repoPath
		if out, err := resetCmd.CombinedOutput(); err != nil {
			log.Printf("training loop: git checkout failed: %s %v", string(out), err)
		}
		cleanCmd := exec.Command("git", "clean", "-fd")
		cleanCmd.Dir = repoPath
		if out, err := cleanCmd.CombinedOutput(); err != nil {
			log.Printf("training loop: git clean failed: %s %v", string(out), err)
		}
	}

	return result, nil
}

// scorePatterns matches common score output formats.
var scorePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)SCORE:\s*([\d.]+)`),
	regexp.MustCompile(`(?i)accuracy:\s*([\d.]+)`),
	regexp.MustCompile(`(?i)pass[_ ]rate:\s*([\d.]+)`),
	regexp.MustCompile(`(\d+)\s+passed.*?(\d+)\s+failed`),
}

// ParseScore extracts a numeric score from test output.
// Supports: "SCORE: 0.85", "accuracy: 0.92", pass/fail counts, or exit code.
func ParseScore(output string, exitCode int) float64 {
	// Try explicit score patterns first
	for _, re := range scorePatterns[:3] {
		if m := re.FindStringSubmatch(output); len(m) >= 2 {
			if v, err := strconv.ParseFloat(m[1], 64); err == nil {
				return v
			}
		}
	}

	// Try pass/fail count pattern
	if m := scorePatterns[3].FindStringSubmatch(output); len(m) >= 3 {
		passed, _ := strconv.ParseFloat(m[1], 64)
		failed, _ := strconv.ParseFloat(m[2], 64)
		total := passed + failed
		if total > 0 {
			return passed / total
		}
	}

	// Fallback: exit code 0 = 1.0, otherwise 0.0
	if exitCode == 0 {
		return 1.0
	}
	return 0.0
}

func truncateOutput(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[len(s)-maxLen:]
}
