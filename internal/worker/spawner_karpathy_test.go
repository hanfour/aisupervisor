package worker

import (
	"strings"
	"testing"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/project"
)

func TestBuildKarpathyOverlay_NoHistory(t *testing.T) {
	task := &project.Task{ID: "t1"}
	overlay := buildKarpathyOverlay(task, "en")
	if overlay != "" {
		t.Errorf("expected empty overlay for no rejection history, got %q", overlay)
	}
}

func TestBuildKarpathyOverlay_WithTags(t *testing.T) {
	task := &project.Task{
		ID: "t1",
		RejectionHistory: []project.Rejection{
			{
				Stage:         project.TaskReady,
				RejectorID:    "mgr-1",
				Reason:        "scope creep",
				ViolationTags: []string{"scope_creep", "no_verification"},
				Timestamp:     time.Now(),
			},
		},
	}
	overlay := buildKarpathyOverlay(task, "en")
	if !strings.Contains(overlay, "Behavioral Guidelines") {
		t.Error("expected guidelines header in overlay")
	}
	if !strings.Contains(overlay, "Only modify code directly related") {
		t.Error("expected scope_creep guideline in overlay")
	}
	if !strings.Contains(overlay, "you MUST verify") {
		t.Error("expected no_verification guideline in overlay")
	}
}

func TestBuildKarpathyOverlay_Deduplicates(t *testing.T) {
	task := &project.Task{
		ID: "t1",
		RejectionHistory: []project.Rejection{
			{ViolationTags: []string{"scope_creep"}},
			{ViolationTags: []string{"scope_creep", "assumptions"}},
		},
	}
	overlay := buildKarpathyOverlay(task, "en")
	count := strings.Count(overlay, "Only modify code directly related")
	if count != 1 {
		t.Errorf("expected scope_creep guideline once, found %d times", count)
	}
	if !strings.Contains(overlay, "explicitly state your assumptions") {
		t.Error("expected assumptions guideline in overlay")
	}
}

func TestBuildKarpathyOverlay_ZhTW(t *testing.T) {
	task := &project.Task{
		ID: "t1",
		RejectionHistory: []project.Rejection{
			{ViolationTags: []string{"assumptions"}},
		},
	}
	overlay := buildKarpathyOverlay(task, "zh-TW")
	if !strings.Contains(overlay, "行為準則") {
		t.Error("expected zh-TW header in overlay")
	}
}
