package company

import (
	"strings"
	"testing"
)

// TestDecomposeFromPRDSystemPrompt_FullStackCoverage pins the
// full-stack coverage requirement into the decompose system prompt.
// A regression that drops any of the eight layer keywords would let
// DecomposeFromPRD fall back to single-slice (e.g. SQL-only) output —
// the bug this prompt was rewritten to fix.
func TestDecomposeFromPRDSystemPrompt_FullStackCoverage(t *testing.T) {
	cases := []struct {
		lang     string
		keywords []string
	}{
		{
			lang: "en",
			keywords: []string{
				"FULL-STACK",
				"MANDATORY COVERAGE",
				"out-of-scope",
				"Database",
				"Backend",
				"Frontend",
				"integration",
				"Background",
				"Tests",
				"Documentation",
				"Infrastructure",
			},
		},
		{
			lang: "zh-TW",
			keywords: []string{
				"全棧任務清單",
				"強制涵蓋",
				"out-of-scope",
				"資料庫",
				"後端",
				"前端",
				"整合",
				"背景任務",
				"測試",
				"文件",
				"基礎設施",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.lang, func(t *testing.T) {
			got := decomposeFromPRDSystemPrompt(tc.lang)
			for _, kw := range tc.keywords {
				if !strings.Contains(got, kw) {
					t.Errorf("decomposeFromPRDSystemPrompt(%q) missing required keyword %q — full-stack coverage rules must mention every layer", tc.lang, kw)
				}
			}
		})
	}
}

// TestDecomposeFromPRDSystemPrompt_NoTaskCountCap verifies the prompt
// no longer caps task count at 15, which previously biased AI toward
// the "easiest 15" (typically database-only) and silently dropped
// frontend/backend/integration/test/infra layers.
func TestDecomposeFromPRDSystemPrompt_NoTaskCountCap(t *testing.T) {
	for _, lang := range []string{"en", "zh-TW"} {
		got := decomposeFromPRDSystemPrompt(lang)
		// The old prompt said "Generate 3-15 tasks" / "生成 3-15 個任務".
		// New prompt should explicitly say not to cap at 15.
		if strings.Contains(got, "3-15") {
			t.Errorf("%s prompt still contains 3-15 cap — should be removed", lang)
		}
		// Sanity: must mention a realistic typical scope so AI knows the floor.
		if !strings.Contains(got, "25-50") {
			t.Errorf("%s prompt should mention 25-50 task typical scope as a floor anchor", lang)
		}
	}
}

// TestDecomposeFromPRDSystemPrompt_OutputFormatStillJSON verifies the
// output-format requirement (valid JSON, no markdown) is preserved.
// Without this, AI may add markdown fences that extractChatJSON has to
// strip — the existing extractor handles fenced JSON, but the cleaner
// contract is "no fences" so we keep the prompt authoritative.
func TestDecomposeFromPRDSystemPrompt_OutputFormatStillJSON(t *testing.T) {
	for _, lang := range []string{"en", "zh-TW"} {
		got := decomposeFromPRDSystemPrompt(lang)
		if !strings.Contains(got, `{"tasks":`) {
			t.Errorf("%s prompt missing JSON output template", lang)
		}
		if !strings.Contains(strings.ToLower(got), "json") {
			t.Errorf("%s prompt missing JSON requirement", lang)
		}
	}
}
