package pixellab

import "context"

// =============================================================
// /generate-image-pixflux  — text → pixel art (32..400 px)
// =============================================================

// PixfluxRequest configures a Pixflux generation. Description is the
// only required field beyond ImageSize. Guidance scales fall in
// [1.0, 20.0]; PixelLab defaults are reasonable, leave at zero to use them.
type PixfluxRequest struct {
	Description           string      `json:"description"`
	NegativeDescription   string      `json:"negative_description,omitempty"`
	ImageSize             ImageSize   `json:"image_size"`
	NoBackground          bool        `json:"no_background,omitempty"`
	OutlineMode           string      `json:"outline,omitempty"` // "single color black outline" / "selective outline" / "lineless"
	ShadingStyle          string      `json:"shading,omitempty"` // "flat shading" / "basic shading" / "medium shading" / "detailed shading"
	DetailLevel           string      `json:"detail,omitempty"`  // "low detail" / "medium detail" / "highly detailed"
	View                  string      `json:"view,omitempty"`    // "side" / "low top-down" / "high top-down"
	Direction             string      `json:"direction,omitempty"`
	IsometricMode         bool        `json:"isometric,omitempty"`
	OrientedReference     bool        `json:"oriented_reference,omitempty"`
	TextGuidanceScale     float64     `json:"text_guidance_scale,omitempty"`
	InitImage             *Base64Image `json:"init_image,omitempty"`
	InitImageStrength     float64     `json:"init_image_strength,omitempty"`
	StyleImage            *Base64Image `json:"style_image,omitempty"`
	StyleImageStrength    float64     `json:"style_image_strength,omitempty"`
	Seed                  int64       `json:"seed,omitempty"`
}

// PixfluxResponse carries the generated image and usage cost.
type PixfluxResponse struct {
	Image Base64Image `json:"image"`
	Usage Usage       `json:"usage"`
}

// GenerateImagePixflux runs the Pixflux generator. Use this for our
// 32×32 worker base sprites — Pixflux is the higher-fidelity model.
func (c *Client) GenerateImagePixflux(ctx context.Context, req PixfluxRequest) (*PixfluxResponse, error) {
	var out PixfluxResponse
	if err := c.post(ctx, "/generate-image-pixflux", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// =============================================================
// /generate-image-bitforge — text + style → pixel art (16..200 px)
// =============================================================

// BitforgeRequest configures a Bitforge generation. Compared to Pixflux
// it's lower-resolution but supports stronger style transfer via
// StyleImage. Useful for ensuring multiple sprites share a consistent
// art direction.
type BitforgeRequest struct {
	Description           string      `json:"description"`
	NegativeDescription   string      `json:"negative_description,omitempty"`
	ImageSize             ImageSize   `json:"image_size"`
	NoBackground          bool        `json:"no_background,omitempty"`
	View                  string      `json:"view,omitempty"`
	Direction             string      `json:"direction,omitempty"`
	IsometricMode         bool        `json:"isometric,omitempty"`
	OrientedReference     bool        `json:"oriented_reference,omitempty"`
	TextGuidanceScale     float64     `json:"text_guidance_scale,omitempty"`
	ExtraGuidanceScale    float64     `json:"extra_guidance_scale,omitempty"`
	StyleStrength         float64     `json:"style_strength,omitempty"`
	InitImage             *Base64Image `json:"init_image,omitempty"`
	InitImageStrength     float64     `json:"init_image_strength,omitempty"`
	StyleImage            *Base64Image `json:"style_image,omitempty"`
	InpaintingImage       *Base64Image `json:"inpainting_image,omitempty"`
	MaskImage             *Base64Image `json:"mask_image,omitempty"`
	ColorImage            *Base64Image `json:"color_image,omitempty"`
	Seed                  int64       `json:"seed,omitempty"`
}

// BitforgeResponse carries the generated image and usage cost.
type BitforgeResponse struct {
	Image Base64Image `json:"image"`
	Usage Usage       `json:"usage"`
}

// GenerateImageBitforge runs the Bitforge generator.
func (c *Client) GenerateImageBitforge(ctx context.Context, req BitforgeRequest) (*BitforgeResponse, error) {
	var out BitforgeResponse
	if err := c.post(ctx, "/generate-image-bitforge", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// =============================================================
// /rotate — base view → other view angles
// =============================================================

// RotateRequest configures a rotation. FromImage is the base view
// (e.g. front-facing). FromDirection / ToDirection are 8-direction
// strings: "south", "south-east", "east", "north-east", "north",
// "north-west", "west", "south-west". The model returns the same
// character rendered from ToDirection.
type RotateRequest struct {
	ImageSize         ImageSize    `json:"image_size"`
	FromImage         *Base64Image `json:"from_image"`
	FromView          string       `json:"from_view,omitempty"`
	ToView            string       `json:"to_view,omitempty"`
	FromDirection     string       `json:"from_direction"`
	ToDirection       string       `json:"to_direction"`
	IsometricMode     bool         `json:"isometric,omitempty"`
	OrientedReference bool         `json:"oriented_reference,omitempty"`
	ImageGuidanceScale float64     `json:"image_guidance_scale,omitempty"`
	MaskImage         *Base64Image `json:"mask_image,omitempty"`
	ColorImage        *Base64Image `json:"color_image,omitempty"`
	Seed              int64        `json:"seed,omitempty"`
}

// RotateResponse carries the rotated image.
type RotateResponse struct {
	Image Base64Image `json:"image"`
	Usage Usage       `json:"usage"`
}

// Rotate produces a different-direction view of the supplied character.
// To build a 4-direction sprite sheet, call this three times with
// ToDirection set to the three remaining cardinal directions.
func (c *Client) Rotate(ctx context.Context, req RotateRequest) (*RotateResponse, error) {
	var out RotateResponse
	if err := c.post(ctx, "/rotate", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// =============================================================
// /animate-with-skeleton — skeleton poses → frame sequence
// =============================================================

// SkeletonPoint is one labelled keypoint. Label must be a value from
// PixelLab's SkeletonLabel enum (NOSE / NECK / "LEFT SHOULDER" /
// "RIGHT SHOULDER" / … / "LEFT LEG" / "RIGHT LEG"); see the constants
// below or internal/sprite for the canonical mapping. ZIndex is
// optional and defaults to 0 when omitted.
type SkeletonPoint struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Label  string  `json:"label"`
	ZIndex float64 `json:"z_index,omitempty"`
}

// AnimateWithSkeletonRequest takes a base reference image plus a 2D
// list of keypoints (outer = per-frame, inner = labelled points within
// that frame) and returns one PNG per frame. Used to drive 6-frame
// walk cycles from a single static reference.
//
// Field names mirror the live API exactly; the JSON wire format is:
//
//	{
//	  "image_size":      {"width": 32, "height": 32},
//	  "reference_image": {"type": "base64", "base64": "..."},
//	  "skeleton_keypoints": [
//	    [{"x":..., "y":..., "label":"NOSE"}, ...],   // frame 0
//	    [...],                                        // frame 1
//	    ...
//	  ],
//	  "direction": "south",
//	  "view":      "low top-down"
//	}
type AnimateWithSkeletonRequest struct {
	ImageSize         ImageSize         `json:"image_size"`
	ReferenceImage    *Base64Image      `json:"reference_image"`
	SkeletonKeypoints [][]SkeletonPoint `json:"skeleton_keypoints,omitempty"`
	View              string            `json:"view,omitempty"`
	Direction         string            `json:"direction,omitempty"`
	IsometricMode     bool              `json:"isometric,omitempty"`
	ObliqueProjection bool              `json:"oblique_projection,omitempty"`
	GuidanceScale     float64           `json:"guidance_scale,omitempty"`
	InitImages        []*Base64Image    `json:"init_images,omitempty"`
	InitImageStrength int               `json:"init_image_strength,omitempty"`
	InpaintingImages  []*Base64Image    `json:"inpainting_images,omitempty"`
	MaskImages        []*Base64Image    `json:"mask_images,omitempty"`
	ColorImage        *Base64Image      `json:"color_image,omitempty"`
	Seed              int64             `json:"seed,omitempty"`
}

// SkeletonLabel constants — the enum PixelLab accepts on Point.label.
// Using these instead of raw strings catches typos at compile time.
const (
	LabelNose          = "NOSE"
	LabelNeck          = "NECK"
	LabelRightShoulder = "RIGHT SHOULDER"
	LabelRightElbow    = "RIGHT ELBOW"
	LabelRightArm      = "RIGHT ARM"
	LabelLeftShoulder  = "LEFT SHOULDER"
	LabelLeftElbow     = "LEFT ELBOW"
	LabelLeftArm       = "LEFT ARM"
	LabelRightHip      = "RIGHT HIP"
	LabelRightKnee     = "RIGHT KNEE"
	LabelRightLeg      = "RIGHT LEG"
	LabelLeftHip       = "LEFT HIP"
	LabelLeftKnee      = "LEFT KNEE"
	LabelLeftLeg       = "LEFT LEG"
	LabelRightEye      = "RIGHT EYE"
	LabelLeftEye       = "LEFT EYE"
	LabelRightEar      = "RIGHT EAR"
	LabelLeftEar       = "LEFT EAR"
)

// AnimateWithSkeletonResponse carries the per-frame images.
type AnimateWithSkeletonResponse struct {
	Images []Base64Image `json:"images"`
	Usage  Usage         `json:"usage"`
}

// AnimateWithSkeleton drives an animation from skeleton poses.
func (c *Client) AnimateWithSkeleton(ctx context.Context, req AnimateWithSkeletonRequest) (*AnimateWithSkeletonResponse, error) {
	var out AnimateWithSkeletonResponse
	if err := c.post(ctx, "/animate-with-skeleton", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// =============================================================
// /animate-with-text — text-driven walk/idle/run cycles (fixed 64×64)
// =============================================================

// AnimateWithTextRequest is the simpler animation entry point: supply a
// base 64×64 image plus a text description ("walk" / "idle" / "run")
// and PixelLab returns 4 frames covering the cycle. Image size is
// fixed at 64×64.
type AnimateWithTextRequest struct {
	Description       string       `json:"description"`
	Action            string       `json:"action,omitempty"` // "walk" / "idle" / "run"
	ImageSize         ImageSize    `json:"image_size"`
	ReferenceImage    *Base64Image `json:"reference_image"`
	View              string       `json:"view,omitempty"`
	Direction         string       `json:"direction,omitempty"`
	OrientedReference bool         `json:"oriented_reference,omitempty"`
	NumFrames         int          `json:"n_frames,omitempty"` // typically 4
	StartFrame        int          `json:"start_frame_index,omitempty"`
	GuidanceScale     float64      `json:"guidance_scale,omitempty"`
	Seed              int64        `json:"seed,omitempty"`
}

// AnimateWithTextResponse carries the cycle frames.
type AnimateWithTextResponse struct {
	Images []Base64Image `json:"images"`
	Usage  Usage         `json:"usage"`
}

// AnimateWithText drives a text-prompted animation cycle (fixed 64×64).
func (c *Client) AnimateWithText(ctx context.Context, req AnimateWithTextRequest) (*AnimateWithTextResponse, error) {
	var out AnimateWithTextResponse
	if err := c.post(ctx, "/animate-with-text", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// =============================================================
// /estimate-skeleton — image → skeleton
// =============================================================

// EstimateSkeletonRequest extracts a pose skeleton from a character image.
// Required input for AnimateWithSkeleton when the caller doesn't have a
// pre-baked walk-cycle skeleton template.
type EstimateSkeletonRequest struct {
	Image *Base64Image `json:"image"`
}

// EstimateSkeletonResponse returns the inferred keypoints. The API
// returns a flat list (one frame, all points) — matching the spec at
// /v1/openapi.json which defines the response as
// {"usage", "keypoints": [Keypoint]}.
type EstimateSkeletonResponse struct {
	Keypoints []SkeletonPoint `json:"keypoints"`
	Usage     Usage           `json:"usage"`
}

// EstimateSkeleton returns the pose keypoints for an existing image.
func (c *Client) EstimateSkeleton(ctx context.Context, req EstimateSkeletonRequest) (*EstimateSkeletonResponse, error) {
	var out EstimateSkeletonResponse
	if err := c.post(ctx, "/estimate-skeleton", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// =============================================================
// /balance — credit balance
// =============================================================

// CreditsResponse mirrors PixelLab's balance payload. usd is the
// remaining credit pool in USD; type is always "usd".
type CreditsResponse struct {
	Type string  `json:"type"`
	USD  float64 `json:"usd"`
}

// Balance returns the account's remaining USD credits. Useful as a
// startup health probe (verifies the API key + connectivity).
func (c *Client) Balance(ctx context.Context) (*CreditsResponse, error) {
	var out CreditsResponse
	if err := c.get(ctx, "/balance", &out); err != nil {
		return nil, err
	}
	return &out, nil
}
