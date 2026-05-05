// Package sprite generates per-worker PixelLab AI sprite sheets for the
// pixel office simulation.
//
// Pipeline (one Generator.GenerateForWorker call):
//
//  1. Build a description prompt from the worker's profile
//     (skill_profile / gender / personality flavor) — see prompt.go.
//  2. PixelLab Pixflux: generate the south-facing 32×32 base view.
//  3. PixelLab Rotate ×3: derive east / north / west views from the base.
//  4. Composer assembles the four 32×32 views into a 768×32 sprite sheet
//     (24-col × 1-row layout matching the frontend's existing FRAME_SIZE
//     constants), repeating each view six times to fill the 6-frame
//     walk-cycle slot. Animation frames-per-direction is left as a
//     follow-up — workers turn correctly today; their legs swing in PR 4.
//  5. Sheet is written atomically to <cacheDir>/<worker_id>/walking.png
//     and that absolute path is returned.
//
// Failure modes:
//
//  - PixelLab errors propagate as-is so callers can surface 401 / 429 /
//    quota issues to the operator.
//  - Decode / compose errors wrap the underlying error with an
//    indication of which step failed.
//  - The generator never falls back to a placeholder: if generation
//    fails, the worker keeps its existing layered appearance.
//
// Concurrency: a single Generator is safe for concurrent calls;
// per-worker output paths don't collide and disk writes are atomic.
package sprite

import (
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"os"
	"path/filepath"

	"github.com/hanfourmini/aisupervisor/internal/pixellab"
)

// FrameSize is the per-frame pixel dimension. Mirrors the frontend's
// FRAME_SIZE in sprites.js.
const FrameSize = 32

// FramesPerDirection is how many walk-cycle frames each direction
// gets in the assembled sheet. Mirrors FRAMES_PER_DIR in sprites.js.
const FramesPerDirection = 6

// Direction is one of the four cardinal directions used by the office
// renderer. Order is south → west → east → north because that's the
// row order the frontend's DIR map uses.
type Direction string

const (
	DirSouth Direction = "south"
	DirWest  Direction = "west"
	DirEast  Direction = "east"
	DirNorth Direction = "north"
)

// AllDirections is the canonical iteration order.
var AllDirections = []Direction{DirSouth, DirWest, DirEast, DirNorth}

// pixelLabClient is the minimal subset of *pixellab.Client we use.
// Defining it here keeps the generator unit-testable with a small fake
// (see generator_test.go) without dragging the full HTTP surface into
// the test setup.
type pixelLabClient interface {
	GenerateImagePixflux(ctx context.Context, req pixellab.PixfluxRequest) (*pixellab.PixfluxResponse, error)
	Rotate(ctx context.Context, req pixellab.RotateRequest) (*pixellab.RotateResponse, error)
}

// Generator orchestrates a single worker's sprite-sheet generation.
type Generator struct {
	client   pixelLabClient
	cacheDir string
}

// NewGenerator returns a Generator. cacheDir is the root under which
// per-worker subdirectories are created; the typical caller is
// company.Manager which passes its dataDir + "sprites".
func NewGenerator(client pixelLabClient, cacheDir string) *Generator {
	return &Generator{client: client, cacheDir: cacheDir}
}

// WorkerProfile is the minimal info the prompt builder needs. The
// generator does not depend on the worker package directly so tests
// don't need to construct full Worker objects.
type WorkerProfile struct {
	ID           string
	Name         string
	SkillProfile string // "coder" / "devops" / "designer" / ...
	Gender       string // "male" / "female" / "neutral" / ""
	Personality  string // optional flavor text from personality store
}

// GenerateForWorker runs the full pipeline and returns the absolute
// path to the saved sprite sheet PNG. Callers persist the path on
// WorkerAppearance.SpriteSheetPath.
func (g *Generator) GenerateForWorker(ctx context.Context, p WorkerProfile) (string, error) {
	if p.ID == "" {
		return "", fmt.Errorf("sprite: worker ID required")
	}

	prompt := BuildPrompt(p)

	// 1. Generate the south-facing base view.
	baseResp, err := g.client.GenerateImagePixflux(ctx, pixellab.PixfluxRequest{
		Description:  prompt,
		ImageSize:    pixellab.ImageSize{Width: FrameSize, Height: FrameSize},
		NoBackground: true,
		View:         "low top-down",
		Direction:    string(DirSouth),
	})
	if err != nil {
		return "", fmt.Errorf("sprite: generate base view: %w", err)
	}
	views := map[Direction][]byte{}
	views[DirSouth], err = baseResp.Image.PNGBytes()
	if err != nil {
		return "", fmt.Errorf("sprite: decode base view PNG: %w", err)
	}

	// 2. Rotate to the three remaining cardinal directions. We pass the
	// south view as FromImage every time (rather than chaining rotations)
	// to keep style drift minimal — each rotation samples once from the
	// canonical reference instead of compounding rotation artefacts.
	baseB64 := baseResp.Image
	for _, dir := range []Direction{DirWest, DirEast, DirNorth} {
		rot, rerr := g.client.Rotate(ctx, pixellab.RotateRequest{
			ImageSize:     pixellab.ImageSize{Width: FrameSize, Height: FrameSize},
			FromImage:     &baseB64,
			FromDirection: string(DirSouth),
			ToDirection:   string(dir),
		})
		if rerr != nil {
			return "", fmt.Errorf("sprite: rotate %s→%s: %w", DirSouth, dir, rerr)
		}
		decoded, derr := rot.Image.PNGBytes()
		if derr != nil {
			return "", fmt.Errorf("sprite: decode %s view PNG: %w", dir, derr)
		}
		views[dir] = decoded
	}

	// 3. Decode each PNG to image.Image so the composer can blit it.
	frames := make(map[Direction]image.Image, len(AllDirections))
	for _, dir := range AllDirections {
		img, decodeErr := decodePNG(views[dir])
		if decodeErr != nil {
			return "", fmt.Errorf("sprite: decode %s frame: %w", dir, decodeErr)
		}
		frames[dir] = img
	}

	// 4. Compose the 768×32 sheet (4 dirs × 6 frames × 32 px wide).
	sheet, err := ComposeSheet(frames)
	if err != nil {
		return "", fmt.Errorf("sprite: compose sheet: %w", err)
	}

	// 5. Write the sheet atomically.
	outPath := g.OutputPath(p.ID)
	if werr := writeAtomic(outPath, sheet); werr != nil {
		return "", fmt.Errorf("sprite: write %s: %w", outPath, werr)
	}
	return outPath, nil
}

// OutputPath returns the absolute path the sheet for workerID is (or
// would be) written to. Useful for callers that want to know the path
// without re-running generation.
func (g *Generator) OutputPath(workerID string) string {
	return filepath.Join(g.cacheDir, workerID, "walking.png")
}

// writeAtomic writes data to path via a temp-file-then-rename. Ensures
// readers (the wails frontend) never see a half-written PNG.
func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// encodeBase64 is a tiny helper used only by tests that need to build
// fixture pixellab.Base64Image values. Keeping it here (vs. inside
// the test file) lets the docstring above reference real production
// types unambiguously.
func encodeBase64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }
