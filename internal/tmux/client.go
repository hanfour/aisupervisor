package tmux

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type SessionInfo struct {
	Name    string
	Windows int
}

type PaneInfo struct {
	SessionName string
	WindowIndex int
	PaneIndex   int
	Title       string
	Active      bool
}

type TmuxClient interface {
	ListSessions() ([]SessionInfo, error)
	ListPanes(session string) ([]PaneInfo, error)
	CapturePane(session string, window, pane, lines int) (string, error)
	SendKeys(session string, window, pane int, keys string) error
	SendLiteralKeys(session string, window, pane int, text string) error
	CreateSession(name string) error
	KillSession(name string) error
	HasSession(name string) (bool, error)
}

// execClient implements TmuxClient using only exec.Command calls to tmux binary.
// This avoids gotmux library socket/connection issues where sessions created via
// exec.Command are not visible to gotmux's internal connection.
type execClient struct{}

func NewClient() (TmuxClient, error) {
	return &execClient{}, nil
}

func (c *execClient) target(session string, window, pane int) string {
	return fmt.Sprintf("%s:%d.%d", session, window, pane)
}

func (c *execClient) ListSessions() ([]SessionInfo, error) {
	out, err := exec.Command("tmux", "list-sessions", "-F", "#{session_name}\t#{session_windows}").Output()
	if err != nil {
		// No server running is not an error — just no sessions
		if strings.Contains(string(out)+err.Error(), "no server running") {
			return nil, nil
		}
		return nil, err
	}
	var result []SessionInfo
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) < 2 {
			continue
		}
		wins, _ := strconv.Atoi(parts[1])
		result = append(result, SessionInfo{Name: parts[0], Windows: wins})
	}
	return result, nil
}

func (c *execClient) ListPanes(session string) ([]PaneInfo, error) {
	out, err := exec.Command("tmux", "list-panes", "-t", session, "-a", "-F",
		"#{session_name}\t#{window_index}\t#{pane_index}\t#{pane_active}").Output()
	if err != nil {
		return nil, fmt.Errorf("listing panes for %q: %w", session, err)
	}
	var result []PaneInfo
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 4)
		if len(parts) < 4 {
			continue
		}
		wIdx, _ := strconv.Atoi(parts[1])
		pIdx, _ := strconv.Atoi(parts[2])
		result = append(result, PaneInfo{
			SessionName: parts[0],
			WindowIndex: wIdx,
			PaneIndex:   pIdx,
			Active:      parts[3] == "1",
		})
	}
	return result, nil
}

func (c *execClient) CapturePane(session string, window, pane, lines int) (string, error) {
	target := c.target(session, window, pane)
	args := []string{"capture-pane", "-t", target, "-p"}
	if lines > 0 {
		args = append(args, "-S", fmt.Sprintf("-%d", lines))
	}
	out, err := exec.Command("tmux", args...).Output()
	if err != nil {
		return "", fmt.Errorf("capture-pane %q: %w", target, err)
	}
	return string(out), nil
}

func (c *execClient) SendKeys(session string, window, pane int, keys string) error {
	target := c.target(session, window, pane)
	// Split keys into parts so special key names like "Enter" are separate args.
	// e.g. "cd /path Enter" → ["cd /path", "Enter"]
	args := []string{"send-keys", "-t", target}
	parts := strings.Split(keys, " ")
	// Rebuild: merge all parts except trailing special keys (Enter, Escape, etc.)
	// Convention: callers append " Enter" at the end of the keys string.
	var textParts []string
	var trailingSpecials []string
	specialKeys := map[string]bool{"Enter": true, "Escape": true, "Tab": true, "Space": true, "BSpace": true, "Up": true, "Down": true, "Left": true, "Right": true, "C-c": true}

	// Scan from the end to find trailing special keys
	for i := len(parts) - 1; i >= 0; i-- {
		if specialKeys[parts[i]] {
			trailingSpecials = append([]string{parts[i]}, trailingSpecials...)
		} else {
			textParts = parts[:i+1]
			break
		}
	}
	if len(textParts) == 0 && len(trailingSpecials) > 0 {
		// All parts are special keys
		args = append(args, trailingSpecials...)
	} else {
		if len(textParts) > 0 {
			args = append(args, strings.Join(textParts, " "))
		}
		args = append(args, trailingSpecials...)
	}
	return exec.Command("tmux", args...).Run()
}

func (c *execClient) SendLiteralKeys(session string, window, pane int, text string) error {
	target := c.target(session, window, pane)
	return exec.Command("tmux", "send-keys", "-t", target, "-l", "--", text).Run()
}

func (c *execClient) CreateSession(name string) error {
	return exec.Command("tmux", "new-session", "-d", "-s", name).Run()
}

func (c *execClient) KillSession(name string) error {
	return exec.Command("tmux", "kill-session", "-t", name).Run()
}

func (c *execClient) HasSession(name string) (bool, error) {
	err := exec.Command("tmux", "has-session", "-t", name).Run()
	if err == nil {
		return true, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}
