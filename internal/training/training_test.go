package training

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoggerWriteAndRead(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}

	pair := ReviewPair{
		TaskID:         "t1",
		ProjectID:      "p1",
		EngineerID:     "w1",
		ManagerID:      "w2",
		EngineerModel:  "codellama:7b",
		ManagerModel:   "claude-sonnet",
		Prompt:         "implement login",
		EngineerOutput: "func Login() {}",
		ManagerOutput:  "func Login(ctx context.Context) error { return nil }",
		Verdict:        VerdictAccepted,
		DurationMs:     5000,
	}

	if err := logger.Log(pair); err != nil {
		t.Fatalf("Log: %v", err)
	}

	// Verify file exists
	path := filepath.Join(dir, "review_pairs.jsonl")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("expected non-empty JSONL file")
	}
}

func TestModelRegistry(t *testing.T) {
	dir := t.TempDir()
	reg, err := NewModelRegistry(dir)
	if err != nil {
		t.Fatalf("NewModelRegistry: %v", err)
	}

	// Initially empty
	if latest := reg.Latest(); latest != nil {
		t.Fatal("expected nil Latest for empty registry")
	}

	// Register versions
	reg.Register(ModelVersion{
		Version:   "v1",
		BaseModel: "codellama:7b",
		Method:    "sft",
		TrainPairs: 50,
	})
	reg.Register(ModelVersion{
		Version:   "v2",
		BaseModel: "codellama:7b",
		ParentVer: "v1",
		Method:    "dpo",
		TrainPairs: 100,
	})

	// Latest
	latest := reg.Latest()
	if latest == nil || latest.Version != "v2" {
		t.Fatalf("expected v2, got %v", latest)
	}

	// List
	list := reg.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(list))
	}

	// Get
	v, ok := reg.Get("v1")
	if !ok || v.TrainPairs != 50 {
		t.Fatalf("expected v1 with 50 pairs, got %v", v)
	}

	// Version chain
	chain := reg.VersionChain("v2")
	if len(chain) != 2 {
		t.Fatalf("expected chain length 2, got %d", len(chain))
	}
	if chain[0].Version != "v2" || chain[1].Version != "v1" {
		t.Fatalf("unexpected chain: %v", chain)
	}

	// Persistence
	reg2, err := NewModelRegistry(dir)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(reg2.List()) != 2 {
		t.Fatalf("expected 2 versions after reload, got %d", len(reg2.List()))
	}
}

func TestExporterNoData(t *testing.T) {
	dir := t.TempDir()
	exporter := NewExporter(dir)

	err := exporter.Export(FormatSFT, ExportFilter{}, 0.1, filepath.Join(dir, "out"))
	if err == nil {
		t.Fatal("expected error for no data")
	}
}

func TestExporterSFT(t *testing.T) {
	dir := t.TempDir()

	// Write test data
	logger, _ := NewLogger(dir)
	for i := 0; i < 5; i++ {
		logger.Log(ReviewPair{
			Prompt:        "test prompt",
			ManagerOutput: "test output",
			Verdict:       VerdictAccepted,
			DurationMs:    1000,
		})
	}

	exporter := NewExporter(dir)
	outDir := filepath.Join(dir, "export")
	err := exporter.Export(FormatSFT, ExportFilter{}, 0.2, outDir)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	// Check train file exists
	if _, err := os.Stat(filepath.Join(outDir, "train_sft.json")); err != nil {
		t.Fatalf("train file missing: %v", err)
	}
}

func TestExporterDPO(t *testing.T) {
	dir := t.TempDir()

	logger, _ := NewLogger(dir)
	for i := 0; i < 5; i++ {
		logger.Log(ReviewPair{
			Prompt:         "test prompt",
			EngineerOutput: "bad output",
			ManagerOutput:  "good output",
			Verdict:        VerdictRejected,
			DurationMs:     2000,
		})
	}

	exporter := NewExporter(dir)
	outDir := filepath.Join(dir, "export")
	err := exporter.Export(FormatDPO, ExportFilter{Verdicts: []Verdict{VerdictRejected}}, 0.2, outDir)
	if err != nil {
		t.Fatalf("Export DPO: %v", err)
	}

	if _, err := os.Stat(filepath.Join(outDir, "train_dpo.json")); err != nil {
		t.Fatalf("DPO train file missing: %v", err)
	}
}

func TestBenchmarkGenerator(t *testing.T) {
	dir := t.TempDir()

	// Write approved pairs
	logger, _ := NewLogger(dir)
	for i := 0; i < 3; i++ {
		logger.Log(ReviewPair{
			TaskID:        "t1",
			Prompt:        "implement feature",
			ManagerOutput: "reference implementation",
			Verdict:       VerdictAccepted,
			DurationMs:    60000,
		})
	}

	gen := NewBenchmarkGenerator(dir)
	suite, err := gen.Generate("test-suite", 10)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(suite.Tasks) != 3 {
		t.Fatalf("expected 3 benchmark tasks, got %d", len(suite.Tasks))
	}

	// List
	suites, _ := gen.ListSuites()
	if len(suites) != 1 {
		t.Fatalf("expected 1 suite, got %d", len(suites))
	}

	// Load
	loaded, err := gen.LoadSuite(suite.ID)
	if err != nil {
		t.Fatalf("LoadSuite: %v", err)
	}
	if len(loaded.Tasks) != 3 {
		t.Fatalf("expected 3 tasks after load, got %d", len(loaded.Tasks))
	}
}

func TestEvalRunPersistence(t *testing.T) {
	dir := t.TempDir()

	run := &EvalRun{
		ID:         "eval-test-1",
		SuiteID:    "suite-1",
		ModelVer:   "v1",
		TotalTasks: 5,
		Passed:     4,
		Failed:     1,
		AvgScore:   0.85,
		PassRate:   0.80,
		StartedAt:  time.Now(),
		CompletedAt: time.Now(),
	}

	if err := SaveEvalRun(dir, run); err != nil {
		t.Fatalf("SaveEvalRun: %v", err)
	}

	// Save another
	run2 := &EvalRun{
		ID:       "eval-test-2",
		SuiteID:  "suite-1",
		ModelVer: "v2",
		AvgScore: 0.92,
	}
	if err := SaveEvalRun(dir, run2); err != nil {
		t.Fatalf("SaveEvalRun 2: %v", err)
	}

	runs, err := LoadEvalRuns(dir)
	if err != nil {
		t.Fatalf("LoadEvalRuns: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("expected 2 eval runs, got %d", len(runs))
	}
	if runs[0].ID != "eval-test-1" {
		t.Fatalf("expected first run ID eval-test-1, got %s", runs[0].ID)
	}
}

func TestLoadEvalRunsEmpty(t *testing.T) {
	dir := t.TempDir()
	runs, err := LoadEvalRuns(dir)
	if err != nil {
		t.Fatalf("LoadEvalRuns empty: %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("expected 0 runs, got %d", len(runs))
	}
}

func TestComputeReviewStats(t *testing.T) {
	dir := t.TempDir()
	logger, _ := NewLogger(dir)

	// Log mixed verdicts
	for i := 0; i < 8; i++ {
		verdict := VerdictAccepted
		if i >= 6 {
			verdict = VerdictRejected
		}
		logger.Log(ReviewPair{
			TaskID:  "t1",
			Verdict: verdict,
		})
	}

	stats, err := ComputeReviewStats(dir)
	if err != nil {
		t.Fatalf("ComputeReviewStats: %v", err)
	}
	if stats.TotalPairs != 8 {
		t.Fatalf("expected 8 pairs, got %d", stats.TotalPairs)
	}
	if stats.Accepted != 6 {
		t.Fatalf("expected 6 accepted, got %d", stats.Accepted)
	}
	if stats.Rejected != 2 {
		t.Fatalf("expected 2 rejected, got %d", stats.Rejected)
	}
	expectedRate := 6.0 / 8.0
	if stats.ApprovalRate < expectedRate-0.01 || stats.ApprovalRate > expectedRate+0.01 {
		t.Fatalf("expected approval rate ~%.2f, got %.2f", expectedRate, stats.ApprovalRate)
	}
}

func TestComputeReviewStatsEmpty(t *testing.T) {
	dir := t.TempDir()
	stats, err := ComputeReviewStats(dir)
	if err != nil {
		t.Fatalf("ComputeReviewStats empty: %v", err)
	}
	if stats.TotalPairs != 0 {
		t.Fatalf("expected 0 pairs, got %d", stats.TotalPairs)
	}
}

func TestPromotionChecker(t *testing.T) {
	dir := t.TempDir()
	reg, _ := NewModelRegistry(dir)
	criteria := DefaultPromotionCriteria()
	checker := NewPromotionChecker(criteria, reg)

	// Not eligible: no training data
	status := checker.Check("v1", nil, 0.5, 10)
	if status.Eligible {
		t.Fatal("should not be eligible with 10 pairs")
	}
	if len(status.Reasons) == 0 {
		t.Fatal("expected reasons for ineligibility")
	}

	// Create eval runs that pass
	var runs []EvalRun
	for i := 0; i < 5; i++ {
		runs = append(runs, EvalRun{
			ModelVer:   "v1",
			AvgScore:   0.9,
			PassRate:   0.95,
			StartedAt:  time.Now(),
			CompletedAt: time.Now(),
		})
	}

	status = checker.Check("v1", runs, 0.9, 200)
	if !status.Eligible {
		t.Fatalf("should be eligible: %v", status.Reasons)
	}
}
