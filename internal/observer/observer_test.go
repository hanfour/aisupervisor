package observer

import (
	"testing"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/detector"
)

func TestPromptObserver(t *testing.T) {
	registry := detector.DefaultRegistry()
	obs := NewPromptObserver(registry)

	content := `
Do you want to proceed?
  ❯ 1. Yes
    2. No
`
	events := obs.Observe(content, "")
	if len(events) == 0 {
		t.Fatal("expected prompt observation event")
	}
	if events[0].Type != ObservationPrompt {
		t.Errorf("expected prompt type, got %s", events[0].Type)
	}
	if events[0].Prompt == nil {
		t.Error("expected prompt match")
	}
}

func TestPromptObserver_NoPrompt(t *testing.T) {
	registry := detector.DefaultRegistry()
	obs := NewPromptObserver(registry)

	events := obs.Observe("just normal output\nnothing special", "")
	if len(events) != 0 {
		t.Errorf("expected no events, got %d", len(events))
	}
}

func TestContentObserver_ErrorDetection(t *testing.T) {
	obs := NewContentObserver(30 * time.Second)

	tests := []struct {
		content string
		hasErr  bool
	}{
		{"error: file not found", true},
		{"FAIL: TestSomething", true},
		{"panic: runtime error", true},
		{"Traceback (most recent call last):", true},
		{"all tests passed", false},
		{"normal output", false},
	}

	for _, tt := range tests {
		events := obs.Observe(tt.content, "prev")
		hasError := false
		for _, e := range events {
			if e.Type == ObservationError {
				hasError = true
			}
		}
		if hasError != tt.hasErr {
			t.Errorf("content=%q: expected error=%v, got %v", tt.content, tt.hasErr, hasError)
		}
	}
}

func TestContentObserver_InputReady(t *testing.T) {
	obs := NewContentObserver(30 * time.Second)

	tests := []struct {
		content  string
		expected bool
	}{
		{"user@host:~$ ", true},
		{"❯ ", true},
		{"> ", true},
		{"running some command...", false},
	}

	for _, tt := range tests {
		events := obs.Observe(tt.content, "prev")
		hasInputReady := false
		for _, e := range events {
			if e.Type == ObservationInputReady {
				hasInputReady = true
			}
		}
		if hasInputReady != tt.expected {
			t.Errorf("content=%q: expected input_ready=%v, got %v", tt.content, tt.expected, hasInputReady)
		}
	}
}

func TestContentObserver_IdleDetection(t *testing.T) {
	obs := NewContentObserver(50 * time.Millisecond)

	// First observation
	events := obs.Observe("content", "")
	for _, e := range events {
		if e.Type == ObservationIdle {
			t.Error("should not be idle on first observation")
		}
	}

	// Wait past threshold
	time.Sleep(60 * time.Millisecond)

	// Same content → should detect idle
	events = obs.Observe("content", "content")
	hasIdle := false
	for _, e := range events {
		if e.Type == ObservationIdle {
			hasIdle = true
		}
	}
	if !hasIdle {
		t.Error("expected idle event after threshold")
	}

	// Should not re-notify
	events = obs.Observe("content", "content")
	for _, e := range events {
		if e.Type == ObservationIdle {
			t.Error("should not re-notify idle")
		}
	}
}
