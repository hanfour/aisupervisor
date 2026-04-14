// internal/ai/credential_pool.go
package ai

import (
	"fmt"
	"sync"
	"time"
)

const (
	StrategyRoundRobin = "round-robin"
	StrategyLeastUsed  = "least-used"
)

// Credential represents a single API key with usage tracking.
type Credential struct {
	ID            string
	APIKey        string
	Provider      string
	UsageCount    int64
	CooldownUntil time.Time
}

// CredentialPool manages a set of API credentials with selection strategies
// and rate-limit cooldown support.
type CredentialPool struct {
	mu       sync.Mutex
	creds    []Credential
	strategy string
	nextIdx  int // for round-robin
}

// NewCredentialPool creates a pool from the given credentials and selection strategy.
// strategy must be StrategyRoundRobin or StrategyLeastUsed.
func NewCredentialPool(creds []Credential, strategy string) *CredentialPool {
	// Defensive copy so callers cannot mutate the internal slice.
	copied := make([]Credential, len(creds))
	copy(copied, creds)
	return &CredentialPool{
		creds:    copied,
		strategy: strategy,
	}
}

// Select picks the next available credential based on the pool's strategy.
// It increments UsageCount on the selected credential.
// Returns an error if the pool is empty or all credentials are in cooldown.
func (p *CredentialPool) Select() (*Credential, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.creds) == 0 {
		return nil, fmt.Errorf("credential pool is empty")
	}

	now := time.Now()

	switch p.strategy {
	case StrategyLeastUsed:
		return p.selectLeastUsed(now)
	default: // round-robin
		return p.selectRoundRobin(now)
	}
}

func (p *CredentialPool) selectRoundRobin(now time.Time) (*Credential, error) {
	n := len(p.creds)
	for i := 0; i < n; i++ {
		idx := (p.nextIdx + i) % n
		if p.creds[idx].CooldownUntil.After(now) {
			continue
		}
		p.nextIdx = (idx + 1) % n
		p.creds[idx].UsageCount++
		cred := p.creds[idx] // return a copy
		return &cred, nil
	}
	return nil, fmt.Errorf("all credentials are in cooldown")
}

func (p *CredentialPool) selectLeastUsed(now time.Time) (*Credential, error) {
	bestIdx := -1
	var bestCount int64
	for i := range p.creds {
		if p.creds[i].CooldownUntil.After(now) {
			continue
		}
		if bestIdx == -1 || p.creds[i].UsageCount < bestCount {
			bestIdx = i
			bestCount = p.creds[i].UsageCount
		}
	}
	if bestIdx == -1 {
		return nil, fmt.Errorf("all credentials are in cooldown")
	}
	p.creds[bestIdx].UsageCount++
	cred := p.creds[bestIdx] // return a copy
	return &cred, nil
}

// MarkRateLimited sets a cooldown period on the credential with the given ID.
// During cooldown, Select() will skip this credential.
func (p *CredentialPool) MarkRateLimited(id string, duration time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i := range p.creds {
		if p.creds[i].ID == id {
			p.creds[i].CooldownUntil = time.Now().Add(duration)
			return
		}
	}
}

// Release is called after a request completes. Currently a no-op placeholder
// for future connection-tracking or concurrency limiting.
func (p *CredentialPool) Release(id string) {
	// no-op — placeholder for future concurrency tracking
}
