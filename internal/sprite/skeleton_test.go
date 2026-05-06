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
	// PixelLab's animate-with-skeleton hardcodes "expected 3 pose images"
	// server-side, so WalkCycleFrames is fixed at 3 even though the
	// composed sheet has FramesPerDirection (6) columns per direction.
	// The generator expands [k0,k1,k2] → [k0,k1,k2,k0,k1,k2] to fill.
	if WalkCycleFrames != 3 {
		t.Fatalf("WalkCycleFrames must be 3 (PixelLab API constraint), got %d", WalkCycleFrames)
	}
	if WalkCycleFrames > FramesPerDirection {
		t.Fatalf("WalkCycleFrames (%d) must fit in FramesPerDirection (%d)",
			WalkCycleFrames, FramesPerDirection)
	}
}

func TestWalkCycleSkeletons_KeypointCount(t *testing.T) {
	for i, frame := range WalkCycleSkeletons() {
		if len(frame) != WalkCycleKeypointCount {
			t.Fatalf("frame %d: expected %d keypoints, got %d",
				i, WalkCycleKeypointCount, len(frame))
		}
	}
}

func TestWalkCycleSkeletons_AllInFrameBounds(t *testing.T) {
	// Every keypoint must fall inside [0, FrameSize] on both axes —
	// PixelLab clamps but out-of-bounds points produce visual glitches.
	for i, frame := range WalkCycleSkeletons() {
		for _, kp := range frame {
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
	for _, kp := range skels[0] {
		first[kp.Label] = true
	}
	for i := 1; i < len(skels); i++ {
		if len(skels[i]) != len(skels[0]) {
			t.Fatalf("frame %d has %d keypoints, frame 0 has %d",
				i, len(skels[i]), len(skels[0]))
		}
		for _, kp := range skels[i] {
			if !first[kp.Label] {
				t.Errorf("frame %d: label %q absent in frame 0", i, kp.Label)
			}
		}
	}
}

func TestWalkCycleSkeletons_LabelsAreSkeletonEnum(t *testing.T) {
	// PixelLab's SkeletonLabel enum is the only accepted vocabulary —
	// the API rejects anything else with a 422 validation error. Ensure
	// every keypoint label we emit is one of the documented values.
	allowed := map[string]bool{
		pixellab.LabelNose:          true,
		pixellab.LabelNeck:          true,
		pixellab.LabelRightShoulder: true,
		pixellab.LabelRightElbow:    true,
		pixellab.LabelRightArm:      true,
		pixellab.LabelLeftShoulder:  true,
		pixellab.LabelLeftElbow:     true,
		pixellab.LabelLeftArm:       true,
		pixellab.LabelRightHip:      true,
		pixellab.LabelRightKnee:     true,
		pixellab.LabelRightLeg:      true,
		pixellab.LabelLeftHip:       true,
		pixellab.LabelLeftKnee:      true,
		pixellab.LabelLeftLeg:       true,
		pixellab.LabelRightEye:      true,
		pixellab.LabelLeftEye:       true,
		pixellab.LabelRightEar:      true,
		pixellab.LabelLeftEar:       true,
	}
	for i, frame := range WalkCycleSkeletons() {
		for _, kp := range frame {
			if !allowed[kp.Label] {
				t.Errorf("frame %d: label %q not in SkeletonLabel enum", i, kp.Label)
			}
		}
	}
}

func TestWalkCycleSkeletons_AntiPhaseLegs(t *testing.T) {
	// Frame 0 (right leg forward) and frame 2 (left leg forward) must
	// be mirror images on the leg keypoints — that's the defining
	// property of a walk cycle. The middle passing frame (1) has both
	// legs at xMid and is exempt.
	skels := WalkCycleSkeletons()
	if len(skels) < 3 {
		t.Skip("need ≥3 frames")
	}
	legL0 := findKP(t, skels[0], pixellab.LabelLeftLeg)
	legR0 := findKP(t, skels[0], pixellab.LabelRightLeg)
	legL2 := findKP(t, skels[2], pixellab.LabelLeftLeg)
	legR2 := findKP(t, skels[2], pixellab.LabelRightLeg)

	if legL0.X >= legR0.X {
		t.Errorf("frame 0: expected left leg behind right (legL.X < legR.X), got L=%.1f R=%.1f",
			legL0.X, legR0.X)
	}
	if legL2.X <= legR2.X {
		t.Errorf("frame 2: expected left leg ahead of right (legL.X > legR.X), got L=%.1f R=%.1f",
			legL2.X, legR2.X)
	}
}

func findKP(t *testing.T, frame []pixellab.SkeletonPoint, label string) pixellab.SkeletonPoint {
	t.Helper()
	for _, kp := range frame {
		if kp.Label == label {
			return kp
		}
	}
	t.Fatalf("keypoint %q not found", label)
	return pixellab.SkeletonPoint{}
}
