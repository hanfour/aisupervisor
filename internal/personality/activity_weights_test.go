package personality

import "testing"

func TestActivityWeights_HighSociability(t *testing.T) {
	p := NewCharacterProfile("w1")
	p.Traits.Sociability = 80

	w := ComputeActivityWeights(p)
	if w.Discussion <= 15 {
		t.Errorf("Discussion weight = %d, want > 15 for high sociability", w.Discussion)
	}
	if w.Watercooler <= 10 {
		t.Errorf("Watercooler weight = %d, want > 10 for high sociability", w.Watercooler)
	}
}

func TestActivityWeights_LowSociability(t *testing.T) {
	p := NewCharacterProfile("w1")
	p.Traits.Sociability = 20

	w := ComputeActivityWeights(p)
	if w.Discussion >= 15 {
		t.Errorf("Discussion weight = %d, want < 15 for low sociability", w.Discussion)
	}
	if w.Thinking <= 30 {
		t.Errorf("Thinking weight = %d, want > 30 for low sociability", w.Thinking)
	}
}

func TestActivityWeights_Tired(t *testing.T) {
	p := NewCharacterProfile("w1")
	p.Mood.Energy = 10

	w := ComputeActivityWeights(p)
	if w.Watercooler <= 20 {
		t.Errorf("Watercooler weight = %d, want > 20 when tired", w.Watercooler)
	}
}

func TestSelectPartner_PrefersHighAffinity(t *testing.T) {
	store := NewStore(t.TempDir())
	r1 := store.GetOrCreateRelationship("w1", "w2")
	r1.Affinity = 90
	r2 := store.GetOrCreateRelationship("w1", "w3")
	r2.Affinity = 30

	counts := map[string]int{}
	for i := 0; i < 1000; i++ {
		partner := SelectPartner("w1", []string{"w2", "w3"}, store)
		counts[partner]++
	}

	if counts["w2"] < counts["w3"] {
		t.Errorf("w2 (affinity 90) should be picked more than w3 (affinity 30): w2=%d, w3=%d", counts["w2"], counts["w3"])
	}
}
