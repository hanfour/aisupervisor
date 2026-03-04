package personality

import "testing"

func TestApplyEvent_TaskCompleted(t *testing.T) {
	p := NewCharacterProfile("w1", "engineer")
	p.Mood.Energy = 80
	p.Mood.Morale = 60

	ApplyEvent(p, EventTaskCompleted)

	if p.Mood.Morale != 70 {
		t.Errorf("Morale = %d, want 70 (+10)", p.Mood.Morale)
	}
	if p.Mood.Energy != 65 {
		t.Errorf("Energy = %d, want 65 (-15)", p.Mood.Energy)
	}
	if p.Mood.Current != MoodExcited {
		t.Errorf("Mood = %s, want excited", p.Mood.Current)
	}
}

func TestApplyEvent_TaskFailed(t *testing.T) {
	p := NewCharacterProfile("w1", "engineer")
	p.Mood.Morale = 60

	ApplyEvent(p, EventTaskFailed)

	if p.Mood.Morale != 40 {
		t.Errorf("Morale = %d, want 40 (-20)", p.Mood.Morale)
	}
	if p.Mood.Current != MoodFrustrated {
		t.Errorf("Mood = %s, want frustrated", p.Mood.Current)
	}
}

func TestApplyEvent_SocialActivity_HighSociability(t *testing.T) {
	p := NewCharacterProfile("w1", "engineer")
	p.Traits.Sociability = 75
	p.Mood.Morale = 60

	ApplyEvent(p, EventSocialActivity)

	if p.Mood.Morale != 68 {
		t.Errorf("Morale = %d, want 68 (+8 high sociability)", p.Mood.Morale)
	}
}

func TestApplyEvent_SocialActivity_LowSociability(t *testing.T) {
	p := NewCharacterProfile("w1", "engineer")
	p.Traits.Sociability = 30
	p.Mood.Morale = 60

	ApplyEvent(p, EventSocialActivity)

	if p.Mood.Morale != 65 {
		t.Errorf("Morale = %d, want 65 (+5 normal)", p.Mood.Morale)
	}
}

func TestAutoMoodFromEnergy(t *testing.T) {
	p := NewCharacterProfile("w1", "engineer")
	p.Mood.Energy = 15
	p.Mood.Current = MoodNeutral

	UpdateAutoMood(p)

	if p.Mood.Current != MoodTired {
		t.Errorf("Mood = %s, want tired (energy < 20)", p.Mood.Current)
	}
}
