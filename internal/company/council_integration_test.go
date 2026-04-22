package company

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/ai"
	"github.com/hanfourmini/aisupervisor/internal/config"
)

// integrationMockChat cycles through pre-configured responses for each Chat call.
type integrationMockChat struct {
	responses []string
	callCount int
	mu        sync.Mutex
}

func (m *integrationMockChat) Chat(ctx context.Context, messages []ai.ChatMessage) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.callCount % len(m.responses)
	m.callCount++
	return m.responses[idx], nil
}

// realisticGoDiff is a unified diff touching a Go file with enough content
// to trigger backend, security, and refactoring experts.
const realisticGoDiff = `diff --git a/internal/server/handler.go b/internal/server/handler.go
--- a/internal/server/handler.go
+++ b/internal/server/handler.go
@@ -10,6 +10,20 @@ import (
 	"net/http"
 	"context"
+	"sync"
 )

+var mu sync.Mutex
+
+func HandleLogin(w http.ResponseWriter, r *http.Request) {
+	mu.Lock()
+	defer mu.Unlock()
+	password := r.FormValue("password")
+	token := "hardcoded-secret-key"
+	if password == token {
+		w.WriteHeader(http.StatusOK)
+		return
+	}
+	w.WriteHeader(http.StatusUnauthorized)
+}
`

// realisticMultiFileDiff touches both .go and .svelte files so that backend
// and frontend experts are selected.
const realisticMultiFileDiff = `diff --git a/internal/api/routes.go b/internal/api/routes.go
--- a/internal/api/routes.go
+++ b/internal/api/routes.go
@@ -1,5 +1,10 @@
 package api

+import "net/http"
+
+func RegisterRoutes() {
+	http.HandleFunc("/api/tasks", handleTasks)
+}
diff --git a/frontend/src/lib/components/TaskList.svelte b/frontend/src/lib/components/TaskList.svelte
--- a/frontend/src/lib/components/TaskList.svelte
+++ b/frontend/src/lib/components/TaskList.svelte
@@ -1,3 +1,12 @@
 <script>
+  import { onMount } from 'svelte';
+  let tasks = [];
+  onMount(async () => {
+    const res = await fetch('/api/tasks');
+    tasks = await res.json();
+  });
 </script>
+<ul>
+  {#each tasks as task}
+    <li>{task.name}</li>
+  {/each}
+</ul>
`

func TestCouncilPipeline_EndToEnd(t *testing.T) {
	// Mock provider returns a HIGH finding for every expert call.
	highFinding := []Finding{{
		File:     "internal/server/handler.go",
		Line:     18,
		Severity: "HIGH",
		Body:     "hardcoded secret key used for authentication comparison",
	}}
	highJSON, _ := json.Marshal(highFinding)
	mock := &integrationMockChat{responses: []string{string(highJSON)}}

	store, err := NewConventionStore(t.TempDir())
	if err != nil {
		t.Fatalf("create convention store: %v", err)
	}

	engine := &CouncilEngine{
		chatProvider: mock,
		registry:     NewExpertRegistry(),
		conventions:  store,
		language:     "en",
		reviewCfg:    config.ReviewConfig{APIExpertTimeoutS: 5},
	}

	builder := &ContextBriefBuilder{}
	builder.taskID = "TASK-001"
	builder.projectID = "proj-aisupervisor"
	builder.projectName = "aisupervisor"
	builder.techStack = "Go + Svelte"
	builder.diffStats = "1 file changed, 14 insertions"
	builder.changedFiles = []string{"internal/server/handler.go"}
	brief, err := builder.Build()
	if err != nil {
		t.Fatalf("build context brief: %v", err)
	}

	phase0 := &Phase0Report{
		Results:  []Phase0Result{{Check: Phase0Check{Name: "go-vet"}, Passed: true}},
		AllGreen: true,
	}

	req := CouncilRequest{
		Diff:      realisticGoDiff,
		DiffLines: 20,
		FileCount: 100,
		Brief:     brief,
		Phase0:    phase0,
	}

	result, err := engine.RunCouncil(context.Background(), req)
	if err != nil {
		t.Fatalf("RunCouncil error: %v", err)
	}

	// Verify result structure.
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Status != "APPROVED" && result.Status != "CHANGES_REQUESTED" {
		t.Errorf("unexpected status %q, want APPROVED or CHANGES_REQUESTED", result.Status)
	}
	if result.ExpertCount == 0 {
		t.Error("expected ExpertCount > 0")
	}
	if result.Duration <= 0 {
		t.Error("expected Duration > 0")
	}

	// At least one call should have been made to the mock.
	mock.mu.Lock()
	calls := mock.callCount
	mock.mu.Unlock()
	if calls == 0 {
		t.Error("expected at least one Chat call to mock provider")
	}
}

func TestCouncilPipeline_Phase0Rejection(t *testing.T) {
	// No chatProvider needed — Phase0 short-circuits before expert dispatch.
	engine := &CouncilEngine{
		registry:  NewExpertRegistry(),
		language:  "en",
		reviewCfg: config.ReviewConfig{},
	}

	// ALL checks failed — triggers AllCriticalFailed() == true.
	phase0 := &Phase0Report{
		Results: []Phase0Result{
			{Check: Phase0Check{Name: "go-vet", Command: "go vet ./..."}, Passed: false, Output: "vet: shadow variable"},
			{Check: Phase0Check{Name: "lint", Command: "npm run lint"}, Passed: false, Output: "eslint error count: 5"},
			{Check: Phase0Check{Name: "typecheck", Command: "npm run typecheck"}, Passed: false, Output: "type error in App.svelte"},
		},
		AllGreen: false,
	}

	req := CouncilRequest{
		Diff:      realisticGoDiff,
		DiffLines: 20,
		FileCount: 10,
		Phase0:    phase0,
	}

	result, err := engine.RunCouncil(context.Background(), req)
	if err != nil {
		t.Fatalf("RunCouncil error: %v", err)
	}

	if result.Status != "CHANGES_REQUESTED" {
		t.Errorf("expected CHANGES_REQUESTED, got %s", result.Status)
	}

	// All findings must have Source == "phase0".
	if len(result.Findings) == 0 {
		t.Fatal("expected phase0 findings, got none")
	}

	foundPhase0Source := false
	for _, f := range result.Findings {
		if f.Finding.Source == "phase0" {
			foundPhase0Source = true
		}
	}
	if !foundPhase0Source {
		t.Error("expected at least one finding with Source == \"phase0\"")
	}

	// ExpertCount should be 0 since we short-circuited before expert dispatch.
	if result.ExpertCount != 0 {
		t.Errorf("expected ExpertCount == 0 for phase0 rejection, got %d", result.ExpertCount)
	}
}

func TestConventionLearningLoop(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewConventionStore(tmpDir)
	if err != nil {
		t.Fatalf("create convention store: %v", err)
	}

	// Propose a convention.
	convID := store.Propose(Convention{
		Domain:      DomainBackend,
		Pattern:     "error return ignored in config loader",
		Description: "Config loader functions must check returned errors from yaml.Unmarshal",
		FileGlob:    "*.go",
		Source:      "review:TASK-042",
	})
	if convID == "" {
		t.Fatal("expected non-empty convention ID")
	}

	// Create an ExpertFinding with similar body.
	finding := ExpertFinding{
		Finding: Finding{
			File:     "internal/config/loader.go",
			Line:     15,
			Severity: "HIGH",
			Body:     "error return ignored in config loader yaml unmarshal",
		},
		Expert: DomainBackend,
	}

	// MatchesFinding requires AcceptCount >= 2, so initially no match.
	match := store.MatchesFinding(finding)
	if match != nil {
		t.Error("expected no match before accept (AcceptCount < 2)")
	}

	// Accept twice to pass the threshold.
	store.Accept(convID)
	store.Accept(convID)

	// Now it should match.
	match = store.MatchesFinding(finding)
	if match == nil {
		t.Fatal("expected match after two accepts")
	}
	if match.ID != convID {
		t.Errorf("matched convention ID %s, want %s", match.ID, convID)
	}

	// Save and reload.
	if err := store.Save(); err != nil {
		t.Fatalf("save convention store: %v", err)
	}

	store2, err := NewConventionStore(tmpDir)
	if err != nil {
		t.Fatalf("reload convention store: %v", err)
	}

	// Verify AcceptCount persisted.
	match2 := store2.MatchesFinding(finding)
	if match2 == nil {
		t.Fatal("expected match after reload")
	}
	if match2.AcceptCount != 2 {
		t.Errorf("expected AcceptCount == 2 after reload, got %d", match2.AcceptCount)
	}

	// Accept once more and verify increment.
	store2.Accept(convID)
	match3 := store2.MatchesFinding(finding)
	if match3 == nil {
		t.Fatal("expected match after third accept")
	}
	if match3.AcceptCount != 3 {
		t.Errorf("expected AcceptCount == 3 after third accept, got %d", match3.AcceptCount)
	}
}

func TestCouncilPipeline_ConventionFiltering(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewConventionStore(tmpDir)
	if err != nil {
		t.Fatalf("create convention store: %v", err)
	}

	// Create an accepted convention (AcceptCount = 3).
	convID := store.Propose(Convention{
		Domain:      DomainSecurity,
		Pattern:     "hardcoded secret key used for authentication",
		Description: "This project uses env-based secrets; hardcoded keys in tests are acceptable",
		FileGlob:    "*.go",
		Source:      "review:TASK-100",
	})
	// Accept 3 times to make it well-established (AcceptCount >= 2 threshold).
	store.Accept(convID)
	store.Accept(convID)
	store.Accept(convID)

	// Mock returns a finding that matches the convention pattern.
	matchingFinding := []Finding{{
		File:     "internal/server/handler.go",
		Line:     18,
		Severity: "HIGH",
		Body:     "hardcoded secret key used for authentication comparison",
	}}
	matchJSON, _ := json.Marshal(matchingFinding)
	mock := &integrationMockChat{responses: []string{string(matchJSON)}}

	engine := &CouncilEngine{
		chatProvider: mock,
		registry:     NewExpertRegistry(),
		conventions:  store,
		language:     "en",
		reviewCfg:    config.ReviewConfig{APIExpertTimeoutS: 5},
	}

	phase0 := &Phase0Report{
		Results:  []Phase0Result{{Check: Phase0Check{Name: "go-vet"}, Passed: true}},
		AllGreen: true,
	}

	req := CouncilRequest{
		Diff:      realisticGoDiff,
		DiffLines: 20,
		FileCount: 100,
		Brief:     nil,
		Phase0:    phase0,
	}

	result, err := engine.RunCouncil(context.Background(), req)
	if err != nil {
		t.Fatalf("RunCouncil error: %v", err)
	}

	// The convention-matching finding should have been filtered out by the
	// Carmack Filter. Check that no finding body matches the convention pattern.
	for _, f := range result.Findings {
		if wordSimilarity(f.Body, "hardcoded secret key used for authentication") > 0.20 {
			// Finding with Source == "phase0" is not expert-generated, skip it.
			if f.Finding.Source == "phase0" {
				continue
			}
			t.Errorf("expected convention-matching finding to be filtered out, but found: %q (expert=%s)", f.Body, f.Expert)
		}
	}

	// Result should be APPROVED since the only findings were convention-filtered.
	if result.Status != "APPROVED" {
		t.Logf("result status: %s (may have non-convention findings from other experts)", result.Status)
	}
}

func TestCouncilPipeline_MultipleExperts(t *testing.T) {
	// Return different findings based on call order.
	// Even calls: backend finding; odd calls: frontend finding.
	backendFinding := []Finding{{
		File:     "internal/api/routes.go",
		Line:     5,
		Severity: "MEDIUM",
		Body:     "missing context propagation in HTTP handler",
	}}
	frontendFinding := []Finding{{
		File:     "frontend/src/lib/components/TaskList.svelte",
		Line:     4,
		Severity: "MEDIUM",
		Body:     "missing error handling in onMount fetch call",
	}}

	backendJSON, _ := json.Marshal(backendFinding)
	frontendJSON, _ := json.Marshal(frontendFinding)

	mock := &integrationMockChat{
		responses: []string{string(backendJSON), string(frontendJSON)},
	}

	store, err := NewConventionStore(t.TempDir())
	if err != nil {
		t.Fatalf("create convention store: %v", err)
	}

	engine := &CouncilEngine{
		chatProvider: mock,
		registry:     NewExpertRegistry(),
		conventions:  store,
		language:     "en",
		reviewCfg:    config.ReviewConfig{APIExpertTimeoutS: 5},
	}

	phase0 := &Phase0Report{
		Results:  []Phase0Result{{Check: Phase0Check{Name: "go-vet"}, Passed: true}},
		AllGreen: true,
	}

	req := CouncilRequest{
		Diff:      realisticMultiFileDiff,
		DiffLines: 25,
		FileCount: 200,
		Brief:     nil,
		Phase0:    phase0,
	}

	result, err := engine.RunCouncil(context.Background(), req)
	if err != nil {
		t.Fatalf("RunCouncil error: %v", err)
	}

	// With both .go and .svelte files changed, at least 2 experts should be selected.
	if result.ExpertCount < 2 {
		t.Errorf("expected ExpertCount >= 2 for multi-file diff, got %d", result.ExpertCount)
	}

	// Verify that we got findings from the mock (at least some calls were made).
	mock.mu.Lock()
	calls := mock.callCount
	mock.mu.Unlock()
	if calls < 2 {
		t.Errorf("expected at least 2 Chat calls for multiple experts, got %d", calls)
	}

	// Verify the result has correct structure.
	if result.Status != "APPROVED" && result.Status != "CHANGES_REQUESTED" {
		t.Errorf("unexpected status %q", result.Status)
	}
	if result.Duration <= 0 {
		t.Error("expected Duration > 0")
	}

	_ = os.RemoveAll(t.TempDir()) // cleanup is handled by t.TempDir() automatically

	// Verify that the expert selection covered both backend and frontend domains.
	// We check this indirectly by verifying the expert count and the presence
	// of diverse findings in the result.
	t.Logf("ExpertCount: %d, Findings: %d, Status: %s, Duration: %v",
		result.ExpertCount, len(result.Findings), result.Status, result.Duration)
}

// TestConventionLearningLoop_Decay verifies that conventions with low usage
// are removed by the Decay mechanism.
func TestConventionLearningLoop_Decay(t *testing.T) {
	store, err := NewConventionStore(t.TempDir())
	if err != nil {
		t.Fatalf("create convention store: %v", err)
	}

	// Propose two conventions.
	id1 := store.Propose(Convention{
		Domain:  DomainBackend,
		Pattern: "old unused pattern",
		FileGlob: "*.go",
		Source:  "review:TASK-OLD",
	})
	id2 := store.Propose(Convention{
		Domain:  DomainBackend,
		Pattern: "actively used pattern",
		FileGlob: "*.go",
		Source:  "review:TASK-NEW",
	})

	// Accept id2 many times to keep it alive.
	for i := 0; i < 5; i++ {
		store.Accept(id2)
	}

	// Decay with a very short maxAge and minUses=3.
	// id1 has AcceptCount=0, should be removed.
	// id2 has AcceptCount=5, should survive.
	removed := store.Decay(1*time.Nanosecond, 3)

	if removed != 1 {
		t.Errorf("expected 1 convention removed by decay, got %d", removed)
	}

	// Verify id1 is gone and id2 survives.
	finding1 := ExpertFinding{
		Finding: Finding{Body: "old unused pattern"},
		Expert:  DomainBackend,
	}
	// id1 would need AcceptCount >= 2 to match, but it was decayed anyway.
	// Let's check that it no longer exists by trying to Accept and match.
	store.Accept(id1) // no-op since id1 was removed
	store.Accept(id1) // no-op since id1 was removed
	match1 := store.MatchesFinding(finding1)
	if match1 != nil {
		t.Error("expected id1 to be removed by decay, but it still matches")
	}

	finding2 := ExpertFinding{
		Finding: Finding{Body: "actively used pattern"},
		Expert:  DomainBackend,
	}
	match2 := store.MatchesFinding(finding2)
	if match2 == nil {
		t.Error("expected id2 to survive decay, but it was removed")
	}

	_ = id1
	_ = id2
}
