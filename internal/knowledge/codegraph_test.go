// internal/knowledge/codegraph_test.go
package knowledge

import (
	"testing"
)

func TestFileNode_Creation(t *testing.T) {
	node := FileNode{
		Path:     "internal/worker/spawner.go",
		Package:  "worker",
		InDegree: 5,
	}
	if node.Path != "internal/worker/spawner.go" {
		t.Errorf("expected path 'internal/worker/spawner.go', got %q", node.Path)
	}
	if node.Package != "worker" {
		t.Errorf("expected package 'worker', got %q", node.Package)
	}
	if node.InDegree != 5 {
		t.Errorf("expected InDegree 5, got %d", node.InDegree)
	}
}

func TestEdge_Creation(t *testing.T) {
	edge := Edge{
		From:   "internal/company/company.go",
		To:     "internal/worker/spawner.go",
		Type:   EdgeImport,
		Weight: 1,
	}
	if edge.Type != EdgeImport {
		t.Errorf("expected type %q, got %q", EdgeImport, edge.Type)
	}
	coChange := Edge{
		From:   "internal/company/company.go",
		To:     "internal/company/review.go",
		Type:   EdgeCoChange,
		Weight: 7,
	}
	if coChange.Type != EdgeCoChange {
		t.Errorf("expected type %q, got %q", EdgeCoChange, coChange.Type)
	}
}

func TestCommunity_Creation(t *testing.T) {
	c := Community{
		ID:    1,
		Name:  "internal/worker",
		Files: []string{"internal/worker/spawner.go", "internal/worker/monitor.go"},
	}
	if c.ID != 1 {
		t.Errorf("expected ID 1, got %d", c.ID)
	}
	if len(c.Files) != 2 {
		t.Errorf("expected 2 files, got %d", len(c.Files))
	}
}

func TestCodeGraph_Creation(t *testing.T) {
	graph := &CodeGraph{
		RepoPath: "/tmp/repo",
		HeadSHA:  "abc123",
		Nodes: []FileNode{
			{Path: "internal/worker/spawner.go", Package: "worker", InDegree: 3},
			{Path: "internal/company/company.go", Package: "company", InDegree: 5},
		},
		Edges: []Edge{
			{From: "internal/company/company.go", To: "internal/worker/spawner.go", Type: EdgeImport, Weight: 1},
		},
		Communities: []Community{
			{ID: 0, Name: "internal/worker", Files: []string{"internal/worker/spawner.go"}},
			{ID: 1, Name: "internal/company", Files: []string{"internal/company/company.go"}},
		},
		GodNodes: []string{"internal/company/company.go"},
	}
	if len(graph.Nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(graph.Nodes))
	}
	if len(graph.GodNodes) != 1 {
		t.Errorf("expected 1 god node, got %d", len(graph.GodNodes))
	}
}

func TestGetCommunityForFile(t *testing.T) {
	graph := &CodeGraph{
		Communities: []Community{
			{ID: 0, Name: "internal/worker", Files: []string{"internal/worker/spawner.go", "internal/worker/monitor.go"}},
			{ID: 1, Name: "internal/company", Files: []string{"internal/company/company.go", "internal/company/review.go"}},
			{ID: 2, Name: "internal/knowledge", Files: []string{"internal/knowledge/types.go"}},
		},
	}

	tests := []struct {
		name     string
		file     string
		wantID   int
		wantName string
		wantNil  bool
	}{
		{"finds worker community", "internal/worker/spawner.go", 0, "internal/worker", false},
		{"finds company community", "internal/company/review.go", 1, "internal/company", false},
		{"returns nil for unknown file", "internal/config/config.go", 0, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetCommunityForFile(graph, tt.file)
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got community %q", got.Name)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil community")
			}
			if got.ID != tt.wantID {
				t.Errorf("expected ID %d, got %d", tt.wantID, got.ID)
			}
			if got.Name != tt.wantName {
				t.Errorf("expected name %q, got %q", tt.wantName, got.Name)
			}
		})
	}
}

func TestGetGodNodes(t *testing.T) {
	graph := &CodeGraph{
		Nodes: []FileNode{
			{Path: "a.go", InDegree: 1},
			{Path: "b.go", InDegree: 10},
			{Path: "c.go", InDegree: 5},
			{Path: "d.go", InDegree: 8},
			{Path: "e.go", InDegree: 3},
			{Path: "f.go", InDegree: 12},
		},
	}

	tests := []struct {
		name string
		topN int
		want []string
	}{
		{"top 3", 3, []string{"f.go", "b.go", "d.go"}},
		{"top 1", 1, []string{"f.go"}},
		{"top exceeds count", 10, []string{"f.go", "b.go", "d.go", "c.go", "e.go", "a.go"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetGodNodes(graph, tt.topN)
			if len(got) != len(tt.want) {
				t.Fatalf("expected %d god nodes, got %d: %v", len(tt.want), len(got), got)
			}
			for i, path := range got {
				if path != tt.want[i] {
					t.Errorf("position %d: expected %q, got %q", i, tt.want[i], path)
				}
			}
		})
	}
}
