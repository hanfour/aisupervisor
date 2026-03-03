package personality

import (
	"math/rand"
	"time"
)

// ActivityWeights holds the relative probability weights for each activity type.
type ActivityWeights struct {
	Discussion  int
	Watercooler int
	Meeting     int
	Thinking    int
	PairProg    int
	Patrol      int
	Habit       int
}

var baseWeights = ActivityWeights{
	Discussion:  15,
	Watercooler: 10,
	Meeting:     5,
	Thinking:    20,
	PairProg:    5,
	Patrol:      0,
	Habit:       10,
}

// ComputeActivityWeights returns activity weights adjusted for the given character's
// personality traits and current mood state.
func ComputeActivityWeights(p *CharacterProfile) ActivityWeights {
	w := baseWeights

	if p.Traits.Sociability > 70 {
		w.Discussion += 8
		w.Watercooler += 5
		w.Meeting += 3
		w.Thinking -= 5
	} else if p.Traits.Sociability < 30 {
		w.Discussion -= 8
		w.Watercooler -= 5
		w.Thinking += 15
	}

	if p.Traits.Focus > 70 {
		w.Discussion -= 3
		w.Thinking += 5
	}

	if p.Mood.Energy < 20 {
		w.Watercooler += 15
		w.Discussion -= 5
		w.Thinking -= 5
	}

	if p.Traits.Ambition > 70 {
		w.PairProg += 5
	}

	w.Discussion = max(w.Discussion, 0)
	w.Watercooler = max(w.Watercooler, 0)
	w.Meeting = max(w.Meeting, 0)
	w.Thinking = max(w.Thinking, 0)
	w.PairProg = max(w.PairProg, 0)
	w.Habit = max(w.Habit, 0)

	return w
}

// SelectPartner picks a conversation partner from candidates, weighted by affinity.
// Higher affinity relationships are more likely to be selected.
func SelectPartner(workerID string, candidates []string, store *Store) string {
	if len(candidates) == 0 {
		return ""
	}
	if len(candidates) == 1 {
		return candidates[0]
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	weights := make([]int, len(candidates))
	total := 0
	for i, c := range candidates {
		rel := store.GetRelationship(workerID, c)
		w := 50
		if rel != nil {
			w = rel.Affinity
		}
		weights[i] = w
		total += w
	}

	roll := r.Intn(total)
	cumulative := 0
	for i, w := range weights {
		cumulative += w
		if roll < cumulative {
			return candidates[i]
		}
	}
	return candidates[len(candidates)-1]
}
