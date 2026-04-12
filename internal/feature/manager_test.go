package feature

import "testing"

func TestNewManager_DefaultFlags(t *testing.T) {
	fm := NewManager()
	if !fm.IsEnabled("growth_engine") {
		t.Error("growth_engine should be enabled by default")
	}
	if fm.IsEnabled("pixel_evolution") {
		t.Error("pixel_evolution should be disabled by default")
	}
}

func TestManager_Toggle(t *testing.T) {
	fm := NewManager()
	fm.Toggle("pixel_evolution", true)
	if !fm.IsEnabled("pixel_evolution") {
		t.Error("pixel_evolution should be enabled after toggle")
	}
	fm.Toggle("growth_engine", false)
	if fm.IsEnabled("growth_engine") {
		t.Error("growth_engine should be disabled after toggle")
	}
}

func TestManager_UnknownFlag(t *testing.T) {
	fm := NewManager()
	if fm.IsEnabled("nonexistent") {
		t.Error("unknown flag should return false")
	}
}
