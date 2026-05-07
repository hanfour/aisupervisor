package sprite

import (
	"fmt"
	"strings"
)

// BuildPrompt translates a WorkerProfile into a PixelLab description
// string. The prompt is engineered to:
//
//   - lock the art style ("32x32 pixel art top-down character") so
//     successive calls produce consistent figures
//   - encode the worker's role from skill_profile so the generated
//     character looks the part (devops in hoodie + headphones, etc.)
//   - encode gender when set, leaving it un-specified otherwise so the
//     model picks something neutral rather than defaulting one way
//   - include the worker's personality flavor text last, where it acts
//     as a softer bias rather than a hard constraint
//
// The exported function lives here (vs. inside generator.go) so it can
// be unit-tested independently of the HTTP client.
func BuildPrompt(p WorkerProfile) string {
	// Framing anchors come first — at 32×32 the model otherwise tends
	// to crop to a portrait/upper-body shot. "head to toe" + "standing
	// centered" + "small RPG game character sprite" lock the model
	// onto a full-body silhouette.
	parts := []string{
		"32x32 pixel art top-down character",
		"full body visible head to toe",
		"standing centered",
		"small RPG game character sprite",
		profileVisualHint(p.SkillProfile),
	}

	if g := genderHint(p.Gender); g != "" {
		parts = append(parts, g)
	}

	if p.Personality != "" {
		parts = append(parts, "(personality: "+strings.TrimSpace(p.Personality)+")")
	}

	// Style anchors come last so they bias rather than dominate the
	// prompt. "simple silhouette" + black outline (set on the
	// PixfluxRequest) keep characters visually consistent across runs
	// even when role descriptions differ widely.
	parts = append(parts,
		"clean line art",
		"vibrant flat colors",
		"simple silhouette",
		"transparent background",
		"facing south",
	)
	return strings.Join(filterEmpty(parts), ", ")
}

// profileVisualHint returns the wardrobe / accessory phrase associated
// with a skill profile. The mapping mirrors the existing
// CHARACTER_CONFIGS in frontend/src/lib/office/sprites.js so the AI
// sprites read as drop-in replacements rather than a stylistic break.
func profileVisualHint(profile string) string {
	switch strings.ToLower(profile) {
	case "coder":
		return "casual t-shirt and jeans, software developer"
	case "hacker":
		return "dark hoodie, security researcher, pulled-up hood, laptop sticker decals"
	case "designer":
		return "creative fashionable outfit, designer with sketch tablet, colorful accessories"
	case "analyst":
		return "smart-casual blazer over button-up, data analyst, glasses"
	case "architect":
		return "executive blazer, software architect, confident pose"
	case "devops":
		return "technical hoodie, devops engineer, headphones, terminal-themed shirt"
	case "reviewer":
		return "professional smart-casual outfit, code reviewer, focused expression"
	case "researcher":
		return "academic casual cardigan, researcher, glasses, holding a notebook"
	case "assistant":
		return "friendly office attire, administrative assistant"
	case "hr":
		return "warm professional outfit, HR specialist, approachable"
	case "":
		return "office worker outfit"
	default:
		return fmt.Sprintf("%s professional outfit", strings.ToLower(profile))
	}
}

// genderHint returns the gender phrase or empty string when the field
// is unset / "neutral" — leaving it unspecified lets the model pick
// something natural rather than defaulting to one gender silently.
func genderHint(g string) string {
	switch strings.ToLower(strings.TrimSpace(g)) {
	case "male", "m":
		return "male character"
	case "female", "f":
		return "female character"
	default:
		return ""
	}
}

// filterEmpty drops empty strings so the joined prompt doesn't carry
// double-commas when an optional field (gender, personality) is unset.
func filterEmpty(s []string) []string {
	out := s[:0]
	for _, v := range s {
		if strings.TrimSpace(v) != "" {
			out = append(out, v)
		}
	}
	return out
}
