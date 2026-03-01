package context

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDecisionTrim(t *testing.T) {
	sc := NewSessionContext("test", 3, 5)

	for i := 0; i < 5; i++ {
		sc.AddDecision(DecisionRecord{
			Summary:   "decision " + string(rune('A'+i)),
			ChosenKey: "y",
		})
	}

	snap := sc.Snapshot()
	if len(snap.Decisions) != 3 {
		t.Fatalf("expected 3 decisions, got %d", len(snap.Decisions))
	}
	// Should keep the last 3: C, D, E
	if snap.Decisions[0].Summary != "decision C" {
		t.Errorf("expected first decision to be 'decision C', got %q", snap.Decisions[0].Summary)
	}
	if snap.Decisions[2].Summary != "decision E" {
		t.Errorf("expected last decision to be 'decision E', got %q", snap.Decisions[2].Summary)
	}
}

func TestActivityTrim(t *testing.T) {
	sc := NewSessionContext("test", 20, 2)

	sc.AddActivity(ActivitySummary{Summary: "a1"})
	sc.AddActivity(ActivitySummary{Summary: "a2"})
	sc.AddActivity(ActivitySummary{Summary: "a3"})

	snap := sc.Snapshot()
	if len(snap.Activities) != 2 {
		t.Fatalf("expected 2 activities, got %d", len(snap.Activities))
	}
	if snap.Activities[0].Summary != "a2" {
		t.Errorf("expected 'a2', got %q", snap.Activities[0].Summary)
	}
}

func TestSnapshotIsolation(t *testing.T) {
	sc := NewSessionContext("test", 20, 10)
	sc.AddDecision(DecisionRecord{Summary: "d1"})

	snap := sc.Snapshot()
	sc.AddDecision(DecisionRecord{Summary: "d2"})

	if len(snap.Decisions) != 1 {
		t.Errorf("snapshot should not be affected by later mutations")
	}
}

func TestDetectProjectGo(t *testing.T) {
	dir := t.TempDir()

	// Create a go.mod file
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/foo\ngo 1.21\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	p := DetectProject(dir)
	if p.Language != "Go" {
		t.Errorf("expected language 'Go', got %q", p.Language)
	}
	if p.BuildTool != "go" {
		t.Errorf("expected build tool 'go', got %q", p.BuildTool)
	}
	if p.Directory != dir {
		t.Errorf("expected directory %q, got %q", dir, p.Directory)
	}
}

func TestDetectProjectTypeScript(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"dependencies":{"next":"14.0.0","react":"18.0.0"}}`), 0o644)
	os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(`{}`), 0o644)

	p := DetectProject(dir)
	if p.Language != "TypeScript" {
		t.Errorf("expected 'TypeScript', got %q", p.Language)
	}
	if p.Framework != "Next.js" {
		t.Errorf("expected framework 'Next.js', got %q", p.Framework)
	}
}

func TestDetectProjectEmpty(t *testing.T) {
	p := DetectProject("")
	if p.Directory != "" {
		t.Errorf("expected empty directory, got %q", p.Directory)
	}
}

func TestParseWorkingDirectory(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "pwd output",
			content: "some output\n/home/user/projects/myapp\n$ ",
			want:    "/home/user/projects/myapp",
		},
		{
			name:    "cd command",
			content: "$ cd /var/log\nsome log output",
			want:    "/var/log",
		},
		{
			name:    "prompt with cwd",
			content: "user@host:/home/user/code$ ls\nfile1 file2",
			want:    "/home/user/code",
		},
		{
			name:    "empty content",
			content: "",
			want:    "",
		},
		{
			name:    "no path",
			content: "hello world\nfoo bar",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseWorkingDirectory(tt.content)
			if got != tt.want {
				t.Errorf("ParseWorkingDirectory() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSummarizeActivity(t *testing.T) {
	content := "$ git status\nOn branch main\n$ go test ./...\nPASS\n$ echo done\ndone"
	summary := SummarizeActivity(content, 500)
	if summary == "" {
		t.Error("expected non-empty summary")
	}
	if len(summary) > 500 {
		t.Error("summary exceeds maxLen")
	}
}

func TestSummarizeActivityTruncate(t *testing.T) {
	content := "$ " + string(make([]byte, 1000))
	summary := SummarizeActivity(content, 50)
	if len(summary) > 50 {
		t.Errorf("expected summary <= 50 chars, got %d", len(summary))
	}
}

func TestFileStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Get non-existent
	sc, err := store.Get("nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if sc != nil {
		t.Error("expected nil for nonexistent session")
	}

	// GetOrCreate
	sc, err = store.GetOrCreate("sess1", 20, 10)
	if err != nil {
		t.Fatal(err)
	}
	sc.SetProject(ProjectInfo{
		Directory: "/tmp/myproject",
		Language:  "Go",
		GitBranch: "main",
	})
	sc.AddDecision(DecisionRecord{
		Timestamp: time.Now(),
		Summary:   "Allow file read",
		ChosenKey: "y",
		Reasoning: "Safe operation",
		Confidence: 0.95,
	})

	// Save
	if err := store.Save(sc); err != nil {
		t.Fatal(err)
	}

	// Verify file exists
	fpath := filepath.Join(dir, "sess1.yaml")
	if _, err := os.Stat(fpath); err != nil {
		t.Fatalf("expected file at %s", fpath)
	}

	// Load from fresh store
	store2, err := NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := store2.Get("sess1")
	if err != nil {
		t.Fatal(err)
	}
	if loaded == nil {
		t.Fatal("expected loaded context")
	}
	if loaded.Project.Language != "Go" {
		t.Errorf("expected language 'Go', got %q", loaded.Project.Language)
	}
	if len(loaded.Decisions) != 1 {
		t.Errorf("expected 1 decision, got %d", len(loaded.Decisions))
	}

	// Delete
	if err := store.Delete("sess1"); err != nil {
		t.Fatal(err)
	}
	sc, _ = store.Get("sess1")
	if sc != nil {
		t.Error("expected nil after delete")
	}
}
