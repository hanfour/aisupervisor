package personality

import "time"

// Relationship represents the social connection between two workers.
type Relationship struct {
	WorkerA          string    `yaml:"worker_a" json:"workerA"`
	WorkerB          string    `yaml:"worker_b" json:"workerB"`
	Affinity         int       `yaml:"affinity" json:"affinity"`
	Trust            int       `yaml:"trust" json:"trust"`
	InteractionCount int       `yaml:"interaction_count" json:"interactionCount"`
	LastInteraction  time.Time `yaml:"last_interaction" json:"lastInteraction"`
	Tags             []string  `yaml:"tags" json:"tags"`
}

// NewRelationship creates a new relationship between two workers with neutral defaults.
func NewRelationship(a, b string) *Relationship {
	return &Relationship{
		WorkerA:  a,
		WorkerB:  b,
		Affinity: 50,
		Trust:    50,
		Tags:     []string{},
	}
}

// AdjustAffinity changes affinity by delta, clamping to [0, 100].
func (r *Relationship) AdjustAffinity(delta int) {
	r.Affinity = clamp(r.Affinity+delta, 0, 100)
}

// AdjustTrust changes trust by delta, clamping to [0, 100].
func (r *Relationship) AdjustTrust(delta int) {
	r.Trust = clamp(r.Trust+delta, 0, 100)
}

// RecordInteraction increments the interaction count and updates the timestamp.
func (r *Relationship) RecordInteraction() {
	r.InteractionCount++
	r.LastInteraction = time.Now()
}

// UpdateTags recalculates automatic tags based on relationship metrics.
func (r *Relationship) UpdateTags(isManagerRelationship bool) {
	tags := make(map[string]bool)
	for _, t := range r.Tags {
		tags[t] = true
	}

	if r.InteractionCount > 20 && r.Affinity > 70 {
		tags["buddy"] = true
	} else {
		delete(tags, "buddy")
	}

	if isManagerRelationship && r.Trust > 75 {
		tags["mentor"] = true
	} else if !isManagerRelationship {
		delete(tags, "mentor")
	}

	r.Tags = make([]string, 0, len(tags))
	for t := range tags {
		r.Tags = append(r.Tags, t)
	}
}

// RelationshipKey returns a canonical key for the pair, ensuring a consistent order.
func RelationshipKey(a, b string) string {
	if a < b {
		return a + ":" + b
	}
	return b + ":" + a
}
