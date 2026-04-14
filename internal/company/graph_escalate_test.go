// internal/company/graph_escalate_test.go
package company

import (
	"testing"

	"github.com/hanfourmini/aisupervisor/internal/knowledge"
)

func TestShouldEscalateReview_SingleCommunity(t *testing.T) {
	graph := &knowledge.CodeGraph{
		Communities: []knowledge.Community{
			{ID: 0, Name: "internal/worker", Files: []string{"internal/worker/spawner.go", "internal/worker/monitor.go"}},
			{ID: 1, Name: "internal/company", Files: []string{"internal/company/company.go"}},
		},
		GodNodes: []string{},
	}

	files := []string{"internal/worker/spawner.go", "internal/worker/monitor.go"}
	if shouldEscalateReview(files, graph) {
		t.Error("expected no escalation for single-community change")
	}
}

func TestShouldEscalateReview_TwoCommunities(t *testing.T) {
	graph := &knowledge.CodeGraph{
		Communities: []knowledge.Community{
			{ID: 0, Name: "internal/worker", Files: []string{"internal/worker/spawner.go"}},
			{ID: 1, Name: "internal/company", Files: []string{"internal/company/company.go"}},
		},
		GodNodes: []string{},
	}

	files := []string{"internal/worker/spawner.go", "internal/company/company.go"}
	if shouldEscalateReview(files, graph) {
		t.Error("expected no escalation for 2-community change (threshold is 3)")
	}
}

func TestShouldEscalateReview_ThreePlusCommunities(t *testing.T) {
	graph := &knowledge.CodeGraph{
		Communities: []knowledge.Community{
			{ID: 0, Name: "internal/worker", Files: []string{"internal/worker/spawner.go"}},
			{ID: 1, Name: "internal/company", Files: []string{"internal/company/company.go"}},
			{ID: 2, Name: "internal/tmux", Files: []string{"internal/tmux/client.go"}},
			{ID: 3, Name: "internal/knowledge", Files: []string{"internal/knowledge/types.go"}},
		},
		GodNodes: []string{},
	}

	files := []string{
		"internal/worker/spawner.go",
		"internal/company/company.go",
		"internal/tmux/client.go",
	}
	if !shouldEscalateReview(files, graph) {
		t.Error("expected escalation for 3-community change")
	}
}

func TestShouldEscalateReview_GodNode(t *testing.T) {
	graph := &knowledge.CodeGraph{
		Communities: []knowledge.Community{
			{ID: 0, Name: "internal/company", Files: []string{"internal/company/company.go"}},
		},
		GodNodes: []string{"internal/company/company.go"},
	}

	// Even a single-community change should escalate if it touches a god node
	files := []string{"internal/company/company.go"}
	if !shouldEscalateReview(files, graph) {
		t.Error("expected escalation when god node is touched")
	}
}

func TestShouldEscalateReview_NilGraph(t *testing.T) {
	files := []string{"internal/worker/spawner.go"}
	if shouldEscalateReview(files, nil) {
		t.Error("expected no escalation for nil graph")
	}
}

func TestShouldEscalateReview_EmptyFiles(t *testing.T) {
	graph := &knowledge.CodeGraph{
		Communities: []knowledge.Community{
			{ID: 0, Name: "internal/worker", Files: []string{"internal/worker/spawner.go"}},
		},
		GodNodes: []string{},
	}

	if shouldEscalateReview(nil, graph) {
		t.Error("expected no escalation for empty file list")
	}
}

func TestShouldEscalateReview_UnknownFilesIgnored(t *testing.T) {
	graph := &knowledge.CodeGraph{
		Communities: []knowledge.Community{
			{ID: 0, Name: "internal/worker", Files: []string{"internal/worker/spawner.go"}},
		},
		GodNodes: []string{},
	}

	// unknown.go is not in any community — should not count
	files := []string{"internal/worker/spawner.go", "unknown.go", "also_unknown.go"}
	if shouldEscalateReview(files, graph) {
		t.Error("expected no escalation when unknown files don't push community count to 3+")
	}
}
