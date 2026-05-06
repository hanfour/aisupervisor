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

// =============================================================
// BuildWalkCycle — per-character skeleton estimation
// =============================================================

// fullEstimatedBaseline returns a 14-keypoint skeleton in the same
// shape PixelLab's EstimateSkeleton would for a typical humanoid.
// Coordinates are intentionally non-default (slightly off-centre, taller
// than the hardcoded template) so tests can verify perturbations are
// applied relative to the *baseline* and not silently substituted.
func fullEstimatedBaseline() []pixellab.SkeletonPoint {
	return []pixellab.SkeletonPoint{
		{Label: pixellab.LabelNose, X: 17, Y: 5},
		{Label: pixellab.LabelNeck, X: 17, Y: 9},
		{Label: pixellab.LabelLeftShoulder, X: 14, Y: 11},
		{Label: pixellab.LabelRightShoulder, X: 20, Y: 11},
		{Label: pixellab.LabelLeftElbow, X: 13, Y: 15},
		{Label: pixellab.LabelRightElbow, X: 21, Y: 15},
		{Label: pixellab.LabelLeftArm, X: 12, Y: 18},
		{Label: pixellab.LabelRightArm, X: 22, Y: 18},
		{Label: pixellab.LabelLeftHip, X: 14, Y: 17},
		{Label: pixellab.LabelRightHip, X: 19, Y: 17},
		{Label: pixellab.LabelLeftKnee, X: 14, Y: 22},
		{Label: pixellab.LabelRightKnee, X: 19, Y: 22},
		{Label: pixellab.LabelLeftLeg, X: 14, Y: 27},
		{Label: pixellab.LabelRightLeg, X: 19, Y: 27},
	}
}

func TestBuildWalkCycle_FrameCount(t *testing.T) {
	cycle, ok := BuildWalkCycle(fullEstimatedBaseline())
	if !ok {
		t.Fatal("expected ok=true with full baseline")
	}
	if len(cycle) != WalkCycleFrames {
		t.Fatalf("got %d frames, want %d", len(cycle), WalkCycleFrames)
	}
}

func TestBuildWalkCycle_PassingFrameMatchesBaseline(t *testing.T) {
	// Frame 1 (index 1) is the passing pose — limbs sit at baseline x.
	baseline := fullEstimatedBaseline()
	cycle, _ := BuildWalkCycle(baseline)
	frame1 := cycle[1]

	for _, kp := range baseline {
		got := findKP(t, frame1, kp.Label)
		if got.X != kp.X {
			t.Errorf("passing frame: %s.X = %.1f, want baseline %.1f", kp.Label, got.X, kp.X)
		}
		if got.Y != kp.Y {
			t.Errorf("passing frame: %s.Y = %.1f, want baseline %.1f", kp.Label, got.Y, kp.Y)
		}
	}
}

func TestBuildWalkCycle_LegsAntiPhase(t *testing.T) {
	baseline := fullEstimatedBaseline()
	cycle, _ := BuildWalkCycle(baseline)

	bL := findKP(t, baseline, pixellab.LabelLeftLeg).X
	bR := findKP(t, baseline, pixellab.LabelRightLeg).X
	f0L := findKP(t, cycle[0], pixellab.LabelLeftLeg).X
	f0R := findKP(t, cycle[0], pixellab.LabelRightLeg).X
	f2L := findKP(t, cycle[2], pixellab.LabelLeftLeg).X
	f2R := findKP(t, cycle[2], pixellab.LabelRightLeg).X

	if f0L != bL-4 || f0R != bR+4 {
		t.Errorf("frame 0 legs: L=%.1f R=%.1f, want L=%.1f R=%.1f (baseline -4 / +4)",
			f0L, f0R, bL-4, bR+4)
	}
	if f2L != bL+4 || f2R != bR-4 {
		t.Errorf("frame 2 legs: L=%.1f R=%.1f, want L=%.1f R=%.1f (baseline +4 / -4)",
			f2L, f2R, bL+4, bR-4)
	}
}

func TestBuildWalkCycle_ArmsCounterPhaseToLegs(t *testing.T) {
	baseline := fullEstimatedBaseline()
	cycle, _ := BuildWalkCycle(baseline)

	bArmL := findKP(t, baseline, pixellab.LabelLeftArm).X
	bArmR := findKP(t, baseline, pixellab.LabelRightArm).X
	f0ArmL := findKP(t, cycle[0], pixellab.LabelLeftArm).X
	f0ArmR := findKP(t, cycle[0], pixellab.LabelRightArm).X

	// In frame 0: legL = baseline-4, legR = baseline+4
	// → armL should be baseline+4, armR baseline-4 (inverse)
	if f0ArmL != bArmL+4 {
		t.Errorf("frame 0 left arm = %.1f, want baseline+4 = %.1f", f0ArmL, bArmL+4)
	}
	if f0ArmR != bArmR-4 {
		t.Errorf("frame 0 right arm = %.1f, want baseline-4 = %.1f", f0ArmR, bArmR-4)
	}
}

func TestBuildWalkCycle_HeadBobOnContactFrames(t *testing.T) {
	baseline := fullEstimatedBaseline()
	cycle, _ := BuildWalkCycle(baseline)

	bNose := findKP(t, baseline, pixellab.LabelNose).Y
	if y := findKP(t, cycle[0], pixellab.LabelNose).Y; y != bNose+1 {
		t.Errorf("frame 0 nose Y = %.1f, want bob (+1) = %.1f", y, bNose+1)
	}
	if y := findKP(t, cycle[1], pixellab.LabelNose).Y; y != bNose {
		t.Errorf("frame 1 nose Y = %.1f, want baseline %.1f", y, bNose)
	}
	if y := findKP(t, cycle[2], pixellab.LabelNose).Y; y != bNose+1 {
		t.Errorf("frame 2 nose Y = %.1f, want bob (+1) = %.1f", y, bNose+1)
	}
}

func TestBuildWalkCycle_ReturnsFalseOnMissingLeg(t *testing.T) {
	baseline := fullEstimatedBaseline()
	pruned := make([]pixellab.SkeletonPoint, 0, len(baseline)-1)
	for _, kp := range baseline {
		if kp.Label == pixellab.LabelLeftLeg {
			continue
		}
		pruned = append(pruned, kp)
	}
	if _, ok := BuildWalkCycle(pruned); ok {
		t.Fatal("expected ok=false when LEFT_LEG missing")
	}
}

func TestBuildWalkCycle_PreservesPassThroughLabels(t *testing.T) {
	// EYE labels aren't perturbed but should pass through if present
	// in the estimated baseline (e.g. the model returned extras).
	baseline := append(fullEstimatedBaseline(),
		pixellab.SkeletonPoint{Label: pixellab.LabelLeftEye, X: 15, Y: 5},
		pixellab.SkeletonPoint{Label: pixellab.LabelRightEye, X: 19, Y: 5},
	)
	cycle, ok := BuildWalkCycle(baseline)
	if !ok {
		t.Fatal("expected ok=true with full required keypoints")
	}
	for _, frame := range cycle {
		eye := findKP(t, frame, pixellab.LabelLeftEye)
		if eye.X != 15 || eye.Y != 5 {
			t.Errorf("LEFT_EYE perturbed unexpectedly: %+v", eye)
		}
	}
}
