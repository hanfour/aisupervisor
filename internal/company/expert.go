package company

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/hanfourmini/aisupervisor/internal/knowledge"
)

// ExpertDomain identifies a review domain.
type ExpertDomain string

const (
	DomainSecurity     ExpertDomain = "security"
	DomainPerformance  ExpertDomain = "performance"
	DomainRefactoring  ExpertDomain = "refactoring"
	DomainTesting      ExpertDomain = "testing"
	DomainFrontend     ExpertDomain = "frontend"
	DomainBackend      ExpertDomain = "backend"
	DomainDatabase     ExpertDomain = "database"
	DomainAPI          ExpertDomain = "api_contract"
	DomainConcurrency  ExpertDomain = "concurrency"
	DomainArchitecture ExpertDomain = "architecture"
)

// Expert represents a domain-specific code reviewer.
type Expert struct {
	Domain       ExpertDomain
	Name         string   // human-readable name for reports
	SystemPrompt string   // domain-specific review instruction
	FilePatterns []string // glob patterns, e.g. "*.go", "*.svelte"
	Keywords     []string // content keywords that trigger selection
	Model        string   // suggested model ("" = default sonnet)
}

// ExecMode indicates how an expert should be executed.
type ExecMode string

const (
	ExecAPI ExecMode = "api"
	ExecCLI ExecMode = "cli"
)

// SelectedExpert is an Expert chosen for a particular review, with context.
type SelectedExpert struct {
	Expert
	AssignedFiles []string
	Reason        string
	Mode          ExecMode
}

// ExpertRegistry holds the catalog of available domain experts.
type ExpertRegistry struct {
	experts []Expert
}

// NewExpertRegistry creates a registry populated with the 10 default domain experts.
func NewExpertRegistry() *ExpertRegistry {
	return &ExpertRegistry{
		experts: []Expert{
			{
				Domain: DomainSecurity,
				Name:   "安全審查員",
				SystemPrompt: `You are a security-focused code reviewer. Analyze the provided diff for:
- Hardcoded secrets, tokens, passwords, API keys
- SQL injection, command injection, XSS vulnerabilities
- Insecure crypto usage or weak hashing
- Path traversal, directory escape
- Unsafe deserialization or eval usage
- Missing input validation or sanitization
- Exposed sensitive data in logs or error messages
- Improper access control or authentication checks

Only report real security issues, not style preferences.
Be concise and specific — cite the exact line and pattern.

Output findings as a JSON array:
[{"file":"...","line":N,"severity":"CRITICAL|HIGH|MEDIUM","body":"..."}]
If no issues found, output an empty array: []`,
				FilePatterns: []string{"*"},
				Keywords:     []string{"password", "token", "crypto", "secret", "exec", "eval", "injection"},
				Model:        "",
			},
			{
				Domain: DomainPerformance,
				Name:   "效能審查員",
				SystemPrompt: `You are a performance-focused code reviewer. Analyze the provided diff for:
- Inefficient algorithms (O(n^2) where O(n) is possible)
- Unnecessary allocations inside hot loops
- Missing pre-allocation for slices/maps
- Unbounded goroutine creation
- Cache misses and poor data locality
- N+1 query patterns
- Blocking operations in critical paths
- Large copies where pointers would suffice

Only report real performance issues with measurable impact.
Be concise and specific — cite the exact line and explain the cost.

Output findings as a JSON array:
[{"file":"...","line":N,"severity":"CRITICAL|HIGH|MEDIUM","body":"..."}]
If no issues found, output an empty array: []`,
				FilePatterns: []string{"*"},
				Keywords:     []string{"loop", "append", "goroutine", "cache", "O(n", "allocat"},
				Model:        "",
			},
			{
				Domain: DomainRefactoring,
				Name:   "重構審查員",
				SystemPrompt: `You are a code quality reviewer focused on maintainability. Analyze the provided diff for:
- Functions too long (>50 lines) or doing too many things
- Deep nesting (>3 levels)
- Duplicated logic that should be extracted
- Poor naming that obscures intent
- Missing error handling or silently swallowed errors
- Dead code or unreachable branches
- Tight coupling that hinders testability
- Violation of single-responsibility principle

Only report structural issues that hurt maintainability, not cosmetic preferences.
Be concise — suggest the refactoring, don't just name the smell.

Output findings as a JSON array:
[{"file":"...","line":N,"severity":"CRITICAL|HIGH|MEDIUM","body":"..."}]
If no issues found, output an empty array: []`,
				FilePatterns: []string{"*"},
				Keywords:     []string{}, // always a candidate
				Model:        "",
			},
			{
				Domain: DomainTesting,
				Name:   "測試審查員",
				SystemPrompt: `You are a testing-focused code reviewer. Analyze the provided diff for:
- Missing test coverage for new public functions
- Tests that don't assert meaningful behavior
- Fragile tests coupled to implementation details
- Missing edge-case coverage (nil, empty, boundary)
- Test helpers that silently swallow errors
- Flaky patterns (time-dependent, order-dependent)
- Mocking that hides real integration issues
- Missing table-driven test patterns where applicable

Only report real testing gaps, not pedantic coverage demands.
Be concise — suggest the test case, not just the gap.

Output findings as a JSON array:
[{"file":"...","line":N,"severity":"CRITICAL|HIGH|MEDIUM","body":"..."}]
If no issues found, output an empty array: []`,
				FilePatterns: []string{"*_test.go", "*.test.*", "*_test.ts", "*.spec.*"},
				Keywords:     []string{"mock", "assert", "expect", "t.Run", "t.Fatal"},
				Model:        "",
			},
			{
				Domain: DomainFrontend,
				Name:   "前端審查員",
				SystemPrompt: `You are a frontend-focused code reviewer. Analyze the provided diff for:
- Reactive state bugs (stale closures, missing dependencies)
- Accessibility issues (missing ARIA, keyboard nav, contrast)
- XSS via unsafe HTML rendering
- Memory leaks (unsubscribed listeners, unmounted intervals)
- Performance issues (unnecessary re-renders, large bundles)
- Missing error/loading states in UI components
- Inconsistent styling or broken responsive layout
- Event handler anti-patterns

Only report real UI/UX bugs, not style preferences.
Be concise and specific — cite the component and pattern.

Output findings as a JSON array:
[{"file":"...","line":N,"severity":"CRITICAL|HIGH|MEDIUM","body":"..."}]
If no issues found, output an empty array: []`,
				FilePatterns: []string{"*.svelte", "*.tsx", "*.jsx", "*.css", "*.vue"},
				Keywords:     []string{"bind:", "$:", "dispatch", "useState", "onClick"},
				Model:        "",
			},
			{
				Domain: DomainBackend,
				Name:   "後端審查員",
				SystemPrompt: `You are a backend-focused Go code reviewer. Analyze the provided diff for:
- Goroutine leaks (missing cancellation, unbounded spawning)
- Context misuse (background context in request handlers)
- HTTP handler issues (missing timeouts, improper status codes)
- Resource leaks (unclosed files, connections, response bodies)
- Error wrapping without context
- Race conditions in shared state
- Improper use of channels (deadlocks, panics on closed channels)
- Missing graceful shutdown handling

Only report real backend bugs, not Go style suggestions.
Be concise — cite the goroutine/handler and the failure mode.

Output findings as a JSON array:
[{"file":"...","line":N,"severity":"CRITICAL|HIGH|MEDIUM","body":"..."}]
If no issues found, output an empty array: []`,
				FilePatterns: []string{"*.go"},
				Keywords:     []string{"goroutine", "chan", "context.", "http.", "Handler"},
				Model:        "",
			},
			{
				Domain: DomainDatabase,
				Name:   "資料庫審查員",
				SystemPrompt: `You are a database-focused code reviewer. Analyze the provided diff for:
- SQL injection via string concatenation
- Missing indexes for WHERE/JOIN columns
- N+1 query patterns
- Schema migrations without rollback plan
- Missing transactions for multi-step operations
- Unbounded queries (no LIMIT on user-facing endpoints)
- Data type mismatches between app and schema
- Missing NOT NULL constraints or foreign keys

Only report real data integrity or query issues.
Be concise — cite the query/migration and the risk.

Output findings as a JSON array:
[{"file":"...","line":N,"severity":"CRITICAL|HIGH|MEDIUM","body":"..."}]
If no issues found, output an empty array: []`,
				FilePatterns: []string{"*.sql", "*.prisma", "*migration*"},
				Keywords:     []string{"SELECT", "INSERT", "UPDATE", "DELETE", "JOIN", "INDEX"},
				Model:        "",
			},
			{
				Domain: DomainAPI,
				Name:   "API 契約審查員",
				SystemPrompt: `You are an API contract reviewer. Analyze the provided diff for:
- Breaking changes to public API signatures
- Missing or inconsistent JSON/YAML struct tags
- Incorrect HTTP methods or status codes
- Missing request validation or error responses
- Inconsistent naming conventions across endpoints
- Missing versioning for breaking changes
- Undocumented new endpoints or fields
- Backward-incompatible schema changes

Only report real API contract issues, not implementation details.
Be concise — cite the endpoint/field and the contract violation.

Output findings as a JSON array:
[{"file":"...","line":N,"severity":"CRITICAL|HIGH|MEDIUM","body":"..."}]
If no issues found, output an empty array: []`,
				FilePatterns: []string{"*.go", "*.proto", "*.graphql"},
				Keywords:     []string{"json:", "yaml:", "Handler", "Route", "endpoint", "API"},
				Model:        "",
			},
			{
				Domain: DomainConcurrency,
				Name:   "並行審查員",
				SystemPrompt: `You are a concurrency-focused code reviewer. Analyze the provided diff for:
- Data races on shared variables
- Mutex misuse (Lock without Unlock, wrong lock scope)
- Deadlock potential (lock ordering violations)
- Channel misuse (send on closed, unbuffered blocking)
- Incorrect atomic operations
- WaitGroup misuse (Add after Wait, wrong count)
- Missing synchronization in concurrent map access
- Context cancellation not propagated

Only report real concurrency bugs, not theoretical races.
Be concise — cite the goroutine/lock and the race window.

Output findings as a JSON array:
[{"file":"...","line":N,"severity":"CRITICAL|HIGH|MEDIUM","body":"..."}]
If no issues found, output an empty array: []`,
				FilePatterns: []string{"*.go"},
				Keywords:     []string{"sync.", "Mutex", "RWMutex", "chan ", "atomic.", "WaitGroup"},
				Model:        "",
			},
			{
				Domain: DomainArchitecture,
				Name:   "架構審查員",
				SystemPrompt: `You are an architecture reviewer assessing structural impact. Analyze the provided diff for:
- Changes to god nodes (highly-depended-upon files)
- Cross-module coupling that violates package boundaries
- Circular dependency introduction
- Interface changes that ripple across consumers
- Missing abstraction layers for external dependencies
- Violation of dependency inversion principle
- Large structural changes without migration plan
- Breaking changes to shared types or protocols

Only report real architectural risks, not minor refactoring opportunities.
Be concise — cite the module boundary and the coupling vector.

Output findings as a JSON array:
[{"file":"...","line":N,"severity":"CRITICAL|HIGH|MEDIUM","body":"..."}]
If no issues found, output an empty array: []`,
				FilePatterns: []string{"*"},
				Keywords:     []string{}, // triggered by god nodes and cross-community
				Model:        "opus",
			},
		},
	}
}

// scoredExpert pairs an expert with its selection score.
type scoredExpert struct {
	expert Expert
	score  int
	reason string
}

// SelectExperts chooses which domain experts to involve for a review based on
// changed files, diff content, code graph topology, and phase-0 results.
func (r *ExpertRegistry) SelectExperts(changedFiles []string, diff string, graph *knowledge.CodeGraph, phase0 *Phase0Report) []SelectedExpert {
	diffLower := strings.ToLower(diff)

	scored := make([]scoredExpert, len(r.experts))
	for i, expert := range r.experts {
		scored[i] = scoredExpert{expert: expert}
	}

	// (a) Score each expert by file pattern and keyword matches.
	for i, se := range scored {
		var reasons []string

		// File pattern scoring: +3 per matching file
		fileScore := 0
		for _, f := range changedFiles {
			if matchesAnyPattern(f, se.expert.FilePatterns) {
				fileScore += 3
			}
		}
		if fileScore > 0 {
			reasons = append(reasons, "file pattern match")
		}

		// Keyword scoring: +2 per keyword found in diff, capped at 10
		kwScore := 0
		for _, kw := range se.expert.Keywords {
			if strings.Contains(diffLower, strings.ToLower(kw)) {
				kwScore += 2
				if len(reasons) == 0 || !strings.HasPrefix(reasons[len(reasons)-1], "keyword match:") {
					reasons = append(reasons, "keyword match: "+kw)
				}
			}
		}
		if kwScore > 10 {
			kwScore = 10
		}

		scored[i].score = fileScore + kwScore
		scored[i].reason = strings.Join(reasons, "; ")
	}

	// (b) Force-add rules from Phase0 failures.
	if phase0 != nil {
		for _, res := range phase0.Results {
			if res.Passed {
				continue
			}
			nameLower := strings.ToLower(res.Check.Name)
			for i := range scored {
				switch {
				case strings.Contains(nameLower, "go") || strings.Contains(nameLower, "vet"):
					if scored[i].expert.Domain == DomainBackend {
						scored[i].score = 100
						scored[i].reason = "phase0 failure: " + res.Check.Name
					}
				case strings.Contains(nameLower, "lint"):
					if scored[i].expert.Domain == DomainRefactoring {
						scored[i].score = 100
						scored[i].reason = "phase0 failure: " + res.Check.Name
					}
				case strings.Contains(nameLower, "test"):
					if scored[i].expert.Domain == DomainTesting {
						scored[i].score = 100
						scored[i].reason = "phase0 failure: " + res.Check.Name
					}
				case strings.Contains(nameLower, "type"):
					if scored[i].expert.Domain == DomainFrontend {
						scored[i].score = 100
						scored[i].reason = "phase0 failure: " + res.Check.Name
					}
				}
			}
		}
	}

	// Force-add rules from code graph.
	if graph != nil {
		// God node touched → force architecture
		for _, f := range changedFiles {
			for _, godNode := range graph.GodNodes {
				if f == godNode {
					for i := range scored {
						if scored[i].expert.Domain == DomainArchitecture {
							scored[i].score = 100
							scored[i].reason = "god node touched: " + f
						}
					}
				}
			}
		}

		// Cross-community: changed files span 3+ communities → force architecture
		communityIDs := map[int]bool{}
		for _, f := range changedFiles {
			if c := knowledge.GetCommunityForFile(graph, f); c != nil {
				communityIDs[c.ID] = true
			}
		}
		if len(communityIDs) >= 3 {
			for i := range scored {
				if scored[i].expert.Domain == DomainArchitecture {
					if scored[i].score < 100 {
						scored[i].score = 100
						scored[i].reason = "cross-community change (spans 3+ communities)"
					}
				}
			}
		}
	}

	// (c) Sort by score descending.
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// (d/e) Take minimum 2, maximum 5.
	selected := make([]scoredExpert, 0, 5)
	for _, se := range scored {
		if len(selected) >= 5 {
			break
		}
		if se.score > 0 {
			selected = append(selected, se)
		}
	}

	// If we need to fill to minimum 2, add refactoring as default filler.
	if len(selected) < 2 {
		for _, se := range scored {
			if se.expert.Domain == DomainRefactoring {
				// Check if already selected
				alreadySelected := false
				for _, s := range selected {
					if s.expert.Domain == DomainRefactoring {
						alreadySelected = true
						break
					}
				}
				if !alreadySelected {
					se.reason = "default filler (minimum 2 experts)"
					selected = append(selected, se)
				}
				break
			}
		}
	}

	// If still under 2, add the next best scoring expert (even with score 0).
	if len(selected) < 2 {
		for _, se := range scored {
			alreadySelected := false
			for _, s := range selected {
				if s.expert.Domain == se.expert.Domain {
					alreadySelected = true
					break
				}
			}
			if !alreadySelected {
				if se.reason == "" {
					se.reason = "default filler (minimum 2 experts)"
				}
				selected = append(selected, se)
				if len(selected) >= 2 {
					break
				}
			}
		}
	}

	// (f) Build SelectedExpert results.
	result := make([]SelectedExpert, 0, len(selected))
	for _, se := range selected {
		assignedFiles := computeAssignedFiles(changedFiles, se.expert.FilePatterns)
		mode := ExecAPI
		if len(assignedFiles) > 3 {
			mode = ExecCLI
		}
		result = append(result, SelectedExpert{
			Expert:        se.expert,
			AssignedFiles: assignedFiles,
			Reason:        se.reason,
			Mode:          mode,
		})
	}

	return result
}

// matchesAnyPattern checks if a file path matches any of the given glob patterns.
// Patterns are matched against the base filename. Patterns containing "migration"
// use substring matching since filepath.Match handles them poorly.
func matchesAnyPattern(filePath string, patterns []string) bool {
	base := filepath.Base(filePath)
	for _, pattern := range patterns {
		// Handle wildcard-all pattern
		if pattern == "*" {
			return true
		}
		// Handle substring patterns like "*migration*"
		if strings.Contains(pattern, "migration") {
			trimmed := strings.Trim(pattern, "*")
			if strings.Contains(base, trimmed) {
				return true
			}
			// Also check the full path for migration patterns
			if strings.Contains(filePath, trimmed) {
				return true
			}
			continue
		}
		// Standard glob match against base filename
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
	}
	return false
}

// computeAssignedFiles returns the subset of changedFiles that match the expert's
// file patterns. If patterns include "*", all files are assigned.
func computeAssignedFiles(changedFiles []string, patterns []string) []string {
	for _, p := range patterns {
		if p == "*" {
			// All files assigned
			result := make([]string, len(changedFiles))
			copy(result, changedFiles)
			return result
		}
	}
	var matched []string
	for _, f := range changedFiles {
		if matchesAnyPattern(f, patterns) {
			matched = append(matched, f)
		}
	}
	return matched
}
