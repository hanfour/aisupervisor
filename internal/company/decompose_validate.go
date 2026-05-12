package company

import (
	"fmt"
	"strings"
)

// Layer identifiers used by the PRD-decomposition layer-coverage
// contract (see decomposeFromPRDSystemPrompt). These are the eight
// "MANDATORY COVERAGE" layers that DecomposeFromPRD asks the AI to
// emit at least one task per. Keeping the identifiers here (rather
// than embedding them in prompt strings) lets the validator
// cross-check the AI output against the same list the prompt
// enumerates.
const (
	layerData        = "data"
	layerAPI         = "api"
	layerFrontend    = "frontend"
	layerIntegration = "integration"
	layerJobs        = "jobs"
	layerTests       = "tests"
	layerDocs        = "docs"
	layerInfra       = "infra"
)

// canonicalLayers lists the eight layers in execution order (data
// before API, API before frontend, …). Topological inference uses
// this ordering to emit dependency hints: a frontend task whose
// titles references the API layer becomes blockedBy the API task,
// not the other way round.
var canonicalLayers = []string{
	layerData, layerAPI, layerFrontend, layerIntegration,
	layerJobs, layerTests, layerDocs, layerInfra,
}

// classificationPriority is the order classifyTaskLayer scans
// keyword lists in — distinct from canonicalLayers, which is the
// execution order. Work-type layers (tests, docs, infra) take
// priority over implementation layers so "Add tests for /login
// endpoint" classifies as tests, not api. Specialised impl layers
// (jobs, integration) come before generic impl (frontend, api,
// data) so "Queue consumer for the orders API" classifies as jobs.
var classificationPriority = []string{
	layerTests, layerDocs, layerInfra,
	layerJobs, layerIntegration,
	layerFrontend, layerAPI, layerData,
}

// layerKeywords maps each layer to substring tokens (lowercase) that,
// when found in a task's title or description, classify the task into
// that layer. Tokens were chosen from the prompt's own examples
// ("schema/*.sql" → data, "endpoint" → api, etc.) and the keywords
// product engineers most often use when naming tasks. The check is
// substring-based and case-insensitive; the first matching layer in
// canonicalLayers order wins so a "test for backend API" task is
// classified as tests, not api.
var layerKeywords = map[string][]string{
	layerData:        {"schema", "migration", "database", "ddl", "table ", "index", "rls", "audit log"},
	layerAPI:         {"endpoint", "api ", "handler", "route", "service", "middleware", "rest", "graphql"},
	layerFrontend:    {"frontend", "component", "page", " ui ", "ui:", "ui/", "spa", "liff", "screen", "view", "form"},
	layerIntegration: {"webhook", "oauth", "sdk", "integration", "third-party", "third party", "callback"},
	layerJobs:        {"cron", "queue", "worker", "background job", "scheduler", "consumer", "pipeline"},
	layerTests:       {"test ", "tests ", "unit test", "integration test", "e2e", "spec.ts", "_test.go", "fixture"},
	layerDocs:        {"documentation", "docs:", "docs/", "readme", "api reference", "runbook", "doc:"},
	layerInfra:       {"dockerfile", "docker compose", "compose.yml", "ci/cd", "ci pipeline", "github actions", ".github/", "deployment", "infra", "k8s", "kubernetes"},
}

// requireNonEmptyDecomposition is the silent-drop guard. AI providers
// occasionally return valid-but-empty JSON ({"tasks": []}) when the
// prompt is too vague or the context window was clipped; the previous
// code would silently emit zero tasks and surface "Generated 0 tasks"
// in the event log, leaving users staring at an empty project. This
// helper turns that into a hard error with the raw AI text (truncated)
// so users see WHY decomposition produced nothing instead of having to
// guess from the absence.
func requireNonEmptyDecomposition(label string, count int, rawAI string) error {
	if count > 0 {
		return nil
	}
	snippet := rawAI
	const maxSnippet = 400
	if len(snippet) > maxSnippet {
		snippet = snippet[:maxSnippet] + "… [truncated]"
	}
	return fmt.Errorf("%s produced zero tasks — AI response did not yield any actionable work (raw: %s)", label, snippet)
}

// classifyTaskLayer assigns a task to one of the eight canonical
// layers by substring-matching its title and description against
// layerKeywords. Returns "" when no keyword matches. The search is
// deliberately ordered by canonicalLayers so tests trump api when a
// title contains both ("test the user API" → tests).
//
// out-of-scope: <layer> markers (lowercase) short-circuit to that
// layer — the prompt instructs the AI to use this exact form when a
// PRD genuinely doesn't need a layer, and the validator treats those
// markers as legitimate coverage rather than missing tasks.
func classifyTaskLayer(title, description string) string {
	hay := strings.ToLower(title + " " + description)

	if strings.HasPrefix(strings.TrimSpace(strings.ToLower(title)), "out-of-scope:") {
		for _, layer := range canonicalLayers {
			if strings.Contains(hay, layer) {
				return layer
			}
		}
	}

	for _, layer := range classificationPriority {
		for _, kw := range layerKeywords[layer] {
			if strings.Contains(hay, kw) {
				return layer
			}
		}
	}
	return ""
}

// LayerCoverage reports which canonical layers a decomposed task
// list addresses. Present lists the layers with ≥1 task (regardless
// of whether the task is implementation or "out-of-scope: <layer>"
// marker — both count as coverage for audit purposes). Missing
// lists the canonical layers with zero tasks AND no out-of-scope
// marker. Callers can use this to decide whether to fail
// decomposition (hard error), kick off the Phase-0 review (gap
// fill), or simply emit a warning.
type LayerCoverage struct {
	Present []string
	Missing []string
}

// analyseLayerCoverage walks the decomposed tasks and returns a
// LayerCoverage report. Tasks that don't match any layer keyword
// are not counted toward coverage; they simply don't contribute to
// any layer. The check is *additive*: a task can only mark its one
// best-matched layer present.
func analyseLayerCoverage(tasks []decomposedTask) LayerCoverage {
	seen := make(map[string]bool, len(canonicalLayers))
	for _, t := range tasks {
		layer := classifyTaskLayer(t.Title, t.Description)
		if layer != "" {
			seen[layer] = true
		}
	}

	cov := LayerCoverage{
		Present: make([]string, 0, len(canonicalLayers)),
		Missing: make([]string, 0, len(canonicalLayers)),
	}
	for _, layer := range canonicalLayers {
		if seen[layer] {
			cov.Present = append(cov.Present, layer)
		} else {
			cov.Missing = append(cov.Missing, layer)
		}
	}
	return cov
}

// inferDependencyHints produces basic topological ordering hints
// based on the canonical layer order: tests depend on the last impl
// task (api / frontend / data) emitted before them, frontend depends
// on the last api task before it, etc. The function returns a map
// from task index → list of predecessor indices (NOT IDs — the
// caller resolves indices to task IDs after persisting).
//
// Hints are conservative: only one immediate predecessor per task
// (the nearest earlier task in a layer this layer canonically
// depends on). Cycles are impossible by construction because the
// edges always go from a later canonicalLayers entry to an earlier
// one. Tasks whose layer is "" (unclassified) get no hints.
//
// This is a heuristic, not a contract: it cannot replace explicit
// task.DependsOn declarations from the AI, but it dramatically
// reduces the "all tasks are ready, workers thrash on conflicting
// files" failure mode that ships of unannotated decompositions.
func inferDependencyHints(tasks []decomposedTask) map[int][]int {
	// layerDependsOn lists, for each layer, the layers whose tasks
	// must complete before this layer's tasks can start. Kept narrow
	// to avoid over-serialising work: tests depend on impl
	// (api / frontend / data), frontend depends on api, docs depend
	// on everything substantive (api / frontend / data), and infra
	// is independent.
	layerDependsOn := map[string][]string{
		layerAPI:      {layerData},
		layerFrontend: {layerAPI},
		layerTests:    {layerAPI, layerFrontend, layerData},
		layerDocs:     {layerAPI, layerFrontend, layerData},
	}

	hints := make(map[int][]int)
	layers := make([]string, len(tasks))
	for i, t := range tasks {
		layers[i] = classifyTaskLayer(t.Title, t.Description)
	}

	for i, layer := range layers {
		deps := layerDependsOn[layer]
		if len(deps) == 0 {
			continue
		}
		for _, dep := range deps {
			// nearest earlier task in dep layer (highest j < i)
			latest := -1
			for j := 0; j < i; j++ {
				if layers[j] == dep {
					latest = j
				}
			}
			if latest >= 0 {
				hints[i] = append(hints[i], latest)
			}
		}
	}
	return hints
}

// formatLayerList renders []string layer names as a human-readable
// comma-joined string suitable for event messages and error text.
// Empty input renders as "<none>" so logs don't show ambiguous "".
func formatLayerList(layers []string) string {
	if len(layers) == 0 {
		return "<none>"
	}
	return strings.Join(layers, ", ")
}
