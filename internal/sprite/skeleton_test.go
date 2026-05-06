package sprite

import (
	"testing"

	"github.com/hanfourmini/aisupervisor/internal/pixellab"
)

func TestWalkCycleSkeletons_FrameCount(t *testing.T) {
	skels := WalkCycleSkeletons()
	if len(skels) != WalkCycleFrames {
		t.Fatalf("expected %d skeleton frames, got %d", WalkCycleFrames, len(skels))
	}
	if WalkCycleFrames != FramesPerDirection {
		t.Fatalf("WalkCycleFrames (%d) must equal FramesPerDirection (%d)",
			WalkCycleFrames, FramesPerDirection)
	}
}

func TestWalkCycleSkeletons_KeypointCount(t *testing.T) {
	for i, s := range WalkCycleSkeletons() {
		if len(s.Keypoints) != WalkCycleKeypointCount {
			t.Fatalf("frame %d: expected %d keypoints, got %d",
				i, WalkCycleKeypointCount, len(s.Keypoints))
		}
	}
}

func TestWalkCycleSkeletons_AllInFrameBounds(t *testing.T) {
	// Every keypoint must fall inside [0, FrameSize] on both axes —
	// PixelLab clamps but out-of-bounds points produce visual glitches.
	for i, s := range WalkCycleSkeletons() {
		for _, kp := range s.Keypoints {
			if kp.X < 0 || kp.X > FrameSize {
				t.Errorf("frame %d label %s: x=%.1f out of [0,%d]", i, kp.Label, kp.X, FrameSize)
			}
			if kp.Y < 0 || kp.Y > FrameSize {
				t.Errorf("frame %d label %s: y=%.1f out of [0,%d]", i, kp.Label, kp.Y, FrameSize)
			}
		}
	}
}

func TestWalkCycleSkeletons_LabelsConsistent(t *testing.T) {
	// The label set must be identical across all frames — PixelLab
	// matches keypoints by label across the frame sequence.
	skels := WalkCycleSkeletons()
	if len(skels) == 0 {
		t.Fatal("no frames")
	}
	first := map[string]bool{}
	for _, kp := range skels[0].Keypoints {
		first[kp.Label] = true
	}
	for i := 1; i < len(skels); i++ {
		if len(skels[i].Keypoints) != len(skels[0].Keypoints) {
			t.Fatalf("frame %d has %d keypoints, frame 0 has %d",
				i, len(skels[i].Keypoints), len(skels[0].Keypoints))
		}
		for _, kp := range skels[i].Keypoints {
			if !first[kp.Label] {
				t.Errorf("frame %d: label %q absent in frame 0", i, kp.Label)
			}
		}
	}
}

func TestWalkCycleSkeletons_AntiPhaseLegs(t *testing.T) {
	// Frame 0 (right foot forward) and frame 3 (left foot forward) must
	// be mirror images on the foot keypoints — that's the defining
	// property of a walk cycle. If this drifts the cycle won't loop.
	skels := WalkCycleSkeletons()
	if len(skels) < 4 {
		t.Skip("need ≥4 frames")
	}
	footL0 := findKP(t, skels[0], kpFootLeft)
	footR0 := findKP(t, skels[0], kpFootRight)
	footL3 := findKP(t, skels[3], kpFootLeft)
	footR3 := findKP(t, skels[3], kpFootRight)

	if footL0.X >= footR0.X {
		t.Errorf("frame 0: expected left foot behind right (footL.X < footR.X), got L=%.1f R=%.1f",
			footL0.X, footR0.X)
	}
	if footL3.X <= footR3.X {
		t.Errorf("frame 3: expected left foot ahead of right (footL.X > footR.X), got L=%.1f R=%.1f",
			footL3.X, footR3.X)
	}
}

func findKP(t *testing.T, s pixellab.Skeleton, label string) pixellab.SkeletonPoint {
	t.Helper()
	for _, kp := range s.Keypoints {
		if kp.Label == label {
			return kp
		}
	}
	t.Fatalf("keypoint %q not found", label)
	return pixellab.SkeletonPoint{}
}
