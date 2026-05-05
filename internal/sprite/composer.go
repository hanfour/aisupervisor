package sprite

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/png"
)

// ComposeSheet stitches one PNG per cardinal direction into a single
// 768×32 sprite sheet (24-col × 1-row layout, FrameSize=32, six frames
// per direction). Layout:
//
//	cols  0 .. 5   south   (frames 0..5)
//	cols  6 .. 11  west    (frames 6..11)
//	cols 12 .. 17  east    (frames 12..17)
//	cols 18 .. 23  north   (frames 18..23)
//
// Walk-cycle animation is left for a follow-up: each direction's six
// columns currently hold the same static frame six times. Once
// animate-with-skeleton is wired the composer's inner loop becomes
// `for f, frame := range frames[dir]` instead of the static repeat.
//
// Returns the encoded PNG bytes ready to write to disk.
func ComposeSheet(frames map[Direction]image.Image) ([]byte, error) {
	for _, dir := range AllDirections {
		f, ok := frames[dir]
		if !ok || f == nil {
			return nil, fmt.Errorf("compose: missing frame for direction %q", dir)
		}
		b := f.Bounds()
		if b.Dx() != FrameSize || b.Dy() != FrameSize {
			return nil, fmt.Errorf("compose: %s frame is %dx%d, want %dx%d",
				dir, b.Dx(), b.Dy(), FrameSize, FrameSize)
		}
	}

	width := FrameSize * FramesPerDirection * len(AllDirections) // 32 × 6 × 4 = 768
	sheet := image.NewRGBA(image.Rect(0, 0, width, FrameSize))

	for dirIdx, dir := range AllDirections {
		frame := frames[dir]
		for f := 0; f < FramesPerDirection; f++ {
			col := dirIdx*FramesPerDirection + f
			dst := image.Rect(col*FrameSize, 0, (col+1)*FrameSize, FrameSize)
			draw.Draw(sheet, dst, frame, frame.Bounds().Min, draw.Src)
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, sheet); err != nil {
		return nil, fmt.Errorf("compose: encode png: %w", err)
	}
	return buf.Bytes(), nil
}

// decodePNG converts raw PNG bytes to an image.Image. Wrapped here
// (vs. inlining png.Decode in generator.go) so the test can swap in a
// fixture-generating helper without touching the rest of the pipeline.
func decodePNG(data []byte) (image.Image, error) {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode png: %w", err)
	}
	return img, nil
}
