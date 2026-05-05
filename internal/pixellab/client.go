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
	"net/http"
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
	}, nil
}

// SetHTTPClient installs a caller-supplied http.Client. Used by tests to
// inject httptest.Server's client (and its TLS config) — production
// code should not need to call this.
func (c *Client) SetHTTPClient(h *http.Client) { c.httpClient = h }

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
// response into out. Non-2xx responses are returned as APIError.
func (c *Client) post(ctx context.Context, path string, body, out any) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("pixellab: marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(buf))
	if err != nil {
		return fmt.Errorf("pixellab: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	return c.do(req, out)
}

// get sends a GET to /<path> with bearer auth and decodes into out.
func (c *Client) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("pixellab: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	return c.do(req, out)
}

// do executes req and decodes the JSON response. Non-2xx maps to APIError.
func (c *Client) do(req *http.Request, out any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("pixellab: %s %s: %w", req.Method, req.URL.Path, err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("pixellab: read response body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &APIError{
			StatusCode: resp.StatusCode,
			Method:     req.Method,
			Path:       req.URL.Path,
			Body:       string(respBody),
		}
	}
	if out == nil || len(respBody) == 0 {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("pixellab: decode response from %s %s: %w (body: %s)",
			req.Method, req.URL.Path, err, truncate(string(respBody), 512))
	}
	return nil
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
