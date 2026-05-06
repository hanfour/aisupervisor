package sprite

import "github.com/hanfourmini/aisupervisor/internal/pixellab"

// WalkCycleFrames is the number of skeleton poses in a walk cycle.
// Matches FramesPerDirection so each direction's six columns hold one
// frame each. Keep these two constants in sync.
const WalkCycleFrames = FramesPerDirection

// Walk-cycle keypoint labels.
//
// PixelLab's animate-with-skeleton accepts arbitrary string labels —
// what matters for visual fidelity is that the same label appears at
// the same anatomical location across every frame in the request.
// We use the standard pixel-art-character minimum: head, neck,
// shoulders, elbows, hands, hips, knees, feet (12 points total).
const (
	kpHead          = "HEAD"
	kpNeck          = "NECK"
	kpShoulderLeft  = "SHOULDER_L"
	kpShoulderRight = "SHOULDER_R"
	kpElbowLeft     = "ELBOW_L"
	kpElbowRight    = "ELBOW_R"
	kpHandLeft      = "HAND_L"
	kpHandRight     = "HAND_R"
	kpHipLeft       = "HIP_L"
	kpHipRight      = "HIP_R"
	kpKneeLeft      = "KNEE_L"
	kpKneeRight     = "KNEE_R"
	kpFootLeft      = "FOOT_L"
	kpFootRight     = "FOOT_R"
)

// WalkCycleKeypointCount is the keypoint count per skeleton frame —
// surfaces the structural invariant for tests.
const WalkCycleKeypointCount = 14

// WalkCycleSkeletons returns the canonical 6-frame side-view walk
// cycle as a slice of skeletons sized to a 32×32 frame. Coordinates
// are in pixel space relative to the top-left of the frame.
//
// The cycle is symmetric around frame 3 — frame 0 (right leg forward)
// mirrors frame 3 (left leg forward), with intermediate frames at 1/2
// and 4/5. This produces a natural-looking 6-frame loop when played
// at ~6 fps (the renderer's animation speed).
//
// PixelLab interprets the keypoints relative to the reference image
// posture; the AnimateWithSkeleton.Direction parameter handles the
// per-direction projection so we reuse the same skeleton template
// for south/west/east/north and let the API redraw appropriately.
func WalkCycleSkeletons() []pixellab.Skeleton {
	// Vertical anchors (y axis grows downward in image coords).
	const (
		yHead     = 6.0
		yNeck     = 10.0
		yShoulder = 12.0
		yElbow    = 16.0
		yHand     = 19.0
		yHip      = 18.0
		yKnee     = 23.0
		yFoot     = 28.0
	)

	// Horizontal centerline (frame is 32 px wide → midline at 16).
	const xMid = 16.0

	// Lateral offsets — how far limbs swing from the centerline.
	const (
		xShoulder = 3.0
		xHip      = 2.5
		xLimb     = 4.0 // peak swing offset for elbows/hands/knees/feet
	)

	// limbPhase returns (frontLegX, backLegX) given a phase in [0,1).
	// Frame 0 has the right leg fully forward; frame 3 has the left
	// fully forward. The arms swing in counter-phase to the legs.
	type pose struct {
		footL, footR, kneeL, kneeR float64 // x offsets from xMid
		handL, handR, elbowL, elbowR float64
	}
	poses := []pose{
		{footL: -xLimb, footR: xLimb, kneeL: -xLimb * 0.6, kneeR: xLimb * 0.6,
			handL: xLimb, handR: -xLimb, elbowL: xLimb * 0.6, elbowR: -xLimb * 0.6},
		{footL: -xLimb * 0.5, footR: xLimb * 0.5, kneeL: -xLimb * 0.3, kneeR: xLimb * 0.3,
			handL: xLimb * 0.5, handR: -xLimb * 0.5, elbowL: xLimb * 0.3, elbowR: -xLimb * 0.3},
		{footL: 0, footR: 0, kneeL: 0, kneeR: 0,
			handL: 0, handR: 0, elbowL: 0, elbowR: 0},
		{footL: xLimb, footR: -xLimb, kneeL: xLimb * 0.6, kneeR: -xLimb * 0.6,
			handL: -xLimb, handR: xLimb, elbowL: -xLimb * 0.6, elbowR: xLimb * 0.6},
		{footL: xLimb * 0.5, footR: -xLimb * 0.5, kneeL: xLimb * 0.3, kneeR: -xLimb * 0.3,
			handL: -xLimb * 0.5, handR: xLimb * 0.5, elbowL: -xLimb * 0.3, elbowR: xLimb * 0.3},
		{footL: 0, footR: 0, kneeL: 0, kneeR: 0,
			handL: 0, handR: 0, elbowL: 0, elbowR: 0},
	}

	out := make([]pixellab.Skeleton, len(poses))
	for i, p := range poses {
		// Slight head bob — 1 px down on the contact frames (0, 3).
		headBob := 0.0
		if i == 0 || i == 3 {
			headBob = 1.0
		}
		out[i] = pixellab.Skeleton{
			Keypoints: []pixellab.SkeletonPoint{
				{Label: kpHead, X: xMid, Y: yHead + headBob},
				{Label: kpNeck, X: xMid, Y: yNeck + headBob},
				{Label: kpShoulderLeft, X: xMid - xShoulder, Y: yShoulder},
				{Label: kpShoulderRight, X: xMid + xShoulder, Y: yShoulder},
				{Label: kpElbowLeft, X: xMid - xShoulder + p.elbowL, Y: yElbow},
				{Label: kpElbowRight, X: xMid + xShoulder + p.elbowR, Y: yElbow},
				{Label: kpHandLeft, X: xMid - xShoulder + p.handL, Y: yHand},
				{Label: kpHandRight, X: xMid + xShoulder + p.handR, Y: yHand},
				{Label: kpHipLeft, X: xMid - xHip, Y: yHip},
				{Label: kpHipRight, X: xMid + xHip, Y: yHip},
				{Label: kpKneeLeft, X: xMid - xHip + p.kneeL, Y: yKnee},
				{Label: kpKneeRight, X: xMid + xHip + p.kneeR, Y: yKnee},
				{Label: kpFootLeft, X: xMid - xHip + p.footL, Y: yFoot},
				{Label: kpFootRight, X: xMid + xHip + p.footR, Y: yFoot},
			},
		}
	}
	return out
}
