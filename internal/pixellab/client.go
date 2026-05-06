// Package pixellab is a thin HTTP client for the PixelLab AI v1 API
// (https://api.pixellab.ai/v1). It wraps the five endpoints we use to
// build per-worker pixel-art sprites:
//
//   - POST /generate-image-pixflux        — text → 32×32..400×400 pixel art
//   - POST /rotate                        — base view → other view angles
//   - POST /animate-with-skeleton         — base + skeleton poses → frames
//   - POST /estimate-skeleton             — image → skeleton (precondition for animate)
//   - GET  /balance                       — credit balance / health probe
//
// Authentication: Bearer token, sourced via NewClient's apiKey arg. The
// caller (typically internal/config) is responsible for resolving the
// final value from PIXELLAB_API_KEY env var or YAML config — this
// package keeps the client construction pure and testable.
//
// Resilience: do() wraps every request in an exponential-backoff retry
// loop driven by RetryPolicy. The policy retries 429/5xx and transient
// network errors, honors the Retry-After header on 429, applies
// per-attempt jitter, and gives up after policy.MaxAttempts. Tests can
// shrink the delays via SetRetryPolicy to keep iteration fast.
//
// All endpoints return base64-encoded PNG bytes plus a usd usage cost.
// Callers receive []byte (decoded PNG) directly so they can stream
// straight to disk without re-decoding.
package pixellab

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// DefaultBaseURL is the production PixelLab AI API endpoint.
const DefaultBaseURL = "https://api.pixellab.ai/v1"

// Client is an HTTP client for the PixelLab API. Construct via NewClient.
// All methods are safe for concurrent use.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	retry      RetryPolicy
}

// RetryPolicy configures automatic retry of transient PixelLab errors.
// A request is retried when the response status is in RetryableStatuses
// or the underlying transport returned a network-level error, up to
// MaxAttempts attempts total (so MaxAttempts=4 means 1 initial try + 3
// retries).
//
// Backoff is min(BaseDelay × 2^(attempt-1), MaxDelay) + random jitter
// in [0, BaseDelay), unless the upstream provided a Retry-After header
// (HTTP date or seconds), in which case the header wins.
//
// Tests should call SetRetryPolicy to shrink delays so the suite stays
// fast; production should use DefaultRetryPolicy.
type RetryPolicy struct {
	MaxAttempts       int
	BaseDelay         time.Duration
	MaxDelay          time.Duration
	RetryableStatuses []int
}

// DefaultRetryPolicy is what NewClient installs. Four attempts (1 + 3
// retries), starting at 500ms, capped at 30s. Retries 429 (rate
// limited) and the typical transient 5xx codes. 401/403/4xx-other are
// not retried — they're terminal auth/validation errors.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts:       4,
		BaseDelay:         500 * time.Millisecond,
		MaxDelay:          30 * time.Second,
		RetryableStatuses: []int{429, 500, 502, 503, 504},
	}
}

// NewClient builds a Client. baseURL may be empty to use DefaultBaseURL.
// apiKey must be non-empty; callers should refuse to construct a Client
// when no key is configured rather than letting requests 401 at runtime.
func NewClient(baseURL, apiKey string) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("pixellab: apiKey is required")
	}
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 120 * time.Second},
		retry:      DefaultRetryPolicy(),
	}, nil
}

// SetHTTPClient installs a caller-supplied http.Client. Used by tests to
// inject httptest.Server's client (and its TLS config) — production
// code should not need to call this.
func (c *Client) SetHTTPClient(h *http.Client) { c.httpClient = h }

// SetRetryPolicy installs a custom retry policy. Tests use this to set
// sub-millisecond delays; production should leave the default in place.
func (c *Client) SetRetryPolicy(p RetryPolicy) { c.retry = p }

// ImageSize is the {width, height} pair every generation/rotation/animate
// endpoint requires. Both dimensions must satisfy the per-endpoint
// constraints (e.g. pixflux: 32..400; rotate: 16..200).
type ImageSize struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Base64Image is PixelLab's universal image envelope: a tagged sum-type
// where Type is always "base64" and Base64 holds the PNG bytes encoded
// as a base64 string.
type Base64Image struct {
	Type   string `json:"type"`
	Base64 string `json:"base64"`
}

// PNGBytes decodes the base64 payload to raw PNG bytes. Returns an
// error if the payload is malformed or the type tag is not "base64".
func (b Base64Image) PNGBytes() ([]byte, error) {
	if b.Type != "" && b.Type != "base64" {
		return nil, fmt.Errorf("pixellab: unsupported image type %q (want base64)", b.Type)
	}
	return base64.StdEncoding.DecodeString(b.Base64)
}

// Usage is the cost-tracking envelope every successful response carries.
// PixelLab bills in USD per call; clients can sum these for budgeting.
type Usage struct {
	Type string  `json:"type"` // always "usd"
	USD  float64 `json:"usd"`
}

// post sends a JSON body to /<path> with bearer auth and decodes the
// response into out. Non-2xx responses surface as APIError after retry
// is exhausted.
func (c *Client) post(ctx context.Context, path string, body, out any) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("pixellab: marshal request: %w", err)
	}
	return c.do(ctx, http.MethodPost, path, buf, out)
}

// get sends a GET to /<path> with bearer auth and decodes into out.
func (c *Client) get(ctx context.Context, path string, out any) error {
	return c.do(ctx, http.MethodGet, path, nil, out)
}

// do executes the request with retry/backoff. The body slice is reused
// (we rebuild a fresh *bytes.Reader per attempt) so the same Marshal
// result feeds every retry. Non-retryable failures (4xx other than 429,
// network errors after MaxAttempts) return immediately.
func (c *Client) do(ctx context.Context, method, path string, body []byte, out any) error {
	policy := c.retry
	if policy.MaxAttempts < 1 {
		policy.MaxAttempts = 1
	}

	var lastErr error
	lastRetryAfter := time.Duration(-1)
	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		if attempt > 1 {
			delay := backoffDelay(policy, attempt-1, lastRetryAfter)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		var bodyReader io.Reader
		if body != nil {
			bodyReader = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
		if err != nil {
			return fmt.Errorf("pixellab: build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("pixellab: %s %s: %w", method, path, err)
			if attempt == policy.MaxAttempts {
				return lastErr
			}
			lastRetryAfter = -1
			continue
		}

		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("pixellab: read response body: %w", readErr)
			if attempt == policy.MaxAttempts {
				return lastErr
			}
			lastRetryAfter = -1
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			apiErr := &APIError{
				StatusCode: resp.StatusCode,
				Method:     method,
				Path:       path,
				Body:       string(respBody),
			}
			if isRetryable(policy.RetryableStatuses, resp.StatusCode) && attempt < policy.MaxAttempts {
				lastErr = apiErr
				lastRetryAfter = parseRetryAfter(resp.Header.Get("Retry-After"))
				continue
			}
			return apiErr
		}

		if out == nil || len(respBody) == 0 {
			return nil
		}
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("pixellab: decode response from %s %s: %w (body: %s)",
				method, path, err, truncate(string(respBody), 512))
		}
		return nil
	}
	// MaxAttempts is at least 1 due to the clamp above, so the loop
	// always runs once — this branch is unreachable but kept for the
	// type checker.
	return lastErr
}

// backoffDelay computes the wait before retry attempt+1. Honors a
// Retry-After hint from the previous response when present (>=0);
// otherwise (-1) uses min(BaseDelay × 2^(retryIdx-1), MaxDelay) + jitter.
func backoffDelay(p RetryPolicy, retryIdx int, retryAfter time.Duration) time.Duration {
	if retryAfter >= 0 {
		if retryAfter > p.MaxDelay {
			return p.MaxDelay
		}
		return retryAfter
	}
	base := p.BaseDelay
	if base <= 0 {
		base = 500 * time.Millisecond
	}
	max := p.MaxDelay
	if max <= 0 {
		max = 30 * time.Second
	}
	exp := base << (retryIdx - 1)
	if exp <= 0 || exp > max {
		exp = max
	}
	jitter := time.Duration(rand.Int63n(int64(base)))
	return exp + jitter
}

// parseRetryAfter handles both formats RFC 7231 §7.1.3 allows: an HTTP
// date (treated as absolute) or a delta-seconds integer. Returns -1
// when the header is absent or malformed (so the caller knows to fall
// back to its own backoff schedule); returns 0 when the header
// explicitly asks for an immediate retry (Retry-After: 0).
func parseRetryAfter(h string) time.Duration {
	h = strings.TrimSpace(h)
	if h == "" {
		return -1
	}
	if secs, err := strconv.Atoi(h); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(h); err == nil {
		d := time.Until(t)
		if d < 0 {
			return 0
		}
		return d
	}
	return -1
}

func isRetryable(codes []int, status int) bool {
	for _, c := range codes {
		if c == status {
			return true
		}
	}
	return false
}

// APIError carries non-2xx responses from PixelLab. The Body field
// preserves the upstream error JSON / HTML for diagnostics; callers can
// type-assert to inspect StatusCode (e.g. distinguish 401 quota-exceeded
// vs. 429 rate-limited).
type APIError struct {
	StatusCode int
	Method     string
	Path       string
	Body       string
}

// Error implements the error interface.
func (e *APIError) Error() string {
	return fmt.Sprintf("pixellab: %s %s -> %d: %s",
		e.Method, e.Path, e.StatusCode, truncate(e.Body, 256))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
