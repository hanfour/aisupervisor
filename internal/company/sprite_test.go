package company

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/hanfourmini/aisupervisor/internal/worker"
)

func TestHasSpriteGenerator_DefaultFalse(t *testing.T) {
	m, _ := testManager(t)
	defer m.Shutdown()

	if m.HasSpriteGenerator() {
		t.Fatal("expected HasSpriteGenerator to be false when nothing is wired")
	}
}

func TestGenerateWorkerSprite_NotConfigured(t *testing.T) {
	m, _ := testManager(t)
	defer m.Shutdown()

	w, err := m.CreateWorker("alice", "")
	if err != nil {
		t.Fatalf("CreateWorker: %v", err)
	}

	_, err = m.GenerateWorkerSprite(context.Background(), w.ID)
	if !errors.Is(err, ErrSpriteGeneratorNotConfigured) {
		t.Fatalf("expected ErrSpriteGeneratorNotConfigured, got %v", err)
	}
}

func TestGetWorkerSpritePNG_WorkerMissing(t *testing.T) {
	m, _ := testManager(t)
	defer m.Shutdown()

	_, err := m.GetWorkerSpritePNG("does-not-exist")
	if err == nil {
		t.Fatal("expected error for missing worker, got nil")
	}
	if errors.Is(err, ErrSpriteNotFound) {
		t.Fatal("missing-worker error should NOT alias ErrSpriteNotFound (sentinel reserved for fallback)")
	}
}

func TestGetWorkerSpritePNG_NoAppearance(t *testing.T) {
	m, _ := testManager(t)
	defer m.Shutdown()

	w, err := m.CreateWorker("bob", "")
	if err != nil {
		t.Fatalf("CreateWorker: %v", err)
	}

	_, err = m.GetWorkerSpritePNG(w.ID)
	if !errors.Is(err, ErrSpriteNotFound) {
		t.Fatalf("expected ErrSpriteNotFound, got %v", err)
	}
}

func TestGetWorkerSpritePNG_FileMissingOnDisk(t *testing.T) {
	m, _ := testManager(t)
	defer m.Shutdown()

	w, err := m.CreateWorker("carol", "")
	if err != nil {
		t.Fatalf("CreateWorker: %v", err)
	}

	// Point at a path that doesn't exist on disk — should map to
	// ErrSpriteNotFound so the GUI binding triggers the layered fallback.
	m.mu.Lock()
	w.Appearance = &worker.WorkerAppearance{
		SpriteSheetPath: filepath.Join(t.TempDir(), "nope.png"),
	}
	m.mu.Unlock()

	_, err = m.GetWorkerSpritePNG(w.ID)
	if !errors.Is(err, ErrSpriteNotFound) {
		t.Fatalf("expected ErrSpriteNotFound for missing file, got %v", err)
	}
}

func TestGetWorkerSpritePNG_ReadsFile(t *testing.T) {
	m, _ := testManager(t)
	defer m.Shutdown()

	w, err := m.CreateWorker("dave", "")
	if err != nil {
		t.Fatalf("CreateWorker: %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "walking.png")
	want := []byte("fake-png-bytes")
	if err := os.WriteFile(path, want, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	m.mu.Lock()
	w.Appearance = &worker.WorkerAppearance{SpriteSheetPath: path}
	m.mu.Unlock()

	got, err := m.GetWorkerSpritePNG(w.ID)
	if err != nil {
		t.Fatalf("GetWorkerSpritePNG: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

// ListWorkersWithoutSprites identifies the workers a batch button
// should generate for. Workers with an existing SpriteSheetPath are
// excluded; those with Appearance==nil or empty SpriteSheetPath are
// included.
func TestListWorkersWithoutSprites(t *testing.T) {
	m, _ := testManager(t)
	defer m.Shutdown()

	wA, _ := m.CreateWorker("a", "")
	wB, _ := m.CreateWorker("b", "")
	wC, _ := m.CreateWorker("c", "")

	// wB has a sprite, wA and wC do not.
	m.mu.Lock()
	wB.Appearance = &worker.WorkerAppearance{SpriteSheetPath: "/tmp/b.png"}
	m.mu.Unlock()

	got := m.ListWorkersWithoutSprites()
	if len(got) != 2 {
		t.Fatalf("got %d, want 2 (wA, wC); ids=%v", len(got), got)
	}
	gotSet := map[string]bool{}
	for _, id := range got {
		gotSet[id] = true
	}
	if !gotSet[wA.ID] || !gotSet[wC.ID] {
		t.Errorf("expected {%s, %s}, got %v", wA.ID, wC.ID, got)
	}
	if gotSet[wB.ID] {
		t.Errorf("wB has a sprite but appeared in result: %v", got)
	}
}

// BatchGenerateWorkerSprites without a generator returns
// ErrSpriteGeneratorNotConfigured up front (no per-worker iteration).
func TestBatchGenerateWorkerSprites_NotConfigured(t *testing.T) {
	m, _ := testManager(t)
	defer m.Shutdown()

	w, _ := m.CreateWorker("alone", "")

	called := 0
	results, err := m.BatchGenerateWorkerSprites(context.Background(),
		[]string{w.ID},
		func(SpriteBatchProgress) { called++ },
	)
	if !errors.Is(err, ErrSpriteGeneratorNotConfigured) {
		t.Fatalf("expected ErrSpriteGeneratorNotConfigured, got %v", err)
	}
	if len(results) != 0 {
		t.Errorf("got %d results, want 0 (should fail before iterating)", len(results))
	}
	if called != 0 {
		t.Errorf("onProgress called %d times, want 0", called)
	}
}

// UpdateWorkerAppearance must not blow away an existing AI sprite path
// when only the layered fields are edited — otherwise users editing skin
// tone / outfit would silently lose their generated character.
func TestUpdateWorkerAppearance_PreservesSpriteSheetPath(t *testing.T) {
	m, _ := testManager(t)
	defer m.Shutdown()

	w, err := m.CreateWorker("eve", "")
	if err != nil {
		t.Fatalf("CreateWorker: %v", err)
	}

	const aiPath = "/tmp/sprites/eve/walking.png"
	m.mu.Lock()
	w.Appearance = &worker.WorkerAppearance{
		BodyRow:         0,
		Outfit:          "outfit1",
		Hair:            "hair1",
		SpriteSheetPath: aiPath,
	}
	m.mu.Unlock()

	if err := m.UpdateWorkerAppearance(w.ID, 3, "outfit5", "hair4"); err != nil {
		t.Fatalf("UpdateWorkerAppearance: %v", err)
	}

	got, ok := m.GetWorker(w.ID)
	if !ok {
		t.Fatal("worker disappeared")
	}
	if got.Appearance == nil {
		t.Fatal("appearance is nil after update")
	}
	if got.Appearance.SpriteSheetPath != aiPath {
		t.Fatalf("SpriteSheetPath was clobbered: got %q want %q",
			got.Appearance.SpriteSheetPath, aiPath)
	}
	if got.Appearance.BodyRow != 3 || got.Appearance.Outfit != "outfit5" || got.Appearance.Hair != "hair4" {
		t.Fatalf("layered fields not applied: %+v", got.Appearance)
	}
}
