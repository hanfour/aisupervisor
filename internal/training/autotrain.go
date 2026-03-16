package training

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"time"
)

// AutoTrainLoop orchestrates automatic fine-tune cycles:
// collect data → fine-tune → benchmark → keep or rollback.
type AutoTrainLoop struct {
	runner    *FinetuneRunner
	evaluator *Evaluator
	registry  *ModelRegistry
	promotion *PromotionChecker
}

// NewAutoTrainLoop creates a new auto-training loop.
func NewAutoTrainLoop(runner *FinetuneRunner, evaluator *Evaluator, registry *ModelRegistry, promotion *PromotionChecker) *AutoTrainLoop {
	return &AutoTrainLoop{
		runner:    runner,
		evaluator: evaluator,
		registry:  registry,
		promotion: promotion,
	}
}

// AutoTrainResult captures the outcome of a single auto-train cycle.
type AutoTrainResult struct {
	JobID      string
	NewVersion string
	OldScore   float64
	NewScore   float64
	Improved   bool
	RolledBack bool
}

// RunCycle executes a complete fine-tune + evaluation cycle.
// If the new model scores lower than the old one, it is rolled back.
func (a *AutoTrainLoop) RunCycle(ctx context.Context, cfg FinetuneConfig, benchSuite *BenchmarkSuite) (*AutoTrainResult, error) {
	result := &AutoTrainResult{}

	// 1. Record old model's latest benchmark score
	oldVersion := ""
	if latest := a.registry.Latest(); latest != nil {
		oldVersion = latest.Version
		result.OldScore = latest.BenchmarkScore
	}

	// 2. Launch fine-tune job and wait for completion
	job, err := a.runner.Launch(cfg)
	if err != nil {
		return nil, fmt.Errorf("launching fine-tune: %w", err)
	}
	result.JobID = job.ID

	// Wait for job to complete (poll every 5s)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
		}

		jobs := a.runner.ListJobs()
		var current *FinetuneJob
		for i := range jobs {
			if jobs[i].ID == job.ID {
				current = &jobs[i]
				break
			}
		}
		if current == nil {
			return nil, fmt.Errorf("job %s disappeared", job.ID)
		}
		if current.Status == "completed" {
			result.NewVersion = current.OutputVersion
			break
		}
		if current.Status == "failed" {
			return nil, fmt.Errorf("fine-tune job failed: %s", current.Error)
		}
	}

	// 3. Run benchmark evaluation on new model
	if a.evaluator != nil && benchSuite != nil && len(benchSuite.Tasks) > 0 {
		// Generate outputs from the new model (via ollama)
		modelOutputs := make(map[string]string)
		for _, task := range benchSuite.Tasks {
			output, err := a.queryModel(ctx, cfg.OutputModel, task.Prompt)
			if err != nil {
				log.Printf("autotrain: failed to query model for task %s: %v", task.ID, err)
				continue
			}
			modelOutputs[task.ID] = output
		}

		evalRun, err := a.evaluator.RunBenchmark(ctx, benchSuite, modelOutputs, result.NewVersion)
		if err != nil {
			log.Printf("autotrain: benchmark failed: %v", err)
		} else {
			result.NewScore = evalRun.AvgScore

			// Update model version with benchmark score
			if err := a.registry.UpdateBenchmarkScore(result.NewVersion, evalRun.AvgScore); err != nil {
				log.Printf("autotrain: failed to update benchmark score: %v", err)
			}
		}
	}

	// 4. Compare new vs old score
	if result.NewScore > result.OldScore {
		result.Improved = true
		log.Printf("autotrain: model improved %.4f → %.4f", result.OldScore, result.NewScore)

		// Check promotion eligibility
		if a.promotion != nil {
			evalRuns, _ := LoadEvalRuns(a.runner.dataDir)
			stats, _ := ComputeReviewStats(a.runner.dataDir)
			status := a.promotion.Check(result.NewVersion, evalRuns, stats.ApprovalRate, stats.TotalPairs)
			if status.Eligible {
				log.Printf("autotrain: model %s eligible for promotion!", result.NewVersion)
			}
		}
	} else {
		// Rollback: remove the new model from ollama
		result.RolledBack = true
		log.Printf("autotrain: no improvement (%.4f <= %.4f), rolling back", result.NewScore, result.OldScore)

		if cfg.OutputModel != "" {
			rmCmd := exec.Command("ollama", "rm", cfg.OutputModel)
			if out, err := rmCmd.CombinedOutput(); err != nil {
				log.Printf("autotrain: ollama rm failed: %s %v", string(out), err)
			}
		}

		// Restore old model as active (if there was one)
		if oldVersion != "" {
			if oldV, ok := a.registry.Get(oldVersion); ok && oldV.OllamaModel != "" {
				log.Printf("autotrain: reverting to model %s (%s)", oldVersion, oldV.OllamaModel)
			}
		}
	}

	return result, nil
}

// queryModel sends a prompt to a local ollama model and returns its output.
func (a *AutoTrainLoop) queryModel(ctx context.Context, modelName, prompt string) (string, error) {
	cmd := exec.CommandContext(ctx, "ollama", "run", modelName, prompt)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
