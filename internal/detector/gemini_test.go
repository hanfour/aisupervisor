package detector

import (
	"testing"
)

func TestGeminiDetector_Detect(t *testing.T) {
	d := NewGeminiDetector()

	tests := []struct {
		name        string
		input       string
		wantMatch   bool
		wantOptions int
	}{
		{
			name: "standard shell command prompt",
			input: `
Run shell command?
  $ ls -la /tmp
(Y)es / (N)o / (M)odify
`,
			wantMatch:   true,
			wantOptions: 3,
		},
		{
			name: "yes/no only",
			input: `
Execute this action?
(Y)es / (N)o
`,
			wantMatch:   true,
			wantOptions: 2,
		},
		{
			name: "no prompt",
			input: `
Thinking about what to do next...
Here is my plan:
1. Read the file
2. Make changes
`,
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match, ok := d.Detect(tt.input)
			if ok != tt.wantMatch {
				t.Errorf("Detect() = %v, want %v", ok, tt.wantMatch)
				return
			}
			if !ok {
				return
			}
			if match.Type != PromptTypeGemini {
				t.Errorf("Type = %v, want %v", match.Type, PromptTypeGemini)
			}
			if len(match.Options) != tt.wantOptions {
				t.Errorf("Options = %d, want %d", len(match.Options), tt.wantOptions)
			}
		})
	}
}

func TestGeminiDetector_ExtractCommand(t *testing.T) {
	d := NewGeminiDetector()

	input := `
Run shell command?
  $ git status
(Y)es / (N)o
`
	match, ok := d.Detect(input)
	if !ok {
		t.Fatal("expected match")
	}
	if match.Summary != "Run: git status" {
		t.Errorf("Summary = %q, want %q", match.Summary, "Run: git status")
	}
}
