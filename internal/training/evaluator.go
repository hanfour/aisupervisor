package training

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// EvalBackend is a simplified interface for evaluation scoring.
// Implementations wrap the AI backend to score model outputs.
type EvalBackend interface {
	Score(ctx context.Context, prompt, modelOutput, referenceOutput string) (EvalScore, error)
}

// EvalScore holds the scoring result from the evaluator.
type EvalScore struct {
	Score    float64 `json:"score"`    // 0.0 - 1.0
	Pass     bool    `json:"pass"`
	Feedback string  `json:"feedback"`
}

// Evaluator runs benchmark tasks against a model and scores the results.
type Evaluator struct {
	backend   EvalBackend
	passScore float64 // minimum score to pass (default 0.7)
}

func NewEvaluator(backend EvalBackend, passScore float64) *Evaluator {
	if passScore <= 0 {
		passScore = 0.7
	}
	return &Evaluator{
		backend:   backend,
		passScore: passScore,
	}
}

// EvalRun represents a complete evaluation run.
type EvalRun struct {
	ID          string            `json:"id"`
	SuiteID     string            `json:"suite_id"`
	ModelVer    string            `json:"model_version"`
	Results     []BenchmarkResult `json:"results"`
	TotalTasks  int               `json:"total_tasks"`
	Passed      int               `json:"passed"`
	Failed      int               `json:"failed"`
	AvgScore    float64           `json:"avg_score"`
	PassRate    float64           `json:"pass_rate"`
	StartedAt   time.Time         `json:"started_at"`
	CompletedAt time.Time         `json:"completed_at"`
}

// RunBenchmark evaluates a model's outputs against a benchmark suite.
// modelOutputs maps benchmark task ID → model's output string.
func (e *Evaluator) RunBenchmark(ctx context.Context, suite *BenchmarkSuite, modelOutputs map[string]string, modelVer string) (*EvalRun, error) {
	run := &EvalRun{
		ID:         fmt.Sprintf("eval-%d", time.Now().UnixMilli()),
		SuiteID:    suite.ID,
		ModelVer:   modelVer,
		TotalTasks: len(suite.Tasks),
		StartedAt:  time.Now(),
	}

	var totalScore float64
	for _, task := range suite.Tasks {
		output, ok := modelOutputs[task.ID]
		if !ok {
			run.Results = append(run.Results, BenchmarkResult{
				TaskID:  task.ID,
				Score:   0,
				Pass:    false,
				Feedback: "no output provided",
			})
			run.Failed++
			continue
		}

		score, err := e.backend.Score(ctx, task.Prompt, output, task.ReferenceOutput)
		if err != nil {
			run.Results = append(run.Results, BenchmarkResult{
				TaskID:  task.ID,
				Score:   0,
				Pass:    false,
				Feedback: fmt.Sprintf("evaluation error: %v", err),
			})
			run.Failed++
			continue
		}

		result := BenchmarkResult{
			TaskID:      task.ID,
			ModelOutput: output,
			Score:       score.Score,
			Pass:        score.Score >= e.passScore,
			Feedback:    score.Feedback,
		}
		run.Results = append(run.Results, result)
		totalScore += score.Score
		if result.Pass {
			run.Passed++
		} else {
			run.Failed++
		}
	}

	run.CompletedAt = time.Now()
	if run.TotalTasks > 0 {
		run.AvgScore = totalScore / float64(run.TotalTasks)
		run.PassRate = float64(run.Passed) / float64(run.TotalTasks)
	}

	return run, nil
}

// SaveEvalRun persists an eval run to the eval_runs.jsonl file.
func SaveEvalRun(dataDir string, run *EvalRun) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(run)
	if err != nil {
		return err
	}
	path := filepath.Join(dataDir, "eval_runs.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(data, '\n'))
	return err
}

// LoadEvalRuns reads all eval runs from the JSONL file.
func LoadEvalRuns(dataDir string) ([]EvalRun, error) {
	path := filepath.Join(dataDir, "eval_runs.jsonl")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var runs []EvalRun
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var run EvalRun
		if json.Unmarshal(scanner.Bytes(), &run) == nil {
			runs = append(runs, run)
		}
	}
	return runs, scanner.Err()
}

// ReviewStats computes statistics from review pairs JSONL.
type ReviewStats struct {
	TotalPairs   int     `json:"total_pairs"`
	Accepted     int     `json:"accepted"`
	Rejected     int     `json:"rejected"`
	ApprovalRate float64 `json:"approval_rate"`
}

// ComputeReviewStats reads review_pairs.jsonl and returns aggregate stats.
func ComputeReviewStats(dataDir string) (ReviewStats, error) {
	path := filepath.Join(dataDir, "review_pairs.jsonl")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ReviewStats{}, nil
		}
		return ReviewStats{}, err
	}
	defer f.Close()

	var stats ReviewStats
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var pair ReviewPair
		if json.Unmarshal(scanner.Bytes(), &pair) == nil {
			stats.TotalPairs++
			switch pair.Verdict {
			case VerdictAccepted:
				stats.Accepted++
			case VerdictRejected:
				stats.Rejected++
			}
		}
	}
	if stats.TotalPairs > 0 {
		stats.ApprovalRate = float64(stats.Accepted) / float64(stats.TotalPairs)
	}
	return stats, scanner.Err()
}

// BuildEvalPrompt creates the evaluation prompt sent to the consultant AI.
func BuildEvalPrompt(taskPrompt, modelOutput, referenceOutput string) string {
	var sb strings.Builder
	sb.WriteString("You are evaluating an AI engineer's code output.\n\n")
	sb.WriteString("## Original Task\n")
	sb.WriteString(taskPrompt)
	sb.WriteString("\n\n## Model Output (to evaluate)\n")
	sb.WriteString(modelOutput)
	sb.WriteString("\n\n## Reference Output (approved by senior engineer)\n")
	sb.WriteString(referenceOutput)
	sb.WriteString("\n\n## Evaluation Criteria\n")
	sb.WriteString("1. Correctness: Does the output solve the task correctly?\n")
	sb.WriteString("2. Code quality: Is the code clean, readable, and well-structured?\n")
	sb.WriteString("3. Completeness: Does it address all requirements?\n")
	sb.WriteString("4. Similarity to reference: How close is it to the approved solution?\n\n")
	sb.WriteString("Respond with:\n")
	sb.WriteString("SCORE: <0.0-1.0>\n")
	sb.WriteString("PASS: <true/false>\n")
	sb.WriteString("FEEDBACK: <brief explanation>\n")
	return sb.String()
}
