package knowledge

import (
	"fmt"
	"sync/atomic"
	"time"
)

var entryIDCounter atomic.Int64

// EntryType categorizes a knowledge entry.
type EntryType string

const (
	EntryBugFix               EntryType = "bug_fix"
	EntryPattern              EntryType = "pattern"
	EntryAntiPattern          EntryType = "anti_pattern"
	EntryArchitectureDecision EntryType = "architecture_decision"
	EntryLessonLearned        EntryType = "lesson_learned"
)

// Entry represents a single piece of organizational knowledge.
type Entry struct {
	ID          string     `yaml:"id" json:"id"`
	Type        EntryType  `yaml:"type" json:"type"`
	Domain      string     `yaml:"domain" json:"domain"` // frontend, backend, auth, database, etc.
	Title       string     `yaml:"title" json:"title"`
	Context     string     `yaml:"context" json:"context"`       // when this knowledge applies
	Resolution  string     `yaml:"resolution" json:"resolution"` // what to do
	Confidence  float64    `yaml:"confidence" json:"confidence"` // 0.0-1.0
	Tags        []string   `yaml:"tags,omitempty" json:"tags,omitempty"`
	CreatedBy   string     `yaml:"created_by" json:"createdBy"`
	ValidatedBy string     `yaml:"validated_by,omitempty" json:"validatedBy,omitempty"`
	ExpiresAt   *time.Time `yaml:"expires_at,omitempty" json:"expiresAt,omitempty"`
	CreatedAt   time.Time  `yaml:"created_at" json:"createdAt"`
}

// NewEntry creates a new knowledge entry with a generated ID.
func NewEntry(entryType EntryType, domain, title, context, resolution, createdBy string) *Entry {
	return &Entry{
		ID:         fmt.Sprintf("kb-%d-%d", time.Now().UnixMilli(), entryIDCounter.Add(1)),
		Type:       entryType,
		Domain:     domain,
		Title:      title,
		Context:    context,
		Resolution: resolution,
		Confidence: 0.5,
		CreatedBy:  createdBy,
		CreatedAt:  time.Now(),
	}
}

// IsExpired returns true if the entry has passed its expiration date.
func (e *Entry) IsExpired() bool {
	if e.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*e.ExpiresAt)
}

// IsValid returns true if the entry is not expired and has sufficient confidence.
func (e *Entry) IsValid(minConfidence float64) bool {
	if e.IsExpired() {
		return false
	}
	return e.Confidence >= minConfidence
}
