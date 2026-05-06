package pixellab

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

// fastRetryPolicy is what every test in this file installs — sub-ms
// delays so the suite stays fast. Production should never use anything
// this aggressive.
func fastRetryPolicy(maxAttempts int) RetryPolicy {
	return RetryPolicy{
		MaxAttempts:       maxAttempts,
		BaseDelay:         time.Millisecond,
		MaxDelay:          5 * time.Millisecond,
		RetryableStatuses: []int{429, 500, 502, 503, 504},
	}
}

func TestRetry_Succeeds_NoRetry(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		_, _ = w.Write([]byte(`{"type":"usd","usd":1.0}`))
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL, "k")
	c.SetHTTPClient(srv.Client())
	c.SetRetryPolicy(fastRetryPolicy(4))

	if _, err := c.Balance(context.Background()); err != nil {
		t.Fatalf("Balance: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Errorf("hits = %d, want 1 (no retry on 200)", got)
	}
}

func TestRetry_RetriesOn429ThenSucceeds(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&hits, 1)
		if n == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"detail":"rate limited"}`))
			return
		}
		_, _ = w.Write([]byte(`{"type":"usd","usd":1.0}`))
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL, "k")
	c.SetHTTPClient(srv.Client())
	c.SetRetryPolicy(fastRetryPolicy(4))

	if _, err := c.Balance(context.Background()); err != nil {
		t.Fatalf("Balance: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 2 {
		t.Errorf("hits = %d, want 2 (1 fail + 1 success)", got)
	}
}

func TestRetry_RetriesOn503Sequence(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&hits, 1)
		if n < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"type":"usd","usd":1.0}`))
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL, "k")
	c.SetHTTPClient(srv.Client())
	c.SetRetryPolicy(fastRetryPolicy(4))

	if _, err := c.Balance(context.Background()); err != nil {
		t.Fatalf("Balance: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 3 {
		t.Errorf("hits = %d, want 3 (2 fails + 1 success)", got)
	}
}

func TestRetry_GivesUpAfterMaxAttempts(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"detail":"persistent rate limit"}`))
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL, "k")
	c.SetHTTPClient(srv.Client())
	c.SetRetryPolicy(fastRetryPolicy(3))

	_, err := c.Balance(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != 429 {
		t.Errorf("status = %d, want 429", apiErr.StatusCode)
	}
	if got := atomic.LoadInt32(&hits); got != 3 {
		t.Errorf("hits = %d, want 3 (MaxAttempts)", got)
	}
}

func TestRetry_DoesNotRetryOn401(t *testing.T) {
	// 401 is NOT in RetryableStatuses — auth errors are terminal. This
	// test guards against accidentally widening the retryable set: a
	// busted API key should fail fast, not burn 4 attempts.
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"detail":"invalid api key"}`))
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL, "k")
	c.SetHTTPClient(srv.Client())
	c.SetRetryPolicy(fastRetryPolicy(4))

	_, err := c.Balance(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Errorf("hits = %d, want 1 (no retry on 401)", got)
	}
}

func TestRetry_DoesNotRetryOn400(t *testing.T) {
	// 400 validation errors (e.g. wrong wire format) should also fail
	// fast — retrying won't make a malformed request valid.
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"detail":"bad request"}`))
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL, "k")
	c.SetHTTPClient(srv.Client())
	c.SetRetryPolicy(fastRetryPolicy(4))

	_, err := c.Balance(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Errorf("hits = %d, want 1 (no retry on 400)", got)
	}
}

func TestRetry_HonorsRetryAfterSeconds(t *testing.T) {
	// When the upstream sets Retry-After: <seconds>, we should sleep
	// for ~that long instead of computing our own backoff. We test by
	// asking for 0 seconds (immediate) so the test stays fast — the
	// header should still be honored as the chosen delay even if it's
	// shorter than our base.
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&hits, 1)
		if n == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{"type":"usd","usd":1.0}`))
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL, "k")
	c.SetHTTPClient(srv.Client())
	c.SetRetryPolicy(RetryPolicy{
		MaxAttempts:       3,
		BaseDelay:         100 * time.Millisecond, // would be slow without Retry-After
		MaxDelay:          time.Second,
		RetryableStatuses: []int{429},
	})

	start := time.Now()
	if _, err := c.Balance(context.Background()); err != nil {
		t.Fatalf("Balance: %v", err)
	}
	elapsed := time.Since(start)
	// Honoring "Retry-After: 0" → ~immediate retry; computing our own
	// backoff would have slept ~100ms+.
	if elapsed > 50*time.Millisecond {
		t.Errorf("elapsed = %v, want <50ms (Retry-After: 0 should be ~instant)", elapsed)
	}
	if got := atomic.LoadInt32(&hits); got != 2 {
		t.Errorf("hits = %d, want 2", got)
	}
}

func TestRetry_HonorsContextCancellation(t *testing.T) {
	// Long Retry-After + cancelled context → return ctx.Err() promptly,
	// don't sleep through the backoff window.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", strconv.Itoa(60)) // 60s
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL, "k")
	c.SetHTTPClient(srv.Client())
	c.SetRetryPolicy(RetryPolicy{
		MaxAttempts:       3,
		BaseDelay:         time.Millisecond,
		MaxDelay:          time.Hour,
		RetryableStatuses: []int{429},
	})

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := c.Balance(ctx)
	elapsed := time.Since(start)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if elapsed > 200*time.Millisecond {
		t.Errorf("elapsed = %v, want <200ms (cancel should preempt 60s backoff)", elapsed)
	}
}

func TestRetry_PreservesRequestBodyAcrossRetries(t *testing.T) {
	// The body []byte gets a fresh bytes.Reader each attempt. If we
	// regressed and reused a consumed reader, the second attempt would
	// receive an empty body and the assertion below would fail.
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 512)
		n, _ := r.Body.Read(buf)
		bodies = append(bodies, string(buf[:n]))
		if len(bodies) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		// Pixflux response shape — minimal valid payload
		_, _ = w.Write([]byte(`{"image":{"type":"base64","base64":"aGk="},"usage":{"type":"usd","usd":0.05}}`))
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL, "k")
	c.SetHTTPClient(srv.Client())
	c.SetRetryPolicy(fastRetryPolicy(3))

	_, err := c.GenerateImagePixflux(context.Background(), PixfluxRequest{
		Description: "test",
		ImageSize:   ImageSize{Width: 32, Height: 32},
	})
	if err != nil {
		t.Fatalf("GenerateImagePixflux: %v", err)
	}
	if len(bodies) != 2 {
		t.Fatalf("got %d hits, want 2", len(bodies))
	}
	if bodies[0] != bodies[1] {
		t.Errorf("retry sent different body:\n  attempt 1: %q\n  attempt 2: %q", bodies[0], bodies[1])
	}
	if bodies[1] == "" {
		t.Error("retry sent empty body — bytes.Reader was consumed and not rebuilt")
	}
}

func TestParseRetryAfter(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"", -1},                  // absent → fall back to default backoff
		{"   ", -1},               // whitespace only → absent
		{"5", 5 * time.Second},    // delta-seconds
		{"0", 0},                  // zero seconds → retry immediately (NOT absent)
		{"abc", -1},               // malformed → absent
	}
	for _, tc := range cases {
		got := parseRetryAfter(tc.in)
		if got != tc.want {
			t.Errorf("parseRetryAfter(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
