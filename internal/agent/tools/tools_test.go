package tools

import (
	"context"
	"encoding/json"
	"testing"
)

type mockTool struct{}

func (m *mockTool) Name() string               { return "mock" }
func (m *mockTool) Description() string         { return "a mock tool" }
func (m *mockTool) Parameters() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (m *mockTool) Execute(_ context.Context, _ json.RawMessage) (string, error) {
	return "ok", nil
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockTool{})
	tool, ok := r.Get("mock")
	if !ok {
		t.Fatal("should find registered tool")
	}
	if tool.Name() != "mock" {
		t.Errorf("expected mock, got %s", tool.Name())
	}
}

func TestRegistry_GetUnknown(t *testing.T) {
	r := NewRegistry()
	_, ok := r.Get("nonexistent")
	if ok {
		t.Error("should not find unregistered tool")
	}
}

func TestRegistry_List(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockTool{})
	if len(r.List()) != 1 {
		t.Errorf("expected 1 tool, got %d", len(r.List()))
	}
}

func TestRegistry_AllowedFilter(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockTool{})
	r.SetAllowed([]string{"other"})
	_, ok := r.Get("mock")
	if ok {
		t.Error("mock should be filtered out")
	}
	if len(r.List()) != 0 {
		t.Error("list should be empty")
	}
}

func TestRegistry_SetAllowedEmpty(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockTool{})
	r.SetAllowed([]string{"other"})
	// Reset allowed to nil by passing empty slice
	r.SetAllowed(nil)
	tool, ok := r.Get("mock")
	if !ok {
		t.Fatal("should find tool after clearing allowed list")
	}
	if tool.Name() != "mock" {
		t.Errorf("expected mock, got %s", tool.Name())
	}
}
