package personality

import "testing"

func TestNewTraits_RandomRange(t *testing.T) {
	traits := NewRandomTraits()
	fields := []struct {
		name string
		val  int
	}{
		{"Sociability", traits.Sociability},
		{"Focus", traits.Focus},
		{"Creativity", traits.Creativity},
		{"Empathy", traits.Empathy},
		{"Ambition", traits.Ambition},
		{"Humor", traits.Humor},
	}
	for _, f := range fields {
		if f.val < 40 || f.val > 80 {
			t.Errorf("%s = %d, want 40-80", f.name, f.val)
		}
	}
}

func TestMoodState_Default(t *testing.T) {
	m := NewDefaultMood()
	if m.Current != MoodNeutral {
		t.Errorf("Current = %s, want neutral", m.Current)
	}
	if m.Energy != 80 {
		t.Errorf("Energy = %d, want 80", m.Energy)
	}
	if m.Morale != 60 {
		t.Errorf("Morale = %d, want 60", m.Morale)
	}
}

func TestMoodState_Clamp(t *testing.T) {
	m := &MoodState{Current: MoodNeutral, Energy: 95, Morale: 95}
	m.AdjustMorale(20)
	if m.Morale != 100 {
		t.Errorf("Morale = %d, want 100 (clamped)", m.Morale)
	}
	m.AdjustEnergy(-200)
	if m.Energy != 0 {
		t.Errorf("Energy = %d, want 0 (clamped)", m.Energy)
	}
}
