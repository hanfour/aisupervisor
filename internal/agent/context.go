package agent

// ContextManager maintains conversation history and handles token budget management.
type ContextManager struct {
	systemPrompt     string
	messages         []Message
	maxContextTokens int
}

// NewContextManager creates a new ContextManager with the given system prompt and token limit.
// If maxTokens is <= 0, it defaults to 200000.
func NewContextManager(systemPrompt string, maxTokens int) *ContextManager {
	if maxTokens <= 0 {
		maxTokens = 200000
	}
	return &ContextManager{
		systemPrompt:     systemPrompt,
		maxContextTokens: maxTokens,
	}
}

// SystemPrompt returns the system prompt.
func (cm *ContextManager) SystemPrompt() string { return cm.systemPrompt }

// AddUser appends a user message to the conversation.
func (cm *ContextManager) AddUser(content string) {
	cm.messages = append(cm.messages, Message{Role: "user", Content: content})
}

// AddAssistant appends an assistant message to the conversation.
func (cm *ContextManager) AddAssistant(msg Message) {
	cm.messages = append(cm.messages, msg)
}

// AddToolResult appends a tool result message to the conversation.
func (cm *ContextManager) AddToolResult(result ToolResult) {
	cm.messages = append(cm.messages, Message{Role: "tool", ToolResult: &result})
}

// Messages returns the current conversation messages.
func (cm *ContextManager) Messages() []Message { return cm.messages }

// Reset clears all messages but preserves the system prompt.
func (cm *ContextManager) Reset() { cm.messages = nil }

// EstimateTokens returns a rough token count estimate using the ~4 chars/token heuristic.
func (cm *ContextManager) EstimateTokens() int {
	total := len(cm.systemPrompt) / 4
	for _, m := range cm.messages {
		total += len(m.Content) / 4
		for _, tc := range m.ToolCalls {
			total += len(tc.Arguments) / 4
		}
		if m.ToolResult != nil {
			total += len(m.ToolResult.Content) / 4
		}
	}
	return total
}

// Truncate removes the oldest messages when the estimated token count exceeds 80% of the limit.
// It always keeps at least the last 10 messages.
func (cm *ContextManager) Truncate() {
	threshold := int(float64(cm.maxContextTokens) * 0.8)
	keep := 10
	for cm.EstimateTokens() > threshold && len(cm.messages) > keep {
		cm.messages = cm.messages[1:]
	}
}
