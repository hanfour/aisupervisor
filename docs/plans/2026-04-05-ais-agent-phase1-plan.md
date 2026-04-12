# ais-agent Phase 1 (MVP) — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a standalone `ais-agent` CLI binary that can receive a prompt, call an LLM with tool use, execute tools (Read/Edit/Write/Bash/Glob/Grep), loop until done, and display an idle `❯` prompt.

**Architecture:** Standalone Go binary in `cmd/ais-agent/`. Core agent logic in `internal/agent/`. OpenAI provider first (format also works with Ollama). Six file/shell tools. REPL shell with `❯` idle marker for aisupervisor monitor compatibility.

**Tech Stack:** Go 1.23+, `github.com/openai/openai-go` (already in go.mod), `os/exec` for Bash tool, `path/filepath` for Glob, `os/exec` + `rg` for Grep.

**Design Doc:** `docs/plans/2026-04-05-ais-agent-runtime-design.md`

---

## Phase 1: Tool System

### Task 1.1: Tool Interface & Registry

**Files:**
- Create: `internal/agent/tools/tools.go`
- Test: `internal/agent/tools/tools_test.go`

**Step 1: Write failing test**

```go
// internal/agent/tools/tools_test.go
package tools

import (
	"context"
	"encoding/json"
	"testing"
)

type mockTool struct{}

func (m *mockTool) Name() string                  { return "mock" }
func (m *mockTool) Description() string            { return "a mock tool" }
func (m *mockTool) Parameters() json.RawMessage    { return json.RawMessage(`{"type":"object"}`) }
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
	list := r.List()
	if len(list) != 1 {
		t.Errorf("expected 1 tool, got %d", len(list))
	}
}

func TestRegistry_AllowedFilter(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockTool{})
	r.SetAllowed([]string{"other"})
	_, ok := r.Get("mock")
	if ok {
		t.Error("mock should be filtered out by allowed list")
	}
	list := r.List()
	if len(list) != 0 {
		t.Error("list should be empty when tool not in allowed set")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/agent/tools/ -v`
Expected: FAIL — package not found

**Step 3: Write minimal implementation**

```go
// internal/agent/tools/tools.go
package tools

import (
	"context"
	"encoding/json"
)

// Tool is the interface all agent tools must implement.
type Tool interface {
	Name() string
	Description() string
	Parameters() json.RawMessage
	Execute(ctx context.Context, args json.RawMessage) (string, error)
}

// Registry manages available tools with optional allow/disallow filtering.
type Registry struct {
	tools   map[string]Tool
	allowed map[string]bool // nil = all allowed
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

func (r *Registry) Register(t Tool) {
	r.tools[t.Name()] = t
}

func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	if !ok {
		return nil, false
	}
	if r.allowed != nil && !r.allowed[name] {
		return nil, false
	}
	return t, true
}

func (r *Registry) List() []Tool {
	var result []Tool
	for _, t := range r.tools {
		if r.allowed != nil && !r.allowed[t.Name()] {
			continue
		}
		result = append(result, t)
	}
	return result
}

func (r *Registry) SetAllowed(names []string) {
	if len(names) == 0 {
		r.allowed = nil
		return
	}
	r.allowed = make(map[string]bool, len(names))
	for _, n := range names {
		r.allowed[n] = true
	}
}
```

**Step 4: Run test to verify it passes**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/agent/tools/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/agent/tools/
git commit -m "feat(ais-agent): add tool interface and registry with allow-list filtering"
```

---

### Task 1.2: Read Tool

**Files:**
- Create: `internal/agent/tools/read.go`
- Test: `internal/agent/tools/read_test.go`

**Step 1: Write failing test**

```go
// internal/agent/tools/read_test.go
package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadTool_BasicRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("line1\nline2\nline3\n"), 0o644)

	tool := &ReadTool{}
	args, _ := json.Marshal(map[string]interface{}{"file_path": path})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "1\tline1") {
		t.Errorf("should contain line numbers, got: %s", result)
	}
	if !strings.Contains(result, "3\tline3") {
		t.Errorf("should contain line 3, got: %s", result)
	}
}

func TestReadTool_WithOffset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("a\nb\nc\nd\ne\n"), 0o644)

	tool := &ReadTool{}
	args, _ := json.Marshal(map[string]interface{}{"file_path": path, "offset": 3, "limit": 2})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "3\tc") {
		t.Errorf("should start at line 3, got: %s", result)
	}
	if strings.Contains(result, "5\te") {
		t.Error("should not contain line 5 with limit 2")
	}
}

func TestReadTool_FileNotFound(t *testing.T) {
	tool := &ReadTool{}
	args, _ := json.Marshal(map[string]interface{}{"file_path": "/nonexistent/file.txt"})
	_, err := tool.Execute(context.Background(), args)
	if err == nil {
		t.Error("should return error for missing file")
	}
}
```

**Step 2: Run test — FAIL**

**Step 3: Write implementation**

```go
// internal/agent/tools/read.go
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type ReadTool struct{}

type readArgs struct {
	FilePath string `json:"file_path"`
	Offset   int    `json:"offset,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}

func (r *ReadTool) Name() string        { return "Read" }
func (r *ReadTool) Description() string  { return "Read a file and return its contents with line numbers." }
func (r *ReadTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"file_path": {"type": "string", "description": "Absolute path to the file"},
			"offset": {"type": "integer", "description": "Line number to start from (1-based)"},
			"limit": {"type": "integer", "description": "Maximum number of lines to read"}
		},
		"required": ["file_path"]
	}`)
}

func (r *ReadTool) Execute(_ context.Context, rawArgs json.RawMessage) (string, error) {
	var args readArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	data, err := os.ReadFile(args.FilePath)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", args.FilePath, err)
	}

	lines := strings.Split(string(data), "\n")
	// Remove trailing empty line from split
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	start := 0
	if args.Offset > 0 {
		start = args.Offset - 1 // convert 1-based to 0-based
	}
	if start > len(lines) {
		start = len(lines)
	}

	end := len(lines)
	if args.Limit > 0 && start+args.Limit < end {
		end = start + args.Limit
	}

	var sb strings.Builder
	for i := start; i < end; i++ {
		fmt.Fprintf(&sb, "%d\t%s\n", i+1, lines[i])
	}
	return sb.String(), nil
}
```

**Step 4: Run test — PASS**

**Step 5: Commit**

```bash
git add internal/agent/tools/read.go internal/agent/tools/read_test.go
git commit -m "feat(ais-agent): add Read tool with line numbers, offset, limit"
```

---

### Task 1.3: Edit Tool

**Files:**
- Create: `internal/agent/tools/edit.go`
- Test: `internal/agent/tools/edit_test.go`

**Step 1: Write failing test**

```go
// internal/agent/tools/edit_test.go
package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestEditTool_BasicReplace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")
	os.WriteFile(path, []byte("func hello() {\n\treturn \"hello\"\n}\n"), 0o644)

	tool := &EditTool{}
	args, _ := json.Marshal(map[string]interface{}{
		"file_path":  path,
		"old_string": "return \"hello\"",
		"new_string": "return \"world\"",
	})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("should return confirmation")
	}

	data, _ := os.ReadFile(path)
	if string(data) != "func hello() {\n\treturn \"world\"\n}\n" {
		t.Errorf("file not updated correctly: %s", string(data))
	}
}

func TestEditTool_NotUnique(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("aaa\naaa\n"), 0o644)

	tool := &EditTool{}
	args, _ := json.Marshal(map[string]interface{}{
		"file_path":  path,
		"old_string": "aaa",
		"new_string": "bbb",
	})
	_, err := tool.Execute(context.Background(), args)
	if err == nil {
		t.Error("should fail when old_string is not unique")
	}
}

func TestEditTool_ReplaceAll(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("aaa\naaa\n"), 0o644)

	tool := &EditTool{}
	args, _ := json.Marshal(map[string]interface{}{
		"file_path":   path,
		"old_string":  "aaa",
		"new_string":  "bbb",
		"replace_all": true,
	})
	_, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("replace_all should succeed: %v", err)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "bbb\nbbb\n" {
		t.Errorf("replace_all not applied: %s", string(data))
	}
}

func TestEditTool_NotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("hello world\n"), 0o644)

	tool := &EditTool{}
	args, _ := json.Marshal(map[string]interface{}{
		"file_path":  path,
		"old_string": "nonexistent",
		"new_string": "replaced",
	})
	_, err := tool.Execute(context.Background(), args)
	if err == nil {
		t.Error("should fail when old_string not found")
	}
}
```

**Step 2: Run test — FAIL**

**Step 3: Write implementation**

```go
// internal/agent/tools/edit.go
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type EditTool struct{}

type editArgs struct {
	FilePath   string `json:"file_path"`
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
	ReplaceAll bool   `json:"replace_all,omitempty"`
}

func (e *EditTool) Name() string        { return "Edit" }
func (e *EditTool) Description() string  { return "Replace exact string occurrences in a file." }
func (e *EditTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"file_path": {"type": "string", "description": "Absolute path to the file"},
			"old_string": {"type": "string", "description": "Exact string to replace (must be unique unless replace_all)"},
			"new_string": {"type": "string", "description": "Replacement string"},
			"replace_all": {"type": "boolean", "description": "Replace all occurrences (default false)"}
		},
		"required": ["file_path", "old_string", "new_string"]
	}`)
}

func (e *EditTool) Execute(_ context.Context, rawArgs json.RawMessage) (string, error) {
	var args editArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	data, err := os.ReadFile(args.FilePath)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", args.FilePath, err)
	}

	content := string(data)
	count := strings.Count(content, args.OldString)

	if count == 0 {
		return "", fmt.Errorf("old_string not found in %s", args.FilePath)
	}
	if count > 1 && !args.ReplaceAll {
		return "", fmt.Errorf("old_string found %d times in %s; use replace_all or provide more context", count, args.FilePath)
	}

	var newContent string
	if args.ReplaceAll {
		newContent = strings.ReplaceAll(content, args.OldString, args.NewString)
	} else {
		newContent = strings.Replace(content, args.OldString, args.NewString, 1)
	}

	if err := os.WriteFile(args.FilePath, []byte(newContent), 0o644); err != nil {
		return "", fmt.Errorf("writing %s: %w", args.FilePath, err)
	}

	if args.ReplaceAll {
		return fmt.Sprintf("Replaced %d occurrences in %s", count, args.FilePath), nil
	}
	return fmt.Sprintf("Replaced 1 occurrence in %s", args.FilePath), nil
}
```

**Step 4: Run test — PASS**

**Step 5: Commit**

```bash
git add internal/agent/tools/edit.go internal/agent/tools/edit_test.go
git commit -m "feat(ais-agent): add Edit tool with unique-check and replace_all"
```

---

### Task 1.4: Write, Bash, Glob, Grep Tools

**Files:**
- Create: `internal/agent/tools/write.go`
- Create: `internal/agent/tools/bash.go`
- Create: `internal/agent/tools/glob.go`
- Create: `internal/agent/tools/grep.go`
- Test: `internal/agent/tools/write_test.go`
- Test: `internal/agent/tools/bash_test.go`
- Test: `internal/agent/tools/glob_test.go`
- Test: `internal/agent/tools/grep_test.go`

**Step 1: Write failing tests**

```go
// internal/agent/tools/write_test.go
package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteTool_CreateFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")

	tool := &WriteTool{}
	args, _ := json.Marshal(map[string]interface{}{"file_path": path, "content": "hello world\n"})
	_, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "hello world\n" {
		t.Errorf("unexpected content: %s", string(data))
	}
}

func TestWriteTool_CreateWithSubdirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "dir", "file.txt")

	tool := &WriteTool{}
	args, _ := json.Marshal(map[string]interface{}{"file_path": path, "content": "nested\n"})
	_, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("should create parent dirs: %v", err)
	}
}
```

```go
// internal/agent/tools/bash_test.go
package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestBashTool_Echo(t *testing.T) {
	tool := &BashTool{}
	args, _ := json.Marshal(map[string]interface{}{"command": "echo hello"})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "hello") {
		t.Errorf("expected hello, got: %s", result)
	}
}

func TestBashTool_ExitCode(t *testing.T) {
	tool := &BashTool{}
	args, _ := json.Marshal(map[string]interface{}{"command": "exit 1"})
	result, err := tool.Execute(context.Background(), args)
	// Should return output (including stderr) but indicate error
	if err == nil && !strings.Contains(result, "exit") {
		t.Error("should indicate non-zero exit")
	}
}

func TestBashTool_Timeout(t *testing.T) {
	tool := &BashTool{}
	args, _ := json.Marshal(map[string]interface{}{"command": "sleep 10", "timeout": 1000})
	_, err := tool.Execute(context.Background(), args)
	if err == nil {
		t.Error("should timeout")
	}
}
```

```go
// internal/agent/tools/glob_test.go
package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGlobTool_MatchFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.go"), []byte(""), 0o644)
	os.WriteFile(filepath.Join(dir, "b.go"), []byte(""), 0o644)
	os.WriteFile(filepath.Join(dir, "c.txt"), []byte(""), 0o644)

	tool := &GlobTool{}
	args, _ := json.Marshal(map[string]interface{}{"pattern": "*.go", "path": dir})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "a.go") || !strings.Contains(result, "b.go") {
		t.Errorf("should find .go files: %s", result)
	}
	if strings.Contains(result, "c.txt") {
		t.Error("should not match .txt files")
	}
}
```

```go
// internal/agent/tools/grep_test.go
package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGrepTool_FindPattern(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.go"), []byte("func main() {\n\tfmt.Println(\"hello\")\n}\n"), 0o644)

	tool := &GrepTool{}
	args, _ := json.Marshal(map[string]interface{}{"pattern": "Println", "path": dir})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "test.go") {
		t.Errorf("should find match in test.go: %s", result)
	}
}
```

**Step 2: Run tests — FAIL**

**Step 3: Write implementations**

```go
// internal/agent/tools/write.go
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type WriteTool struct{}

type writeArgs struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

func (w *WriteTool) Name() string        { return "Write" }
func (w *WriteTool) Description() string  { return "Create or overwrite a file with the given content." }
func (w *WriteTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"file_path": {"type": "string", "description": "Absolute path to the file"},
			"content": {"type": "string", "description": "Content to write"}
		},
		"required": ["file_path", "content"]
	}`)
}

func (w *WriteTool) Execute(_ context.Context, rawArgs json.RawMessage) (string, error) {
	var args writeArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(args.FilePath), 0o755); err != nil {
		return "", fmt.Errorf("creating directories: %w", err)
	}
	if err := os.WriteFile(args.FilePath, []byte(args.Content), 0o644); err != nil {
		return "", fmt.Errorf("writing %s: %w", args.FilePath, err)
	}
	return fmt.Sprintf("Wrote %d bytes to %s", len(args.Content), args.FilePath), nil
}
```

```go
// internal/agent/tools/bash.go
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

type BashTool struct{}

type bashArgs struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"` // milliseconds, default 120000
}

func (b *BashTool) Name() string        { return "Bash" }
func (b *BashTool) Description() string  { return "Execute a shell command and return stdout+stderr." }
func (b *BashTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {"type": "string", "description": "Shell command to execute"},
			"timeout": {"type": "integer", "description": "Timeout in milliseconds (default 120000)"}
		},
		"required": ["command"]
	}`)
}

func (b *BashTool) Execute(ctx context.Context, rawArgs json.RawMessage) (string, error) {
	var args bashArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	timeout := 120 * time.Second
	if args.Timeout > 0 {
		timeout = time.Duration(args.Timeout) * time.Millisecond
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", args.Command)
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return string(output), fmt.Errorf("command timed out after %v", timeout)
	}
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w\nOutput: %s", err, string(output))
	}
	return string(output), nil
}
```

```go
// internal/agent/tools/glob.go
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type GlobTool struct{}

type globArgs struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path,omitempty"`
}

func (g *GlobTool) Name() string        { return "Glob" }
func (g *GlobTool) Description() string  { return "Find files matching a glob pattern." }
func (g *GlobTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {"type": "string", "description": "Glob pattern (e.g. **/*.go)"},
			"path": {"type": "string", "description": "Directory to search in (default: cwd)"}
		},
		"required": ["pattern"]
	}`)
}

func (g *GlobTool) Execute(_ context.Context, rawArgs json.RawMessage) (string, error) {
	var args globArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	dir := args.Path
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}

	pattern := filepath.Join(dir, args.Pattern)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("glob error: %w", err)
	}

	if len(matches) == 0 {
		return "No files found matching pattern.", nil
	}
	return strings.Join(matches, "\n"), nil
}
```

```go
// internal/agent/tools/grep.go
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type GrepTool struct{}

type grepArgs struct {
	Pattern    string `json:"pattern"`
	Path       string `json:"path,omitempty"`
	Glob       string `json:"glob,omitempty"`
	OutputMode string `json:"output_mode,omitempty"` // "content", "files_with_matches" (default)
}

func (g *GrepTool) Name() string        { return "Grep" }
func (g *GrepTool) Description() string  { return "Search file contents using regex patterns." }
func (g *GrepTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {"type": "string", "description": "Regex pattern to search for"},
			"path": {"type": "string", "description": "Directory or file to search (default: cwd)"},
			"glob": {"type": "string", "description": "File glob filter (e.g. *.go)"},
			"output_mode": {"type": "string", "description": "files_with_matches (default) or content"}
		},
		"required": ["pattern"]
	}`)
}

func (g *GrepTool) Execute(ctx context.Context, rawArgs json.RawMessage) (string, error) {
	var args grepArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	// Try rg first, fall back to grep
	rgPath, rgErr := exec.LookPath("rg")
	if rgErr == nil {
		return g.runRipgrep(ctx, rgPath, args)
	}
	return g.runGrep(ctx, args)
}

func (g *GrepTool) runRipgrep(ctx context.Context, rgPath string, args grepArgs) (string, error) {
	cmdArgs := []string{args.Pattern}

	if args.OutputMode == "content" {
		cmdArgs = append(cmdArgs, "-n")
	} else {
		cmdArgs = append(cmdArgs, "-l")
	}
	if args.Glob != "" {
		cmdArgs = append(cmdArgs, "--glob", args.Glob)
	}
	if args.Path != "" {
		cmdArgs = append(cmdArgs, args.Path)
	}

	cmd := exec.CommandContext(ctx, rgPath, cmdArgs...)
	output, err := cmd.CombinedOutput()
	result := strings.TrimSpace(string(output))
	if err != nil {
		if result == "" {
			return "No matches found.", nil
		}
		return result, nil
	}
	return result, nil
}

func (g *GrepTool) runGrep(ctx context.Context, args grepArgs) (string, error) {
	cmdArgs := []string{"-r", "-n", args.Pattern}
	if args.Path != "" {
		cmdArgs = append(cmdArgs, args.Path)
	} else {
		cmdArgs = append(cmdArgs, ".")
	}

	cmd := exec.CommandContext(ctx, "grep", cmdArgs...)
	output, err := cmd.CombinedOutput()
	result := strings.TrimSpace(string(output))
	if err != nil {
		if result == "" {
			return "No matches found.", nil
		}
		return result, nil
	}
	return result, nil
}
```

**Step 4: Run all tool tests**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/agent/tools/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/agent/tools/
git commit -m "feat(ais-agent): add Write, Bash, Glob, Grep tools"
```

---

## Phase 1: Provider & Core Loop

### Task 1.5: Provider Interface & Types

**Files:**
- Create: `internal/agent/provider.go`
- Test: `internal/agent/provider_test.go`

**Step 1: Write failing test**

```go
// internal/agent/provider_test.go
package agent

import (
	"encoding/json"
	"testing"
)

func TestMessage_JSON(t *testing.T) {
	msg := Message{
		Role:    "assistant",
		Content: "hello",
		ToolCalls: []ToolCall{
			{ID: "tc1", Name: "Read", Arguments: json.RawMessage(`{"file_path":"/tmp/x"}`)},
		},
	}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.Role != "assistant" {
		t.Errorf("expected assistant, got %s", decoded.Role)
	}
	if len(decoded.ToolCalls) != 1 {
		t.Errorf("expected 1 tool call, got %d", len(decoded.ToolCalls))
	}
}

func TestEXPCalculation_StopReason(t *testing.T) {
	resp := ChatResponse{StopReason: "tool_use"}
	if resp.StopReason != "tool_use" {
		t.Error("stop reason mismatch")
	}
}
```

**Step 2: Run test — FAIL**

**Step 3: Write implementation**

```go
// internal/agent/provider.go
package agent

import (
	"context"
	"encoding/json"
)

// Message represents a single message in the conversation.
type Message struct {
	Role       string       `json:"role"` // "user", "assistant", "system", "tool"
	Content    string       `json:"content,omitempty"`
	ToolCalls  []ToolCall   `json:"toolCalls,omitempty"`
	ToolResult *ToolResult  `json:"toolResult,omitempty"`
}

// ToolCall represents the LLM requesting a tool execution.
type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ToolResult represents the result of a tool execution.
type ToolResult struct {
	CallID  string `json:"callId"`
	Content string `json:"content"`
	IsError bool   `json:"isError,omitempty"`
}

// ToolDefinition describes a tool for the LLM.
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// ChatRequest holds all parameters for a chat completion call.
type ChatRequest struct {
	Model        string
	Messages     []Message
	Tools        []ToolDefinition
	SystemPrompt string
	MaxTokens    int
	Temperature  float64
	OnChunk      func(text string) // streaming callback; nil = no streaming
}

// ChatResponse holds the LLM's response.
type ChatResponse struct {
	Message      Message
	StopReason   string // "end_turn", "tool_use", "max_tokens"
	InputTokens  int
	OutputTokens int
}

// Provider is the interface for LLM backends.
type Provider interface {
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	Name() string
	MaxContextTokens() int
}
```

**Step 4: Run test — PASS**

**Step 5: Commit**

```bash
git add internal/agent/provider.go internal/agent/provider_test.go
git commit -m "feat(ais-agent): add Provider interface and message types"
```

---

### Task 1.6: Context Manager

**Files:**
- Create: `internal/agent/context.go`
- Test: `internal/agent/context_test.go`

**Step 1: Write failing test**

```go
// internal/agent/context_test.go
package agent

import "testing"

func TestContextManager_AddAndGet(t *testing.T) {
	cm := NewContextManager("You are helpful.", 100000)
	cm.AddUser("hello")
	cm.AddAssistant(Message{Role: "assistant", Content: "hi"})

	msgs := cm.Messages()
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[0].Content != "hello" {
		t.Error("first message should be user hello")
	}
}

func TestContextManager_SystemPrompt(t *testing.T) {
	cm := NewContextManager("Be concise.", 100000)
	if cm.SystemPrompt() != "Be concise." {
		t.Error("system prompt mismatch")
	}
}

func TestContextManager_EstimateTokens(t *testing.T) {
	cm := NewContextManager("system", 1000)
	cm.AddUser("hello world") // ~2-3 tokens
	tokens := cm.EstimateTokens()
	if tokens <= 0 {
		t.Error("should estimate some tokens")
	}
}

func TestContextManager_Truncate(t *testing.T) {
	cm := NewContextManager("sys", 100) // very small budget
	for i := 0; i < 50; i++ {
		cm.AddUser("this is a moderately long message to fill the context window quickly")
		cm.AddAssistant(Message{Role: "assistant", Content: "response"})
	}
	cm.Truncate()
	msgs := cm.Messages()
	if len(msgs) >= 100 {
		t.Error("truncate should have removed old messages")
	}
	// Last message should be preserved
	last := msgs[len(msgs)-1]
	if last.Content != "response" {
		t.Errorf("last message should be preserved, got: %s", last.Content)
	}
}
```

**Step 2: Run test — FAIL**

**Step 3: Write implementation**

```go
// internal/agent/context.go
package agent

// ContextManager manages conversation history and token budgeting.
type ContextManager struct {
	systemPrompt    string
	messages        []Message
	maxContextTokens int
}

func NewContextManager(systemPrompt string, maxTokens int) *ContextManager {
	if maxTokens <= 0 {
		maxTokens = 200000
	}
	return &ContextManager{
		systemPrompt:    systemPrompt,
		maxContextTokens: maxTokens,
	}
}

func (cm *ContextManager) SystemPrompt() string { return cm.systemPrompt }

func (cm *ContextManager) AddUser(content string) {
	cm.messages = append(cm.messages, Message{Role: "user", Content: content})
}

func (cm *ContextManager) AddAssistant(msg Message) {
	cm.messages = append(cm.messages, msg)
}

func (cm *ContextManager) AddToolResult(result ToolResult) {
	cm.messages = append(cm.messages, Message{
		Role:       "tool",
		ToolResult: &result,
	})
}

func (cm *ContextManager) Messages() []Message {
	return cm.messages
}

func (cm *ContextManager) Reset() {
	cm.messages = nil
}

// EstimateTokens returns a rough token count (chars / 4).
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

// Truncate removes oldest messages when context exceeds 80% budget.
// Keeps at least the last 10 messages.
func (cm *ContextManager) Truncate() {
	threshold := int(float64(cm.maxContextTokens) * 0.8)
	keep := 10

	for cm.EstimateTokens() > threshold && len(cm.messages) > keep {
		cm.messages = cm.messages[1:]
	}
}
```

**Step 4: Run test — PASS**

**Step 5: Commit**

```bash
git add internal/agent/context.go internal/agent/context_test.go
git commit -m "feat(ais-agent): add context manager with token estimation and truncation"
```

---

### Task 1.7: OpenAI Provider

**Files:**
- Create: `internal/agent/providers/openai.go`
- Test: `internal/agent/providers/openai_test.go`

**Step 1: Write failing test**

```go
// internal/agent/providers/openai_test.go
package providers

import (
	"testing"
)

func TestNewOpenAI_Defaults(t *testing.T) {
	p := NewOpenAI("test-key", "gpt-4o", "")
	if p.Name() != "openai" {
		t.Errorf("expected openai, got %s", p.Name())
	}
	if p.MaxContextTokens() != 128000 {
		t.Errorf("expected 128000, got %d", p.MaxContextTokens())
	}
}

func TestNewOpenAI_CustomBaseURL(t *testing.T) {
	p := NewOpenAI("key", "gpt-4o-mini", "https://custom.api.com/v1")
	if p.Name() != "openai" {
		t.Error("name should still be openai")
	}
}

func TestModelContextSize(t *testing.T) {
	tests := []struct {
		model    string
		expected int
	}{
		{"gpt-4o", 128000},
		{"gpt-4o-mini", 128000},
		{"gpt-4-turbo", 128000},
		{"gpt-3.5-turbo", 16385},
		{"unknown-model", 128000},
	}
	for _, tt := range tests {
		p := NewOpenAI("key", tt.model, "")
		if p.MaxContextTokens() != tt.expected {
			t.Errorf("model %s: expected %d, got %d", tt.model, tt.expected, p.MaxContextTokens())
		}
	}
}
```

**Step 2: Run test — FAIL**

**Step 3: Write implementation**

```go
// internal/agent/providers/openai.go
package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hanfourmini/aisupervisor/internal/agent"
	oai "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

type OpenAIProvider struct {
	client oai.Client
	model  string
}

func NewOpenAI(apiKey, model, baseURL string) *OpenAIProvider {
	var opts []option.RequestOption
	if apiKey != "" {
		opts = append(opts, option.WithAPIKey(apiKey))
	}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	if model == "" {
		model = "gpt-4o"
	}
	return &OpenAIProvider{
		client: oai.NewClient(opts...),
		model:  model,
	}
}

func (p *OpenAIProvider) Name() string { return "openai" }

func (p *OpenAIProvider) MaxContextTokens() int {
	switch {
	case strings.HasPrefix(p.model, "gpt-4o"), strings.HasPrefix(p.model, "gpt-4-turbo"):
		return 128000
	case strings.HasPrefix(p.model, "gpt-3.5"):
		return 16385
	default:
		return 128000
	}
}

func (p *OpenAIProvider) Chat(ctx context.Context, req agent.ChatRequest) (*agent.ChatResponse, error) {
	messages := p.convertMessages(req)
	tools := p.convertTools(req.Tools)

	params := oai.ChatCompletionNewParams{
		Model:    p.model,
		Messages: messages,
	}
	if len(tools) > 0 {
		params.Tools = tools
	}
	if req.MaxTokens > 0 {
		params.MaxCompletionTokens = oai.Int(int64(req.MaxTokens))
	}

	resp, err := p.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("openai chat: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("empty response from OpenAI")
	}

	choice := resp.Choices[0]
	result := &agent.ChatResponse{
		InputTokens:  int(resp.Usage.PromptTokens),
		OutputTokens: int(resp.Usage.CompletionTokens),
	}

	// Map finish reason
	switch choice.FinishReason {
	case "tool_calls":
		result.StopReason = "tool_use"
	case "length":
		result.StopReason = "max_tokens"
	default:
		result.StopReason = "end_turn"
	}

	// Convert message
	result.Message = agent.Message{
		Role:    "assistant",
		Content: choice.Message.Content,
	}

	// Convert tool calls
	for _, tc := range choice.Message.ToolCalls {
		result.Message.ToolCalls = append(result.Message.ToolCalls, agent.ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: json.RawMessage(tc.Function.Arguments),
		})
	}

	return result, nil
}

func (p *OpenAIProvider) convertMessages(req agent.ChatRequest) []oai.ChatCompletionMessageParamUnion {
	var msgs []oai.ChatCompletionMessageParamUnion

	if req.SystemPrompt != "" {
		msgs = append(msgs, oai.SystemMessage(req.SystemPrompt))
	}

	for _, m := range req.Messages {
		switch m.Role {
		case "user":
			msgs = append(msgs, oai.UserMessage(m.Content))
		case "assistant":
			if len(m.ToolCalls) > 0 {
				toolCalls := make([]oai.ChatCompletionMessageToolCallParam, len(m.ToolCalls))
				for i, tc := range m.ToolCalls {
					toolCalls[i] = oai.ChatCompletionMessageToolCallParam{
						ID:   tc.ID,
						Type: "function",
						Function: oai.ChatCompletionMessageToolCallFunctionParam{
							Name:      tc.Name,
							Arguments: string(tc.Arguments),
						},
					}
				}
				msgs = append(msgs, oai.ChatCompletionAssistantMessageParam{
					Content:   oai.String(m.Content),
					ToolCalls: toolCalls,
				})
			} else {
				msgs = append(msgs, oai.AssistantMessage(m.Content))
			}
		case "tool":
			if m.ToolResult != nil {
				msgs = append(msgs, oai.ToolMessage(m.ToolResult.Content, m.ToolResult.CallID))
			}
		}
	}
	return msgs
}

func (p *OpenAIProvider) convertTools(tools []agent.ToolDefinition) []oai.ChatCompletionToolParam {
	if len(tools) == 0 {
		return nil
	}
	result := make([]oai.ChatCompletionToolParam, len(tools))
	for i, t := range tools {
		result[i] = oai.ChatCompletionToolParam{
			Type: "function",
			Function: oai.FunctionDefinitionParam{
				Name:        t.Name,
				Description: oai.String(t.Description),
				Parameters:  oai.RawUnionParam[oai.FunctionParameters](t.Parameters),
			},
		}
	}
	return result
}
```

Note: The exact `openai-go` SDK types may need adjustment based on the actual SDK version (v1.12.0). The implementing engineer should check `go doc github.com/openai/openai-go` for the correct param struct names and adjust accordingly. The core pattern (convert messages → call API → convert response) is correct.

**Step 4: Run test — PASS**

**Step 5: Commit**

```bash
git add internal/agent/providers/
git commit -m "feat(ais-agent): add OpenAI provider with tool use support"
```

---

### Task 1.8: Agentic Loop

**Files:**
- Create: `internal/agent/loop.go`
- Test: `internal/agent/loop_test.go`

**Step 1: Write failing test**

```go
// internal/agent/loop_test.go
package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hanfourmini/aisupervisor/internal/agent/tools"
)

// mockProvider simulates an LLM for testing.
type mockProvider struct {
	responses []ChatResponse
	callIdx   int
}

func (m *mockProvider) Name() string          { return "mock" }
func (m *mockProvider) MaxContextTokens() int { return 100000 }
func (m *mockProvider) Chat(_ context.Context, _ ChatRequest) (*ChatResponse, error) {
	if m.callIdx >= len(m.responses) {
		return &ChatResponse{Message: Message{Role: "assistant", Content: "done"}, StopReason: "end_turn"}, nil
	}
	resp := m.responses[m.callIdx]
	m.callIdx++
	return &resp, nil
}

func TestLoop_SimpleResponse(t *testing.T) {
	provider := &mockProvider{
		responses: []ChatResponse{
			{Message: Message{Role: "assistant", Content: "Hello!"}, StopReason: "end_turn"},
		},
	}
	registry := tools.NewRegistry()
	loop := NewLoop(provider, registry, "Be helpful.", 200000)

	var output string
	loop.OnOutput = func(text string) { output = text }
	err := loop.Run(context.Background(), "Hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "Hello!" {
		t.Errorf("expected Hello!, got: %s", output)
	}
}

func TestLoop_ToolUse(t *testing.T) {
	provider := &mockProvider{
		responses: []ChatResponse{
			{
				Message: Message{
					Role: "assistant",
					ToolCalls: []ToolCall{
						{ID: "tc1", Name: "Read", Arguments: json.RawMessage(`{"file_path":"/dev/null"}`)},
					},
				},
				StopReason: "tool_use",
			},
			{
				Message:    Message{Role: "assistant", Content: "File is empty."},
				StopReason: "end_turn",
			},
		},
	}
	registry := tools.NewRegistry()
	registry.Register(&tools.ReadTool{})

	loop := NewLoop(provider, registry, "Be helpful.", 200000)
	var output string
	loop.OnOutput = func(text string) { output = text }
	err := loop.Run(context.Background(), "Read /dev/null")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "File is empty." {
		t.Errorf("expected 'File is empty.', got: %s", output)
	}
	if provider.callIdx != 2 {
		t.Errorf("expected 2 LLM calls, got %d", provider.callIdx)
	}
}

func TestLoop_MaxIterations(t *testing.T) {
	// Provider always returns tool_use → infinite loop
	provider := &mockProvider{
		responses: []ChatResponse{},
	}
	// Override to always return tool_use
	infiniteProvider := &infiniteToolProvider{}
	registry := tools.NewRegistry()
	registry.Register(&tools.ReadTool{})

	loop := NewLoop(infiniteProvider, registry, "sys", 200000)
	loop.MaxIterations = 3
	err := loop.Run(context.Background(), "loop forever")
	if err == nil {
		t.Error("should return max iterations error")
	}
}

type infiniteToolProvider struct{ calls int }

func (p *infiniteToolProvider) Name() string          { return "infinite" }
func (p *infiniteToolProvider) MaxContextTokens() int { return 100000 }
func (p *infiniteToolProvider) Chat(_ context.Context, _ ChatRequest) (*ChatResponse, error) {
	p.calls++
	return &ChatResponse{
		Message: Message{
			Role:      "assistant",
			ToolCalls: []ToolCall{{ID: "tc", Name: "Read", Arguments: json.RawMessage(`{"file_path":"/dev/null"}`)}},
		},
		StopReason: "tool_use",
	}, nil
}
```

**Step 2: Run test — FAIL**

**Step 3: Write implementation**

```go
// internal/agent/loop.go
package agent

import (
	"context"
	"fmt"

	"github.com/hanfourmini/aisupervisor/internal/agent/tools"
)

// Loop is the core agentic loop: LLM → tool → LLM → ... → done.
type Loop struct {
	provider      Provider
	registry      *tools.Registry
	context       *ContextManager
	MaxIterations int
	OnOutput      func(text string)        // called when LLM produces final text
	OnToolCall    func(name, args string)   // called when a tool is invoked
	TotalInput    int
	TotalOutput   int
}

func NewLoop(provider Provider, registry *tools.Registry, systemPrompt string, maxTokens int) *Loop {
	return &Loop{
		provider:      provider,
		registry:      registry,
		context:       NewContextManager(systemPrompt, maxTokens),
		MaxIterations: 200,
	}
}

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
			for _, call := range resp.Message.ToolCalls {
				result := l.executeTool(ctx, call)
				l.context.AddToolResult(result)
			}
			continue

		case "max_tokens":
			l.context.Truncate()
			continue

		default:
			if l.OnOutput != nil && resp.Message.Content != "" {
				l.OnOutput(resp.Message.Content)
			}
			return nil
		}
	}
	return fmt.Errorf("max iterations (%d) reached", l.MaxIterations)
}

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

	// Truncate very large outputs
	const maxOutput = 10000
	if len(output) > maxOutput {
		output = output[:maxOutput] + "\n[truncated, showing first 10000 chars]"
	}

	return ToolResult{
		CallID:  call.ID,
		Content: output,
	}
}

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

// ResetHistory clears conversation for a new prompt (keeps system prompt).
func (l *Loop) ResetHistory() {
	l.context.Reset()
}
```

**Step 4: Run test — PASS**

**Step 5: Commit**

```bash
git add internal/agent/loop.go internal/agent/loop_test.go
git commit -m "feat(ais-agent): add agentic loop with tool execution and max iterations"
```

---

### Task 1.9: CLI Entry Point & REPL

**Files:**
- Create: `cmd/ais-agent/main.go`

**Step 1: Write the REPL binary**

```go
// cmd/ais-agent/main.go
package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"context"
	"os/signal"

	"github.com/hanfourmini/aisupervisor/internal/agent"
	"github.com/hanfourmini/aisupervisor/internal/agent/providers"
	"github.com/hanfourmini/aisupervisor/internal/agent/tools"
)

var version = "0.1.0"

func main() {
	providerName := flag.String("provider", "openai", "LLM provider (openai, anthropic, ollama)")
	model := flag.String("model", "", "Model name (e.g. gpt-4o, claude-sonnet-4-20250514)")
	apiKey := flag.String("api-key", "", "API key (default: from env OPENAI_API_KEY / ANTHROPIC_API_KEY)")
	baseURL := flag.String("base-url", "", "Override API base URL")
	systemPrompt := flag.String("append-system-prompt", "", "Additional system prompt text")
	allowedTools := flag.String("allowed-tools", "", "Comma-separated list of allowed tools")
	maxTokens := flag.Int("max-tokens", 0, "Max context tokens (default: provider-specific)")
	maxIterations := flag.Int("max-iterations", 200, "Max agentic loop iterations")
	permissionMode := flag.String("permission-mode", "bypassPermissions", "Permission mode: plan, acceptEdits, bypassPermissions")
	showVersion := flag.Bool("version", false, "Show version")
	flag.Parse()

	if *showVersion {
		fmt.Printf("ais-agent v%s\n", version)
		os.Exit(0)
	}

	// Resolve API key from env if not provided
	if *apiKey == "" {
		switch *providerName {
		case "openai":
			*apiKey = os.Getenv("OPENAI_API_KEY")
		case "anthropic":
			*apiKey = os.Getenv("ANTHROPIC_API_KEY")
		}
	}

	// Create provider
	provider, err := createProvider(*providerName, *apiKey, *model, *baseURL)
	if err != nil {
		log.Fatalf("Failed to create provider: %v", err)
	}

	// Set up tool registry
	registry := tools.NewRegistry()
	registerTools(registry, *permissionMode)
	if *allowedTools != "" {
		registry.SetAllowed(strings.Split(*allowedTools, ","))
	}

	// Build system prompt
	sysPrompt := "You are an expert software engineer. Use the available tools to complete tasks. When done, provide a summary of what you did."
	if *systemPrompt != "" {
		sysPrompt += "\n\n" + *systemPrompt
	}

	// Create loop
	tokenBudget := *maxTokens
	if tokenBudget == 0 {
		tokenBudget = provider.MaxContextTokens()
	}
	loop := agent.NewLoop(provider, registry, sysPrompt, tokenBudget)
	loop.MaxIterations = *maxIterations
	loop.OnOutput = func(text string) {
		fmt.Println(text)
	}
	loop.OnToolCall = func(name, args string) {
		fmt.Printf("  [tool] %s\n", name)
	}

	// Handle Ctrl+C
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Print banner
	fmt.Printf("ais-agent v%s (%s/%s)\n", version, provider.Name(), *model)

	// REPL
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer for large prompts

	for {
		fmt.Print("❯ ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if input == "/stop" || input == "/exit" {
			fmt.Println("Goodbye.")
			break
		}
		if input == "/reset" {
			loop.ResetHistory()
			fmt.Println("Conversation reset.")
			continue
		}

		if err := loop.Run(ctx, input); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		fmt.Printf("  [tokens] in:%d out:%d\n", loop.TotalInput, loop.TotalOutput)
	}
}

func createProvider(name, apiKey, model, baseURL string) (agent.Provider, error) {
	switch name {
	case "openai":
		return providers.NewOpenAI(apiKey, model, baseURL), nil
	// Phase 2: case "anthropic", "ollama"
	default:
		return nil, fmt.Errorf("unknown provider: %s", name)
	}
}

func registerTools(registry *tools.Registry, permMode string) {
	// Always available
	registry.Register(&tools.ReadTool{})
	registry.Register(&tools.GlobTool{})
	registry.Register(&tools.GrepTool{})

	switch permMode {
	case "acceptEdits", "bypassPermissions":
		registry.Register(&tools.EditTool{})
		registry.Register(&tools.WriteTool{})
		registry.Register(&tools.BashTool{})
	case "plan":
		// Read-only tools only
	}
}
```

**Step 2: Build and test**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go build -o /tmp/ais-agent ./cmd/ais-agent/`
Expected: binary built at `/tmp/ais-agent`

Run: `/tmp/ais-agent --version`
Expected: `ais-agent v0.1.0`

**Step 3: Commit**

```bash
git add cmd/ais-agent/
git commit -m "feat(ais-agent): add CLI entry point with REPL, flag parsing, and permission modes"
```

---

### Task 1.10: Integration Smoke Test

**Manual verification — not automated.**

Run with a real OpenAI API key:
```bash
OPENAI_API_KEY=sk-... /tmp/ais-agent --provider openai --model gpt-4o-mini
```

Test these scenarios:
1. Ask it to read a file: "Read the file /tmp/ais-agent" → should use Read tool
2. Ask it to create a file: "Create a file /tmp/test-ais.txt with content hello" → should use Write tool
3. Ask it to run a command: "Run ls -la /tmp" → should use Bash tool
4. Type `/stop` → should exit cleanly

Verify that:
- `❯` prompt appears after each response
- Tool calls are logged with `[tool]` prefix
- Token usage is printed after each turn
- The loop terminates (doesn't hang)

**Commit:**

```bash
git commit --allow-empty -m "chore(ais-agent): Phase 1 MVP verified — REPL + OpenAI + 6 tools"
```

---

## Execution Order Summary

| Task | What | Files | Est. Commits |
|------|------|-------|-------------|
| 1.1 | Tool interface + registry | 2 | 1 |
| 1.2 | Read tool | 2 | 1 |
| 1.3 | Edit tool | 2 | 1 |
| 1.4 | Write, Bash, Glob, Grep tools | 8 | 1 |
| 1.5 | Provider interface + types | 2 | 1 |
| 1.6 | Context manager | 2 | 1 |
| 1.7 | OpenAI provider | 2 | 1 |
| 1.8 | Agentic loop | 2 | 1 |
| 1.9 | CLI entry point + REPL | 1 | 1 |
| 1.10 | Smoke test | 0 | 1 |

**Total: ~21 files, ~10 commits, ~1,200 lines**

Tasks 1.1-1.4 (tools) are independent. Tasks 1.5-1.7 (provider) are independent. Task 1.8 depends on 1.1-1.7. Task 1.9 depends on 1.8.

**Parallelizable groups:**
- Worktree A: Tasks 1.1-1.4 (tools)
- Worktree B: Tasks 1.5-1.7 (provider + context)
- Then merge and do 1.8-1.9 sequentially
