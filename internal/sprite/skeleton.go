package sprite

import "github.com/hanfourmini/aisupervisor/internal/pixellab"

// WalkCycleFrames is the number of skeleton keyframes we request from
// PixelLab's animate-with-skeleton endpoint. The endpoint is hardcoded
// server-side to accept exactly 3 keyframes (returns 500 "Expected 3
// pose images" otherwise), so this constant is fixed at 3 even though
// the composed sheet has FramesPerDirection (6) columns per direction
// — the generator repeats the 3 returned frames to fill the slot.
const WalkCycleFrames = 3

// WalkCycleKeypointCount is the keypoint count per skeleton frame —
// surfaces the structural invariant for tests. We use the 14 visible
// pixel-art-character anatomy points: nose / neck / both shoulders /
// elbows / arms (= hands) / hips / knees / legs (= feet).
const WalkCycleKeypointCount = 14

// WalkCycleSkeletons returns the canonical 3-keyframe side-view walk
// cycle as a 2D point slice (outer = per-frame, inner = labelled
// points). Coordinates are in pixel space relative to the top-left of
// a 32×32 frame.
//
// Three keyframes only: frame 0 contact (right leg forward), frame 1
// passing (limbs centred), frame 2 contact-mirror (left leg forward).
// PixelLab's animate-with-skeleton hardcodes "expected 3 pose images"
// server-side, so this is the only count the API accepts. The
// generator repeats these to fill the sheet's 6 columns per direction.
//
// Labels use PixelLab's SkeletonLabel enum vocabulary (see
// pixellab/endpoints.go Label* constants) — the API rejects anything
// outside that enum with a 422 validation error.
//
// PixelLab interprets the keypoints relative to the reference image
// posture; the AnimateWithSkeleton.Direction parameter handles the
// per-direction projection so we reuse the same template for
// south/west/east/north and let the API redraw appropriately.
func WalkCycleSkeletons() [][]pixellab.SkeletonPoint {
	const (
		yNose     = 6.0
		yNeck     = 10.0
		yShoulder = 12.0
		yElbow    = 16.0
		yArm      = 19.0 // hand height
		yHip      = 18.0
		yKnee     = 23.0
		yLeg      = 28.0 // foot height
	)

	// Horizontal centerline (frame is 32 px wide → midline at 16).
	const xMid = 16.0

	// Lateral offsets — how far limbs swing from the centerline.
	const (
		xShoulder = 3.0
		xHip      = 2.5
		xLimb     = 4.0 // peak swing offset for elbows/arms/knees/legs
	)

	type pose struct {
		legL, legR, kneeL, kneeR   float64 // x offsets from xMid
		armL, armR, elbowL, elbowR float64
	}
	// Three canonical walk-cycle keyframes (the only count the API accepts):
	//   0: right leg forward, left arm forward (contact)
	//   1: passing — limbs centred (zero offsets)
	//   2: left leg forward, right arm forward (contact, mirror of 0)
	poses := []pose{
		{legL: -xLimb, legR: xLimb, kneeL: -xLimb * 0.6, kneeR: xLimb * 0.6,
			armL: xLimb, armR: -xLimb, elbowL: xLimb * 0.6, elbowR: -xLimb * 0.6},
		{legL: 0, legR: 0, kneeL: 0, kneeR: 0,
			armL: 0, armR: 0, elbowL: 0, elbowR: 0},
		{legL: xLimb, legR: -xLimb, kneeL: xLimb * 0.6, kneeR: -xLimb * 0.6,
			armL: -xLimb, armR: xLimb, elbowL: -xLimb * 0.6, elbowR: xLimb * 0.6},
	}

	out := make([][]pixellab.SkeletonPoint, len(poses))
	for i, p := range poses {
		// 1px head bob on the two contact frames (0, 2) for liveliness;
		// passing frame (1) sits flat.
		bob := 0.0
		if i == 0 || i == 2 {
			bob = 1.0
		}
		out[i] = []pixellab.SkeletonPoint{
			{Label: pixellab.LabelNose, X: xMid, Y: yNose + bob},
			{Label: pixellab.LabelNeck, X: xMid, Y: yNeck + bob},
			{Label: pixellab.LabelLeftShoulder, X: xMid - xShoulder, Y: yShoulder},
			{Label: pixellab.LabelRightShoulder, X: xMid + xShoulder, Y: yShoulder},
			{Label: pixellab.LabelLeftElbow, X: xMid - xShoulder + p.elbowL, Y: yElbow},
			{Label: pixellab.LabelRightElbow, X: xMid + xShoulder + p.elbowR, Y: yElbow},
			{Label: pixellab.LabelLeftArm, X: xMid - xShoulder + p.armL, Y: yArm},
			{Label: pixellab.LabelRightArm, X: xMid + xShoulder + p.armR, Y: yArm},
			{Label: pixellab.LabelLeftHip, X: xMid - xHip, Y: yHip},
			{Label: pixellab.LabelRightHip, X: xMid + xHip, Y: yHip},
			{Label: pixellab.LabelLeftKnee, X: xMid - xHip + p.kneeL, Y: yKnee},
			{Label: pixellab.LabelRightKnee, X: xMid + xHip + p.kneeR, Y: yKnee},
			{Label: pixellab.LabelLeftLeg, X: xMid - xHip + p.legL, Y: yLeg},
			{Label: pixellab.LabelRightLeg, X: xMid + xHip + p.legR, Y: yLeg},
		}
	}
	return out
}

// requiredWalkLabels are the keypoints BuildWalkCycle must see in the
// estimated baseline to proceed; if any is missing, the caller falls
// back to the hardcoded WalkCycleSkeletons template.
var requiredWalkLabels = []string{
	pixellab.LabelLeftShoulder, pixellab.LabelRightShoulder,
	pixellab.LabelLeftElbow, pixellab.LabelRightElbow,
	pixellab.LabelLeftArm, pixellab.LabelRightArm,
	pixellab.LabelLeftHip, pixellab.LabelRightHip,
	pixellab.LabelLeftKnee, pixellab.LabelRightKnee,
	pixellab.LabelLeftLeg, pixellab.LabelRightLeg,
}

// BuildWalkCycle takes a single estimated skeleton (the character's
// actual proportions, recovered via PixelLab's /estimate-skeleton from
// the south-facing reference) and produces a 3-keyframe walk cycle
// that respects those proportions instead of the hardcoded template.
//
// Returns nil + false if the baseline lacks any of the limb keypoints
// the cycle perturbs (LEFT/RIGHT × SHOULDER/ELBOW/ARM/HIP/KNEE/LEG);
// the caller falls back to WalkCycleSkeletons() when that happens.
// Other labels (NOSE, NECK, EYE, EAR) pass through unchanged when
// present and are dropped silently when missing.
//
// Cycle:
//
//	frame 0 (contact-right): right leg/arm forward, left leg/arm back
//	frame 1 (passing):       baseline pose unchanged + 1 px head bob cleared
//	frame 2 (contact-left):  mirror of frame 0
//
// Limb swing magnitude `xLimb` is fixed at 4 px (matching the static
// template). For a 32×32 frame this yields visible foot/arm motion
// without exceeding the canvas.
func BuildWalkCycle(baseline []pixellab.SkeletonPoint) ([][]pixellab.SkeletonPoint, bool) {
	byLabel := make(map[string]pixellab.SkeletonPoint, len(baseline))
	for _, kp := range baseline {
		byLabel[kp.Label] = kp
	}
	for _, label := range requiredWalkLabels {
		if _, ok := byLabel[label]; !ok {
			return nil, false
		}
	}

	const xLimb = 4.0

	// Compose three frames by perturbing the relevant limb x-offsets.
	// Frame 1 (passing) is the unperturbed baseline; frames 0 and 2
	// add/subtract xLimb to legs and reverse signs for arms.
	out := make([][]pixellab.SkeletonPoint, WalkCycleFrames)
	for i := 0; i < WalkCycleFrames; i++ {
		// signL/signR: which side leads at this frame.
		var legL, legR float64
		switch i {
		case 0: // right leg forward
			legL, legR = -xLimb, +xLimb
		case 2: // left leg forward
			legL, legR = +xLimb, -xLimb
		default: // 1, passing
			legL, legR = 0, 0
		}
		// Arms swing in counter-phase to the legs.
		armL, armR := -legL, -legR

		frame := make([]pixellab.SkeletonPoint, 0, len(baseline))
		for _, kp := range baseline {
			perturbed := kp
			switch kp.Label {
			case pixellab.LabelLeftLeg:
				perturbed.X += legL
			case pixellab.LabelRightLeg:
				perturbed.X += legR
			case pixellab.LabelLeftKnee:
				perturbed.X += legL * 0.6
			case pixellab.LabelRightKnee:
				perturbed.X += legR * 0.6
			case pixellab.LabelLeftArm:
				perturbed.X += armL
			case pixellab.LabelRightArm:
				perturbed.X += armR
			case pixellab.LabelLeftElbow:
				perturbed.X += armL * 0.6
			case pixellab.LabelRightElbow:
				perturbed.X += armR * 0.6
			case pixellab.LabelNose, pixellab.LabelNeck:
				// 1 px head bob on contact frames (0, 2) for liveliness.
				if i == 0 || i == 2 {
					perturbed.Y += 1.0
				}
			}
			frame = append(frame, perturbed)
		}
		out[i] = frame
	}
	return out, true
}
