package detector

type PromptType string

const (
	PromptTypeClaudeCode PromptType = "claude_code"
	PromptTypeGemini     PromptType = "gemini"
	PromptTypeGeneric    PromptType = "generic"
)

type ResponseOption struct {
	Key   string // the key to send (e.g. "1", "Y")
	Label string // human-readable label (e.g. "Yes", "No")
}

type PromptMatch struct {
	Type        PromptType
	Summary     string
	Options     []ResponseOption
	FullContext  string
}

type Detector interface {
	Name() string
	Detect(paneContent string) (*PromptMatch, bool)
	Priority() int
}
