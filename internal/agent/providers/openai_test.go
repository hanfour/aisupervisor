package providers

import (
	"testing"
)

func TestNewOpenAI_Defaults(t *testing.T) {
	p := NewOpenAI("test-key", "", "")
	if p.Name() != "openai" {
		t.Errorf("Name: got %q, want %q", p.Name(), "openai")
	}
	if p.model != "gpt-4o" {
		t.Errorf("default model: got %q, want %q", p.model, "gpt-4o")
	}
}

func TestNewOpenAI_CustomModel(t *testing.T) {
	p := NewOpenAI("test-key", "gpt-3.5-turbo", "")
	if p.model != "gpt-3.5-turbo" {
		t.Errorf("model: got %q, want %q", p.model, "gpt-3.5-turbo")
	}
}

func TestModelContextSize(t *testing.T) {
	tests := []struct {
		model string
		want  int
	}{
		{"gpt-4o", 128000},
		{"gpt-4o-mini", 128000},
		{"gpt-4o-2024-08-06", 128000},
		{"gpt-4-turbo", 128000},
		{"gpt-4-1", 1047576},
		{"gpt-4.1", 1047576},
		{"gpt-4.1-mini", 1047576},
		{"gpt-3.5-turbo", 16385},
		{"gpt-3.5-turbo-16k", 16385},
		{"o1", 200000},
		{"o1-mini", 128000},
		{"o3", 200000},
		{"o3-mini", 200000},
		{"o4-mini", 200000},
		{"unknown-model", 128000},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			p := NewOpenAI("key", tt.model, "")
			got := p.MaxContextTokens()
			if got != tt.want {
				t.Errorf("MaxContextTokens(%q): got %d, want %d", tt.model, got, tt.want)
			}
		})
	}
}
