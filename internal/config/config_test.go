package config

import (
	"strings"
	"testing"
)

func TestValidateDefaults(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default config should be valid: %v", err)
	}
}

func TestValidateInvalidTier(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WorkerTiers = []WorkerTierConfig{
		{Tier: "intern", CLITool: "claude"},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid tier")
	}
	if !strings.Contains(err.Error(), "invalid worker tier") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateNegativeMaxWorkers(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WorkerTiers = []WorkerTierConfig{
		{Tier: "engineer", CLITool: "aider", MaxWorkers: -1},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for negative max_workers")
	}
	if !strings.Contains(err.Error(), "max_workers") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateBenchmarkScoreRange(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Training.Promotion.MinBenchmarkScore = 1.5
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for out-of-range benchmark score")
	}
	if !strings.Contains(err.Error(), "min_benchmark_score") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateApprovalRateRange(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Training.Promotion.MinApprovalRate = -0.1
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for negative approval rate")
	}
	if !strings.Contains(err.Error(), "min_approval_rate") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateAutoTriggerNegative(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Training.Finetune.AutoTrigger = -5
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for negative auto_trigger")
	}
	if !strings.Contains(err.Error(), "auto_trigger") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateConfidenceThreshold(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Decision.ConfidenceThreshold = 2.0
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for confidence > 1")
	}
	if !strings.Contains(err.Error(), "confidence_threshold") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateValidTiers(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WorkerTiers = []WorkerTierConfig{
		{Tier: "consultant", CLITool: "claude", MaxWorkers: 1},
		{Tier: "manager", CLITool: "claude", MaxWorkers: 2},
		{Tier: "engineer", CLITool: "aider", MaxWorkers: 6},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid tiers should pass: %v", err)
	}
}

func TestValidateInvalidCLITool(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WorkerTiers = []WorkerTierConfig{
		{Tier: "engineer", CLITool: "vim"},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid cli_tool")
	}
	if !strings.Contains(err.Error(), "invalid cli_tool") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateInvalidReadyCheckRegex(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WorkerTiers = []WorkerTierConfig{
		{Tier: "engineer", CLITool: "aider", ReadyCheck: "[invalid"},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid ready_check regex")
	}
	if !strings.Contains(err.Error(), "invalid ready_check regex") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateValidReadyCheck(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WorkerTiers = []WorkerTierConfig{
		{Tier: "engineer", CLITool: "aider", ReadyCheck: `^>\s*$`},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid ready_check should pass: %v", err)
	}
}

func TestKarpathyGuidelines_AllTagsHaveEntries(t *testing.T) {
	guidelines := KarpathyGuidelines()
	expectedTags := []string{"assumptions", "overengineered", "scope_creep", "no_verification"}
	for _, tag := range expectedTags {
		if _, ok := guidelines[tag]; !ok {
			t.Errorf("missing guideline for tag %q", tag)
		}
	}
}

func TestClassifyViolations_ScopeCreep(t *testing.T) {
	output := "REJECTED: The changes include unrelated reformatting and out of scope modifications to the config parser."
	tags := ClassifyViolations(output)
	found := false
	for _, tag := range tags {
		if tag == "scope_creep" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected scope_creep tag, got %v", tags)
	}
}

func TestClassifyViolations_MultipleViolations(t *testing.T) {
	output := "REJECTED: Code is overengineered with unnecessary abstraction layers. Also, no test was written for the new endpoint."
	tags := ClassifyViolations(output)
	if len(tags) < 2 {
		t.Errorf("expected at least 2 tags, got %v", tags)
	}
}

func TestClassifyViolations_NoMatch(t *testing.T) {
	output := "REJECTED: The logic is incorrect, the sorting algorithm returns wrong results."
	tags := ClassifyViolations(output)
	if len(tags) != 0 {
		t.Errorf("expected 0 tags for generic rejection, got %v", tags)
	}
}

func TestClassifyViolations_CaseInsensitive(t *testing.T) {
	output := "REJECTED: Worker made an ASSUMPTION about the API format without checking."
	tags := ClassifyViolations(output)
	found := false
	for _, tag := range tags {
		if tag == "assumptions" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected assumptions tag, got %v", tags)
	}
}
