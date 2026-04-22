package company

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Convention represents a learned code review pattern that the council
// accumulates across review sessions, so accepted practices are not
// repeatedly flagged.
type Convention struct {
	ID          string       `yaml:"id" json:"id"`
	Domain      ExpertDomain `yaml:"domain" json:"domain"`
	Pattern     string       `yaml:"pattern" json:"pattern"`             // machine-matchable pattern description
	Description string       `yaml:"description" json:"description"`     // human-readable
	FileGlob    string       `yaml:"file_glob" json:"file_glob"`         // applicable scope, "*.go" or "*"
	Source      string       `yaml:"source" json:"source"`               // "review:{taskID}"
	AcceptedAt  time.Time    `yaml:"accepted_at" json:"accepted_at"`
	AcceptCount int          `yaml:"accept_count" json:"accept_count"`   // how many reviews confirmed this
	LastUsed    time.Time    `yaml:"last_used" json:"last_used"`         // last time this was matched in filtering
}

// ExpertFinding extends Finding with expert attribution.
// Defined here so conventions can match against it; also used by synthesis.
type ExpertFinding struct {
	Finding
	Expert     ExpertDomain `json:"expert"`
	Principle  string       `json:"principle,omitempty"`
	Confidence float64      `json:"confidence,omitempty"`
}

// ConventionStore manages the persistence and querying of learned conventions.
type ConventionStore struct {
	mu          sync.RWMutex
	conventions []Convention
	indexPath   string // path to index.yaml
	mdPath      string // path to conventions.md
	idSeq       int
}

// conventionIndex is the YAML serialization wrapper.
type conventionIndex struct {
	Conventions []Convention `yaml:"conventions"`
}

// NewConventionStore creates a ConventionStore, creating the conventions/
// subdirectory under dataDir if needed and loading index.yaml if it exists.
func NewConventionStore(dataDir string) (*ConventionStore, error) {
	convDir := filepath.Join(dataDir, "conventions")
	if err := os.MkdirAll(convDir, 0o755); err != nil {
		return nil, fmt.Errorf("create conventions dir: %w", err)
	}

	cs := &ConventionStore{
		indexPath: filepath.Join(convDir, "index.yaml"),
		mdPath:    filepath.Join(convDir, "conventions.md"),
	}

	// Load existing index if present.
	data, err := os.ReadFile(cs.indexPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("read index.yaml: %w", err)
		}
		// File doesn't exist — start empty.
		return cs, nil
	}

	var idx conventionIndex
	if err := yaml.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("parse index.yaml: %w", err)
	}
	cs.conventions = idx.Conventions

	// Initialize idSeq from max existing ID.
	for _, c := range cs.conventions {
		if n := parseConvID(c.ID); n > cs.idSeq {
			cs.idSeq = n
		}
	}

	return cs, nil
}

// parseConvID extracts the numeric part from "conv-NNN".
func parseConvID(id string) int {
	if !strings.HasPrefix(id, "conv-") {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimPrefix(id, "conv-"))
	if err != nil {
		return 0
	}
	return n
}

// Save writes both index.yaml and conventions.md.
func (cs *ConventionStore) Save() error {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	// Write index.yaml
	idx := conventionIndex{Conventions: cs.conventions}
	data, err := yaml.Marshal(&idx)
	if err != nil {
		return fmt.Errorf("marshal conventions: %w", err)
	}
	if err := os.WriteFile(cs.indexPath, data, 0o644); err != nil {
		return fmt.Errorf("write index.yaml: %w", err)
	}

	// Write conventions.md
	md := cs.renderMarkdown()
	if err := os.WriteFile(cs.mdPath, []byte(md), 0o644); err != nil {
		return fmt.Errorf("write conventions.md: %w", err)
	}

	return nil
}

// renderMarkdown generates the human-readable Markdown representation
// of all conventions, grouped by domain.
func (cs *ConventionStore) renderMarkdown() string {
	var sb strings.Builder
	sb.WriteString("# Project Conventions\n\n")
	sb.WriteString("> Auto-maintained by council review system. Manual edits are preserved.\n\n")

	// Group by domain, preserving insertion order of domains.
	domainOrder := []ExpertDomain{}
	groups := map[ExpertDomain][]Convention{}
	for _, c := range cs.conventions {
		if _, seen := groups[c.Domain]; !seen {
			domainOrder = append(domainOrder, c.Domain)
		}
		groups[c.Domain] = append(groups[c.Domain], c)
	}

	// Sort domains alphabetically for stable output.
	sort.Slice(domainOrder, func(i, j int) bool {
		return domainOrder[i] < domainOrder[j]
	})

	for _, domain := range domainOrder {
		convs := groups[domain]
		sb.WriteString(fmt.Sprintf("## %s\n\n", domain))

		for _, c := range convs {
			sb.WriteString(fmt.Sprintf("### %s: %s\n", c.ID, c.Pattern))
			sb.WriteString(fmt.Sprintf("- **Applicable**: `%s`\n", c.FileGlob))
			sb.WriteString(fmt.Sprintf("- **Description**: %s\n", c.Description))
			sb.WriteString(fmt.Sprintf("- **Source**: %s (%s)\n", c.Source, c.AcceptedAt.Format("2006-01-02")))
			sb.WriteString(fmt.Sprintf("- **References**: %d\n", c.AcceptCount))
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// Propose adds a new convention to the store, assigning it a unique ID.
// Returns the assigned ID (e.g., "conv-001").
func (cs *ConventionStore) Propose(conv Convention) string {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.idSeq++
	conv.ID = fmt.Sprintf("conv-%03d", cs.idSeq)
	now := time.Now()
	conv.AcceptedAt = now
	conv.AcceptCount = 0
	conv.LastUsed = now
	cs.conventions = append(cs.conventions, conv)
	return conv.ID
}

// Accept marks a convention as confirmed by another review, incrementing
// its AcceptCount and updating LastUsed.
func (cs *ConventionStore) Accept(id string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	for i := range cs.conventions {
		if cs.conventions[i].ID == id {
			cs.conventions[i].AcceptCount++
			cs.conventions[i].LastUsed = time.Now()
			return
		}
	}
}

// FindRelevant returns conventions matching the given domain and file path
// that have been accepted at least twice (AcceptCount >= 2).
func (cs *ConventionStore) FindRelevant(domain ExpertDomain, filePath string) []Convention {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	base := filepath.Base(filePath)
	var results []Convention

	for _, c := range cs.conventions {
		if c.AcceptCount < 2 {
			continue
		}
		// Domain must match. Empty query domain matches all conventions.
		if domain != "" && c.Domain != "" && c.Domain != domain {
			continue
		}
		// FileGlob matching. Empty glob or "*" matches everything.
		if c.FileGlob != "" && c.FileGlob != "*" {
			matched, err := filepath.Match(c.FileGlob, base)
			if err != nil || !matched {
				continue
			}
		}
		results = append(results, c)
	}
	return results
}

// MatchesFinding returns the first convention that matches an ExpertFinding
// based on domain and word similarity (Jaccard > 20%). Only accepted
// conventions (AcceptCount >= 2) are considered. Returns nil if no match.
func (cs *ConventionStore) MatchesFinding(f ExpertFinding) *Convention {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	for i := range cs.conventions {
		c := &cs.conventions[i]
		if c.AcceptCount < 2 {
			continue
		}
		// Domain must match.
		if c.Domain != "" && c.Domain != f.Expert {
			continue
		}
		// Word similarity check on pattern vs finding body.
		// Threshold set at 0.20 — low enough to catch reformulated descriptions
		// of the same concept (e.g., "error ignored" vs "return value discarded")
		// while filtering truly unrelated findings.
		if wordSimilarity(c.Pattern, f.Body) > 0.20 {
			// Return a copy to avoid holding the lock on internal data.
			conv := *c
			return &conv
		}
	}
	return nil
}

// Decay removes conventions that haven't been used recently and have low
// acceptance. Removes conventions where time.Since(LastUsed) > maxAge AND
// AcceptCount < minUses. Returns the count of removed conventions.
func (cs *ConventionStore) Decay(maxAge time.Duration, minUses int) int {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	now := time.Now()
	var kept []Convention
	removed := 0

	for _, c := range cs.conventions {
		if now.Sub(c.LastUsed) > maxAge && c.AcceptCount < minUses {
			removed++
			continue
		}
		kept = append(kept, c)
	}
	cs.conventions = kept
	return removed
}

// wordSimilarity computes the Jaccard similarity between two strings,
// treating them as sets of whitespace-delimited, lowercased words.
// Returns |intersection| / |union|, or 0 if both strings are empty.
func wordSimilarity(a, b string) float64 {
	wordsA := toWordSet(a)
	wordsB := toWordSet(b)
	if len(wordsA) == 0 && len(wordsB) == 0 {
		return 0
	}

	intersection := 0
	for w := range wordsA {
		if wordsB[w] {
			intersection++
		}
	}

	union := len(wordsA)
	for w := range wordsB {
		if !wordsA[w] {
			union++
		}
	}

	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

// toWordSet splits a string into lowercased tokens and returns them as a set.
// It splits on whitespace and common punctuation (dots, commas, colons, etc.)
// to improve matching of identifiers like "yaml.Unmarshal".
func toWordSet(s string) map[string]bool {
	set := make(map[string]bool)
	tokenize := func(r rune) bool {
		switch r {
		case ' ', '\t', '\n', '\r', '.', ',', ':', ';', '(', ')', '[', ']', '{', '}', '/', '\\':
			return true
		}
		return false
	}
	for _, w := range strings.FieldsFunc(strings.ToLower(s), tokenize) {
		if w != "" {
			set[w] = true
		}
	}
	return set
}
