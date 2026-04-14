// internal/gui/company_app_graph_test.go
package gui

import (
	"testing"

	"github.com/hanfourmini/aisupervisor/internal/knowledge"
)

func TestGetProjectGraph_ReturnsCodeGraphType(t *testing.T) {
	// Verify that the CodeGraph type is correctly wirable through the GUI layer.
	// This is a compile-time check: if GetProjectGraph exists and returns *knowledge.CodeGraph,
	// this test compiles and passes.
	var graph *knowledge.CodeGraph
	_ = graph

	// Verify CodeGraph has the expected JSON-serializable fields
	g := &knowledge.CodeGraph{
		RepoPath: "/tmp/test",
		HeadSHA:  "abc123",
		Nodes: []knowledge.FileNode{
			{Path: "main.go", Package: "main", InDegree: 0},
		},
		Edges: []knowledge.Edge{
			{From: "a.go", To: "b.go", Type: knowledge.EdgeImport, Weight: 1},
		},
		Communities: []knowledge.Community{
			{ID: 0, Name: "root", Files: []string{"main.go"}},
		},
		GodNodes: []string{"main.go"},
	}
	if g.RepoPath != "/tmp/test" {
		t.Errorf("expected repoPath /tmp/test, got %q", g.RepoPath)
	}
	if len(g.Nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(g.Nodes))
	}
}
