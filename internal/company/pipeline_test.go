package company

import (
	"context"
	"strings"
	"testing"

	"github.com/hanfourmini/aisupervisor/internal/ai"
)

// stubChat is a fake ai.ChatProvider whose Chat method replays a
// preset response string (the same payload regardless of input). The
// tests in this file exercise the company-side post-processing
// (parsing, AddTask integration), not the upstream AI; a fixed reply
// keeps the tests hermetic and deterministic.
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

// TestDecomposeFromPRDSystemPrompt_FullStackCoverage pins the
// full-stack coverage requirement into the decompose system prompt.
// A regression that drops any of the eight layer keywords would let
// DecomposeFromPRD fall back to single-slice (e.g. SQL-only) output —
// the bug this prompt was rewritten to fix.
func TestDecomposeFromPRDSystemPrompt_FullStackCoverage(t *testing.T) {
	cases := []struct {
		lang     string
		keywords []string
	}{
		{
			lang: "en",
			keywords: []string{
				"FULL-STACK",
				"MANDATORY COVERAGE",
				"out-of-scope",
				"Database",
				"Backend",
				"Frontend",
				"integration",
				"Background",
				"Tests",
				"Documentation",
				"Infrastructure",
			},
		},
		{
			lang: "zh-TW",
			keywords: []string{
				"全棧任務清單",
				"強制涵蓋",
				"out-of-scope",
				"資料庫",
				"後端",
				"前端",
				"整合",
				"背景任務",
				"測試",
				"文件",
				"基礎設施",
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

// TestDecomposeFromPRDSystemPrompt_NoTaskCountCap verifies the prompt
// no longer caps task count at 15, which previously biased AI toward
// the "easiest 15" (typically database-only) and silently dropped
// frontend/backend/integration/test/infra layers.
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

// TestDecomposeFromPRDSystemPrompt_OutputFormatStillJSON verifies the
// output-format requirement (valid JSON, no markdown) is preserved.
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
// Phase-0 review tests (PR #35 — this PR)
// =============================================================

// TestReviewDecomposedTasks_AppendsGapFillTasks covers the success path:
// reviewer reports two gaps and emits two new tasks, both should be
// added to the project. Existing tasks are untouched.
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

// TestReviewDecomposedTasks_NoGapsIsNoOp verifies the empty-gaps path:
// reviewer reports no gaps, no tasks should be added.
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

// TestReviewDecomposedTasks_EmptyTaskListSkipsReview verifies the early-
// return when there is nothing to review against.
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

// TestReviewDecomposedTasksSystemPrompt_LayerCoverage pins the eight
// layers + key directives into the review system prompt.
func TestReviewDecomposedTasksSystemPrompt_LayerCoverage(t *testing.T) {
	cases := []struct {
		lang     string
		keywords []string
	}{
		{
			lang: "en",
			keywords: []string{
				"GAPS",
				"EIGHT LAYERS",
				"Database",
				"Backend",
				"Frontend",
				"integration",
				"Background",
				"Tests",
				"Documentation",
				"Infrastructure",
				"Do NOT",
				"\"gaps\":",
				"\"tasks\":",
			},
		},
		{
			lang: "zh-TW",
			keywords: []string{
				"找出缺漏",
				"八層",
				"資料庫",
				"後端",
				"前端",
				"整合",
				"背景任務",
				"測試",
				"文件",
				"基礎設施",
				"禁止事項",
				"\"gaps\":",
				"\"tasks\":",
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
