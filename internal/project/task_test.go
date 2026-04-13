package project

import (
	"testing"
)

func TestRejection_HasViolationTags(t *testing.T) {
	r := Rejection{
		Stage:         TaskReady,
		RejectorID:    "mgr-1",
		Reason:        "scope creep",
		ViolationTags: []string{"scope_creep", "no_verification"},
	}
	if len(r.ViolationTags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(r.ViolationTags))
	}
	if r.ViolationTags[0] != "scope_creep" {
		t.Errorf("expected scope_creep, got %s", r.ViolationTags[0])
	}
}
