package training

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"
)

// BenchmarkTask is a single task used to evaluate model quality.
type BenchmarkTask struct {
	ID              string `json:"id"`
	Prompt          string `json:"prompt"`
	ReferenceOutput string `json:"reference_output"` // manager-approved output
	DiffSummary     string `json:"diff_summary"`
	SourceTaskID    string `json:"source_task_id"`
	Difficulty      string `json:"difficulty"` // "easy", "medium", "hard" (based on review count)
}

// BenchmarkSuite is a collection of benchmark tasks.
type BenchmarkSuite struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Tasks     []BenchmarkTask `json:"tasks"`
	CreatedAt time.Time       `json:"created_at"`
}

// BenchmarkResult holds the evaluation result for a single benchmark task.
type BenchmarkResult struct {
	TaskID       string  `json:"task_id"`
	ModelOutput  string  `json:"model_output"`
	Score        float64 `json:"score"`       // 0.0 - 1.0
	Pass         bool    `json:"pass"`
	EvaluatorID  string  `json:"evaluator_id"`
	Feedback     string  `json:"feedback"`
}

// BenchmarkGenerator creates benchmark suites from approved review pairs.
type BenchmarkGenerator struct {
	dataDir string
}

func NewBenchmarkGenerator(dataDir string) *BenchmarkGenerator {
	return &BenchmarkGenerator{dataDir: dataDir}
}

// Generate creates a benchmark suite from approved review pairs.
func (g *BenchmarkGenerator) Generate(name string, maxTasks int) (*BenchmarkSuite, error) {
	pairs, err := g.loadApprovedPairs()
	if err != nil {
		return nil, err
	}
	if len(pairs) == 0 {
		return nil, fmt.Errorf("no approved review pairs available")
	}

	// Shuffle and select
	rand.Shuffle(len(pairs), func(i, j int) {
		pairs[i], pairs[j] = pairs[j], pairs[i]
	})
	if maxTasks > 0 && len(pairs) > maxTasks {
		pairs = pairs[:maxTasks]
	}

	suite := &BenchmarkSuite{
		ID:        fmt.Sprintf("bench-%d", time.Now().UnixMilli()),
		Name:      name,
		CreatedAt: time.Now(),
	}

	for i, p := range pairs {
		difficulty := "medium"
		if p.DurationMs < 30000 {
			difficulty = "easy"
		} else if p.DurationMs > 120000 {
			difficulty = "hard"
		}

		suite.Tasks = append(suite.Tasks, BenchmarkTask{
			ID:              fmt.Sprintf("bt-%d", i+1),
			Prompt:          p.Prompt,
			ReferenceOutput: p.ManagerOutput,
			DiffSummary:     p.DiffSummary,
			SourceTaskID:    p.TaskID,
			Difficulty:      difficulty,
		})
	}

	// Save benchmark suite
	if err := g.saveSuite(suite); err != nil {
		return nil, err
	}

	return suite, nil
}

// LoadSuite loads a benchmark suite by ID.
func (g *BenchmarkGenerator) LoadSuite(id string) (*BenchmarkSuite, error) {
	path := filepath.Join(g.dataDir, "benchmarks", id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var suite BenchmarkSuite
	return &suite, json.Unmarshal(data, &suite)
}

// ListSuites lists all available benchmark suite IDs.
func (g *BenchmarkGenerator) ListSuites() ([]string, error) {
	dir := filepath.Join(g.dataDir, "benchmarks")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			ids = append(ids, e.Name()[:len(e.Name())-5])
		}
	}
	return ids, nil
}

func (g *BenchmarkGenerator) loadApprovedPairs() ([]ReviewPair, error) {
	path := filepath.Join(g.dataDir, "review_pairs.jsonl")
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var pairs []ReviewPair
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
	for scanner.Scan() {
		var pair ReviewPair
		if err := json.Unmarshal(scanner.Bytes(), &pair); err != nil {
			continue
		}
		if pair.Verdict == VerdictAccepted {
			pairs = append(pairs, pair)
		}
	}
	return pairs, scanner.Err()
}

func (g *BenchmarkGenerator) saveSuite(suite *BenchmarkSuite) error {
	dir := filepath.Join(g.dataDir, "benchmarks")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(suite, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, suite.ID+".json"), data, 0o644)
}
