package verify

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Config holds verification pipeline configuration.
type Config struct {
	Enabled     bool   `yaml:"enabled"`
	DockerImage string `yaml:"docker_image"`
	TimeoutSec  int    `yaml:"timeout_sec"`
	LintCmd     string `yaml:"lint_cmd"`
	BuildCmd    string `yaml:"build_cmd"`
	TestCmd     string `yaml:"test_cmd"`
}

// DefaultConfig returns sensible defaults for verification.
func DefaultConfig() Config {
	return Config{
		Enabled:     false,
		DockerImage: "golang:1.25",
		TimeoutSec:  300,
		LintCmd:     "golangci-lint run ./...",
		BuildCmd:    "go build ./...",
		TestCmd:     "go test ./...",
	}
}

// Runner executes the verification pipeline.
type Runner struct {
	cfg     Config
	repoDir string
}

// NewRunner creates a new verification runner.
func NewRunner(cfg Config, repoDir string) *Runner {
	return &Runner{cfg: cfg, repoDir: repoDir}
}

// RunAll executes lint, build, and test steps sequentially.
// Returns early on first failure if stopOnFail is true.
func (r *Runner) RunAll(ctx context.Context, stopOnFail bool) *VerificationResult {
	result := &VerificationResult{Passed: true}

	steps := []struct {
		name StepName
		cmd  string
	}{
		{StepLint, r.cfg.LintCmd},
		{StepBuild, r.cfg.BuildCmd},
		{StepTest, r.cfg.TestCmd},
	}

	for _, step := range steps {
		if step.cmd == "" {
			continue
		}
		sr := r.runStep(ctx, step.name, step.cmd)
		result.Steps = append(result.Steps, sr)
		if !sr.Passed {
			result.Passed = false
			if stopOnFail {
				break
			}
		}
	}

	if result.Passed {
		result.Summary = "All verification steps passed"
	} else {
		if f := result.FirstFailure(); f != nil {
			result.Summary = fmt.Sprintf("Failed at %s step", f.Step)
		}
	}

	return result
}

func (r *Runner) runStep(ctx context.Context, name StepName, cmdStr string) StepResult {
	timeout := time.Duration(r.cfg.TimeoutSec) * time.Second
	stepCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()

	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return StepResult{Step: name, Passed: false, Output: "empty command"}
	}

	cmd := exec.CommandContext(stepCtx, parts[0], parts[1:]...)
	cmd.Dir = r.repoDir

	out, err := cmd.CombinedOutput()
	duration := time.Since(start)
	output := string(out)

	if err != nil {
		return StepResult{
			Step:     name,
			Passed:   false,
			Output:   output,
			Duration: duration,
		}
	}

	return StepResult{
		Step:     name,
		Passed:   true,
		Output:   output,
		Duration: duration,
	}
}
