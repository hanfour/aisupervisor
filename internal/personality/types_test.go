package personality

import (
	"testing"
	"time"
)

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

func TestGenerateRandomBirthday_EngineerAgeRange(t *testing.T) {
	for i := 0; i < 20; i++ {
		bd := GenerateRandomBirthday("engineer")
		age := ageFromBirthday(bd)
		if age < 21 || age > 30 {
			t.Errorf("engineer age = %d, want 21-30", age)
		}
	}
}

func TestGenerateRandomBirthday_ManagerAgeRange(t *testing.T) {
	for i := 0; i < 20; i++ {
		bd := GenerateRandomBirthday("manager")
		age := ageFromBirthday(bd)
		if age < 27 || age > 40 {
			t.Errorf("manager age = %d, want 27-40", age)
		}
	}
}

func TestGenerateRandomBirthday_ConsultantAgeRange(t *testing.T) {
	for i := 0; i < 20; i++ {
		bd := GenerateRandomBirthday("consultant")
		age := ageFromBirthday(bd)
		if age < 34 || age > 55 {
			t.Errorf("consultant age = %d, want 34-55", age)
		}
	}
}

func TestNewCharacterProfile_HasBirthday(t *testing.T) {
	p := NewCharacterProfile("w1", "engineer")
	if p.Birthday == nil {
		t.Fatal("Birthday should not be nil")
	}
	age := ageFromBirthday(*p.Birthday)
	if age < 21 || age > 30 {
		t.Errorf("profile birthday age = %d, want 21-30", age)
	}
}

func ageFromBirthday(bd time.Time) int {
	now := time.Now()
	age := now.Year() - bd.Year()
	if now.YearDay() < bd.YearDay() {
		age--
	}
	return age
}
