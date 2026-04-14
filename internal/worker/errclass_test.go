package worker

import (
	"errors"
	"testing"
)

func TestClassifyError_RateLimit(t *testing.T) {
	err := errors.New("API error: rate limit exceeded (429)")
	action := ClassifyError(err)
	if action != ActionRetry {
		t.Errorf("expected retry for rate limit, got %s", action)
	}
}

func TestClassifyError_ContextLength(t *testing.T) {
	err := errors.New("context length exceeded: too many tokens")
	action := ClassifyError(err)
	if action != ActionCompress {
		t.Errorf("expected compress for context length, got %s", action)
	}
}

func TestClassifyError_InvalidKey(t *testing.T) {
	err := errors.New("authentication failed: invalid api key (401)")
	action := ClassifyError(err)
	if action != ActionAbandon {
		t.Errorf("expected abandon for invalid key, got %s", action)
	}
}

func TestClassifyError_Timeout(t *testing.T) {
	err := errors.New("connection timeout after 30s")
	action := ClassifyError(err)
	if action != ActionRetry {
		t.Errorf("expected retry for timeout, got %s", action)
	}
}

func TestClassifyError_Unknown(t *testing.T) {
	err := errors.New("some unknown error occurred")
	action := ClassifyError(err)
	if action != ActionRetry {
		t.Errorf("expected retry for unknown error, got %s", action)
	}
}

func TestClassifyError_Nil(t *testing.T) {
	action := ClassifyError(nil)
	if action != ActionRetry {
		t.Errorf("expected retry for nil error, got %s", action)
	}
}
