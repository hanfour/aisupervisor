package sprite

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/png"
)

// ComposeSheet stitches per-direction frame slices into a single
// 768×32 sprite sheet (24-col × 1-row layout, FrameSize=32, six frames
// per direction). Layout:
//
//	cols  0 .. 5   south frames 0..5
//	cols  6 .. 11  west  frames 0..5
//	cols 12 .. 17  east  frames 0..5
//	cols 18 .. 23  north frames 0..5
//
// Each direction's slice must contain exactly FramesPerDirection
// frames; pass the same image six times to reproduce the static
// behaviour from earlier revisions.
//
// Returns the encoded PNG bytes ready to write to disk.
func ComposeSheet(frames map[Direction][]image.Image) ([]byte, error) {
	for _, dir := range AllDirections {
		seq, ok := frames[dir]
		if !ok || seq == nil {
			return nil, fmt.Errorf("compose: missing frames for direction %q", dir)
		}
		if len(seq) != FramesPerDirection {
			return nil, fmt.Errorf("compose: %s has %d frames, want %d",
				dir, len(seq), FramesPerDirection)
		}
		for i, f := range seq {
			if f == nil {
				return nil, fmt.Errorf("compose: %s frame %d is nil", dir, i)
			}
			b := f.Bounds()
			if b.Dx() != FrameSize || b.Dy() != FrameSize {
				return nil, fmt.Errorf("compose: %s frame %d is %dx%d, want %dx%d",
					dir, i, b.Dx(), b.Dy(), FrameSize, FrameSize)
			}
		}
	}

	width := FrameSize * FramesPerDirection * len(AllDirections) // 32 × 6 × 4 = 768
	sheet := image.NewRGBA(image.Rect(0, 0, width, FrameSize))

	for dirIdx, dir := range AllDirections {
		seq := frames[dir]
		for f, frame := range seq {
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
