package company

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Phase0Check represents a single mechanical verification check.
type Phase0Check struct {
	Name    string
	Command string
	Timeout time.Duration
}

// Phase0Result holds the outcome of a single check.
type Phase0Result struct {
	Check   Phase0Check
	Passed  bool
	Output  string
	Elapsed time.Duration
}

// Phase0Report aggregates all check results.
type Phase0Report struct {
	Results  []Phase0Result
	AllGreen bool
	Summary  string
}

// AllCriticalFailed returns true if ALL checks failed (none passed).
// Returns false if there are no results (nothing failed).
func (r *Phase0Report) AllCriticalFailed() bool {
	if len(r.Results) == 0 {
		return false
	}
	for _, res := range r.Results {
		if res.Passed {
			return false
		}
	}
	return true
}

// ToFindings converts failed checks to Finding structs with Severity:"CRITICAL", Source:"phase0".
func (r *Phase0Report) ToFindings() []Finding {
	var findings []Finding
	for i, res := range r.Results {
		if res.Passed {
			continue
		}
		findings = append(findings, Finding{
			ID:       fmt.Sprintf("#p0-%d", i+1),
			Severity: "CRITICAL",
			Source:   "phase0",
			Body:     fmt.Sprintf("[%s] %s failed: %s", res.Check.Name, res.Check.Command, res.Output),
		})
	}
	return findings
}

// detectChecks auto-detects which mechanical checks to run based on project files.
func detectChecks(dir, verifyCmd string) []Phase0Check {
	var checks []Phase0Check

	// If verifyCmd is provided, add it as the primary check with a longer timeout.
	if verifyCmd != "" {
		checks = append(checks, Phase0Check{
			Name:    "verify",
			Command: verifyCmd,
			Timeout: 2 * time.Minute,
		})
	}

	// Detect Go project via go.mod
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
		checks = append(checks, Phase0Check{
			Name:    "go-vet",
			Command: "go vet ./...",
			Timeout: 60 * time.Second,
		})
	}

	// Detect Node.js project via package.json
	pkgPath := filepath.Join(dir, "package.json")
	if data, err := os.ReadFile(pkgPath); err == nil {
		var pkg struct {
			Scripts map[string]string `json:"scripts"`
		}
		if json.Unmarshal(data, &pkg) == nil && pkg.Scripts != nil {
			if _, ok := pkg.Scripts["lint"]; ok {
				checks = append(checks, Phase0Check{
					Name:    "lint",
					Command: "npm run lint",
					Timeout: 60 * time.Second,
				})
			}
			if _, ok := pkg.Scripts["typecheck"]; ok {
				checks = append(checks, Phase0Check{
					Name:    "typecheck",
					Command: "npm run typecheck",
					Timeout: 60 * time.Second,
				})
			}
		}
	}

	return checks
}

// runPhase0Checks runs all checks in parallel and returns an aggregated report.
func runPhase0Checks(ctx context.Context, workDir string, checks []Phase0Check) *Phase0Report {
	results := make([]Phase0Result, len(checks))
	var wg sync.WaitGroup

	for i, check := range checks {
		wg.Add(1)
		go func(idx int, c Phase0Check) {
			defer wg.Done()
			results[idx] = executeCheck(ctx, workDir, c)
		}(i, check)
	}
	wg.Wait()

	allGreen := true
	var summaryParts []string
	for _, r := range results {
		if !r.Passed {
			allGreen = false
			summaryParts = append(summaryParts, fmt.Sprintf("%s: FAIL (%v)", r.Check.Name, r.Elapsed.Truncate(time.Millisecond)))
		} else {
			summaryParts = append(summaryParts, fmt.Sprintf("%s: PASS (%v)", r.Check.Name, r.Elapsed.Truncate(time.Millisecond)))
		}
	}

	return &Phase0Report{
		Results:  results,
		AllGreen: allGreen,
		Summary:  strings.Join(summaryParts, "; "),
	}
}

// executeCheck runs a single check command with its timeout and returns the result.
func executeCheck(ctx context.Context, workDir string, check Phase0Check) Phase0Result {
	checkCtx, cancel := context.WithTimeout(ctx, check.Timeout)
	defer cancel()

	start := time.Now()
	cmd := exec.CommandContext(checkCtx, "sh", "-c", check.Command)
	cmd.Dir = workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	elapsed := time.Since(start)

	output := stdout.String()
	if errOutput := stderr.String(); errOutput != "" {
		if output != "" {
			output += "\n"
		}
		output += errOutput
	}
	output = truncateOutput(output, 2048)

	return Phase0Result{
		Check:   check,
		Passed:  err == nil,
		Output:  output,
		Elapsed: elapsed,
	}
}

// truncateOutput truncates s to maxLen, keeping first half + "... (truncated) ..." + last half.
func truncateOutput(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	marker := "... (truncated) ..."
	half := (maxLen - len(marker)) / 2
	return s[:half] + marker + s[len(s)-half:]
}
