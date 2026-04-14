package worker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadGraphReport_Exists(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "graphify-out")
	os.MkdirAll(outDir, 0755)
	os.WriteFile(filepath.Join(outDir, "GRAPH_REPORT.md"), []byte("# Graph Report\nNode: main.go (degree 15)"), 0644)

	report := readGraphReport(dir)
	if report == "" {
		t.Error("expected non-empty report")
	}
	if !strings.Contains(report, "Project Architecture") {
		t.Error("expected header in report")
	}
	if !strings.Contains(report, "main.go") {
		t.Error("expected content from report file")
	}
}

func TestReadGraphReport_NotExists(t *testing.T) {
	dir := t.TempDir()
	report := readGraphReport(dir)
	if report != "" {
		t.Errorf("expected empty report for missing file, got %q", report)
	}
}

func TestReadGraphReport_TruncatesLongContent(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "graphify-out")
	os.MkdirAll(outDir, 0755)
	// Write content longer than maxGraphReportLen (4000)
	longContent := strings.Repeat("x", 5000)
	os.WriteFile(filepath.Join(outDir, "GRAPH_REPORT.md"), []byte(longContent), 0644)

	report := readGraphReport(dir)
	if len(report) > 4200 { // 4000 content + header/footer
		t.Errorf("report should be truncated, got length %d", len(report))
	}
}
