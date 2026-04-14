// internal/knowledge/graphify_test.go
package knowledge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGraphifyIntegration_IsAvailable_WhenNotInstalled(t *testing.T) {
	gi := NewGraphifyIntegration("/tmp/nonexistent-repo")
	// In CI/test environments, graphify is almost certainly not in PATH.
	// This test verifies the method doesn't panic and returns a bool.
	_ = gi.IsAvailable()
	// We can't assert false because the test runner might have graphify installed.
	// Instead, verify the method works without errors.
}

func TestGraphifyIntegration_HasGraph_NoOutputDir(t *testing.T) {
	dir := t.TempDir()
	gi := NewGraphifyIntegration(dir)
	if gi.HasGraph() {
		t.Error("expected HasGraph() false when graphify-out/ does not exist")
	}
}

func TestGraphifyIntegration_HasGraph_WithOutputDir(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "graphify-out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "graph.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	gi := NewGraphifyIntegration(dir)
	if !gi.HasGraph() {
		t.Error("expected HasGraph() true when graphify-out/graph.json exists")
	}
}

func TestGraphifyIntegration_GetReport_NoFile(t *testing.T) {
	dir := t.TempDir()
	gi := NewGraphifyIntegration(dir)
	_, err := gi.GetReport()
	if err == nil {
		t.Error("expected error when GRAPH_REPORT.md does not exist")
	}
}

func TestGraphifyIntegration_GetReport_WithFile(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "graphify-out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "# Graph Report\nSome analysis here.\n"
	if err := os.WriteFile(filepath.Join(outDir, "GRAPH_REPORT.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	gi := NewGraphifyIntegration(dir)
	report, err := gi.GetReport()
	if err != nil {
		t.Fatalf("GetReport error: %v", err)
	}
	if report != content {
		t.Errorf("expected report %q, got %q", content, report)
	}
}

func TestGraphifyIntegration_GetGraphFromGraphify_ParsesJSON(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "graphify-out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a minimal graphify-compatible graph.json
	graphifyData := graphifyOutput{
		Nodes: []graphifyNode{
			{Path: "internal/worker/spawner.go", Package: "worker", Connections: 3},
			{Path: "internal/tmux/client.go", Package: "tmux", Connections: 5},
			{Path: "internal/company/company.go", Package: "company", Connections: 2},
		},
		Edges: []graphifyEdge{
			{Source: "internal/worker/spawner.go", Target: "internal/tmux/client.go", Type: "import"},
			{Source: "internal/company/company.go", Target: "internal/worker/spawner.go", Type: "call"},
		},
		Communities: []graphifyCommunity{
			{ID: 0, Name: "worker-cluster", Members: []string{"internal/worker/spawner.go"}},
			{ID: 1, Name: "infra-cluster", Members: []string{"internal/tmux/client.go"}},
			{ID: 2, Name: "company-cluster", Members: []string{"internal/company/company.go"}},
		},
	}

	data, _ := json.Marshal(graphifyData)
	if err := os.WriteFile(filepath.Join(outDir, "graph.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	gi := NewGraphifyIntegration(dir)
	graph, err := gi.GetGraphFromGraphify()
	if err != nil {
		t.Fatalf("GetGraphFromGraphify error: %v", err)
	}

	if len(graph.Nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(graph.Nodes))
	}
	if len(graph.Edges) != 2 {
		t.Errorf("expected 2 edges, got %d", len(graph.Edges))
	}
	if len(graph.Communities) != 3 {
		t.Errorf("expected 3 communities, got %d", len(graph.Communities))
	}

	// Verify InDegree was computed from edges
	degreeMap := make(map[string]int)
	for _, n := range graph.Nodes {
		degreeMap[n.Path] = n.InDegree
	}
	if degreeMap["internal/tmux/client.go"] != 1 {
		t.Errorf("expected tmux InDegree 1 (from edge), got %d", degreeMap["internal/tmux/client.go"])
	}

	// Verify GodNodes includes highest InDegree
	if len(graph.GodNodes) == 0 {
		t.Error("expected at least 1 god node")
	}
}
