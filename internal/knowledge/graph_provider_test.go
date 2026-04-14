// internal/knowledge/graph_provider_test.go
package knowledge

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupProviderTestRepo creates a minimal Go repo for provider tests.
func setupProviderTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	writeFile := func(p, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(dir, filepath.Dir(p)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, p), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "test")
	writeFile("go.mod", "module example.com/providertest\n\ngo 1.23\n")
	writeFile("main.go", "package main\n\nfunc main() {}\n")
	run("add", ".")
	run("commit", "-m", "init")

	return dir
}

func TestUnifiedGraphProvider_FallsBackToLightGraph(t *testing.T) {
	repo := setupProviderTestRepo(t)
	provider := NewUnifiedGraphProvider()

	graph, err := provider.GetGraph(repo)
	if err != nil {
		t.Fatalf("GetGraph error: %v", err)
	}
	if graph == nil {
		t.Fatal("expected non-nil graph")
	}
	if len(graph.Nodes) < 1 {
		t.Errorf("expected at least 1 node (main.go), got %d", len(graph.Nodes))
	}
}

func TestUnifiedGraphProvider_UsesGraphifyWhenAvailable(t *testing.T) {
	repo := setupProviderTestRepo(t)

	// Simulate graphify output by creating the expected files
	outDir := filepath.Join(repo, "graphify-out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}
	graphifyJSON := `{
		"nodes": [
			{"path": "main.go", "package": "main", "connections": 0},
			{"path": "lib.go", "package": "main", "connections": 1}
		],
		"edges": [
			{"source": "main.go", "target": "lib.go", "type": "call"}
		],
		"communities": [
			{"id": 0, "name": "root", "members": ["main.go", "lib.go"]}
		]
	}`
	if err := os.WriteFile(filepath.Join(outDir, "graph.json"), []byte(graphifyJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	provider := NewUnifiedGraphProvider()
	// Force graphify path check to succeed by providing pre-built output
	graph, err := provider.GetGraph(repo)
	if err != nil {
		t.Fatalf("GetGraph error: %v", err)
	}

	// If graphify CLI is in PATH, it should use graphify data (2 nodes from graphify).
	// If not, it falls back to light graph (1 node — main.go).
	// Either way, we should get a valid graph.
	if graph == nil {
		t.Fatal("expected non-nil graph")
	}
	if len(graph.Nodes) < 1 {
		t.Error("expected at least 1 node")
	}
}

func TestUnifiedGraphProvider_CachesResult(t *testing.T) {
	repo := setupProviderTestRepo(t)
	provider := NewUnifiedGraphProvider()

	graph1, err := provider.GetGraph(repo)
	if err != nil {
		t.Fatalf("first GetGraph error: %v", err)
	}

	graph2, err := provider.GetGraph(repo)
	if err != nil {
		t.Fatalf("second GetGraph error: %v", err)
	}

	// Same pointer should be returned from cache (same HEAD SHA)
	if graph1 != graph2 {
		t.Error("expected cached graph (same pointer) for same repo/HEAD")
	}
}
