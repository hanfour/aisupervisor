package growth

import (
	"testing"
)

func TestClassifyFeedback_Praise(t *testing.T) {
	tests := []struct{ msg string }{
		{"Great job on the API!"},
		{"做得好，前端很漂亮"},
		{"This is perfect"},
	}
	for _, tt := range tests {
		if ClassifyFeedback(tt.msg) != FeedbackPraise {
			t.Errorf("expected praise for %q", tt.msg)
		}
	}
}

func TestClassifyFeedback_Correction(t *testing.T) {
	tests := []struct{ msg string }{
		{"This is wrong, fix the error handling"},
		{"不對，這裡要修改"},
		{"Don't use global variables"},
	}
	for _, tt := range tests {
		if ClassifyFeedback(tt.msg) != FeedbackCorrection {
			t.Errorf("expected correction for %q", tt.msg)
		}
	}
}

func TestClassifyFeedback_Preference(t *testing.T) {
	if ClassifyFeedback("I prefer using interfaces for this") != FeedbackPreference {
		t.Error("expected preference")
	}
}

func TestInferBranches(t *testing.T) {
	branches := InferBranches("The CSS styling and UI component look great")
	found := false
	for _, b := range branches {
		if b == BranchFrontend {
			found = true
		}
	}
	if !found {
		t.Error("should infer frontend branch from CSS/UI keywords")
	}
}

func TestInferBranches_Multiple(t *testing.T) {
	branches := InferBranches("The API security needs improvement")
	branchSet := make(map[SkillBranch]bool)
	for _, b := range branches {
		branchSet[b] = true
	}
	if !branchSet[BranchBackend] {
		t.Error("should infer backend from API")
	}
	if !branchSet[BranchSecurity] {
		t.Error("should infer security from security keyword")
	}
}

func TestInferBranches_Empty(t *testing.T) {
	branches := InferBranches("Thanks for the update")
	if len(branches) != 0 {
		t.Errorf("expected no branches, got %v", branches)
	}
}
