package detector

import "testing"

func TestGenericDetector_Detect(t *testing.T) {
	d := NewGenericDetector()

	tests := []struct {
		name      string
		input     string
		wantMatch bool
	}{
		{
			name:      "bracket y/n",
			input:     "Are you sure? [y/n] ",
			wantMatch: true,
		},
		{
			name:      "paren y/n",
			input:     "Continue? (y/n) ",
			wantMatch: true,
		},
		{
			name:      "yes/no",
			input:     "Do you want to continue? yes/no",
			wantMatch: true,
		},
		{
			name:      "confirm question",
			input:     "Please confirm? ",
			wantMatch: true,
		},
		{
			name:      "no prompt",
			input:     "Building project...\nDone.",
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := d.Detect(tt.input)
			if ok != tt.wantMatch {
				t.Errorf("Detect() = %v, want %v", ok, tt.wantMatch)
			}
		})
	}
}

func TestRegistry_PriorityOrder(t *testing.T) {
	r := DefaultRegistry()

	// Claude-specific prompt should match claude detector first
	input := `
Do you want to proceed?
  ❯ 1. Yes
    2. No
`
	match, ok := r.Detect(input)
	if !ok {
		t.Fatal("expected match")
	}
	if match.Type != PromptTypeClaudeCode {
		t.Errorf("Type = %v, want %v (claude should match first)", match.Type, PromptTypeClaudeCode)
	}
}
