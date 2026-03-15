package claudecli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/ai"
)

// Backend implements ai.ChatProvider using the Claude CLI in print mode.
type Backend struct {
	path string // resolved path to claude binary
}

// New returns a Backend if the claude CLI is found, otherwise nil.
// It searches PATH first, then well-known install locations for macOS
// .app bundles where PATH may be minimal.
func New() *Backend {
	// Ensure PATH includes common binary dirs (critical for .app bundles
	// launched from Finder which have minimal PATH).
	ensureCLIPath()

	if path, err := exec.LookPath("claude"); err == nil {
		log.Printf("claudecli.New: found via LookPath: %s", path)
		return &Backend{path: path}
	}

	home, _ := os.UserHomeDir()
	if home == "" {
		log.Printf("claudecli.New: HOME is empty, cannot search")
		return nil
	}

	candidates := []string{
		filepath.Join(home, ".local", "bin", "claude"),
		filepath.Join(home, ".claude", "local", "bin", "claude"),
		"/usr/local/bin/claude",
		"/opt/homebrew/bin/claude",
	}

	npmDirs := []string{
		filepath.Join(home, ".volta", "bin"),
		filepath.Join(home, ".fnm", "aliases", "default", "bin"),
	}
	for _, d := range npmDirs {
		candidates = append(candidates, filepath.Join(d, "claude"))
	}

	nvmDir := filepath.Join(home, ".nvm", "versions", "node")
	if entries, err := os.ReadDir(nvmDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				candidates = append(candidates, filepath.Join(nvmDir, e.Name(), "bin", "claude"))
			}
		}
	}

	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			log.Printf("claudecli.New: found at candidate path: %s", c)
			return &Backend{path: c}
		}
	}

	log.Printf("claudecli.New: not found (HOME=%s, PATH=%s)", home, os.Getenv("PATH"))
	return nil
}

// ensureCLIPath expands PATH with common binary directories so claude
// can be found when the app is launched from Finder with minimal PATH.
func ensureCLIPath() {
	home, _ := os.UserHomeDir()
	if home == "" {
		return
	}
	extraPaths := []string{
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, ".claude", "local", "bin"),
		"/usr/local/bin",
		"/opt/homebrew/bin",
		filepath.Join(home, ".volta", "bin"),
		filepath.Join(home, ".fnm", "aliases", "default", "bin"),
	}
	// nvm versions
	nvmDir := filepath.Join(home, ".nvm", "versions", "node")
	if entries, err := os.ReadDir(nvmDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				extraPaths = append(extraPaths, filepath.Join(nvmDir, e.Name(), "bin"))
			}
		}
	}

	currentPath := os.Getenv("PATH")
	changed := false
	for _, p := range extraPaths {
		if _, err := os.Stat(p); err == nil && !strings.Contains(currentPath, p) {
			currentPath = p + ":" + currentPath
			changed = true
		}
	}
	if changed {
		os.Setenv("PATH", currentPath)
	}
}

// Path returns the resolved path to the claude binary.
func (b *Backend) Path() string { return b.path }

// cliResponse is the JSON structure returned by `claude -p --output-format json`.
type cliResponse struct {
	Type   string `json:"type"`
	Result string `json:"result"`
}

// Chat sends a prompt to the Claude CLI and returns the response text.
func (b *Backend) Chat(ctx context.Context, messages []ai.ChatMessage) (string, error) {
	var systemPrompt string
	var userParts []string

	for _, m := range messages {
		switch m.Role {
		case "system":
			systemPrompt = m.Content
		case "assistant":
			userParts = append(userParts, "[Previous assistant response]\n"+m.Content)
		default:
			if m.Content != "" {
				userParts = append(userParts, m.Content)
			}
		}
	}

	prompt := strings.Join(userParts, "\n\n")
	if prompt == "" {
		prompt = "(Start the conversation)"
	}

	args := []string{"-p", "--output-format", "json"}
	if systemPrompt != "" {
		args = append(args, "--append-system-prompt", systemPrompt)
	}
	args = append(args, prompt)

	// Use a 90-second timeout to avoid hanging
	timeout := 90 * time.Second
	cliCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cliCtx, b.path, args...)

	// Remove all Claude Code env vars to avoid "nested session" detection.
	env := os.Environ()
	for _, key := range []string{"CLAUDECODE", "CLAUDE_CODE_ENTRYPOINT"} {
		env = filterEnv(env, key)
	}
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	log.Printf("claudecli: running %s %v", b.path, args[:3]) // log without full prompt

	if err := cmd.Run(); err != nil {
		stderrStr := stderr.String()
		if len(stderrStr) > 300 {
			stderrStr = stderrStr[:300]
		}
		return "", fmt.Errorf("claude CLI failed: %w (stderr: %s)", err, stderrStr)
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return "", fmt.Errorf("empty response from claude CLI")
	}

	// Try parsing as JSON response first
	var resp cliResponse
	if err := json.Unmarshal([]byte(output), &resp); err == nil && resp.Result != "" {
		return resp.Result, nil
	}

	// Multi-line JSON stream: take the last result line
	lines := strings.Split(output, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		var r cliResponse
		if err := json.Unmarshal([]byte(line), &r); err == nil && r.Result != "" {
			return r.Result, nil
		}
	}

	return output, nil
}

// filterEnv removes a key from an env slice.
func filterEnv(env []string, key string) []string {
	prefix := key + "="
	result := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.HasPrefix(e, prefix) {
			result = append(result, e)
		}
	}
	return result
}
