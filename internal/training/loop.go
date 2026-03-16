package training

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/project"
)

// TrainingLoop drives the code iteration cycle: run tests, compare scores,
// commit improvements or rollback regressions.
type TrainingLoop struct{}

// IterationResult captures the outcome of a single training iteration.
type IterationResult struct {
	Iteration      int
	CommitHash     string
	TestOutput     string
	Score          float64
	BenchmarkScore float64
	Improved       bool
	RolledBack     bool
	Plateau        bool
}

// EvaluateAndDecide runs the test command, parses the score, and either
// commits the changes (if improved) or rolls back (if not).
// It supports context-based timeout, benchmark scoring, and plateau detection.
func (l *TrainingLoop) EvaluateAndDecide(ctx context.Context, repoPath, branchName string, cfg *project.TrainingTaskConfig) (*IterationResult, error) {
	if cfg.TestCmd == "" {
		return nil, fmt.Errorf("test_cmd is empty")
	}

	// Snapshot the current HEAD before running, for atomic rollback
	baseCommit, err := gitHeadHash(repoPath)
	if err != nil {
		return nil, fmt.Errorf("getting HEAD hash: %w", err)
	}

	// Determine timeout
	timeout := time.Duration(cfg.TestTimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Minute // default 5 min
	}
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Run test command with timeout
	score, output, exitCode, err := runCmd(cmdCtx, repoPath, cfg.TestCmd)
	if err != nil {
		return nil, fmt.Errorf("running test cmd: %w", err)
	}
	if score < 0 {
		score = ParseScore(output, exitCode)
	}

	// Run benchmark command if configured (separate scoring)
	var benchScore float64
	if cfg.BenchmarkCmd != "" {
		benchCtx, benchCancel := context.WithTimeout(ctx, timeout)
		defer benchCancel()
		bs, benchOut, benchExit, berr := runCmd(benchCtx, repoPath, cfg.BenchmarkCmd)
		if berr != nil {
			log.Printf("training: benchmark cmd failed: %v", berr)
		} else {
			if bs < 0 {
				bs = ParseScore(benchOut, benchExit)
			}
			benchScore = bs
		}
	}

	result := &IterationResult{
		Iteration:      cfg.CurrentIter + 1,
		TestOutput:     truncateOutput(output, 4000),
		Score:          score,
		BenchmarkScore: benchScore,
	}

	// Plateau detection: include current score before checking
	// (ScoreHistory is appended by the caller after EvaluateAndDecide returns,
	// so we check with the current score appended to get accurate detection)
	plateauLimit := cfg.PlateauLimit
	if plateauLimit <= 0 {
		plateauLimit = 3
	}
	historyWithCurrent := append(cfg.ScoreHistory, score)
	if isPlateaued(historyWithCurrent, cfg.BestScore, plateauLimit) {
		result.Plateau = true
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
		if hash, err := gitHeadHash(repoPath); err == nil {
			result.CommitHash = hash
		}
	} else {
		// No improvement — atomic rollback to pre-iteration state
		result.RolledBack = true

		resetCmd := exec.Command("git", "reset", "--hard", baseCommit)
		resetCmd.Dir = repoPath
		if out, err := resetCmd.CombinedOutput(); err != nil {
			log.Printf("training loop: git reset --hard failed: %s %v", string(out), err)
		}

		// Verify rollback succeeded
		if currentHash, err := gitHeadHash(repoPath); err == nil {
			if currentHash != baseCommit {
				log.Printf("training loop: rollback verification failed: expected %s, got %s", baseCommit, currentHash)
			}
		}
	}

	return result, nil
}

// runCmd executes a shell command with context and returns (score, output, exitCode, error).
// Score is -1 if the command output doesn't embed a score directly.
func runCmd(ctx context.Context, dir, command string) (float64, string, int, error) {
	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if ctx.Err() != nil {
			return -1, "command timed out", -1, fmt.Errorf("command timed out: %w", ctx.Err())
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return -1, "", -1, fmt.Errorf("exec error: %w", err)
		}
	}

	output := stdout.String() + "\n" + stderr.String()
	return -1, output, exitCode, nil
}

// gitHeadHash returns the current HEAD commit hash.
func gitHeadHash(repoPath string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// isPlateaued returns true if the last N scores show no improvement over bestScore.
func isPlateaued(history []float64, bestScore float64, limit int) bool {
	if len(history) < limit {
		return false
	}
	recent := history[len(history)-limit:]
	for _, s := range recent {
		if s > bestScore {
			return false
		}
	}
	// All recent scores <= bestScore → plateau
	return true
}

// scorePatterns matches common score output formats.
var scorePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)SCORE:\s*([\d.]+)`),
	regexp.MustCompile(`(?i)accuracy:\s*([\d.]+)`),
	regexp.MustCompile(`(?i)pass[_ ]rate:\s*([\d.]+)`),
	regexp.MustCompile(`(?i)(?:score|accuracy|result):\s*([\d.]+)%`),
	regexp.MustCompile(`(\d+)\s+passed.*?(\d+)\s+failed`),
}

// ParseScore extracts a numeric score from test output.
// Supports: "SCORE: 0.85", "accuracy: 92%", pass/fail counts, or exit code.
func ParseScore(output string, exitCode int) float64 {
	// Try explicit score patterns (non-percentage)
	for _, re := range scorePatterns[:3] {
		if m := re.FindStringSubmatch(output); len(m) >= 2 {
			if v, err := strconv.ParseFloat(m[1], 64); err == nil {
				return v
			}
		}
	}

	// Try percentage patterns (e.g., "accuracy: 85%")
	if m := scorePatterns[3].FindStringSubmatch(output); len(m) >= 2 {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil {
			return v / 100.0
		}
	}

	// Try pass/fail count pattern
	if m := scorePatterns[4].FindStringSubmatch(output); len(m) >= 3 {
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
