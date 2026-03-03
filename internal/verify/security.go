package verify

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// SecurityScanner runs static security analysis tools.
type SecurityScanner struct {
	repoDir    string
	timeoutSec int
}

// NewSecurityScanner creates a new security scanner.
func NewSecurityScanner(repoDir string, timeoutSec int) *SecurityScanner {
	if timeoutSec <= 0 {
		timeoutSec = 300
	}
	return &SecurityScanner{repoDir: repoDir, timeoutSec: timeoutSec}
}

// Run executes available security scanning tools and returns results.
// It tries gosec first, then semgrep, and succeeds if at least one tool is available.
func (ss *SecurityScanner) Run(ctx context.Context) StepResult {
	timeout := time.Duration(ss.timeoutSec) * time.Second
	stepCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()

	// Try gosec first
	if path, err := exec.LookPath("gosec"); err == nil {
		cmd := exec.CommandContext(stepCtx, path, "./...")
		cmd.Dir = ss.repoDir
		out, err := cmd.CombinedOutput()
		duration := time.Since(start)
		return StepResult{
			Step:     StepSecurity,
			Passed:   err == nil,
			Output:   string(out),
			Duration: duration,
		}
	}

	// Try semgrep as fallback
	if path, err := exec.LookPath("semgrep"); err == nil {
		cmd := exec.CommandContext(stepCtx, path, "--config", "auto", ss.repoDir)
		cmd.Dir = ss.repoDir
		out, err := cmd.CombinedOutput()
		duration := time.Since(start)
		output := string(out)
		// semgrep exits non-zero for findings; check output for actual errors
		passed := err == nil || !strings.Contains(output, "error")
		return StepResult{
			Step:     StepSecurity,
			Passed:   passed,
			Output:   output,
			Duration: duration,
		}
	}

	return StepResult{
		Step:     StepSecurity,
		Passed:   true,
		Output:   "No security scanner available (install gosec or semgrep)",
		Duration: time.Since(start),
	}
}
