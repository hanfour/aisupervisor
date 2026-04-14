package company

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"

	"github.com/hanfourmini/aisupervisor/internal/ai"
	"github.com/hanfourmini/aisupervisor/internal/knowledge"
)

type ReviewStrategy string

const (
	ReviewLight    ReviewStrategy = "light"
	ReviewStandard ReviewStrategy = "standard"
	ReviewDebate   ReviewStrategy = "debate"
)

type Finding struct {
	ID       string `json:"id"`
	File     string `json:"file"`
	Line     int    `json:"line,omitempty"`
	Severity string `json:"severity"`
	Body     string `json:"body"`
	Source   string `json:"source"`
}

type DebateResult struct {
	Status   string    `json:"status"`
	Summary  string    `json:"summary"`
	Comments []Finding `json:"comments"`
}

func selectStrategy(diffLines, fileCount int, debateThreshold, lightMaxLines, lightMaxFiles int) ReviewStrategy {
	if diffLines >= debateThreshold {
		return ReviewDebate
	}
	if diffLines < lightMaxLines && fileCount <= lightMaxFiles {
		return ReviewLight
	}
	return ReviewStandard
}

// shouldEscalateReview returns true if the changed files span 3+ communities
// or touch any god node, indicating a cross-cutting change that needs debate review.
func shouldEscalateReview(files []string, graph *knowledge.CodeGraph) bool {
	if graph == nil || len(files) == 0 {
		return false
	}

	// Check for god node touches
	godSet := make(map[string]bool, len(graph.GodNodes))
	for _, g := range graph.GodNodes {
		godSet[g] = true
	}
	for _, f := range files {
		if godSet[f] {
			return true
		}
	}

	// Count distinct communities touched
	communityIDs := make(map[int]bool)
	for _, f := range files {
		if c := knowledge.GetCommunityForFile(graph, f); c != nil {
			communityIDs[c.ID] = true
		}
	}
	return len(communityIDs) >= 3
}

// extractChangedFiles parses a unified diff and returns the file paths that were changed.
func extractChangedFiles(diff string) []string {
	var files []string
	seen := make(map[string]bool)
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "+++ b/") {
			path := strings.TrimPrefix(line, "+++ b/")
			if !seen[path] {
				seen[path] = true
				files = append(files, path)
			}
		}
	}
	return files
}

func severityRank(s string) int {
	switch s {
	case "CRITICAL":
		return 3
	case "HIGH":
		return 2
	case "MEDIUM":
		return 1
	default:
		return 0
	}
}

// mergeFindings deduplicates findings by file:line, keeping higher severity.
func mergeFindings(all []Finding) []Finding {
	type key struct {
		file string
		line int
	}
	best := make(map[key]Finding)
	for _, f := range all {
		k := key{f.File, f.Line}
		if existing, ok := best[k]; !ok || severityRank(f.Severity) > severityRank(existing.Severity) {
			best[k] = f
		}
	}

	result := make([]Finding, 0, len(best))
	for _, f := range best {
		result = append(result, f)
	}
	sort.Slice(result, func(i, j int) bool {
		return severityRank(result[i].Severity) > severityRank(result[j].Severity)
	})

	for i := range result {
		result[i].ID = fmt.Sprintf("#%d", i+1)
	}
	return result
}

// tallyVotes returns findings that survive voting.
// A finding survives if net score (KEEP=+1, DROP=-1) >= 1.
func tallyVotes(findings []Finding, votes ...map[string]string) []Finding {
	scores := make(map[string]int)
	for _, v := range votes {
		for id, decision := range v {
			switch decision {
			case "KEEP":
				scores[id]++
			case "DROP":
				scores[id]--
			}
		}
	}

	var survived []Finding
	for _, f := range findings {
		if scores[f.ID] >= 1 {
			survived = append(survived, f)
		}
	}
	return survived
}

// runDebate executes the full 3-round debate review pipeline.
// The caller is responsible for setting a timeout on ctx.
func runDebate(ctx context.Context, cp ai.ChatProvider, diff, pkbContext string, analysisModel, voteModel, synthesisModel string, fastConverge int, lang string) (*DebateResult, error) {

	// Round 1: Parallel Analysis
	var wg sync.WaitGroup
	var findingsA, findingsB []Finding
	var errA, errB error

	wg.Add(2)
	go func() {
		defer wg.Done()
		findingsA, errA = runAnalysisAgent(ctx, cp, diff, pkbContext, "impact", analysisModel, lang)
	}()
	go func() {
		defer wg.Done()
		findingsB, errB = runAnalysisAgent(ctx, cp, diff, pkbContext, "quality", analysisModel, lang)
	}()
	wg.Wait()

	if errA != nil && errB != nil {
		return nil, fmt.Errorf("both analysis agents failed: %w; %w", errA, errB)
	}

	var allFindings []Finding
	if errA == nil {
		allFindings = append(allFindings, findingsA...)
	}
	if errB == nil {
		allFindings = append(allFindings, findingsB...)
	}
	merged := mergeFindings(allFindings)

	// Fast convergence
	if len(merged) == 0 {
		return &DebateResult{Status: "APPROVED", Summary: "No issues found"}, nil
	}
	if len(merged) <= fastConverge {
		return runSynthesis(ctx, cp, merged, synthesisModel, lang)
	}

	// Round 2: Mailbox Voting
	var votes1, votes2 map[string]string
	var errV1, errV2 error

	wg.Add(2)
	go func() {
		defer wg.Done()
		votes1, errV1 = runVoteAgent(ctx, cp, merged, voteModel, lang)
	}()
	go func() {
		defer wg.Done()
		votes2, errV2 = runVoteAgent(ctx, cp, merged, voteModel, lang)
	}()
	wg.Wait()

	survived := merged
	if errV1 == nil && errV2 == nil {
		survived = tallyVotes(merged, votes1, votes2)
	} else if errV1 == nil {
		// One voter available — use single-voter tally (finding survives if KEEP)
		log.Printf("debate: vote agent 2 failed (%v), using single voter", errV2)
		survived = tallyVotes(merged, votes1, votes1)
	} else if errV2 == nil {
		log.Printf("debate: vote agent 1 failed (%v), using single voter", errV1)
		survived = tallyVotes(merged, votes2, votes2)
	} else {
		log.Printf("debate: both vote agents failed (v1=%v, v2=%v), passing all %d findings to synthesis", errV1, errV2, len(merged))
	}

	if len(survived) == 0 {
		return &DebateResult{Status: "APPROVED", Summary: "All findings dropped by vote"}, nil
	}

	// Round 3: Synthesis
	return runSynthesis(ctx, cp, survived, synthesisModel, lang)
}

func runAnalysisAgent(ctx context.Context, cp ai.ChatProvider, diff, pkbContext, role, model, lang string) ([]Finding, error) {
	var systemPrompt string
	if role == "impact" {
		systemPrompt = impactAnalystPrompt(lang)
	} else {
		systemPrompt = qualityAuditorPrompt(lang)
	}
	userPrompt := fmt.Sprintf("Diff:\n```\n%s\n```", diff)
	if pkbContext != "" {
		userPrompt = pkbContext + "\n\n" + userPrompt
	}

	msgs := []ai.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}
	text, err := ai.ChatWithModelOrFallback(ctx, cp, msgs, model)
	if err != nil {
		return nil, err
	}

	var result struct {
		Findings []Finding `json:"findings"`
	}
	extracted := extractChatJSON(text)
	if err := json.Unmarshal([]byte(extracted), &result); err != nil {
		return nil, fmt.Errorf("parse analysis: %w", err)
	}
	for i := range result.Findings {
		result.Findings[i].Source = role
	}
	return result.Findings, nil
}

func runVoteAgent(ctx context.Context, cp ai.ChatProvider, findings []Finding, model, lang string) (map[string]string, error) {
	findingsJSON, err := json.Marshal(findings)
	if err != nil {
		return nil, fmt.Errorf("marshal findings: %w", err)
	}
	msgs := []ai.ChatMessage{
		{Role: "system", Content: voteAgentPrompt(lang)},
		{Role: "user", Content: string(findingsJSON)},
	}
	text, err := ai.ChatWithModelOrFallback(ctx, cp, msgs, model)
	if err != nil {
		return nil, err
	}

	var result map[string]string
	extracted := extractChatJSON(text)
	if err := json.Unmarshal([]byte(extracted), &result); err != nil {
		return nil, fmt.Errorf("parse votes: %w", err)
	}
	return result, nil
}

func runSynthesis(ctx context.Context, cp ai.ChatProvider, findings []Finding, model, lang string) (*DebateResult, error) {
	findingsJSON, err := json.Marshal(findings)
	if err != nil {
		return nil, fmt.Errorf("marshal findings: %w", err)
	}
	msgs := []ai.ChatMessage{
		{Role: "system", Content: synthesisPrompt(lang)},
		{Role: "user", Content: string(findingsJSON)},
	}
	text, err := ai.ChatWithModelOrFallback(ctx, cp, msgs, model)
	if err != nil {
		return nil, err
	}

	var result DebateResult
	extracted := extractChatJSON(text)
	if err := json.Unmarshal([]byte(extracted), &result); err != nil {
		return nil, fmt.Errorf("parse synthesis: %w", err)
	}
	return &result, nil
}

// runSinglePassReview runs a single ChatProvider call for light/standard reviews.
func runSinglePassReview(ctx context.Context, cp ai.ChatProvider, diff, pkbContext, model, lang string, strategy ReviewStrategy) (*DebateResult, error) {
	systemPrompt := singlePassReviewPrompt(lang, strategy)
	userPrompt := fmt.Sprintf("Diff:\n```\n%s\n```", diff)
	if pkbContext != "" {
		userPrompt = pkbContext + "\n\n" + userPrompt
	}

	msgs := []ai.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}
	text, err := ai.ChatWithModelOrFallback(ctx, cp, msgs, model)
	if err != nil {
		return nil, err
	}

	var result DebateResult
	extracted := extractChatJSON(text)
	if err := json.Unmarshal([]byte(extracted), &result); err != nil {
		return nil, fmt.Errorf("parse review: %w", err)
	}
	return &result, nil
}

// --- Prompts ---

func impactAnalystPrompt(lang string) string {
	if lang == "en" {
		return `You are an Impact Analyst reviewing code changes.
Focus on: broken references, deleted API consumers, missing migrations, contract violations.
Search the diff for removed exports, renamed functions, or changed signatures.

Respond with JSON only:
{"findings": [{"file": "path", "line": 42, "severity": "CRITICAL|HIGH|MEDIUM", "body": "description"}]}
If no issues found, respond: {"findings": []}`
	}
	return `你是一位影響分析師，負責審查程式碼變更。
重點：斷裂的引用、被刪除的 API 使用者、缺失的 migration、合約違反。
搜尋 diff 中被移除的 export、重新命名的函式、或變更的簽名。

只用 JSON 回應：
{"findings": [{"file": "路徑", "line": 42, "severity": "CRITICAL|HIGH|MEDIUM", "body": "描述"}]}
如果沒有問題：{"findings": []}`
}

func qualityAuditorPrompt(lang string) string {
	if lang == "en" {
		return `You are a Quality Auditor reviewing code changes.
Focus on: code quality, security vulnerabilities, missing tests, error handling, OWASP issues.
Check naming conventions, edge cases, and adherence to project patterns.

Respond with JSON only:
{"findings": [{"file": "path", "line": 42, "severity": "CRITICAL|HIGH|MEDIUM", "body": "description"}]}
If no issues found, respond: {"findings": []}`
	}
	return `你是一位品質稽核師，負責審查程式碼變更。
重點：程式碼品質、安全漏洞、缺失的測試、錯誤處理、OWASP 問題。
檢查命名規範、邊界情況、以及是否符合專案模式。

只用 JSON 回應：
{"findings": [{"file": "路徑", "line": 42, "severity": "CRITICAL|HIGH|MEDIUM", "body": "描述"}]}
如果沒有問題：{"findings": []}`
}

func voteAgentPrompt(lang string) string {
	if lang == "en" {
		return `You are a review finding validator. For each finding, vote KEEP or DROP.
KEEP = real issue worth fixing. DROP = false positive, nitpick, or already handled.
Respond with JSON only: {"#1": "KEEP", "#2": "DROP", ...}`
	}
	return `你是一位審查結果驗證者。對每個 finding 投票 KEEP 或 DROP。
KEEP = 真正需要修復的問題。DROP = 誤報、吹毛求疵、或已處理。
只用 JSON 回應：{"#1": "KEEP", "#2": "DROP", ...}`
}

func synthesisPrompt(lang string) string {
	if lang == "en" {
		return `Synthesize the following code review findings into a final verdict.
If any CRITICAL or HIGH findings exist, status is "CHANGES_REQUESTED".
If only MEDIUM findings, status is "APPROVED".
Respond with JSON only:
{"status": "APPROVED|CHANGES_REQUESTED", "summary": "one line", "comments": [{"file": "...", "line": 0, "severity": "...", "body": "..."}]}`
	}
	return `將以下程式碼審查結果合成為最終判決。
如果有 CRITICAL 或 HIGH 的 finding，status 為 "CHANGES_REQUESTED"。
如果只有 MEDIUM，status 為 "APPROVED"。
只用 JSON 回應：
{"status": "APPROVED|CHANGES_REQUESTED", "summary": "一行摘要", "comments": [{"file": "...", "line": 0, "severity": "...", "body": "..."}]}`
}

func singlePassReviewPrompt(lang string, strategy ReviewStrategy) string {
	detail := "Be thorough."
	if strategy == ReviewLight {
		detail = "Be concise — this is a small change."
	}
	if lang == "en" {
		return fmt.Sprintf(`Review the following code diff. %s
Check: correctness, security, error handling, test coverage.
Only flag CRITICAL or HIGH issues for rejection. MEDIUM issues get APPROVED with notes.

Respond with JSON only:
{"status": "APPROVED|CHANGES_REQUESTED", "summary": "one line", "comments": [{"file": "...", "line": 0, "severity": "CRITICAL|HIGH|MEDIUM", "body": "..."}]}`, detail)
	}
	if strategy == ReviewLight {
		detail = "請簡潔 — 這是小幅變更。"
	} else {
		detail = "請仔細審查。"
	}
	return fmt.Sprintf(`審查以下程式碼 diff。%s
檢查：正確性、安全性、錯誤處理、測試覆蓋率。
只有 CRITICAL 或 HIGH 問題才退回。MEDIUM 問題給予 APPROVED 並附註。

只用 JSON 回應：
{"status": "APPROVED|CHANGES_REQUESTED", "summary": "一行摘要", "comments": [{"file": "...", "line": 0, "severity": "CRITICAL|HIGH|MEDIUM", "body": "..."}]}`, detail)
}
