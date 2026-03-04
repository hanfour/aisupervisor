package personality

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStore_SaveAndLoad_Profiles(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	p := NewCharacterProfile("w1", "engineer")
	p.Traits.Sociability = 72
	p.Narrative.Description = "測試角色"
	s.SetProfile(p)

	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "personalities.yaml")); err != nil {
		t.Fatalf("personalities.yaml not found: %v", err)
	}

	s2 := NewStore(dir)
	if err := s2.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	p2 := s2.GetProfile("w1")
	if p2 == nil {
		t.Fatal("profile not found after load")
	}
	if p2.Traits.Sociability != 72 {
		t.Errorf("Sociability = %d, want 72", p2.Traits.Sociability)
	}
	if p2.Narrative.Description != "測試角色" {
		t.Errorf("Description = %q, want '測試角色'", p2.Narrative.Description)
	}
}

func TestStore_SaveAndLoad_Relationships(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	r := NewRelationship("w1", "w2")
	r.Affinity = 75
	s.SetRelationship(r)

	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	s2 := NewStore(dir)
	if err := s2.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	r2 := s2.GetRelationship("w1", "w2")
	if r2 == nil {
		t.Fatal("relationship not found after load")
	}
	if r2.Affinity != 75 {
		t.Errorf("Affinity = %d, want 75", r2.Affinity)
	}
}

func TestStore_DeleteProfile(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	s.SetProfile(NewCharacterProfile("w1", "engineer"))
	s.DeleteProfile("w1")
	if s.GetProfile("w1") != nil {
		t.Error("profile should be nil after delete")
	}
}

func TestStore_UpdateRelationship(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	s.UpdateRelationship("a", "b", func(r *Relationship) {
		r.AdjustAffinity(10)
	})
	rel := s.GetRelationship("a", "b")
	if rel == nil || rel.Affinity != 60 {
		t.Fatalf("expected affinity 60 after UpdateRelationship, got %v", rel)
	}
}

func TestStore_UpdateProfile(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	p := NewCharacterProfile("w1", "engineer")
	s.SetProfile(p)

	ok := s.UpdateProfile("w1", func(p *CharacterProfile) {
		p.Mood.Morale = 99
	})
	if !ok {
		t.Fatal("expected UpdateProfile to return true")
	}
	got := s.GetProfile("w1")
	if got.Mood.Morale != 99 {
		t.Fatalf("expected morale 99, got %d", got.Mood.Morale)
	}

	ok = s.UpdateProfile("nonexistent", func(p *CharacterProfile) {})
	if ok {
		t.Fatal("expected UpdateProfile to return false for nonexistent")
	}
}

func TestStore_Save_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	p := NewCharacterProfile("w1", "engineer")
	s.SetProfile(p)

	if err := s.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// No .tmp files should remain
	matches, _ := filepath.Glob(filepath.Join(dir, "*.tmp"))
	if len(matches) > 0 {
		t.Fatalf("temp files left behind: %v", matches)
	}

	// Data should be loadable
	s2 := NewStore(dir)
	if err := s2.Load(); err != nil {
		t.Fatalf("Load after atomic save failed: %v", err)
	}
	if s2.GetProfile("w1") == nil {
		t.Fatal("profile lost after atomic save")
	}
}

func TestStore_GetRelationshipAffinity(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	if a := s.GetRelationshipAffinity("a", "b"); a != 50 {
		t.Fatalf("expected default 50, got %d", a)
	}

	s.UpdateRelationship("a", "b", func(r *Relationship) {
		r.AdjustAffinity(20)
	})
	if a := s.GetRelationshipAffinity("a", "b"); a != 70 {
		t.Fatalf("expected 70, got %d", a)
	}
}
