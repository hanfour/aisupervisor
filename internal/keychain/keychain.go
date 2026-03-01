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
	ExpiresAt    time.Time `json:"-"`
}

// rawCredentials matches the keychain JSON structure.
type rawCredentials struct {
	ClaudeAiOauth struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		ExpiresAt    int64  `json:"expiresAt"` // Unix timestamp in milliseconds
	} `json:"claudeAiOauth"`
}

func ReadClaudeCredentials() (*ClaudeCredentials, error) {
	cmd := exec.Command("security", "find-generic-password",
		"-s", "Claude Code-credentials", "-w")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("keychain read failed (is Claude Code installed?): %w", err)
	}

	raw := strings.TrimSpace(string(out))

	var wrapper rawCredentials
	if err := json.Unmarshal([]byte(raw), &wrapper); err != nil {
		return nil, fmt.Errorf("parsing credentials: %w", err)
	}

	oauth := wrapper.ClaudeAiOauth
	if oauth.AccessToken == "" {
		return nil, fmt.Errorf("no accessToken found in keychain credentials")
	}

	return &ClaudeCredentials{
		AccessToken:  oauth.AccessToken,
		RefreshToken: oauth.RefreshToken,
		ExpiresAt:    time.UnixMilli(oauth.ExpiresAt),
	}, nil
}

func (c *ClaudeCredentials) IsExpired() bool {
	return time.Now().After(c.ExpiresAt.Add(-5 * time.Minute))
}
