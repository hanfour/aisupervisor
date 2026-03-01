package detector

import (
	"testing"
)

func TestClaudeDetector_Detect(t *testing.T) {
	d := NewClaudeDetector()

	tests := []struct {
		name        string
		input       string
		wantMatch   bool
		wantType    PromptType
		wantOptions int
	}{
		{
			name: "standard proceed prompt with numbered options",
			input: `
╭──────────────────────────────────╮
│  Bash command                    │
│  rm -rf /tmp/test                │
╰──────────────────────────────────╯
  Do you want to proceed?
  ❯ 1. Yes
    2. Yes, and don't ask again
    3. No, and tell Claude
`,
			wantMatch:   true,
			wantType:    PromptTypeClaudeCode,
			wantOptions: 3,
		},
		{
			name: "proceed prompt without box",
			input: `
Reading file: src/main.go
Do you want to proceed?
  ❯ 1. Yes
    2. No
`,
			wantMatch:   true,
			wantType:    PromptTypeClaudeCode,
			wantOptions: 2,
		},
		{
			name: "allow/deny prompt",
			input: `
Claude wants to run: ls -la
Allow once  Deny
`,
			wantMatch:   true,
			wantType:    PromptTypeClaudeCode,
			wantOptions: 2,
		},
		{
			name: "no prompt - regular output",
			input: `
Building project...
$ go build ./...
Done.
`,
			wantMatch: false,
		},
		{
			name: "empty content",
			input:     "",
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match, ok := d.Detect(tt.input)
			if ok != tt.wantMatch {
				t.Errorf("Detect() match = %v, want %v", ok, tt.wantMatch)
				return
			}
			if !ok {
				return
			}
			if match.Type != tt.wantType {
				t.Errorf("Type = %v, want %v", match.Type, tt.wantType)
			}
			if len(match.Options) != tt.wantOptions {
				t.Errorf("Options count = %d, want %d", len(match.Options), tt.wantOptions)
			}
		})
	}
}

func TestClaudeDetector_ExtractSummary(t *testing.T) {
	d := NewClaudeDetector()

	input := `
╭──────────────────────────────────╮
│  Bash command                    │
│  npm install express             │
╰──────────────────────────────────╯
  Do you want to proceed?
  ❯ 1. Yes
`
	match, ok := d.Detect(input)
	if !ok {
		t.Fatal("expected match")
	}
	if match.Summary == "" {
		t.Error("expected non-empty summary")
	}
}
