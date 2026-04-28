package company

import (
	"context"
	"strings"
	"testing"

	"github.com/hanfourmini/aisupervisor/internal/ai"
	"github.com/hanfourmini/aisupervisor/internal/project"
)

// stubChat is a fake ai.ChatProvider whose Chat replays a preset
// response (same payload regardless of input). Tests in this file
// exercise company-side post-processing (parsing, AddTask integration),
// not upstream AI; a fixed reply keeps tests hermetic and deterministic.
type stubChat struct {
	reply string
	err   error
	calls int
}

func (s *stubChat) Chat(ctx context.Context, messages []ai.ChatMessage) (string, error) {
	s.calls++
	if s.err != nil {
		return "", s.err
	}
	return s.reply, nil
}

// =============================================================
// Decompose prompt regression tests (PR #34)
// =============================================================

func TestDecomposeFromPRDSystemPrompt_FullStackCoverage(t *testing.T) {
	cases := []struct {
		lang     string
		keywords []string
	}{
		{
			lang: "en",
			keywords: []string{
				"FULL-STACK", "MANDATORY COVERAGE", "out-of-scope",
				"Database", "Backend", "Frontend", "integration",
				"Background", "Tests", "Documentation", "Infrastructure",
			},
		},
		{
			lang: "zh-TW",
			keywords: []string{
				"全棧任務清單", "強制涵蓋", "out-of-scope",
				"資料庫", "後端", "前端", "整合",
				"背景任務", "測試", "文件", "基礎設施",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.lang, func(t *testing.T) {
			got := decomposeFromPRDSystemPrompt(tc.lang)
			for _, kw := range tc.keywords {
				if !strings.Contains(got, kw) {
					t.Errorf("decomposeFromPRDSystemPrompt(%q) missing required keyword %q — full-stack coverage rules must mention every layer", tc.lang, kw)
				}
			}
		})
	}
}

func TestDecomposeFromPRDSystemPrompt_NoTaskCountCap(t *testing.T) {
	for _, lang := range []string{"en", "zh-TW"} {
		got := decomposeFromPRDSystemPrompt(lang)
		if strings.Contains(got, "3-15") {
			t.Errorf("%s prompt still contains 3-15 cap — should be removed", lang)
		}
		if !strings.Contains(got, "25-50") {
			t.Errorf("%s prompt should mention 25-50 task typical scope as a floor anchor", lang)
		}
	}
}

func TestDecomposeFromPRDSystemPrompt_OutputFormatStillJSON(t *testing.T) {
	for _, lang := range []string{"en", "zh-TW"} {
		got := decomposeFromPRDSystemPrompt(lang)
		if !strings.Contains(got, `{"tasks":`) {
			t.Errorf("%s prompt missing JSON output template", lang)
		}
		if !strings.Contains(strings.ToLower(got), "json") {
			t.Errorf("%s prompt missing JSON requirement", lang)
		}
	}
}

// =============================================================
// Phase-0 review tests (PR #35)
// =============================================================

func TestReviewDecomposedTasks_AppendsGapFillTasks(t *testing.T) {
	m, _ := testManager(t)
	p, err := m.CreateProject("rdt-test", "desc", t.TempDir(), "main", nil)
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if _, err := m.AddTask(p.ID, "Schema base", "tenant DDL", "write tenant DDL", nil, 1, "", "code"); err != nil {
		t.Fatalf("AddTask seed: %v", err)
	}

	m.chatProvider = &stubChat{reply: `{
        "gaps": ["Backend API layer", "End-to-end tests"],
        "tasks": [
          {"title": "Implement user CRUD API", "description": "REST endpoints", "prompt": "build /users routes", "type": "code", "priority": 20},
          {"title": "Add e2e smoke tests", "description": "playwright", "prompt": "tests/e2e/smoke.spec.ts", "type": "code", "priority": 21}
        ]
    }`}

	if err := m.ReviewDecomposedTasks(context.Background(), p.ID, "PRD content here"); err != nil {
		t.Fatalf("ReviewDecomposedTasks: %v", err)
	}

	tasks := m.projectStore.TasksForProject(p.ID)
	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks (1 seed + 2 gap-fill), got %d", len(tasks))
	}
	titles := map[string]bool{}
	for _, tk := range tasks {
		titles[tk.Title] = true
	}
	for _, want := range []string{"Schema base", "Implement user CRUD API", "Add e2e smoke tests"} {
		if !titles[want] {
			t.Errorf("expected task %q to be present, got %v", want, titles)
		}
	}
}

func TestReviewDecomposedTasks_NoGapsIsNoOp(t *testing.T) {
	m, _ := testManager(t)
	p, _ := m.CreateProject("rdt-nogap", "desc", t.TempDir(), "main", nil)
	m.AddTask(p.ID, "Existing task", "", "do thing", nil, 1, "", "code")

	m.chatProvider = &stubChat{reply: `{"gaps": [], "tasks": []}`}

	if err := m.ReviewDecomposedTasks(context.Background(), p.ID, "PRD"); err != nil {
		t.Fatalf("ReviewDecomposedTasks: %v", err)
	}
	if got := len(m.projectStore.TasksForProject(p.ID)); got != 1 {
		t.Fatalf("no-gap review should not add tasks, got %d", got)
	}
}

func TestReviewDecomposedTasks_EmptyTaskListSkipsReview(t *testing.T) {
	m, _ := testManager(t)
	p, _ := m.CreateProject("rdt-empty", "desc", t.TempDir(), "main", nil)
	chat := &stubChat{reply: `{"gaps": ["everything"], "tasks": [{"title":"x","description":"","prompt":"","type":"code","priority":1}]}`}
	m.chatProvider = chat

	if err := m.ReviewDecomposedTasks(context.Background(), p.ID, "PRD"); err != nil {
		t.Fatalf("expected nil error on empty task list, got %v", err)
	}
	if chat.calls != 0 {
		t.Errorf("reviewer should not be called when project has no tasks (got %d calls)", chat.calls)
	}
	if got := len(m.projectStore.TasksForProject(p.ID)); got != 0 {
		t.Errorf("project should still have 0 tasks after no-op review, got %d", got)
	}
}

func TestReviewDecomposedTasksSystemPrompt_LayerCoverage(t *testing.T) {
	cases := []struct {
		lang     string
		keywords []string
	}{
		{
			lang: "en",
			keywords: []string{
				"GAPS", "EIGHT LAYERS",
				"Database", "Backend", "Frontend", "integration",
				"Background", "Tests", "Documentation", "Infrastructure",
				"Do NOT", "\"gaps\":", "\"tasks\":",
			},
		},
		{
			lang: "zh-TW",
			keywords: []string{
				"找出缺漏", "八層",
				"資料庫", "後端", "前端", "整合",
				"背景任務", "測試", "文件", "基礎設施",
				"禁止事項", "\"gaps\":", "\"tasks\":",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.lang, func(t *testing.T) {
			got := reviewDecomposedTasksSystemPrompt(tc.lang)
			for _, kw := range tc.keywords {
				if !strings.Contains(got, kw) {
					t.Errorf("review prompt(%q) missing required keyword %q", tc.lang, kw)
				}
			}
		})
	}
}

// =============================================================
// Project verify-and-iterate tests (PR #36 — this PR)
// =============================================================

func TestRunProjectVerification_NoVerifyCmd(t *testing.T) {
	m, _ := testManager(t)
	p := &project.Project{ID: "p1", VerifyCmd: ""}
	passed, output, err := m.runProjectVerification(context.Background(), p)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !passed {
		t.Fatal("empty verify_cmd should trivially pass")
	}
	if output != "" {
		t.Errorf("expected empty output, got %q", output)
	}
}

func TestRunProjectVerification_PassingCommand(t *testing.T) {
	m, _ := testManager(t)
	p := &project.Project{
		ID:        "p1",
		RepoPath:  t.TempDir(),
		VerifyCmd: "echo all-good && exit 0",
	}
	passed, output, err := m.runProjectVerification(context.Background(), p)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !passed {
		t.Fatal("exit 0 should pass")
	}
	if !strings.Contains(output, "all-good") {
		t.Errorf("output missing stdout, got %q", output)
	}
}

func TestRunProjectVerification_FailingCommand(t *testing.T) {
	m, _ := testManager(t)
	p := &project.Project{
		ID:        "p1",
		RepoPath:  t.TempDir(),
		VerifyCmd: "echo my error message >&2 && exit 1",
	}
	passed, output, err := m.runProjectVerification(context.Background(), p)
	if err != nil {
		t.Fatalf("err should be nil for non-zero exit (it's a test failure, not exec error), got %v", err)
	}
	if passed {
		t.Fatal("exit 1 should fail verification")
	}
	if !strings.Contains(output, "my error message") {
		t.Errorf("expected stderr captured in output, got %q", output)
	}
}

func TestDecomposeFromVerifyFailure_AppendsFixTasks(t *testing.T) {
	m, _ := testManager(t)
	p, err := m.CreateProject("dvf", "desc", t.TempDir(), "main", nil)
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	p.VerifyIterations = 1
	m.projectStore.SaveProject(p)

	m.chatProvider = &stubChat{reply: `{
        "tasks": [
          {"title": "Fix missing import in api/handler.go", "description": "add foo import", "prompt": "edit api/handler.go line 12", "type": "code", "priority": 101},
          {"title": "Fix failing TC-006", "description": "encrypt fixture", "prompt": "tests/fixtures.sql", "type": "code", "priority": 102}
        ]
    }`}

	if err := m.decomposeFromVerifyFailure(context.Background(), p.ID, "ERROR: undefined: foo"); err != nil {
		t.Fatalf("decomposeFromVerifyFailure: %v", err)
	}

	tasks := m.projectStore.TasksForProject(p.ID)
	if len(tasks) != 2 {
		t.Fatalf("expected 2 fix tasks, got %d", len(tasks))
	}
	for _, tk := range tasks {
		if !strings.HasPrefix(tk.Description, "[verify-iter-1] ") {
			t.Errorf("task %q description missing iteration prefix, got %q", tk.Title, tk.Description)
		}
	}
}

func TestDecomposeFromVerifyFailureSystemPrompt_KeyDirectives(t *testing.T) {
	cases := []struct {
		lang     string
		keywords []string
	}{
		{
			lang: "en",
			keywords: []string{
				"SMALLEST set of fix tasks",
				"One distinct error class = one task",
				"verify_cmd",
				"100 + iteration",
				"\"tasks\":",
			},
		},
		{
			lang: "zh-TW",
			keywords: []string{
				"最小集合的修復任務",
				"一類錯誤一個任務",
				"verify_cmd",
				"100 + 迭代次數",
				"\"tasks\":",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.lang, func(t *testing.T) {
			got := decomposeFromVerifyFailureSystemPrompt(tc.lang)
			for _, kw := range tc.keywords {
				if !strings.Contains(got, kw) {
					t.Errorf("decomposeFromVerifyFailureSystemPrompt(%q) missing keyword %q", tc.lang, kw)
				}
			}
		})
	}
}

func TestProjectPhase_VerifyingExists(t *testing.T) {
	if string(project.PhaseVerifying) != "verifying" {
		t.Fatalf("PhaseVerifying = %q, want %q", project.PhaseVerifying, "verifying")
	}
}
