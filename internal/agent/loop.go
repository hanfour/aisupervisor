package agent

import (
	"context"
	"fmt"

	"github.com/hanfourmini/aisupervisor/internal/agent/tools"
)

// Loop implements the agentic loop: prompt → LLM → tool execution → repeat.
type Loop struct {
	provider      Provider
	registry      *tools.Registry
	context       *ContextManager
	MaxIterations int
	OnOutput      func(text string)
	OnToolCall    func(name, args string)
	TotalInput    int
	TotalOutput   int
}

// NewLoop creates a new agentic loop with the given provider, tool registry, system prompt, and token budget.
func NewLoop(provider Provider, registry *tools.Registry, systemPrompt string, maxTokens int) *Loop {
	return &Loop{
		provider:      provider,
		registry:      registry,
		context:       NewContextManager(systemPrompt, maxTokens),
		MaxIterations: 200,
	}
}

// Run executes the agentic loop for a single user prompt.
// It adds the prompt to context, calls the LLM, and processes tool calls until
// the model produces a final response or the iteration limit is reached.
func (l *Loop) Run(ctx context.Context, userPrompt string) error {
	l.context.AddUser(userPrompt)

	for iteration := 0; iteration < l.MaxIterations; iteration++ {
		req := ChatRequest{
			SystemPrompt: l.context.SystemPrompt(),
			Messages:     l.context.Messages(),
			Tools:        l.buildToolDefs(),
		}

		resp, err := l.provider.Chat(ctx, req)
		if err != nil {
			return fmt.Errorf("LLM call failed: %w", err)
		}

		l.TotalInput += resp.InputTokens
		l.TotalOutput += resp.OutputTokens
		l.context.AddAssistant(resp.Message)

		switch resp.StopReason {
		case "end_turn":
			if l.OnOutput != nil && resp.Message.Content != "" {
				l.OnOutput(resp.Message.Content)
			}
			return nil

		case "tool_use":
			if len(resp.Message.ToolCalls) == 0 {
				// Provider returned tool_use but no actual tool calls — treat as end_turn
				return nil
			}
			for _, call := range resp.Message.ToolCalls {
				result := l.executeTool(ctx, call)
				l.context.AddToolResult(result)
			}
			continue

		case "max_tokens":
			prevLen := len(l.context.Messages())
			l.context.Truncate()
			if len(l.context.Messages()) == prevLen {
				return fmt.Errorf("context too large to truncate further")
			}
			continue

		default:
			// Unknown stop reason — treat as end_turn.
			if l.OnOutput != nil && resp.Message.Content != "" {
				l.OnOutput(resp.Message.Content)
			}
			return nil
		}
	}

	return fmt.Errorf("max iterations (%d) reached", l.MaxIterations)
}

// executeTool runs a single tool call and returns the result.
func (l *Loop) executeTool(ctx context.Context, call ToolCall) ToolResult {
	if l.OnToolCall != nil {
		l.OnToolCall(call.Name, string(call.Arguments))
	}

	tool, ok := l.registry.Get(call.Name)
	if !ok {
		return ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Error: unknown tool %q", call.Name),
			IsError: true,
		}
	}

	output, err := tool.Execute(ctx, call.Arguments)
	if err != nil {
		return ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Error: %v", err),
			IsError: true,
		}
	}

	const maxOutput = 10000
	if len(output) > maxOutput {
		output = output[:maxOutput] + "\n[truncated, showing first 10000 chars]"
	}

	return ToolResult{CallID: call.ID, Content: output}
}

// buildToolDefs converts registered tools to ToolDefinition for the LLM request.
func (l *Loop) buildToolDefs() []ToolDefinition {
	registered := l.registry.List()
	defs := make([]ToolDefinition, len(registered))
	for i, t := range registered {
		defs[i] = ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Parameters(),
		}
	}
	return defs
}

// ResetHistory clears the conversation history but preserves the system prompt.
func (l *Loop) ResetHistory() {
	l.context.Reset()
}
