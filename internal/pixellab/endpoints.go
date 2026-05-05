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

// SkeletonPoint is one labelled keypoint (e.g. "left_hip", "head").
type SkeletonPoint struct {
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Label string  `json:"label"`
}

// Skeleton is one frame's pose (all keypoints).
type Skeleton struct {
	Keypoints []SkeletonPoint `json:"keypoints"`
}

// AnimateWithSkeletonRequest takes a base reference image plus a list of
// pose skeletons (one per frame) and returns one PNG per frame, the
// character bent into each pose. Used for building 6-frame walk cycles.
type AnimateWithSkeletonRequest struct {
	ImageSize         ImageSize    `json:"image_size"`
	ReferenceImage    *Base64Image `json:"reference_image"`
	Skeletons         []Skeleton   `json:"skeletons"`
	ReferenceSkeleton *Skeleton    `json:"reference_skeleton,omitempty"`
	View              string       `json:"view,omitempty"`
	Direction         string       `json:"direction,omitempty"`
	IsometricMode     bool         `json:"isometric,omitempty"`
	OrientedReference bool         `json:"oriented_reference,omitempty"`
	GuidanceScale     float64      `json:"guidance_scale,omitempty"`
	MaskImage         *Base64Image `json:"mask_image,omitempty"`
	ColorImage        *Base64Image `json:"color_image,omitempty"`
	Seed              int64        `json:"seed,omitempty"`
}

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

// EstimateSkeletonResponse returns the inferred keypoints.
type EstimateSkeletonResponse struct {
	Skeleton Skeleton `json:"skeleton"`
	Usage    Usage    `json:"usage"`
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
