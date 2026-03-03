package personality

import (
	"math/rand"
	"time"
)

// Mood constants
const (
	MoodHappy      = "happy"
	MoodNeutral    = "neutral"
	MoodStressed   = "stressed"
	MoodFrustrated = "frustrated"
	MoodExcited    = "excited"
	MoodTired      = "tired"
)

// PersonalityTraits represents the core personality dimensions of a character.
type PersonalityTraits struct {
	Sociability int `yaml:"sociability" json:"sociability"`
	Focus       int `yaml:"focus" json:"focus"`
	Creativity  int `yaml:"creativity" json:"creativity"`
	Empathy     int `yaml:"empathy" json:"empathy"`
	Ambition    int `yaml:"ambition" json:"ambition"`
	Humor       int `yaml:"humor" json:"humor"`
}

// MoodState represents the current emotional state of a character.
type MoodState struct {
	Current string `yaml:"current" json:"current"`
	Energy  int    `yaml:"energy" json:"energy"`
	Morale  int    `yaml:"morale" json:"morale"`
}

// PersonalHabits represents a character's daily habits and preferences.
type PersonalHabits struct {
	CoffeeTime       string   `yaml:"coffee_time" json:"coffeeTime"`
	FavoriteSpot     string   `yaml:"favorite_spot" json:"favoriteSpot"`
	WorkStyle        string   `yaml:"work_style" json:"workStyle"`
	SocialPreference string   `yaml:"social_preference" json:"socialPreference"`
	Quirks           []string `yaml:"quirks" json:"quirks"`
}

// Narrative holds descriptive text that gives a character flavor.
type Narrative struct {
	Description  string   `yaml:"description" json:"description"`
	Catchphrases []string `yaml:"catchphrases" json:"catchphrases"`
	Backstory    string   `yaml:"backstory" json:"backstory"`
}

// GrowthEntry records a personality change triggered by an event.
type GrowthEntry struct {
	Event   string         `yaml:"event" json:"event"`
	Date    time.Time      `yaml:"date" json:"date"`
	Changes map[string]int `yaml:"changes" json:"changes"`
}

// CharacterProfile is the full personality profile for a worker.
type CharacterProfile struct {
	WorkerID  string            `yaml:"worker_id" json:"workerId"`
	Traits    PersonalityTraits `yaml:"traits" json:"traits"`
	Mood      MoodState         `yaml:"mood" json:"mood"`
	Habits    PersonalHabits    `yaml:"habits" json:"habits"`
	Narrative Narrative         `yaml:"narrative" json:"narrative"`
	GrowthLog []GrowthEntry    `yaml:"growth_log" json:"growthLog"`
}

// NewRandomTraits generates personality traits with values in the range [40, 80].
func NewRandomTraits() PersonalityTraits {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	randRange := func() int { return 40 + r.Intn(41) } // 40-80 inclusive
	return PersonalityTraits{
		Sociability: randRange(),
		Focus:       randRange(),
		Creativity:  randRange(),
		Empathy:     randRange(),
		Ambition:    randRange(),
		Humor:       randRange(),
	}
}

// NewDefaultMood returns the starting mood state for a new character.
func NewDefaultMood() MoodState {
	return MoodState{
		Current: MoodNeutral,
		Energy:  80,
		Morale:  60,
	}
}

// clamp restricts v to the range [min, max].
func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// AdjustMorale changes morale by delta, clamping to [0, 100].
func (m *MoodState) AdjustMorale(delta int) {
	m.Morale = clamp(m.Morale+delta, 0, 100)
}

// AdjustEnergy changes energy by delta, clamping to [0, 100].
func (m *MoodState) AdjustEnergy(delta int) {
	m.Energy = clamp(m.Energy+delta, 0, 100)
}

// NewCharacterProfile creates a new profile with random traits and default mood.
func NewCharacterProfile(workerID string) *CharacterProfile {
	return &CharacterProfile{
		WorkerID: workerID,
		Traits:   NewRandomTraits(),
		Mood:     NewDefaultMood(),
	}
}
