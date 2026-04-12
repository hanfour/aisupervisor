package company

import "testing"

func TestSelectStrategy(t *testing.T) {
	tests := []struct {
		name      string
		diffLines int
		fileCount int
		want      ReviewStrategy
	}{
		{"tiny change", 10, 1, ReviewLight},
		{"small multi-file", 30, 4, ReviewStandard},
		{"medium change", 100, 5, ReviewStandard},
		{"large change", 400, 10, ReviewDebate},
		{"exactly at threshold", 300, 5, ReviewDebate},
		{"light boundary files", 40, 3, ReviewLight},
		{"light boundary lines", 50, 2, ReviewStandard},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectStrategy(tt.diffLines, tt.fileCount, 300, 50, 3)
			if got != tt.want {
				t.Errorf("selectStrategy(%d, %d) = %s, want %s", tt.diffLines, tt.fileCount, got, tt.want)
			}
		})
	}
}

func TestMergeFindings(t *testing.T) {
	all := []Finding{
		{File: "main.go", Line: 10, Severity: "HIGH", Body: "issue A", Source: "impact"},
		{File: "main.go", Line: 10, Severity: "MEDIUM", Body: "issue A dup", Source: "quality"},
		{File: "util.go", Line: 5, Severity: "CRITICAL", Body: "issue B", Source: "impact"},
	}
	merged := mergeFindings(all)
	if len(merged) != 2 {
		t.Fatalf("mergeFindings: got %d, want 2", len(merged))
	}
	for _, f := range merged {
		if f.File == "main.go" && f.Severity != "HIGH" {
			t.Errorf("dedup should keep higher severity: got %s", f.Severity)
		}
	}
	// CRITICAL should be first (higher rank)
	if merged[0].Severity != "CRITICAL" {
		t.Errorf("first finding should be CRITICAL, got %s", merged[0].Severity)
	}
}

func TestTallyVotes(t *testing.T) {
	findings := []Finding{
		{ID: "#1"}, {ID: "#2"}, {ID: "#3"},
	}
	votes1 := map[string]string{"#1": "KEEP", "#2": "DROP", "#3": "KEEP"}
	votes2 := map[string]string{"#1": "KEEP", "#2": "DROP", "#3": "DROP"}

	survived := tallyVotes(findings, votes1, votes2)
	if len(survived) != 1 || survived[0].ID != "#1" {
		t.Errorf("tallyVotes: got %v, want only #1", survived)
	}
}
