package config

import (
	"testing"
)

func TestKarpathyGuidelines_FullPipeline(t *testing.T) {
	// 1. Simulate rejection output from a real review
	rejectionOutput := `REJECTED
The implementation has several issues:
1. You assumed the input would always be JSON without verifying the content-type header
2. The code includes an unnecessary abstraction layer with a Strategy pattern for a single discount type
3. Several unrelated files were reformatted and import ordering was changed
4. No tests were written for the new validation logic`

	// 2. Classify violations
	tags := ClassifyViolations(rejectionOutput)

	// 3. Verify all 4 violations detected
	tagSet := make(map[string]bool)
	for _, tag := range tags {
		tagSet[tag] = true
	}

	expected := []string{"assumptions", "overengineered", "scope_creep", "no_verification"}
	for _, e := range expected {
		if !tagSet[e] {
			t.Errorf("expected tag %q not found in %v", e, tags)
		}
	}

	// 4. Verify guidelines exist for all tags
	guidelines := KarpathyGuidelines()
	for _, tag := range tags {
		if g, ok := guidelines[tag]; !ok || g == "" {
			t.Errorf("missing or empty guideline for tag %q", tag)
		}
	}
}
