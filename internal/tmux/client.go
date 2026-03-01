package tmux

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/GianlucaP106/gotmux/gotmux"
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
}

type gotmuxClient struct {
	tmux *gotmux.Tmux
}

func NewClient() (TmuxClient, error) {
	t, err := gotmux.NewTmux("")
	if err != nil {
		return nil, fmt.Errorf("failed to connect to tmux: %w", err)
	}
	return &gotmuxClient{tmux: t}, nil
}

func (c *gotmuxClient) ListSessions() ([]SessionInfo, error) {
	sessions, err := c.tmux.ListSessions()
	if err != nil {
		return nil, err
	}
	var result []SessionInfo
	for _, s := range sessions {
		result = append(result, SessionInfo{
			Name:    s.Name,
			Windows: s.Windows,
		})
	}
	return result, nil
}

func (c *gotmuxClient) ListPanes(session string) ([]PaneInfo, error) {
	s, err := c.tmux.GetSessionByName(session)
	if err != nil {
		return nil, fmt.Errorf("session %q not found: %w", session, err)
	}

	windows, err := s.ListWindows()
	if err != nil {
		return nil, err
	}

	var result []PaneInfo
	for _, w := range windows {
		panes, err := w.ListPanes()
		if err != nil {
			return nil, err
		}
		for _, p := range panes {
			result = append(result, PaneInfo{
				SessionName: session,
				WindowIndex: w.Index,
				PaneIndex:   p.Index,
				Active:      p.Active,
			})
		}
	}
	return result, nil
}

func (c *gotmuxClient) CapturePane(session string, window, pane, lines int) (string, error) {
	s, err := c.tmux.GetSessionByName(session)
	if err != nil {
		return "", fmt.Errorf("session %q not found: %w", session, err)
	}

	w, err := s.GetWindowByIndex(window)
	if err != nil {
		return "", fmt.Errorf("window %d not found: %w", window, err)
	}

	panes, err := w.ListPanes()
	if err != nil {
		return "", err
	}

	for _, p := range panes {
		if p.Index == pane {
			content, err := p.Capture()
			if err != nil {
				return "", err
			}
			if lines > 0 {
				allLines := strings.Split(content, "\n")
				if len(allLines) > lines {
					allLines = allLines[len(allLines)-lines:]
				}
				return strings.Join(allLines, "\n"), nil
			}
			return content, nil
		}
	}
	return "", fmt.Errorf("pane %d not found in window %d", pane, window)
}

func (c *gotmuxClient) SendKeys(session string, window, pane int, keys string) error {
	s, err := c.tmux.GetSessionByName(session)
	if err != nil {
		return fmt.Errorf("session %q not found: %w", session, err)
	}

	w, err := s.GetWindowByIndex(window)
	if err != nil {
		return fmt.Errorf("window %d not found: %w", window, err)
	}

	panes, err := w.ListPanes()
	if err != nil {
		return err
	}

	for _, p := range panes {
		if p.Index == pane {
			return p.SendKeys(keys)
		}
	}
	return fmt.Errorf("pane %d not found in window %d", pane, window)
}

func (c *gotmuxClient) SendLiteralKeys(session string, window, pane int, text string) error {
	target := fmt.Sprintf("%s:%d.%d", session, window, pane)
	cmd := exec.Command("tmux", "send-keys", "-t", target, "-l", "--", text)
	return cmd.Run()
}
