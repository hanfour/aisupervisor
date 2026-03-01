package keychain

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type ClaudeCredentials struct {
	AccessToken  string    `json:"accessToken"`
	RefreshToken string    `json:"refreshToken"`
	ExpiresAt    time.Time `json:"expiresAt"`
}

func ReadClaudeCredentials() (*ClaudeCredentials, error) {
	cmd := exec.Command("security", "find-generic-password",
		"-s", "Claude Code-credentials", "-w")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("keychain read failed (is Claude Code installed?): %w", err)
	}

	raw := strings.TrimSpace(string(out))

	var creds ClaudeCredentials
	if err := json.Unmarshal([]byte(raw), &creds); err != nil {
		return nil, fmt.Errorf("parsing credentials: %w", err)
	}

	return &creds, nil
}

func (c *ClaudeCredentials) IsExpired() bool {
	return time.Now().After(c.ExpiresAt.Add(-5 * time.Minute))
}
