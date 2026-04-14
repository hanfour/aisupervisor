// internal/ai/credential_pool_test.go
package ai

import (
	"testing"
	"time"
)

func TestNewCredentialPool_RoundRobin(t *testing.T) {
	creds := []Credential{
		{ID: "key1", APIKey: "sk-aaa", Provider: "anthropic"},
		{ID: "key2", APIKey: "sk-bbb", Provider: "anthropic"},
		{ID: "key3", APIKey: "sk-ccc", Provider: "anthropic"},
	}
	pool := NewCredentialPool(creds, StrategyRoundRobin)
	if pool == nil {
		t.Fatal("expected non-nil pool")
	}

	// Should cycle through keys in order
	seen := make(map[string]int)
	for i := 0; i < 6; i++ {
		cred, err := pool.Select()
		if err != nil {
			t.Fatalf("Select() error: %v", err)
		}
		seen[cred.ID]++
		pool.Release(cred.ID)
	}
	for _, id := range []string{"key1", "key2", "key3"} {
		if seen[id] != 2 {
			t.Errorf("expected key %s selected 2 times, got %d", id, seen[id])
		}
	}
}

func TestNewCredentialPool_LeastUsed(t *testing.T) {
	creds := []Credential{
		{ID: "key1", APIKey: "sk-aaa", Provider: "anthropic", UsageCount: 10},
		{ID: "key2", APIKey: "sk-bbb", Provider: "anthropic", UsageCount: 2},
		{ID: "key3", APIKey: "sk-ccc", Provider: "anthropic", UsageCount: 5},
	}
	pool := NewCredentialPool(creds, StrategyLeastUsed)

	cred, err := pool.Select()
	if err != nil {
		t.Fatalf("Select() error: %v", err)
	}
	if cred.ID != "key2" {
		t.Errorf("expected least-used key2, got %s", cred.ID)
	}
	pool.Release(cred.ID)
}

func TestCredentialPool_SkipsCooledDown(t *testing.T) {
	creds := []Credential{
		{ID: "key1", APIKey: "sk-aaa", Provider: "anthropic"},
		{ID: "key2", APIKey: "sk-bbb", Provider: "anthropic"},
	}
	pool := NewCredentialPool(creds, StrategyRoundRobin)

	// Mark key1 as rate-limited for 1 hour
	pool.MarkRateLimited("key1", 1*time.Hour)

	// Should only return key2 now
	for i := 0; i < 3; i++ {
		cred, err := pool.Select()
		if err != nil {
			t.Fatalf("Select() error: %v", err)
		}
		if cred.ID != "key2" {
			t.Errorf("expected key2 (key1 is cooled down), got %s", cred.ID)
		}
		pool.Release(cred.ID)
	}
}

func TestCredentialPool_MarkRateLimited(t *testing.T) {
	creds := []Credential{
		{ID: "key1", APIKey: "sk-aaa", Provider: "anthropic"},
	}
	pool := NewCredentialPool(creds, StrategyRoundRobin)

	pool.MarkRateLimited("key1", 50*time.Millisecond)

	// Immediately after marking, should get error (all cooled down)
	_, err := pool.Select()
	if err == nil {
		t.Error("expected error when all credentials are cooled down")
	}

	// Wait for cooldown to expire
	time.Sleep(60 * time.Millisecond)

	cred, err := pool.Select()
	if err != nil {
		t.Fatalf("Select() after cooldown expired: %v", err)
	}
	if cred.ID != "key1" {
		t.Errorf("expected key1 after cooldown, got %s", cred.ID)
	}
	pool.Release(cred.ID)
}

func TestCredentialPool_EmptyReturnsError(t *testing.T) {
	pool := NewCredentialPool(nil, StrategyRoundRobin)
	_, err := pool.Select()
	if err == nil {
		t.Error("expected error for empty pool")
	}
}

func TestCredentialPool_SingleCredential(t *testing.T) {
	creds := []Credential{
		{ID: "only", APIKey: "sk-only", Provider: "openai"},
	}
	pool := NewCredentialPool(creds, StrategyRoundRobin)

	for i := 0; i < 5; i++ {
		cred, err := pool.Select()
		if err != nil {
			t.Fatalf("Select() error: %v", err)
		}
		if cred.ID != "only" {
			t.Errorf("expected 'only', got %s", cred.ID)
		}
		if cred.APIKey != "sk-only" {
			t.Errorf("expected APIKey 'sk-only', got %s", cred.APIKey)
		}
		pool.Release(cred.ID)
	}
}

func TestCredentialPool_UsageCountIncremented(t *testing.T) {
	creds := []Credential{
		{ID: "key1", APIKey: "sk-aaa", Provider: "anthropic"},
	}
	pool := NewCredentialPool(creds, StrategyRoundRobin)

	cred, _ := pool.Select()
	if cred.UsageCount != 1 {
		t.Errorf("expected UsageCount 1 after Select, got %d", cred.UsageCount)
	}
	pool.Release(cred.ID)

	cred2, _ := pool.Select()
	if cred2.UsageCount != 2 {
		t.Errorf("expected UsageCount 2 after second Select, got %d", cred2.UsageCount)
	}
	pool.Release(cred2.ID)
}
