package personality

import "testing"

func TestNewRelationship_Defaults(t *testing.T) {
	r := NewRelationship("w1", "w2")
	if r.Affinity != 50 {
		t.Errorf("Affinity = %d, want 50", r.Affinity)
	}
	if r.Trust != 50 {
		t.Errorf("Trust = %d, want 50", r.Trust)
	}
}

func TestRelationship_AdjustAffinity(t *testing.T) {
	r := NewRelationship("w1", "w2")
	r.AdjustAffinity(30)
	if r.Affinity != 80 {
		t.Errorf("Affinity = %d, want 80", r.Affinity)
	}
	r.AdjustAffinity(30) // should clamp
	if r.Affinity != 100 {
		t.Errorf("Affinity = %d, want 100", r.Affinity)
	}
}

func TestRelationship_AutoTags(t *testing.T) {
	r := NewRelationship("w1", "w2")
	r.Affinity = 75
	r.InteractionCount = 25
	r.UpdateTags(false)
	if !containsTag(r.Tags, "buddy") {
		t.Error("expected 'buddy' tag")
	}
}

func TestRelationship_MentorTag(t *testing.T) {
	r := NewRelationship("w1", "w2")
	r.Trust = 80
	r.UpdateTags(true) // isManagerRelationship=true
	if !containsTag(r.Tags, "mentor") {
		t.Error("expected 'mentor' tag")
	}
}

func containsTag(tags []string, target string) bool {
	for _, t := range tags {
		if t == target {
			return true
		}
	}
	return false
}
