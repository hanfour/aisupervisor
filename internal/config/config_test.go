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
