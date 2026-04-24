package worker

import (
	"errors"
	"strings"
)

// ErrorAction represents the recommended action for a classified error.
type ErrorAction string

const (
	ActionRetry    ErrorAction = "retry"
	ActionRotate   ErrorAction = "rotate"
	ActionAbandon  ErrorAction = "abandon"
	ActionCompress ErrorAction = "compress"
)

// ErrRuntimeFallbackExhausted marks a spawn failure that already exhausted
// the runtime-fallback mechanism (DetectReady timed out on the requested
// runtime AND on the claude fallback). ClassifyError routes this sentinel
// to ActionAbandon so SpawnForTask's outer 3x retry loop stops immediately
// instead of re-triggering the same end-to-end timeout chain.
var ErrRuntimeFallbackExhausted = errors.New("runtime fallback exhausted")

// errorPatterns maps keywords to actions, checked in priority order.
var errorPatterns = []struct {
	keywords []string
	action   ErrorAction
}{
	{[]string{"context length", "too many tokens", "maximum context", "token limit"}, ActionCompress},
	{[]string{"invalid api key", "invalid_api_key", "401", "403", "unauthorized", "forbidden"}, ActionAbandon},
	{[]string{"billing", "payment required", "402", "insufficient_quota"}, ActionAbandon},
	{[]string{"rate limit", "429", "quota", "too many requests"}, ActionRetry},
	{[]string{"timeout", "connection", "timed out", "connect error", "eof"}, ActionRetry},
}

// ClassifyError examines an error message and returns the recommended action.
func ClassifyError(err error) ErrorAction {
	if err == nil {
		return ActionRetry
	}
	if errors.Is(err, ErrRuntimeFallbackExhausted) {
		return ActionAbandon
	}
	lower := strings.ToLower(err.Error())
	for _, p := range errorPatterns {
		for _, kw := range p.keywords {
			if strings.Contains(lower, kw) {
				return p.action
			}
		}
	}
	return ActionRetry
}
