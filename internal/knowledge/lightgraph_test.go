// internal/knowledge/lightgraph_test.go
package knowledge

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupTempRepo creates a temp git repo with known Go files for testing.
func setupTempRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Init git repo
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

	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "test")

	// Create directory structure:
	// internal/worker/spawner.go   — imports internal/tmux
	// internal/worker/monitor.go   — imports internal/tmux
	// internal/tmux/client.go      — no imports within repo
	// internal/company/company.go  — imports internal/worker, internal/tmux
	mkdirAll := func(p string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(dir, p), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile := func(p, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, p), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// go.mod
	writeFile("go.mod", "module example.com/testrepo\n\ngo 1.23\n")

	mkdirAll("internal/tmux")
	writeFile("internal/tmux/client.go", `package tmux

type Client struct{}
`)

	mkdirAll("internal/worker")
	writeFile("internal/worker/spawner.go", `package worker

import "example.com/testrepo/internal/tmux"

var _ = tmux.Client{}
`)
	writeFile("internal/worker/monitor.go", `package worker

import "example.com/testrepo/internal/tmux"

var _ = tmux.Client{}
`)

	mkdirAll("internal/company")
	writeFile("internal/company/company.go", `package company

import (
	"example.com/testrepo/internal/worker"
	"example.com/testrepo/internal/tmux"
)

var _ = worker.Spawner{}
var _ = tmux.Client{}
`)

	// Commit 1: all files
	run("add", ".")
	run("commit", "-m", "initial")

	// Commit 2: change worker/spawner.go and company/company.go together (co-change)
	writeFile("internal/worker/spawner.go", `package worker

import "example.com/testrepo/internal/tmux"

var _ = tmux.Client{}
// updated
`)
	writeFile("internal/company/company.go", `package company

import (
	"example.com/testrepo/internal/worker"
	"example.com/testrepo/internal/tmux"
)

var _ = worker.Spawner{}
var _ = tmux.Client{}
// updated
`)
	run("add", ".")
	run("commit", "-m", "co-change worker+company")

	return dir
}

func TestBuildLightGraph_Nodes(t *testing.T) {
	repo := setupTempRepo(t)
	graph, err := BuildLightGraph(repo)
	if err != nil {
		t.Fatalf("BuildLightGraph error: %v", err)
	}

	// Should have 4 Go files as nodes
	if len(graph.Nodes) != 4 {
		t.Errorf("expected 4 nodes, got %d", len(graph.Nodes))
		for _, n := range graph.Nodes {
			t.Logf("  node: %s (pkg=%s, indegree=%d)", n.Path, n.Package, n.InDegree)
		}
	}

	// Verify packages are correctly extracted
	pkgs := make(map[string]string)
	for _, n := range graph.Nodes {
		pkgs[n.Path] = n.Package
	}
	if pkgs["internal/tmux/client.go"] != "tmux" {
		t.Errorf("expected package 'tmux' for client.go, got %q", pkgs["internal/tmux/client.go"])
	}
	if pkgs["internal/worker/spawner.go"] != "worker" {
		t.Errorf("expected package 'worker' for spawner.go, got %q", pkgs["internal/worker/spawner.go"])
	}
}

func TestBuildLightGraph_ImportEdges(t *testing.T) {
	repo := setupTempRepo(t)
	graph, err := BuildLightGraph(repo)
	if err != nil {
		t.Fatalf("BuildLightGraph error: %v", err)
	}

	// Count import edges
	importEdges := 0
	for _, e := range graph.Edges {
		if e.Type == EdgeImport {
			importEdges++
		}
	}

	// Expected import edges:
	// worker/spawner.go -> tmux/client.go
	// worker/monitor.go -> tmux/client.go
	// company/company.go -> worker/spawner.go (first file in worker pkg)
	// company/company.go -> tmux/client.go
	if importEdges < 3 {
		t.Errorf("expected at least 3 import edges, got %d", importEdges)
		for _, e := range graph.Edges {
			if e.Type == EdgeImport {
				t.Logf("  import: %s -> %s", e.From, e.To)
			}
		}
	}
}

func TestBuildLightGraph_CoChangeEdges(t *testing.T) {
	repo := setupTempRepo(t)
	graph, err := BuildLightGraph(repo)
	if err != nil {
		t.Fatalf("BuildLightGraph error: %v", err)
	}

	// Should have co-change edges from commit 2 (spawner.go + company.go changed together)
	found := false
	for _, e := range graph.Edges {
		if e.Type == EdgeCoChange {
			if (e.From == "internal/worker/spawner.go" && e.To == "internal/company/company.go") ||
				(e.From == "internal/company/company.go" && e.To == "internal/worker/spawner.go") {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected co-change edge between spawner.go and company.go")
		for _, e := range graph.Edges {
			if e.Type == EdgeCoChange {
				t.Logf("  co-change: %s -> %s (weight=%d)", e.From, e.To, e.Weight)
			}
		}
	}
}

func TestBuildLightGraph_InDegree(t *testing.T) {
	repo := setupTempRepo(t)
	graph, err := BuildLightGraph(repo)
	if err != nil {
		t.Fatalf("BuildLightGraph error: %v", err)
	}

	// tmux/client.go should have highest InDegree (imported by 3 files)
	degreeMap := make(map[string]int)
	for _, n := range graph.Nodes {
		degreeMap[n.Path] = n.InDegree
	}
	tmuxDeg := degreeMap["internal/tmux/client.go"]
	if tmuxDeg < 2 {
		t.Errorf("expected tmux/client.go InDegree >= 2, got %d", tmuxDeg)
	}
}

func TestBuildLightGraph_Communities(t *testing.T) {
	repo := setupTempRepo(t)
	graph, err := BuildLightGraph(repo)
	if err != nil {
		t.Fatalf("BuildLightGraph error: %v", err)
	}

	// Should have 3 communities: internal/worker, internal/tmux, internal/company
	if len(graph.Communities) != 3 {
		t.Errorf("expected 3 communities, got %d", len(graph.Communities))
		for _, c := range graph.Communities {
			t.Logf("  community: %q files=%v", c.Name, c.Files)
		}
	}

	// Worker community should have 2 files
	for _, c := range graph.Communities {
		if c.Name == "internal/worker" {
			if len(c.Files) != 2 {
				t.Errorf("expected 2 files in worker community, got %d", len(c.Files))
			}
		}
	}
}

func TestBuildLightGraph_GodNodes(t *testing.T) {
	repo := setupTempRepo(t)
	graph, err := BuildLightGraph(repo)
	if err != nil {
		t.Fatalf("BuildLightGraph error: %v", err)
	}

	// GodNodes should contain tmux/client.go (highest InDegree)
	if len(graph.GodNodes) == 0 {
		t.Fatal("expected at least 1 god node")
	}
	if graph.GodNodes[0] != "internal/tmux/client.go" {
		t.Errorf("expected top god node to be tmux/client.go, got %q", graph.GodNodes[0])
	}
}

func TestBuildLightGraph_HeadSHA(t *testing.T) {
	repo := setupTempRepo(t)
	graph, err := BuildLightGraph(repo)
	if err != nil {
		t.Fatalf("BuildLightGraph error: %v", err)
	}
	if graph.HeadSHA == "" {
		t.Error("expected non-empty HeadSHA")
	}
	if len(graph.HeadSHA) < 7 {
		t.Errorf("HeadSHA too short: %q", graph.HeadSHA)
	}
}

func TestBuildLightGraph_CacheRoundTrip(t *testing.T) {
	repo := setupTempRepo(t)

	// Build once — should create cache
	graph1, err := BuildLightGraph(repo)
	if err != nil {
		t.Fatalf("BuildLightGraph error: %v", err)
	}

	// Build again — should load from cache (same HEAD)
	graph2, err := BuildLightGraph(repo)
	if err != nil {
		t.Fatalf("BuildLightGraph (cached) error: %v", err)
	}

	if graph1.HeadSHA != graph2.HeadSHA {
		t.Errorf("cached graph HeadSHA mismatch: %q vs %q", graph1.HeadSHA, graph2.HeadSHA)
	}
	if len(graph1.Nodes) != len(graph2.Nodes) {
		t.Errorf("cached graph node count mismatch: %d vs %d", len(graph1.Nodes), len(graph2.Nodes))
	}
}
