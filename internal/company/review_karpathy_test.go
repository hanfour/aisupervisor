package company

import (
	"testing"

	"github.com/hanfourmini/aisupervisor/internal/config"
)

func TestClassifyViolations_IntegrationWithReview(t *testing.T) {
	output := "REJECTED\nThe code has unrelated changes to the logger module and assumed the API returns JSON without checking."
	tags := config.ClassifyViolations(output)

	hasScope := false
	hasAssumptions := false
	for _, tag := range tags {
		if tag == "scope_creep" {
			hasScope = true
		}
		if tag == "assumptions" {
			hasAssumptions = true
		}
	}
	if !hasScope {
		t.Error("expected scope_creep tag")
	}
	if !hasAssumptions {
		t.Error("expected assumptions tag")
	}
}
