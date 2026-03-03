package company

import (
	"testing"

	"github.com/hanfourmini/aisupervisor/internal/project"
	"github.com/hanfourmini/aisupervisor/internal/worker"
)

func TestModelStrategy_TierDefault(t *testing.T) {
	ms := NewModelStrategy()

	w := &worker.Worker{Tier: worker.TierEngineer}
	model := ms.ResolveModel(w, nil)
	if model != "sonnet" {
		t.Errorf("engineer default = %q, want sonnet", model)
	}

	w = &worker.Worker{Tier: worker.TierConsultant}
	model = ms.ResolveModel(w, nil)
	if model != "opus" {
		t.Errorf("consultant default = %q, want opus", model)
	}

	w = &worker.Worker{Tier: worker.TierManager}
	model = ms.ResolveModel(w, nil)
	if model != "opus" {
		t.Errorf("manager default = %q, want opus", model)
	}
}

func TestModelStrategy_SkillProfileOverride(t *testing.T) {
	ms := NewModelStrategy()

	w := &worker.Worker{Tier: worker.TierEngineer, SkillProfile: "architect"}
	model := ms.ResolveModel(w, nil)
	if model != "opus" {
		t.Errorf("architect profile = %q, want opus", model)
	}

	w = &worker.Worker{Tier: worker.TierEngineer, SkillProfile: "coder"}
	model = ms.ResolveModel(w, nil)
	if model != "sonnet" {
		t.Errorf("coder profile = %q, want sonnet", model)
	}
}

func TestModelStrategy_TaskTypeOverride(t *testing.T) {
	ms := NewModelStrategy()

	w := &worker.Worker{Tier: worker.TierEngineer}
	task := &project.Task{Type: project.TaskTypeResearch}
	model := ms.ResolveModel(w, task)
	if model != "opus" {
		t.Errorf("research task = %q, want opus", model)
	}
}

func TestModelStrategy_WorkerOverride(t *testing.T) {
	ms := NewModelStrategy()

	w := &worker.Worker{
		Tier:         worker.TierEngineer,
		SkillProfile: "architect",
		ModelVersion: "haiku",
	}
	task := &project.Task{Type: project.TaskTypeResearch}
	model := ms.ResolveModel(w, task)
	if model != "haiku" {
		t.Errorf("worker override = %q, want haiku", model)
	}
}

func TestModelStrategy_Priority(t *testing.T) {
	ms := NewModelStrategy()

	// Worker model > task type > profile > tier
	w := &worker.Worker{Tier: worker.TierEngineer, SkillProfile: "architect", ModelVersion: "haiku"}
	task := &project.Task{Type: project.TaskTypeResearch}
	if ms.ResolveModel(w, task) != "haiku" {
		t.Error("worker model should have highest priority")
	}

	// Without worker model: task type > profile
	w = &worker.Worker{Tier: worker.TierEngineer, SkillProfile: "coder"}
	task = &project.Task{Type: project.TaskTypeResearch}
	if ms.ResolveModel(w, task) != "opus" {
		t.Error("task type should override profile")
	}

	// Without task override: profile > tier
	w = &worker.Worker{Tier: worker.TierEngineer, SkillProfile: "architect"}
	task = &project.Task{Type: project.TaskTypeCode}
	if ms.ResolveModel(w, task) != "opus" {
		t.Error("profile should override tier default")
	}
}

func TestModelStrategy_SetOverrides(t *testing.T) {
	ms := NewModelStrategy()

	ms.SetTierDefault(worker.TierEngineer, "haiku")
	w := &worker.Worker{Tier: worker.TierEngineer}
	if ms.ResolveModel(w, nil) != "haiku" {
		t.Error("custom tier default not applied")
	}

	ms.SetProfileOverride("coder", "opus")
	w = &worker.Worker{Tier: worker.TierEngineer, SkillProfile: "coder"}
	if ms.ResolveModel(w, nil) != "opus" {
		t.Error("custom profile override not applied")
	}
}
