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

// WalkCycleSkeletons returns the canonical 6-frame side-view walk
// cycle as a 2D point slice (outer = per-frame, inner = labelled
// points). Coordinates are in pixel space relative to the top-left of
// a 32×32 frame.
//
// The cycle is symmetric around frame 3 — frame 0 (right leg forward)
// mirrors frame 3 (left leg forward), with intermediate frames at 1/2
// and 4/5. Played at ~6 fps it produces a natural-looking loop.
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
