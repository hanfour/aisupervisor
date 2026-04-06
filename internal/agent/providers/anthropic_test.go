package providers

import (
	"testing"
)

func TestNewAnthropic_Defaults(t *testing.T) {
	p := NewAnthropic("test-key", "", "")
	if p.Name() != "anthropic" {
		t.Errorf("Name: got %q, want %q", p.Name(), "anthropic")
	}
	if p.model != "claude-sonnet-4-20250514" {
		t.Errorf("default model: got %q, want %q", p.model, "claude-sonnet-4-20250514")
	}
	if got := p.MaxContextTokens(); got != 200000 {
		t.Errorf("MaxContextTokens: got %d, want %d", got, 200000)
	}
}

func TestNewAnthropic_CustomModel(t *testing.T) {
	p := NewAnthropic("test-key", "claude-opus-4-20250514", "")
	if p.model != "claude-opus-4-20250514" {
		t.Errorf("model: got %q, want %q", p.model, "claude-opus-4-20250514")
	}
}

func TestNewAnthropic_CustomBaseURL(t *testing.T) {
	p := NewAnthropic("test-key", "", "https://custom.proxy.example.com")
	if p.Name() != "anthropic" {
		t.Errorf("Name: got %q, want %q", p.Name(), "anthropic")
	}
}

func TestAnthropicModelContextSize(t *testing.T) {
	tests := []struct {
		model string
		want  int
	}{
		{"claude-opus-4-20250514", 200000},
		{"claude-sonnet-4-20250514", 200000},
		{"claude-haiku-4-5-20251001", 200000},
		{"claude-3-5-sonnet-20241022", 200000},
		{"claude-3-opus-20240229", 200000},
		{"claude-3-haiku-20240307", 200000},
		{"unknown-claude-model", 200000},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			p := NewAnthropic("key", tt.model, "")
			got := p.MaxContextTokens()
			if got != tt.want {
				t.Errorf("MaxContextTokens(%q): got %d, want %d", tt.model, got, tt.want)
			}
		})
	}
}
