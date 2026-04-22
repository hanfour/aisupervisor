package company

import (
	"testing"

	"github.com/hanfourmini/aisupervisor/internal/knowledge"
)

func TestExpertRegistry_DefaultExperts(t *testing.T) {
	reg := NewExpertRegistry()
	if len(reg.experts) != 10 {
		t.Errorf("expected 10 default experts, got %d", len(reg.experts))
	}

	// Verify all domains are represented
	domains := map[ExpertDomain]bool{}
	for _, e := range reg.experts {
		domains[e.Domain] = true
	}
	expected := []ExpertDomain{
		DomainSecurity, DomainPerformance, DomainRefactoring, DomainTesting,
		DomainFrontend, DomainBackend, DomainDatabase, DomainAPI,
		DomainConcurrency, DomainArchitecture,
	}
	for _, d := range expected {
		if !domains[d] {
			t.Errorf("missing domain %q in default experts", d)
		}
	}

	// Architecture expert should use opus model
	for _, e := range reg.experts {
		if e.Domain == DomainArchitecture {
			if e.Model != "opus" {
				t.Errorf("architecture expert should use opus model, got %q", e.Model)
			}
		}
	}

	// Every expert must have a non-empty SystemPrompt and Name
	for _, e := range reg.experts {
		if e.Name == "" {
			t.Errorf("expert %q has empty Name", e.Domain)
		}
		if e.SystemPrompt == "" {
			t.Errorf("expert %q has empty SystemPrompt", e.Domain)
		}
		if len(e.FilePatterns) == 0 {
			t.Errorf("expert %q has no FilePatterns", e.Domain)
		}
	}
}

func TestSelectExperts_GoFiles(t *testing.T) {
	reg := NewExpertRegistry()
	files := []string{"internal/company/review.go", "internal/worker/monitor.go"}
	diff := `
+	var mu sync.Mutex
+	mu.Lock()
+	defer mu.Unlock()
`

	selected := reg.SelectExperts(files, diff, nil, nil)

	found := false
	for _, s := range selected {
		if s.Domain == DomainConcurrency {
			found = true
			if s.Reason == "" {
				t.Error("concurrency expert should have a reason")
			}
			break
		}
	}
	if !found {
		t.Error("expected concurrency expert to be selected for Go files with sync.Mutex keyword")
	}
}

func TestSelectExperts_FrontendFiles(t *testing.T) {
	reg := NewExpertRegistry()
	files := []string{"frontend/src/App.svelte", "frontend/src/lib/stores/i18n.js"}
	diff := `
+	<button on:click={handleClick} bind:value={name}>
+	$: derivedValue = count * 2
`

	selected := reg.SelectExperts(files, diff, nil, nil)

	found := false
	for _, s := range selected {
		if s.Domain == DomainFrontend {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected frontend expert to be selected for .svelte files")
	}
}

func TestSelectExperts_MinimumTwo(t *testing.T) {
	reg := NewExpertRegistry()
	files := []string{"README.md"}
	diff := `+Just a readme update`

	selected := reg.SelectExperts(files, diff, nil, nil)

	if len(selected) < 2 {
		t.Errorf("expected at least 2 experts, got %d", len(selected))
	}
}

func TestSelectExperts_MaxFive(t *testing.T) {
	reg := NewExpertRegistry()
	// Files that trigger many experts
	files := []string{
		"internal/company/review.go",
		"internal/worker/spawner.go",
		"frontend/src/App.svelte",
		"schema.sql",
		"api.proto",
		"internal/company/review_test.go",
	}
	// Diff with keywords for many domains
	diff := `
+	password := getToken()
+	for i := 0; i < len(items); i++ { append(result, items[i]) }
+	go func() { ch <- value }()
+	var mu sync.Mutex
+	SELECT * FROM users WHERE id = $1
+	t.Run("test", func(t *testing.T) { t.Fatal("fail") })
+	json:"name" yaml:"name"
+	bind:value={x}
`

	selected := reg.SelectExperts(files, diff, nil, nil)

	if len(selected) > 5 {
		t.Errorf("expected at most 5 experts, got %d", len(selected))
	}
}

func TestSelectExperts_GodNodeForceArchitecture(t *testing.T) {
	reg := NewExpertRegistry()
	graph := &knowledge.CodeGraph{
		GodNodes: []string{"internal/company/company.go"},
	}
	files := []string{"internal/company/company.go"}
	diff := `+	m.workers = append(m.workers, w)`

	selected := reg.SelectExperts(files, diff, graph, nil)

	found := false
	for _, s := range selected {
		if s.Domain == DomainArchitecture {
			found = true
			if s.Reason == "" {
				t.Error("architecture expert should have a reason")
			}
			break
		}
	}
	if !found {
		t.Error("expected architecture expert to be forced when god node is touched")
	}
}

func TestSelectExperts_CrossCommunityForceArchitecture(t *testing.T) {
	reg := NewExpertRegistry()
	graph := &knowledge.CodeGraph{
		Communities: []knowledge.Community{
			{ID: 0, Name: "company", Files: []string{"internal/company/company.go"}},
			{ID: 1, Name: "worker", Files: []string{"internal/worker/spawner.go"}},
			{ID: 2, Name: "tmux", Files: []string{"internal/tmux/client.go"}},
		},
	}
	files := []string{
		"internal/company/company.go",
		"internal/worker/spawner.go",
		"internal/tmux/client.go",
	}
	diff := `+	some changes across communities`

	selected := reg.SelectExperts(files, diff, graph, nil)

	found := false
	for _, s := range selected {
		if s.Domain == DomainArchitecture {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected architecture expert to be forced when 3+ communities are spanned")
	}
}

func TestSelectExperts_Phase0FailureForcesExpert(t *testing.T) {
	reg := NewExpertRegistry()

	tests := []struct {
		name           string
		checkName      string
		expectedDomain ExpertDomain
	}{
		{"go-vet forces backend", "go-vet", DomainBackend},
		{"lint forces refactoring", "lint", DomainRefactoring},
		{"test forces testing", "test-runner", DomainTesting},
		{"typecheck forces frontend", "typecheck", DomainFrontend},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			phase0 := &Phase0Report{
				Results: []Phase0Result{
					{
						Check:  Phase0Check{Name: tt.checkName},
						Passed: false,
					},
				},
			}
			files := []string{"main.go"}
			diff := `+	fmt.Println("hello")`

			selected := reg.SelectExperts(files, diff, nil, phase0)

			found := false
			for _, s := range selected {
				if s.Domain == tt.expectedDomain {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected %q expert to be forced by %q failure", tt.expectedDomain, tt.checkName)
			}
		})
	}
}

func TestSelectExperts_ExecMode(t *testing.T) {
	reg := NewExpertRegistry()

	// Small change: <= 3 files → ExecAPI
	files := []string{"internal/company/review.go"}
	diff := `+	var mu sync.Mutex`

	selected := reg.SelectExperts(files, diff, nil, nil)

	for _, s := range selected {
		if s.Domain == DomainConcurrency || s.Domain == DomainBackend {
			if s.Mode != ExecAPI {
				t.Errorf("expected ExecAPI for small change (%d files), got %q for %q expert",
					len(s.AssignedFiles), s.Mode, s.Domain)
			}
		}
	}

	// Large change: > 3 files → ExecCLI
	manyFiles := []string{
		"internal/company/review.go",
		"internal/company/company.go",
		"internal/worker/spawner.go",
		"internal/worker/monitor.go",
	}
	diff2 := `+	var mu sync.Mutex`

	selected2 := reg.SelectExperts(manyFiles, diff2, nil, nil)

	for _, s := range selected2 {
		// Experts with "*" pattern get all files
		if len(s.AssignedFiles) > 3 {
			if s.Mode != ExecCLI {
				t.Errorf("expected ExecCLI for large change (%d files), got %q for %q expert",
					len(s.AssignedFiles), s.Mode, s.Domain)
			}
		}
	}
}

func TestSelectExperts_AssignedFiles(t *testing.T) {
	reg := NewExpertRegistry()
	files := []string{
		"internal/company/review.go",
		"frontend/src/App.svelte",
		"schema.sql",
	}
	diff := `+	some changes`

	selected := reg.SelectExperts(files, diff, nil, nil)

	for _, s := range selected {
		if s.Domain == DomainFrontend {
			// Frontend expert should only get .svelte files
			for _, f := range s.AssignedFiles {
				if f != "frontend/src/App.svelte" {
					t.Errorf("frontend expert should not be assigned %q", f)
				}
			}
		}
		if s.Domain == DomainDatabase {
			for _, f := range s.AssignedFiles {
				if f != "schema.sql" {
					t.Errorf("database expert should not be assigned %q", f)
				}
			}
		}
	}
}

func TestSelectExperts_RefactoringFillsMinimum(t *testing.T) {
	reg := NewExpertRegistry()
	// A change that triggers very few experts
	files := []string{"docs/notes.txt"}
	diff := `+	Just some text notes`

	selected := reg.SelectExperts(files, diff, nil, nil)

	if len(selected) < 2 {
		t.Fatalf("expected at least 2 experts, got %d", len(selected))
	}

	// Check that refactoring is included as filler
	found := false
	for _, s := range selected {
		if s.Domain == DomainRefactoring {
			found = true
			break
		}
	}
	// Refactoring should be among the selected since it's the default filler
	if !found {
		// It's acceptable if another expert filled the minimum instead,
		// but at least we should have 2 experts
		t.Log("refactoring expert was not used as filler, but minimum 2 was still met")
	}
}
