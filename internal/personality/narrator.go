package personality

import (
	"context"
	"encoding/json"
	"fmt"
)

type AIGenerator interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

type Narrator struct {
	ai AIGenerator
}

func NewNarrator(ai AIGenerator) *Narrator {
	return &Narrator{ai: ai}
}

func (n *Narrator) GeneratePersonality(ctx context.Context, name string, traits PersonalityTraits) (*Narrative, error) {
	prompt := fmt.Sprintf(
		`You are a character designer for a pixel-art office simulation game.
Generate a personality profile for a character named "%s" with these traits (0-100 scale):
- Sociability: %d
- Focus: %d
- Creativity: %d
- Empathy: %d
- Ambition: %d
- Humor: %d

Respond in JSON format (use Traditional Chinese 繁體中文):
{"description": "50-100字性格描述", "catchphrases": ["口頭禪1", "口頭禪2", "口頭禪3"], "backstory": "50字背景故事"}`,
		name, traits.Sociability, traits.Focus, traits.Creativity,
		traits.Empathy, traits.Ambition, traits.Humor,
	)

	resp, err := n.ai.Generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("generate personality: %w", err)
	}

	var narrative Narrative
	if err := json.Unmarshal([]byte(resp), &narrative); err != nil {
		narrative.Description = resp
	}
	return &narrative, nil
}

func (n *Narrator) GenerateDialogue(ctx context.Context, profile *CharacterProfile, situation string) (string, error) {
	prompt := fmt.Sprintf(
		`You are "%s" in an office. Your personality: %s
Your catchphrases: %v
Current mood: %s (energy: %d, morale: %d)
Situation: %s

Generate a single short sentence (under 20 characters, Traditional Chinese) that this character would say. Just the dialogue, no quotes.`,
		profile.WorkerID, profile.Narrative.Description,
		profile.Narrative.Catchphrases,
		profile.Mood.Current, profile.Mood.Energy, profile.Mood.Morale,
		situation,
	)

	return n.ai.Generate(ctx, prompt)
}

func (n *Narrator) GenerateGrowthSummary(ctx context.Context, profile *CharacterProfile) (string, error) {
	prompt := fmt.Sprintf(
		`Character profile: %s
Recent growth events: %d entries
Current traits: Sociability=%d, Focus=%d, Creativity=%d, Empathy=%d, Ambition=%d, Humor=%d

Write a 1-2 sentence growth summary in Traditional Chinese describing how this character has evolved recently.`,
		profile.Narrative.Description,
		len(profile.GrowthLog),
		profile.Traits.Sociability, profile.Traits.Focus, profile.Traits.Creativity,
		profile.Traits.Empathy, profile.Traits.Ambition, profile.Traits.Humor,
	)

	return n.ai.Generate(ctx, prompt)
}
