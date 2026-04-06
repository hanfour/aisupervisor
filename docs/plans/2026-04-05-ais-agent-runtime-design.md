# ais-agent: Multi-Provider AI Coding Agent Runtime — Design Doc

> **Status:** Approved
> **Date:** 2026-04-05
> **Context:** aisupervisor needs a unified coding agent runtime that supports multiple LLM providers (Anthropic, OpenAI, Ollama) as an alternative to Claude Code CLI and aider.

## Goal

Build `ais-agent`, a standalone Go CLI binary that runs as an interactive coding agent in tmux panes. It plugs into aisupervisor's existing `CLITool` mechanism with zero changes to the monitor/tmux architecture.

## Motivation

- **Cost**: Level 1-2 workers use expensive Claude API; cheaper models (Ollama local, GPT-4o-mini) suffice for simple tasks
- **Unified runtime**: Currently maintaining two CLI tool integrations (claude + aider) with divergent idle detection, ready checks, and behavior
- **Control**: Full ownership of the agent loop, tool execution, permission model, and context management
- **Growth integration**: Level-based CLITool switching — low-level workers use ais-agent with cheap models, senior workers use Claude Code CLI

## Architecture

### Standalone Binary

```
aisupervisor (supervisor process)
  └─ tmux pane
       └─ ais-agent --provider openai --model gpt-4o --permission-mode acceptEdits
            ├─ REPL prompt: ❯
            ├─ Receives prompt via tmux SendKeys
            ├─ Agentic loop (LLM → tool → LLM → ...)
            └─ Returns to ❯ when done
```

Why standalone binary over embedded:
- Zero disruption to existing tmux/spawner/monitor architecture
- `CLITool = "ais-agent"` — same pattern as claude/aider
- Worker crash doesn't affect supervisor
- Can be tested and used independently
- Incremental rollout: replace aider first, then optionally claude

### Module Layout

```
cmd/ais-agent/
  main.go                         # CLI entry, flag parsing, REPL shell

internal/agent/
  loop.go                         # Core agentic loop
  provider.go                     # Provider interface + registry
  context.go                      # Conversation history + context window management
  permission.go                   # Permission mode (plan/acceptEdits/bypass)
  tools/
    registry.go                   # Tool interface + registry
    read.go, edit.go, write.go    # File tools
    bash.go                       # Shell execution
    glob.go, grep.go              # Search tools
  providers/
    openai.go                     # OpenAI Chat Completions API
    anthropic.go                  # Anthropic Messages API
    ollama.go                     # Ollama local API
```

## LLM Provider Abstraction

### Interface

```go
type Provider interface {
    Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
    Name() string
    MaxContextTokens() int
}

type ChatRequest struct {
    Model        string
    Messages     []Message
    Tools        []ToolDefinition
    SystemPrompt string
    MaxTokens    int
    Temperature  float64
    OnChunk      func(text string)  // streaming callback
}

type ChatResponse struct {
    Message      Message
    StopReason   string   // "end_turn", "tool_use", "max_tokens"
    InputTokens  int
    OutputTokens int
}
```

### Provider Differences

| | Anthropic | OpenAI | Ollama |
|---|---|---|---|
| API format | Messages API (`/v1/messages`) | Chat Completions (`/v1/chat/completions`) | Ollama API (`/api/chat`) |
| Tool use | Native `tool_use` content block | `function_calling` | Function calling (supported models) or prompt-based fallback |
| Streaming | SSE `content_block_delta` | SSE `data: {...}` | NDJSON |
| Auth | `x-api-key` header | `Authorization: Bearer` | None (local) |
| Base URL override | Yes (for proxies) | Yes (Azure, OpenRouter) | Yes (custom host) |

### Ollama Prompt-Based Fallback

For local models without function calling support, tool definitions are injected into the system prompt and the model responds with `<tool_call>{"name":"...", "arguments":{...}}</tool_call>` format, parsed by the agent.

## Tool System

### Six Core Tools

| Tool | Parameters | Behavior |
|------|-----------|----------|
| Read | file_path, offset?, limit? | Read file with line numbers (cat -n format) |
| Edit | file_path, old_string, new_string, replace_all? | Exact string replacement; old_string must be unique |
| Write | file_path, content | Create or overwrite file |
| Bash | command, timeout? | Execute shell command, 120s default timeout |
| Glob | pattern, path? | File pattern matching, return paths |
| Grep | pattern, path?, glob?, output_mode? | Content search using rg or grep fallback |

### Permission Modes

```go
type PermissionMode int
const (
    ModePlan        // Read, Glob, Grep only (Level 1)
    ModeAcceptEdits // + Edit, Write (Level 2-3); Bash auto-approved
    ModeBypass      // All tools auto-approved (Level 4-5)
)
```

In aisupervisor context, no human is at the terminal. All permission prompts auto-approve with logging. The mode restricts which tools are available, not whether to prompt.

## Agentic Loop

```go
func (l *Loop) Run(ctx context.Context, userPrompt string) error {
    l.history.AddUser(userPrompt)

    for iteration := 0; iteration < l.maxIterations; iteration++ {
        resp, err := l.provider.Chat(ctx, l.buildRequest())
        l.history.AddAssistant(resp.Message)
        l.tokenUsage.Add(resp.InputTokens, resp.OutputTokens)

        switch resp.StopReason {
        case "end_turn":
            l.print(resp.Message.Content)
            return nil  // Back to ❯ prompt
        case "tool_use":
            for _, call := range resp.Message.ToolCalls {
                result := l.executeTool(ctx, call)
                l.history.AddToolResult(result)
            }
            continue
        case "max_tokens":
            l.history.Compact()
            continue
        }
    }
    return ErrMaxIterations
}
```

- Max iterations: 200 (matches Claude Code)
- `/stop` command: graceful exit
- `Ctrl+C`: interrupt current iteration, stay in REPL

### Context Window Management

- Token counting: tiktoken (OpenAI) or char/4 estimate (Anthropic/Ollama)
- Compaction trigger: history exceeds 80% of MaxContextTokens
- Strategy: summarize oldest turns via LLM, keep last 5 turns full
- Tool result truncation: cap at 10,000 chars per result

## CLI Interface

```bash
ais-agent \
  --provider anthropic \
  --model claude-sonnet-4-20250514 \
  --api-key $ANTHROPIC_API_KEY \
  --base-url https://api.anthropic.com \
  --permission-mode acceptEdits \
  --allowed-tools Read,Edit,Write,Bash,Glob,Grep \
  --append-system-prompt "You are a junior developer..." \
  --max-tokens 200000 \
  --max-iterations 200
```

REPL output:
```
ais-agent v0.1.0 (openai/gpt-4o)
❯ [awaiting prompt]
  ... LLM + tool execution ...
❯ [idle]
```

`❯` is the idle marker. aisupervisor ReadyCheck regex: `^❯\s*$`

## Integration with aisupervisor

### Changes to Existing Code

| File | Change |
|------|--------|
| `internal/config/config.go` | Add `"ais-agent"` to `validCLITools` |
| `internal/growth/config_mapper.go` | Add `CLITool`, `Provider` fields to `LevelConfig` |
| `internal/worker/spawner.go` | Update `buildGrowthSkillArgs` to emit ais-agent flags per level |

### No Changes Needed

Monitor, tmux client, session manager, completion detection — all handled via existing `ReadyCheck` regex mechanism.

### Growth System Level Mapping

| Level | CLITool | Provider | Model |
|-------|---------|----------|-------|
| 1 | ais-agent | ollama | llama3 |
| 2 | ais-agent | openai | gpt-4o-mini |
| 3 | ais-agent | openai | gpt-4o |
| 4 | claude | — | claude-sonnet-4-20250514 |
| 5 | claude | — | claude-opus-4-20250514 |

### Config Example

```yaml
worker_tiers:
  - tier: engineer
    cli_tool: "ais-agent"
    cli_args: "--provider ollama --model llama3"
    ready_check: "^❯\\s*$"
  - tier: manager
    cli_tool: "claude"
```

## Implementation Phases

### Phase 1 — MVP
- REPL shell + flag parsing
- Agentic loop
- OpenAI provider (most universal format; Ollama also compatible)
- All 6 tools
- Basic history management (truncation, no compaction)
- **Verify**: run in terminal, complete a coding task, return to ❯

### Phase 2 — Multi-Provider
- Anthropic provider (native tool use)
- Ollama provider + prompt-based tool fallback
- Permission modes
- Context compaction strategy
- **Verify**: all 3 providers complete a simple coding task

### Phase 3 — aisupervisor Integration
- config.go: add ais-agent to valid tools
- config_mapper.go: add CLITool/Provider fields
- spawner.go: update buildGrowthSkillArgs
- **Verify**: aisupervisor spawns ais-agent worker, detects completion

### Phase 4 — Advanced
- Streaming output
- Token budget tracking + cost estimation
- OpenRouter / Azure OpenAI (OpenAI-compatible, base_url override)
- Conversation export for debugging

## Size Estimate

~1,700 lines Go code + tests across ~15 files.
