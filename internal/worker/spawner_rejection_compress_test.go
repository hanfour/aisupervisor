package worker

import (
	"strings"
	"testing"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/project"
)

func TestCompressRejectionHistory_ShortHistory(t *testing.T) {
	task := &project.Task{
		RejectionHistory: []project.Rejection{
			{
				Reason:        "Missing error handling in handler.go",
				ViolationTags: []string{"error-handling"},
				Timestamp:     time.Now().Add(-2 * time.Hour),
			},
			{
				Reason:        "Tests not covering edge case",
				ViolationTags: []string{"test-coverage"},
				Timestamp:     time.Now().Add(-1 * time.Hour),
			},
		},
	}

	result := compressRejectionHistory(task, 3000)

	// Under limit, should contain both full reasons
	if !strings.Contains(result, "Missing error handling in handler.go") {
		t.Error("expected first full reason in output")
	}
	if !strings.Contains(result, "Tests not covering edge case") {
		t.Error("expected second full reason in output")
	}
}

func TestCompressRejectionHistory_LongHistoryCompressesOld(t *testing.T) {
	// Create 5 rejections where total text exceeds 3000 chars
	longReason := strings.Repeat("This is a verbose rejection reason with detailed feedback. ", 30) // ~1740 chars each
	task := &project.Task{
		RejectionHistory: []project.Rejection{
			{
				Reason:        longReason,
				ViolationTags: []string{"style", "naming"},
				Timestamp:     time.Now().Add(-5 * time.Hour),
			},
			{
				Reason:        longReason,
				ViolationTags: []string{"error-handling"},
				Timestamp:     time.Now().Add(-4 * time.Hour),
			},
			{
				Reason:        longReason,
				ViolationTags: []string{"test-coverage"},
				Timestamp:     time.Now().Add(-3 * time.Hour),
			},
			{
				Reason:        "Recent rejection: fix the nil pointer",
				ViolationTags: []string{"bug"},
				Timestamp:     time.Now().Add(-2 * time.Hour),
			},
			{
				Reason:        "Latest rejection: add unit tests",
				ViolationTags: []string{"test-coverage"},
				Timestamp:     time.Now().Add(-1 * time.Hour),
			},
		},
	}

	result := compressRejectionHistory(task, 3000)

	// Last 2 rejections should be present in full
	if !strings.Contains(result, "Recent rejection: fix the nil pointer") {
		t.Error("expected second-to-last rejection in full")
	}
	if !strings.Contains(result, "Latest rejection: add unit tests") {
		t.Error("expected last rejection in full")
	}

	// Old rejections should be compressed
	if !strings.Contains(result, "Previously rejected 3 times") {
		t.Error("expected compressed summary of old rejections")
	}

	// Compressed summary should include violation tags
	if !strings.Contains(result, "style") || !strings.Contains(result, "naming") {
		t.Error("expected violation tags in compressed summary")
	}

	// Full text of old rejections should NOT appear
	if strings.Contains(result, "This is a verbose rejection reason") {
		t.Error("old rejection full text should be compressed away")
	}
}

func TestCompressRejectionHistory_Empty(t *testing.T) {
	task := &project.Task{}
	result := compressRejectionHistory(task, 3000)
	if result != "" {
		t.Errorf("expected empty string for no rejections, got %q", result)
	}
}

func TestCompressRejectionHistory_ExactlyTwoLong(t *testing.T) {
	// With only 2 rejections that exceed the limit, both should still be kept in full
	// because we always keep the last 2
	longReason := strings.Repeat("X", 2000)
	task := &project.Task{
		RejectionHistory: []project.Rejection{
			{
				Reason:        longReason,
				ViolationTags: []string{"a"},
				Timestamp:     time.Now().Add(-2 * time.Hour),
			},
			{
				Reason:        longReason,
				ViolationTags: []string{"b"},
				Timestamp:     time.Now().Add(-1 * time.Hour),
			},
		},
	}

	result := compressRejectionHistory(task, 3000)

	// Both should be present (they are the last 2)
	if !strings.Contains(result, longReason) {
		t.Error("with only 2 rejections, both should be kept in full")
	}
	// Should NOT contain "Previously rejected"
	if strings.Contains(result, "Previously rejected") {
		t.Error("should not compress when there are only 2 rejections")
	}
}

func TestCompressRejectionHistory_SingleRejection(t *testing.T) {
	task := &project.Task{
		RejectionHistory: []project.Rejection{
			{
				Reason:        "Fix the bug",
				ViolationTags: []string{"bug"},
				Timestamp:     time.Now(),
			},
		},
	}

	result := compressRejectionHistory(task, 3000)
	if !strings.Contains(result, "Fix the bug") {
		t.Error("single rejection should appear in full")
	}
}
