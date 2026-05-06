package pixellab

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// =============================================================
// Construction
// =============================================================

func TestNewClient_RejectsEmptyAPIKey(t *testing.T) {
	if _, err := NewClient("", ""); err == nil {
		t.Fatal("expected error for empty apiKey")
	}
}

func TestNewClient_DefaultBaseURL(t *testing.T) {
	c, err := NewClient("", "key")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c.baseURL != DefaultBaseURL {
		t.Errorf("baseURL = %q, want %q", c.baseURL, DefaultBaseURL)
	}
}

func TestNewClient_TrimsTrailingSlash(t *testing.T) {
	c, _ := NewClient("https://example.com/v1/", "key")
	if c.baseURL != "https://example.com/v1" {
		t.Errorf("baseURL = %q, want trimmed", c.baseURL)
	}
}

// =============================================================
// Auth header propagation
// =============================================================

func TestClient_SendsBearerAuthHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"type":"usd","usd":12.34}`))
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL, "secret-token")
	c.SetHTTPClient(srv.Client())

	if _, err := c.Balance(context.Background()); err != nil {
		t.Fatalf("Balance: %v", err)
	}
	if gotAuth != "Bearer secret-token" {
		t.Errorf("Authorization header = %q, want %q", gotAuth, "Bearer secret-token")
	}
}

// =============================================================
// /balance
// =============================================================

func TestClient_Balance(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/balance" || r.Method != http.MethodGet {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"type":"usd","usd":42.5}`))
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL, "k")
	c.SetHTTPClient(srv.Client())

	got, err := c.Balance(context.Background())
	if err != nil {
		t.Fatalf("Balance: %v", err)
	}
	if got.USD != 42.5 || got.Type != "usd" {
		t.Errorf("Balance() = %+v, want USD=42.5 Type=usd", got)
	}
}

// =============================================================
// /generate-image-pixflux
// =============================================================

func TestClient_GenerateImagePixflux(t *testing.T) {
	pngBytes := []byte("fake-png-bytes")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/generate-image-pixflux" || r.Method != http.MethodPost {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		var body PixfluxRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.Description != "a pixel art devops engineer in hoodie" {
			t.Errorf("description = %q", body.Description)
		}
		if body.ImageSize.Width != 32 || body.ImageSize.Height != 32 {
			t.Errorf("image_size = %+v", body.ImageSize)
		}
		resp := PixfluxResponse{
			Image: Base64Image{Type: "base64", Base64: base64.StdEncoding.EncodeToString(pngBytes)},
			Usage: Usage{Type: "usd", USD: 0.05},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL, "k")
	c.SetHTTPClient(srv.Client())

	got, err := c.GenerateImagePixflux(context.Background(), PixfluxRequest{
		Description: "a pixel art devops engineer in hoodie",
		ImageSize:   ImageSize{Width: 32, Height: 32},
	})
	if err != nil {
		t.Fatalf("GenerateImagePixflux: %v", err)
	}
	decoded, err := got.Image.PNGBytes()
	if err != nil {
		t.Fatalf("PNGBytes: %v", err)
	}
	if string(decoded) != string(pngBytes) {
		t.Errorf("decoded = %q, want %q", decoded, pngBytes)
	}
	if got.Usage.USD != 0.05 {
		t.Errorf("usage = %v, want 0.05", got.Usage.USD)
	}
}

// =============================================================
// /rotate
// =============================================================

func TestClient_Rotate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rotate" {
			t.Errorf("path = %s", r.URL.Path)
		}
		var body RotateRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.FromDirection != "south" || body.ToDirection != "east" {
			t.Errorf("dirs = %s -> %s", body.FromDirection, body.ToDirection)
		}
		resp := RotateResponse{
			Image: Base64Image{Type: "base64", Base64: base64.StdEncoding.EncodeToString([]byte("rotated"))},
			Usage: Usage{Type: "usd", USD: 0.02},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL, "k")
	c.SetHTTPClient(srv.Client())

	got, err := c.Rotate(context.Background(), RotateRequest{
		ImageSize:     ImageSize{Width: 32, Height: 32},
		FromImage:     &Base64Image{Type: "base64", Base64: "aGVsbG8="},
		FromDirection: "south",
		ToDirection:   "east",
	})
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if got.Image.Base64 == "" {
		t.Error("expected base64 payload")
	}
}

// =============================================================
// /animate-with-skeleton
// =============================================================

func TestClient_AnimateWithSkeleton(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/animate-with-skeleton" {
			t.Errorf("path = %s", r.URL.Path)
		}
		var body AnimateWithSkeletonRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		if len(body.SkeletonKeypoints) != 6 {
			t.Errorf("skeleton_keypoints = %d, want 6", len(body.SkeletonKeypoints))
		}
		resp := AnimateWithSkeletonResponse{
			Images: make([]Base64Image, 6),
			Usage:  Usage{Type: "usd", USD: 0.30},
		}
		for i := range resp.Images {
			resp.Images[i] = Base64Image{Type: "base64", Base64: base64.StdEncoding.EncodeToString([]byte{byte('A' + i)})}
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL, "k")
	c.SetHTTPClient(srv.Client())

	frames := make([][]SkeletonPoint, 6)
	for i := range frames {
		frames[i] = []SkeletonPoint{{X: 16, Y: 16, Label: LabelNose}}
	}
	got, err := c.AnimateWithSkeleton(context.Background(), AnimateWithSkeletonRequest{
		ImageSize:         ImageSize{Width: 32, Height: 32},
		ReferenceImage:    &Base64Image{Type: "base64", Base64: "aGVsbG8="},
		SkeletonKeypoints: frames,
	})
	if err != nil {
		t.Fatalf("AnimateWithSkeleton: %v", err)
	}
	if len(got.Images) != 6 {
		t.Errorf("got %d frames, want 6", len(got.Images))
	}
}

// Wire-format assertion: decode the request body as a generic map and
// assert the live PixelLab API contract directly (field names, label
// vocabulary). Prevents silent re-introduction of e.g. `skeletons` or
// `HEAD` — the bugs PR #41 fixed against the live service.
func TestClient_AnimateWithSkeleton_WireFormat(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		_ = json.NewEncoder(w).Encode(AnimateWithSkeletonResponse{
			Images: []Base64Image{{Type: "base64", Base64: "Zg=="}},
			Usage:  Usage{Type: "usd", USD: 0.01},
		})
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL, "k")
	c.SetHTTPClient(srv.Client())

	_, _ = c.AnimateWithSkeleton(context.Background(), AnimateWithSkeletonRequest{
		ImageSize:      ImageSize{Width: 32, Height: 32},
		ReferenceImage: &Base64Image{Type: "base64", Base64: "aGVsbG8="},
		SkeletonKeypoints: [][]SkeletonPoint{
			{{X: 16, Y: 6, Label: LabelNose}},
			{{X: 16, Y: 6, Label: LabelNeck}},
			{{X: 16, Y: 6, Label: LabelLeftLeg}},
		},
		Direction: "south",
		View:      "low top-down",
	})

	// Top-level field name must be `skeleton_keypoints` (NOT `skeletons`)
	if _, ok := body["skeleton_keypoints"]; !ok {
		t.Fatalf("expected key 'skeleton_keypoints' in wire body, got: %v", keys(body))
	}
	if _, ok := body["skeletons"]; ok {
		t.Errorf("legacy 'skeletons' key reappeared in wire body — regression")
	}

	// Structure: 2D array, outer length = number of keyframes
	frames, ok := body["skeleton_keypoints"].([]any)
	if !ok {
		t.Fatalf("skeleton_keypoints is %T, want []any", body["skeleton_keypoints"])
	}
	if len(frames) != 3 {
		t.Errorf("frames = %d, want 3", len(frames))
	}

	// Each label must be from PixelLab's SkeletonLabel enum
	allowed := map[string]bool{
		LabelNose: true, LabelNeck: true, LabelLeftLeg: true,
		// (full enum verified in sprite package; this asserts what the
		// test-payload pushes through is preserved end-to-end)
	}
	for fi, frame := range frames {
		points, ok := frame.([]any)
		if !ok {
			t.Fatalf("frame %d is %T, want []any", fi, frame)
		}
		for pi, p := range points {
			pt, ok := p.(map[string]any)
			if !ok {
				t.Fatalf("frame %d point %d is %T", fi, pi, p)
			}
			label, _ := pt["label"].(string)
			if !allowed[label] {
				t.Errorf("frame %d point %d: label %q not in expected set", fi, pi, label)
			}
		}
	}
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// =============================================================
// /estimate-skeleton
// =============================================================

func TestClient_EstimateSkeleton(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := EstimateSkeletonResponse{
			Keypoints: []SkeletonPoint{
				{X: 16, Y: 8, Label: LabelNose},
				{X: 16, Y: 12, Label: LabelNeck},
			},
			Usage: Usage{Type: "usd", USD: 0.01},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL, "k")
	c.SetHTTPClient(srv.Client())

	got, err := c.EstimateSkeleton(context.Background(), EstimateSkeletonRequest{
		Image: &Base64Image{Type: "base64", Base64: "aGVsbG8="},
	})
	if err != nil {
		t.Fatalf("EstimateSkeleton: %v", err)
	}
	if len(got.Keypoints) != 2 {
		t.Errorf("keypoints = %d, want 2", len(got.Keypoints))
	}
}

// =============================================================
// Error path: non-2xx response
// =============================================================

func TestClient_APIErrorOn401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid_api_key"}`))
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL, "bad-key")
	c.SetHTTPClient(srv.Client())

	_, err := c.Balance(context.Background())
	if err == nil {
		t.Fatal("expected error for 401")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != 401 {
		t.Errorf("status = %d, want 401", apiErr.StatusCode)
	}
	if !strings.Contains(apiErr.Body, "invalid_api_key") {
		t.Errorf("body should preserve upstream error, got %q", apiErr.Body)
	}
}

// =============================================================
// Base64Image.PNGBytes()
// =============================================================

func TestBase64Image_PNGBytes(t *testing.T) {
	original := []byte("\x89PNG\r\n\x1a\nrest-of-png")
	b := Base64Image{Type: "base64", Base64: base64.StdEncoding.EncodeToString(original)}
	got, err := b.PNGBytes()
	if err != nil {
		t.Fatalf("PNGBytes: %v", err)
	}
	if string(got) != string(original) {
		t.Errorf("decoded mismatch")
	}
}

func TestBase64Image_PNGBytes_RejectsUnknownType(t *testing.T) {
	b := Base64Image{Type: "url", Base64: "ignored"}
	if _, err := b.PNGBytes(); err == nil {
		t.Error("expected error for non-base64 type")
	}
}
