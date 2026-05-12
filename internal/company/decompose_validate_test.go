package company

import (
	"strings"
	"testing"
)

func TestRequireNonEmptyDecomposition_PassesOnNonZero(t *testing.T) {
	if err := requireNonEmptyDecomposition("test", 3, "ignored"); err != nil {
		t.Errorf("expected nil for count=3, got %v", err)
	}
}

func TestRequireNonEmptyDecomposition_FailsOnZero(t *testing.T) {
	err := requireNonEmptyDecomposition("PRD decompose", 0, `{"tasks": []}`)
	if err == nil {
		t.Fatal("expected error for empty tasks, got nil")
	}
	if !strings.Contains(err.Error(), "PRD decompose") {
		t.Errorf("error missing label: %v", err)
	}
	if !strings.Contains(err.Error(), "zero tasks") {
		t.Errorf("error missing zero-task wording: %v", err)
	}
	if !strings.Contains(err.Error(), `tasks": []`) {
		t.Errorf("error missing raw snippet: %v", err)
	}
}

func TestRequireNonEmptyDecomposition_TruncatesLongSnippet(t *testing.T) {
	big := strings.Repeat("x", 1000)
	err := requireNonEmptyDecomposition("label", 0, big)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "truncated") {
		t.Errorf("expected truncation marker, got %v", err)
	}
}

func TestClassifyTaskLayer_KnownKeywords(t *testing.T) {
	cases := []struct {
		title       string
		description string
		want        string
	}{
		{"Design users table schema", "DDL for the user table", layerData},
		{"Implement /login endpoint", "POST handler", layerAPI},
		{"Build login page component", "React page", layerFrontend},
		{"LINE webhook signature verification", "third-party webhook", layerIntegration},
		{"Queue consumer for billing", "background worker", layerJobs},
		{"Add e2e tests for checkout", "Playwright spec", layerTests},
		{"Write API reference docs", "docs for endpoints", layerDocs},
		{"Add Dockerfile and compose.yml", "container infra", layerInfra},
		{"Refactor stuff", "no keyword here", ""},
	}
	for _, tc := range cases {
		got := classifyTaskLayer(tc.title, tc.description)
		if got != tc.want {
			t.Errorf("classifyTaskLayer(%q,%q) = %q, want %q", tc.title, tc.description, got, tc.want)
		}
	}
}

func TestClassifyTaskLayer_TestsTrumpAPIWhenBothMatch(t *testing.T) {
	// A test for the API layer should classify as tests (the work
	// item is "write tests"), not api — otherwise the topological
	// hinter would treat the test as a peer of the API rather than
	// a follower.
	got := classifyTaskLayer("Write unit tests for /login endpoint", "")
	if got != layerTests {
		t.Errorf("test+api title = %q, want %q", got, layerTests)
	}
}

func TestClassifyTaskLayer_OutOfScopeMarker(t *testing.T) {
	got := classifyTaskLayer("out-of-scope: frontend", "PRD does not require this layer")
	if got != layerFrontend {
		t.Errorf("out-of-scope marker → %q, want %q", got, layerFrontend)
	}
}

func TestAnalyseLayerCoverage_ReportsPresentAndMissing(t *testing.T) {
	tasks := []decomposedTask{
		{Title: "Migrate users table", Description: "schema work"},
		{Title: "Implement /me endpoint", Description: "handler"},
		{Title: "out-of-scope: jobs", Description: "no background work"},
	}
	cov := analyseLayerCoverage(tasks)

	wantPresent := map[string]bool{layerData: true, layerAPI: true, layerJobs: true}
	for _, p := range cov.Present {
		if !wantPresent[p] {
			t.Errorf("unexpected layer in Present: %q (got %v)", p, cov.Present)
		}
	}
	if len(cov.Present) != len(wantPresent) {
		t.Errorf("Present count = %d, want %d (%v)", len(cov.Present), len(wantPresent), cov.Present)
	}

	missingSet := map[string]bool{}
	for _, m := range cov.Missing {
		missingSet[m] = true
	}
	for _, want := range []string{layerFrontend, layerIntegration, layerTests, layerDocs, layerInfra} {
		if !missingSet[want] {
			t.Errorf("expected %q in Missing, got %v", want, cov.Missing)
		}
	}
}

func TestAnalyseLayerCoverage_NoTasksAllMissing(t *testing.T) {
	cov := analyseLayerCoverage(nil)
	if len(cov.Present) != 0 {
		t.Errorf("Present = %v, want empty", cov.Present)
	}
	if len(cov.Missing) != len(canonicalLayers) {
		t.Errorf("Missing count = %d, want %d", len(cov.Missing), len(canonicalLayers))
	}
}

func TestInferDependencyHints_TestsDependOnLatestImpl(t *testing.T) {
	tasks := []decomposedTask{
		{Title: "Migrate users table", Description: "DDL"},         // 0 data
		{Title: "Implement /me endpoint", Description: "handler"},  // 1 api
		{Title: "Build profile page component", Description: "UI"}, // 2 frontend
		{Title: "Add tests for /me endpoint", Description: ""},     // 3 tests
	}
	hints := inferDependencyHints(tasks)

	// api → data
	if !hasInt(hints[1], 0) {
		t.Errorf("api task (1) should depend on data (0), got %v", hints[1])
	}
	// frontend → api
	if !hasInt(hints[2], 1) {
		t.Errorf("frontend task (2) should depend on api (1), got %v", hints[2])
	}
	// tests → frontend (latest) and api and data
	for _, want := range []int{0, 1, 2} {
		if !hasInt(hints[3], want) {
			t.Errorf("tests task (3) should depend on %d, got %v", want, hints[3])
		}
	}
}

func TestInferDependencyHints_DataAndInfraIndependent(t *testing.T) {
	tasks := []decomposedTask{
		{Title: "Add Dockerfile", Description: "infra"},        // 0 infra
		{Title: "Migrate users table", Description: "DDL"},     // 1 data
		{Title: "GitHub Actions CI", Description: "ci/cd"},     // 2 infra
	}
	hints := inferDependencyHints(tasks)
	if len(hints[0]) != 0 || len(hints[1]) != 0 || len(hints[2]) != 0 {
		t.Errorf("expected no hints for infra+data: hints=%v", hints)
	}
}

func TestInferDependencyHints_UnclassifiedTasksGetNoHints(t *testing.T) {
	tasks := []decomposedTask{
		{Title: "Implement /me endpoint", Description: "handler"}, // 0 api
		{Title: "Refactor stuff", Description: "vague"},           // 1 unknown
	}
	hints := inferDependencyHints(tasks)
	if len(hints[1]) != 0 {
		t.Errorf("unclassified task should have no hints, got %v", hints[1])
	}
}

func TestInferDependencyHints_EmptyInput(t *testing.T) {
	hints := inferDependencyHints(nil)
	if len(hints) != 0 {
		t.Errorf("expected empty hint map, got %v", hints)
	}
}

func TestFormatLayerList_Empty(t *testing.T) {
	if got := formatLayerList(nil); got != "<none>" {
		t.Errorf("empty → %q, want <none>", got)
	}
}

func TestFormatLayerList_Joined(t *testing.T) {
	got := formatLayerList([]string{layerData, layerAPI})
	if got != "data, api" {
		t.Errorf("formatLayerList → %q, want %q", got, "data, api")
	}
}

func hasInt(s []int, want int) bool {
	for _, v := range s {
		if v == want {
			return true
		}
	}
	return false
}
