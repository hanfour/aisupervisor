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
	pixfluxCalls  int
	rotateCalls   int
	animateCalls  int
	estimateCalls int
	pixfluxErr    error
	rotateErr     error
	animateErr    error
	estimateErr   error
	// Colour returned by GenerateImagePixflux (the "south" base view).
	southPNG []byte
	// Colour returned for each rotation; key is ToDirection.
	rotPNGByDir map[string][]byte
	// Per-direction 6-frame animation responses; key is Direction.
	// When nil, AnimateWithSkeleton synthesises 6 frames by tinting
	// the reference PNG with frame-index variation so frames are
	// pairwise distinct (proves the cycle isn't a static repeat).
	animFramesByDir map[string][][]byte
	// EstimateSkeleton response. When nil, returns an empty keypoint
	// list — the generator falls back to WalkCycleSkeletons() in that
	// case, matching the behaviour we want when PixelLab can't infer a
	// skeleton from the reference image.
	estimateKeypoints []pixellab.SkeletonPoint
	// Last received Pixflux request — used by tests that assert the
	// generator passes the right style controls.
	lastPixfluxReq pixellab.PixfluxRequest
}

func (f *fakePixelLab) EstimateSkeleton(ctx context.Context, req pixellab.EstimateSkeletonRequest) (*pixellab.EstimateSkeletonResponse, error) {
	f.estimateCalls++
	if f.estimateErr != nil {
		return nil, f.estimateErr
	}
	return &pixellab.EstimateSkeletonResponse{
		Keypoints: f.estimateKeypoints,
		Usage:     pixellab.Usage{Type: "usd", USD: 0.01},
	}, nil
}

func (f *fakePixelLab) GenerateImagePixflux(ctx context.Context, req pixellab.PixfluxRequest) (*pixellab.PixfluxResponse, error) {
	f.pixfluxCalls++
	f.lastPixfluxReq = req
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

func (f *fakePixelLab) AnimateWithSkeleton(ctx context.Context, req pixellab.AnimateWithSkeletonRequest) (*pixellab.AnimateWithSkeletonResponse, error) {
	f.animateCalls++
	if f.animateErr != nil {
		return nil, f.animateErr
	}
	// If the test pre-populated explicit per-direction frames, return those.
	if f.animFramesByDir != nil {
		if seq, ok := f.animFramesByDir[req.Direction]; ok {
			imgs := make([]pixellab.Base64Image, len(seq))
			for i, raw := range seq {
				imgs[i] = pixellab.Base64Image{Type: "base64", Base64: encodeBase64(raw)}
			}
			return &pixellab.AnimateWithSkeletonResponse{
				Images: imgs,
				Usage:  pixellab.Usage{Type: "usd", USD: 0.06},
			}, nil
		}
	}
	// Default: synthesise 6 frames by tinting the reference PNG with
	// per-frame variation so frames are pairwise distinct.
	if req.ReferenceImage == nil {
		return nil, &pixellab.APIError{StatusCode: 400, Method: "POST", Path: "/animate-with-skeleton", Body: "missing reference"}
	}
	refRaw, err := req.ReferenceImage.PNGBytes()
	if err != nil {
		return nil, err
	}
	refImg, err := decodePNG(refRaw)
	if err != nil {
		return nil, err
	}
	out := make([]pixellab.Base64Image, WalkCycleFrames)
	for i := 0; i < WalkCycleFrames; i++ {
		out[i] = pixellab.Base64Image{
			Type:   "base64",
			Base64: encodeBase64(tintedPNG(refImg, uint8(i*15))), // stride 15 / frame
		}
	}
	return &pixellab.AnimateWithSkeletonResponse{
		Images: out,
		Usage:  pixellab.Usage{Type: "usd", USD: 0.06},
	}, nil
}

// tintedPNG returns a copy of img with red increased by `delta`. Used
// only by the fake to manufacture frames that differ pairwise — proves
// the composer doesn't silently repeat a single frame across columns.
func tintedPNG(img image.Image, delta uint8) []byte {
	b := img.Bounds()
	out := image.NewRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.At(x, y).RGBA()
			out.Set(x, y, color.RGBA{
				R: uint8(min32(int(uint8(r>>8))+int(delta), 255)),
				G: uint8(g >> 8),
				B: uint8(bl >> 8),
				A: uint8(a >> 8),
			})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, out)
	return buf.Bytes()
}

func min32(a, b int) int {
	if a < b {
		return a
	}
	return b
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
		"full body visible head to toe",                   // framing anchor
		"standing centered",
		"classic Dragon Quest JRPG overworld hero sprite", // franchise style anchor
		"16-bit retro pixel art style",
		"chibi proportions",
		"devops engineer",
		"male character",
		"calm and methodical",
		"simple silhouette",
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

// Generator must apply the style controls (outline / detail / guidance
// scale) on every Pixflux call, not just include them in the prompt
// text. Without these, the model produced inconsistent framing and
// stylistic drift between workers — the symptom that prompted PR #47.
func TestGenerator_GenerateForWorker_AppliesStyleControls(t *testing.T) {
	fake := &fakePixelLab{
		southPNG: fakeColorPNG(t, color.RGBA{0, 0, 0, 255}),
		rotPNGByDir: map[string][]byte{
			"west":  fakeColorPNG(t, color.RGBA{0, 0, 0, 255}),
			"east":  fakeColorPNG(t, color.RGBA{0, 0, 0, 255}),
			"north": fakeColorPNG(t, color.RGBA{0, 0, 0, 255}),
		},
	}
	g := NewGenerator(fake, t.TempDir())
	if _, err := g.GenerateForWorker(context.Background(), WorkerProfile{
		ID: "w-style", SkillProfile: "coder",
	}); err != nil {
		t.Fatalf("GenerateForWorker: %v", err)
	}

	req := fake.lastPixfluxReq
	if req.OutlineMode != "single color black outline" {
		t.Errorf("OutlineMode = %q, want %q", req.OutlineMode, "single color black outline")
	}
	if req.DetailLevel != "low detail" {
		t.Errorf("DetailLevel = %q, want %q", req.DetailLevel, "low detail")
	}
	if req.TextGuidanceScale < 7 || req.TextGuidanceScale > 12 {
		t.Errorf("TextGuidanceScale = %v, want roughly 7..12 (default ≈ 4 too low for consistency)",
			req.TextGuidanceScale)
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
	repeat := func(img image.Image) []image.Image {
		out := make([]image.Image, FramesPerDirection)
		for i := range out {
			out[i] = img
		}
		return out
	}
	frames := map[Direction][]image.Image{
		DirSouth: repeat(mk(color.RGBA{255, 0, 0, 255})),   // red
		DirWest:  repeat(mk(color.RGBA{0, 255, 0, 255})),   // green
		DirEast:  repeat(mk(color.RGBA{0, 0, 255, 255})),   // blue
		DirNorth: repeat(mk(color.RGBA{255, 255, 0, 255})), // yellow
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
	repeat := func(img image.Image) []image.Image {
		out := make([]image.Image, FramesPerDirection)
		for i := range out {
			out[i] = img
		}
		return out
	}
	_, err := ComposeSheet(map[Direction][]image.Image{
		DirSouth: repeat(mk()),
		// DirWest missing
		DirEast:  repeat(mk()),
		DirNorth: repeat(mk()),
	})
	if err == nil {
		t.Error("expected error for missing direction")
	}
}

func TestComposeSheet_RejectsWrongFrameSize(t *testing.T) {
	mk := func(w, h int) image.Image {
		return image.NewRGBA(image.Rect(0, 0, w, h))
	}
	repeat := func(img image.Image) []image.Image {
		out := make([]image.Image, FramesPerDirection)
		for i := range out {
			out[i] = img
		}
		return out
	}
	_, err := ComposeSheet(map[Direction][]image.Image{
		DirSouth: repeat(mk(48, 48)), // wrong size
		DirWest:  repeat(mk(FrameSize, FrameSize)),
		DirEast:  repeat(mk(FrameSize, FrameSize)),
		DirNorth: repeat(mk(FrameSize, FrameSize)),
	})
	if err == nil {
		t.Error("expected error for non-32x32 frame")
	}
}

func TestComposeSheet_RejectsWrongFrameCount(t *testing.T) {
	mk := func() image.Image {
		return image.NewRGBA(image.Rect(0, 0, FrameSize, FrameSize))
	}
	repeat := func(img image.Image, n int) []image.Image {
		out := make([]image.Image, n)
		for i := range out {
			out[i] = img
		}
		return out
	}
	_, err := ComposeSheet(map[Direction][]image.Image{
		DirSouth: repeat(mk(), 5), // wrong count — should be FramesPerDirection (6)
		DirWest:  repeat(mk(), FramesPerDirection),
		DirEast:  repeat(mk(), FramesPerDirection),
		DirNorth: repeat(mk(), FramesPerDirection),
	})
	if err == nil {
		t.Error("expected error for wrong frame count")
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
	// API call counts: 1 pixflux + 3 rotations + 1 estimate + 4 animations.
	if fake.pixfluxCalls != 1 {
		t.Errorf("pixfluxCalls = %d, want 1", fake.pixfluxCalls)
	}
	if fake.rotateCalls != 3 {
		t.Errorf("rotateCalls = %d, want 3", fake.rotateCalls)
	}
	if fake.estimateCalls != 1 {
		t.Errorf("estimateCalls = %d, want 1 (south reference only)", fake.estimateCalls)
	}
	if fake.animateCalls != 4 {
		t.Errorf("animateCalls = %d, want 4 (one per direction)", fake.animateCalls)
	}
}

// Generator should fall back to the hardcoded skeleton when
// EstimateSkeleton fails — the worker still gets a sprite, quality may
// degrade slightly. The test confirms the pipeline doesn't error out
// just because estimate failed.
func TestGenerator_GenerateForWorker_FallsBackOnEstimateFailure(t *testing.T) {
	fake := &fakePixelLab{
		southPNG: fakeColorPNG(t, color.RGBA{120, 80, 80, 255}),
		rotPNGByDir: map[string][]byte{
			"west": fakeColorPNG(t, color.RGBA{120, 80, 80, 255}),
			"east": fakeColorPNG(t, color.RGBA{120, 80, 80, 255}),
			"north": fakeColorPNG(t, color.RGBA{120, 80, 80, 255}),
		},
		estimateErr: &pixellab.APIError{StatusCode: 500, Method: "POST", Path: "/estimate-skeleton", Body: "internal error"},
	}
	g := NewGenerator(fake, t.TempDir())
	_, err := g.GenerateForWorker(context.Background(), WorkerProfile{
		ID: "w-fallback", SkillProfile: "coder",
	})
	if err != nil {
		t.Fatalf("GenerateForWorker should fall back, not propagate estimate error: %v", err)
	}
	if fake.estimateCalls != 1 {
		t.Errorf("estimateCalls = %d, want 1 (called once even though it errored)", fake.estimateCalls)
	}
	if fake.animateCalls != 4 {
		t.Errorf("animateCalls = %d, want 4 (pipeline still completes)", fake.animateCalls)
	}
}

// Generator should also fall back when EstimateSkeleton returns a
// keypoint set without the required limbs (e.g. only NOSE/NECK/EYES).
func TestGenerator_GenerateForWorker_FallsBackOnIncompleteEstimate(t *testing.T) {
	fake := &fakePixelLab{
		southPNG: fakeColorPNG(t, color.RGBA{120, 80, 80, 255}),
		rotPNGByDir: map[string][]byte{
			"west": fakeColorPNG(t, color.RGBA{120, 80, 80, 255}),
			"east": fakeColorPNG(t, color.RGBA{120, 80, 80, 255}),
			"north": fakeColorPNG(t, color.RGBA{120, 80, 80, 255}),
		},
		estimateKeypoints: []pixellab.SkeletonPoint{
			{Label: pixellab.LabelNose, X: 16, Y: 6},
			{Label: pixellab.LabelNeck, X: 16, Y: 10},
			// no shoulders/arms/legs → BuildWalkCycle returns ok=false
		},
	}
	g := NewGenerator(fake, t.TempDir())
	_, err := g.GenerateForWorker(context.Background(), WorkerProfile{
		ID: "w-incomplete-est", SkillProfile: "coder",
	})
	if err != nil {
		t.Fatalf("GenerateForWorker should fall back on incomplete estimate: %v", err)
	}
	if fake.animateCalls != 4 {
		t.Errorf("animateCalls = %d, want 4 (pipeline still completes)", fake.animateCalls)
	}
}

// Walk-cycle proof: each direction's 6 columns must contain pairwise
// distinct frames. If the composer regressed to repeating a single
// frame, this test catches it. The fake's tintedPNG synthesiser
// guarantees frame-to-frame variation regardless of skeleton choice.
func TestGenerator_WalkCycle_FramesAreDistinct(t *testing.T) {
	fake := &fakePixelLab{
		southPNG: fakeColorPNG(t, color.RGBA{120, 80, 80, 255}),
		rotPNGByDir: map[string][]byte{
			"west":  fakeColorPNG(t, color.RGBA{120, 80, 80, 255}),
			"east":  fakeColorPNG(t, color.RGBA{120, 80, 80, 255}),
			"north": fakeColorPNG(t, color.RGBA{120, 80, 80, 255}),
		},
	}
	g := NewGenerator(fake, t.TempDir())
	path, err := g.GenerateForWorker(context.Background(), WorkerProfile{
		ID: "w-cycle", SkillProfile: "coder",
	})
	if err != nil {
		t.Fatalf("GenerateForWorker: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read sprite: %v", err)
	}
	sheet, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode sprite: %v", err)
	}

	// PixelLab's animate-with-skeleton returns exactly WalkCycleFrames
	// (3) frames per direction; the generator repeats them to fill the
	// sheet's FramesPerDirection (6) columns as [k0, k1, k2, k0, k1, k2].
	// So within a direction, the *first 3 columns* must all be distinct
	// (proves we're not collapsing to static), while col[i] == col[i+3]
	// (proves we're applying the documented repeat pattern, not e.g.
	// silently emitting blank frames in the second half).
	colourAt := func(col int) uint32 {
		cx := col*FrameSize + FrameSize/2
		cy := FrameSize / 2
		r, gC, b, a := sheet.At(cx, cy).RGBA()
		return (uint32(r>>8) << 24) | (uint32(gC>>8) << 16) | (uint32(b>>8) << 8) | uint32(a>>8)
	}
	for dirIdx := 0; dirIdx < 4; dirIdx++ {
		seen := map[uint32]int{}
		for f := 0; f < WalkCycleFrames; f++ {
			col := dirIdx*FramesPerDirection + f
			key := colourAt(col)
			if prev, ok := seen[key]; ok {
				t.Errorf("dir %d: keyframes %d and %d share colour 0x%08x — walk cycle collapsed to static",
					dirIdx, prev, f, key)
				break
			}
			seen[key] = f
		}
		// Repeat invariant: col[k] == col[k + WalkCycleFrames].
		for f := 0; f < WalkCycleFrames; f++ {
			a := colourAt(dirIdx*FramesPerDirection + f)
			b := colourAt(dirIdx*FramesPerDirection + f + WalkCycleFrames)
			if a != b {
				t.Errorf("dir %d: col %d (0x%08x) != col %d (0x%08x) — repeat pattern broken",
					dirIdx, f, a, f+WalkCycleFrames, b)
			}
		}
	}
}

func TestGenerator_GenerateForWorker_PropagatesAnimateError(t *testing.T) {
	fake := &fakePixelLab{
		southPNG: fakeColorPNG(t, color.RGBA{0, 0, 0, 255}),
		rotPNGByDir: map[string][]byte{
			"west":  fakeColorPNG(t, color.RGBA{0, 0, 0, 255}),
			"east":  fakeColorPNG(t, color.RGBA{0, 0, 0, 255}),
			"north": fakeColorPNG(t, color.RGBA{0, 0, 0, 255}),
		},
		animateErr: &pixellab.APIError{StatusCode: 429, Method: "POST", Path: "/animate-with-skeleton", Body: "rate_limited"},
	}
	g := NewGenerator(fake, t.TempDir())
	_, err := g.GenerateForWorker(context.Background(), WorkerProfile{ID: "w1", SkillProfile: "coder"})
	if err == nil {
		t.Fatal("expected error to propagate from animate failure")
	}
	if !strings.Contains(err.Error(), "animate") {
		t.Errorf("error should identify animate, got: %v", err)
	}
}

func TestGenerator_GenerateForWorker_RejectsWrongFrameCountFromAPI(t *testing.T) {
	// Simulate API returning fewer frames than requested — must be
	// caught and surfaced rather than producing a malformed sheet.
	short := [][]byte{fakeColorPNG(t, color.RGBA{0, 0, 0, 255})}
	fake := &fakePixelLab{
		southPNG: fakeColorPNG(t, color.RGBA{0, 0, 0, 255}),
		rotPNGByDir: map[string][]byte{
			"west":  fakeColorPNG(t, color.RGBA{0, 0, 0, 255}),
			"east":  fakeColorPNG(t, color.RGBA{0, 0, 0, 255}),
			"north": fakeColorPNG(t, color.RGBA{0, 0, 0, 255}),
		},
		animFramesByDir: map[string][][]byte{
			"south": short, "west": short, "east": short, "north": short,
		},
	}
	g := NewGenerator(fake, t.TempDir())
	_, err := g.GenerateForWorker(context.Background(), WorkerProfile{ID: "w1", SkillProfile: "coder"})
	if err == nil {
		t.Fatal("expected error when API returns fewer frames than FramesPerDirection")
	}
	if !strings.Contains(err.Error(), "frames") {
		t.Errorf("error should mention frame count, got: %v", err)
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
