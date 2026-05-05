package sprite

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hanfourmini/aisupervisor/internal/pixellab"
)

// fakeColorPNG returns a 32×32 PNG filled with a single colour. Tests
// compose four of these (one per direction) with distinct colours so
// the assembled sprite sheet's per-column pixels can be inspected to
// confirm each direction landed in the correct slot.
func fakeColorPNG(t *testing.T, c color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, FrameSize, FrameSize))
	for y := 0; y < FrameSize; y++ {
		for x := 0; x < FrameSize; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode fixture: %v", err)
	}
	return buf.Bytes()
}

// fakePixelLab is a minimal stand-in for *pixellab.Client that the
// generator can drive. It returns one of four colour PNGs per
// direction so the composer's slot ordering is testable downstream.
type fakePixelLab struct {
	pixfluxCalls int
	rotateCalls  int
	pixfluxErr   error
	rotateErr    error
	// Colour returned by GenerateImagePixflux (the "south" base view).
	southPNG []byte
	// Colour returned for each rotation; key is ToDirection.
	rotPNGByDir map[string][]byte
}

func (f *fakePixelLab) GenerateImagePixflux(ctx context.Context, req pixellab.PixfluxRequest) (*pixellab.PixfluxResponse, error) {
	f.pixfluxCalls++
	if f.pixfluxErr != nil {
		return nil, f.pixfluxErr
	}
	return &pixellab.PixfluxResponse{
		Image: pixellab.Base64Image{Type: "base64", Base64: encodeBase64(f.southPNG)},
		Usage: pixellab.Usage{Type: "usd", USD: 0.05},
	}, nil
}

func (f *fakePixelLab) Rotate(ctx context.Context, req pixellab.RotateRequest) (*pixellab.RotateResponse, error) {
	f.rotateCalls++
	if f.rotateErr != nil {
		return nil, f.rotateErr
	}
	png, ok := f.rotPNGByDir[req.ToDirection]
	if !ok {
		png = f.southPNG // fallback so tests don't crash if mapping incomplete
	}
	return &pixellab.RotateResponse{
		Image: pixellab.Base64Image{Type: "base64", Base64: encodeBase64(png)},
		Usage: pixellab.Usage{Type: "usd", USD: 0.02},
	}, nil
}

// =============================================================
// BuildPrompt
// =============================================================

func TestBuildPrompt_IncludesProfileAndStyleAnchors(t *testing.T) {
	got := BuildPrompt(WorkerProfile{
		ID:           "w1",
		Name:         "Ethan",
		SkillProfile: "devops",
		Gender:       "male",
		Personality:  "calm and methodical",
	})
	wantSubstrings := []string{
		"32x32 pixel art top-down character",
		"devops engineer",
		"male character",
		"calm and methodical",
		"transparent background",
		"facing south",
	}
	for _, s := range wantSubstrings {
		if !strings.Contains(got, s) {
			t.Errorf("prompt missing %q\nfull: %s", s, got)
		}
	}
}

func TestBuildPrompt_OmitsBlankOptionals(t *testing.T) {
	got := BuildPrompt(WorkerProfile{ID: "w1", SkillProfile: "coder"})
	// gender unset → no "male/female character" phrase
	if strings.Contains(got, "male character") || strings.Contains(got, "female character") {
		t.Errorf("unset gender should not produce a gender phrase: %s", got)
	}
	// personality unset → no "(personality:" phrase
	if strings.Contains(got, "(personality:") {
		t.Errorf("unset personality should not appear: %s", got)
	}
	// no double-comma artifacts
	if strings.Contains(got, ",,") || strings.Contains(got, ", ,") {
		t.Errorf("found empty join slot: %s", got)
	}
}

func TestBuildPrompt_UnknownProfileFallsBackGracefully(t *testing.T) {
	got := BuildPrompt(WorkerProfile{ID: "w1", SkillProfile: "qa-lead"})
	if !strings.Contains(got, "qa-lead professional outfit") {
		t.Errorf("unknown profile should fall through to generic outfit phrase: %s", got)
	}
}

// =============================================================
// ComposeSheet
// =============================================================

func TestComposeSheet_LayoutAndDimensions(t *testing.T) {
	mk := func(c color.RGBA) image.Image {
		img := image.NewRGBA(image.Rect(0, 0, FrameSize, FrameSize))
		for y := 0; y < FrameSize; y++ {
			for x := 0; x < FrameSize; x++ {
				img.Set(x, y, c)
			}
		}
		return img
	}
	frames := map[Direction]image.Image{
		DirSouth: mk(color.RGBA{255, 0, 0, 255}),   // red
		DirWest:  mk(color.RGBA{0, 255, 0, 255}),   // green
		DirEast:  mk(color.RGBA{0, 0, 255, 255}),   // blue
		DirNorth: mk(color.RGBA{255, 255, 0, 255}), // yellow
	}

	sheetBytes, err := ComposeSheet(frames)
	if err != nil {
		t.Fatalf("ComposeSheet: %v", err)
	}
	sheet, err := png.Decode(bytes.NewReader(sheetBytes))
	if err != nil {
		t.Fatalf("decode sheet: %v", err)
	}
	wantW := FrameSize * FramesPerDirection * 4
	if sheet.Bounds().Dx() != wantW || sheet.Bounds().Dy() != FrameSize {
		t.Fatalf("sheet bounds = %v, want %dx%d", sheet.Bounds(), wantW, FrameSize)
	}

	// Pick one pixel from each direction's slot and verify the colour.
	cases := []struct {
		dir   string
		colCenter int
		want  color.RGBA
	}{
		{"south", 16, color.RGBA{255, 0, 0, 255}},
		{"west", 16 + FrameSize*FramesPerDirection*1, color.RGBA{0, 255, 0, 255}},
		{"east", 16 + FrameSize*FramesPerDirection*2, color.RGBA{0, 0, 255, 255}},
		{"north", 16 + FrameSize*FramesPerDirection*3, color.RGBA{255, 255, 0, 255}},
	}
	for _, tc := range cases {
		got := sheet.At(tc.colCenter, FrameSize/2)
		r, g, b, a := got.RGBA()
		want := tc.want
		if uint8(r>>8) != want.R || uint8(g>>8) != want.G || uint8(b>>8) != want.B || uint8(a>>8) != want.A {
			t.Errorf("%s slot pixel = (%d,%d,%d,%d), want (%d,%d,%d,%d)",
				tc.dir, r>>8, g>>8, b>>8, a>>8, want.R, want.G, want.B, want.A)
		}
	}
}

func TestComposeSheet_RejectsMissingDirection(t *testing.T) {
	mk := func() image.Image {
		return image.NewRGBA(image.Rect(0, 0, FrameSize, FrameSize))
	}
	_, err := ComposeSheet(map[Direction]image.Image{
		DirSouth: mk(),
		// DirWest missing
		DirEast:  mk(),
		DirNorth: mk(),
	})
	if err == nil {
		t.Error("expected error for missing direction")
	}
}

func TestComposeSheet_RejectsWrongFrameSize(t *testing.T) {
	mk := func(w, h int) image.Image {
		return image.NewRGBA(image.Rect(0, 0, w, h))
	}
	_, err := ComposeSheet(map[Direction]image.Image{
		DirSouth: mk(48, 48), // wrong size
		DirWest:  mk(FrameSize, FrameSize),
		DirEast:  mk(FrameSize, FrameSize),
		DirNorth: mk(FrameSize, FrameSize),
	})
	if err == nil {
		t.Error("expected error for non-32x32 frame")
	}
}

// =============================================================
// Generator end-to-end
// =============================================================

func TestGenerator_GenerateForWorker_HappyPath(t *testing.T) {
	red := fakeColorPNG(t, color.RGBA{255, 0, 0, 255})
	green := fakeColorPNG(t, color.RGBA{0, 255, 0, 255})
	blue := fakeColorPNG(t, color.RGBA{0, 0, 255, 255})
	yellow := fakeColorPNG(t, color.RGBA{255, 255, 0, 255})

	fake := &fakePixelLab{
		southPNG: red,
		rotPNGByDir: map[string][]byte{
			"west":  green,
			"east":  blue,
			"north": yellow,
		},
	}
	cacheDir := t.TempDir()
	g := NewGenerator(fake, cacheDir)

	got, err := g.GenerateForWorker(context.Background(), WorkerProfile{
		ID:           "w-test-1",
		Name:         "Ethan",
		SkillProfile: "devops",
		Gender:       "male",
	})
	if err != nil {
		t.Fatalf("GenerateForWorker: %v", err)
	}

	wantPath := filepath.Join(cacheDir, "w-test-1", "walking.png")
	if got != wantPath {
		t.Errorf("path = %q, want %q", got, wantPath)
	}
	// File must exist and be a valid PNG of the expected dimensions.
	data, err := os.ReadFile(got)
	if err != nil {
		t.Fatalf("read sprite: %v", err)
	}
	sheet, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode sprite: %v", err)
	}
	if sheet.Bounds().Dx() != 768 || sheet.Bounds().Dy() != 32 {
		t.Errorf("sprite sheet dims = %v, want 768x32", sheet.Bounds())
	}
	// API call counts: 1 pixflux + 3 rotations.
	if fake.pixfluxCalls != 1 {
		t.Errorf("pixfluxCalls = %d, want 1", fake.pixfluxCalls)
	}
	if fake.rotateCalls != 3 {
		t.Errorf("rotateCalls = %d, want 3", fake.rotateCalls)
	}
}

func TestGenerator_GenerateForWorker_PropagatesPixfluxError(t *testing.T) {
	fake := &fakePixelLab{
		pixfluxErr: &pixellab.APIError{StatusCode: 401, Method: "POST", Path: "/generate-image-pixflux", Body: "invalid_api_key"},
		southPNG:   fakeColorPNG(t, color.RGBA{0, 0, 0, 255}),
	}
	g := NewGenerator(fake, t.TempDir())
	_, err := g.GenerateForWorker(context.Background(), WorkerProfile{ID: "w1", SkillProfile: "coder"})
	if err == nil {
		t.Fatal("expected error to propagate from pixflux failure")
	}
	if !strings.Contains(err.Error(), "generate base view") {
		t.Errorf("error should identify the failing step, got: %v", err)
	}
}

func TestGenerator_GenerateForWorker_PropagatesRotateError(t *testing.T) {
	fake := &fakePixelLab{
		southPNG:  fakeColorPNG(t, color.RGBA{0, 0, 0, 255}),
		rotateErr: &pixellab.APIError{StatusCode: 429, Method: "POST", Path: "/rotate", Body: "rate_limited"},
	}
	g := NewGenerator(fake, t.TempDir())
	_, err := g.GenerateForWorker(context.Background(), WorkerProfile{ID: "w1", SkillProfile: "coder"})
	if err == nil {
		t.Fatal("expected error to propagate from rotate failure")
	}
	if !strings.Contains(err.Error(), "rotate") {
		t.Errorf("error should identify rotate, got: %v", err)
	}
}

func TestGenerator_RejectsEmptyWorkerID(t *testing.T) {
	fake := &fakePixelLab{southPNG: fakeColorPNG(t, color.RGBA{0, 0, 0, 255})}
	g := NewGenerator(fake, t.TempDir())
	_, err := g.GenerateForWorker(context.Background(), WorkerProfile{SkillProfile: "coder"})
	if err == nil {
		t.Error("expected error for empty worker ID")
	}
}

func TestGenerator_OutputPath(t *testing.T) {
	g := NewGenerator(nil, "/tmp/sprites")
	got := g.OutputPath("w-abc-1")
	want := "/tmp/sprites/w-abc-1/walking.png"
	if got != want {
		t.Errorf("OutputPath = %q, want %q", got, want)
	}
}

func TestGenerator_AtomicWrite_NoTempLeftover(t *testing.T) {
	fake := &fakePixelLab{
		southPNG: fakeColorPNG(t, color.RGBA{0, 0, 0, 255}),
		rotPNGByDir: map[string][]byte{
			"west":  fakeColorPNG(t, color.RGBA{0, 0, 0, 255}),
			"east":  fakeColorPNG(t, color.RGBA{0, 0, 0, 255}),
			"north": fakeColorPNG(t, color.RGBA{0, 0, 0, 255}),
		},
	}
	cacheDir := t.TempDir()
	g := NewGenerator(fake, cacheDir)

	if _, err := g.GenerateForWorker(context.Background(), WorkerProfile{ID: "w-atom", SkillProfile: "coder"}); err != nil {
		t.Fatalf("GenerateForWorker: %v", err)
	}
	// The .tmp companion file must have been renamed away.
	matches, _ := filepath.Glob(filepath.Join(cacheDir, "w-atom", "*.tmp"))
	if len(matches) != 0 {
		t.Errorf("expected no .tmp leftovers, found %v", matches)
	}
}
