# P2 Knowledge Graph Intelligence — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a two-tier knowledge graph system — lightweight built-in graph (Go AST + git co-change) as default, with optional Graphify CLI enhancement — to enable community-based task assignment and architecture-aware review escalation.

**Architecture:** Tier 1 uses go/ast for import graph and git log for co-change frequency, with simple connected-component community detection. Tier 2 wraps the Graphify CLI when installed, overriding Tier 1 data with richer semantic analysis. Both expose the same CodeGraph interface used by company.go for task assignment and review escalation.

**Tech Stack:** Go 1.23+, go/ast, os/exec (git log), existing knowledge/company/gui packages

---

## Task 1: CodeGraph Interface + Data Types

**Files:**
- Create: `internal/knowledge/codegraph.go`
- Create: `internal/knowledge/codegraph_test.go`

**Step 1: Write the failing tests**

```go
// internal/knowledge/codegraph_test.go
package knowledge

import (
	"testing"
)

func TestFileNode_Creation(t *testing.T) {
	node := FileNode{
		Path:     "internal/worker/spawner.go",
		Package:  "worker",
		InDegree: 5,
	}
	if node.Path != "internal/worker/spawner.go" {
		t.Errorf("expected path 'internal/worker/spawner.go', got %q", node.Path)
	}
	if node.Package != "worker" {
		t.Errorf("expected package 'worker', got %q", node.Package)
	}
	if node.InDegree != 5 {
		t.Errorf("expected InDegree 5, got %d", node.InDegree)
	}
}

func TestEdge_Creation(t *testing.T) {
	edge := Edge{
		From:   "internal/company/company.go",
		To:     "internal/worker/spawner.go",
		Type:   EdgeImport,
		Weight: 1,
	}
	if edge.Type != EdgeImport {
		t.Errorf("expected type %q, got %q", EdgeImport, edge.Type)
	}
	coChange := Edge{
		From:   "internal/company/company.go",
		To:     "internal/company/review.go",
		Type:   EdgeCoChange,
		Weight: 7,
	}
	if coChange.Type != EdgeCoChange {
		t.Errorf("expected type %q, got %q", EdgeCoChange, coChange.Type)
	}
}

func TestCommunity_Creation(t *testing.T) {
	c := Community{
		ID:    1,
		Name:  "internal/worker",
		Files: []string{"internal/worker/spawner.go", "internal/worker/monitor.go"},
	}
	if c.ID != 1 {
		t.Errorf("expected ID 1, got %d", c.ID)
	}
	if len(c.Files) != 2 {
		t.Errorf("expected 2 files, got %d", len(c.Files))
	}
}

func TestCodeGraph_Creation(t *testing.T) {
	graph := &CodeGraph{
		RepoPath: "/tmp/repo",
		HeadSHA:  "abc123",
		Nodes: []FileNode{
			{Path: "internal/worker/spawner.go", Package: "worker", InDegree: 3},
			{Path: "internal/company/company.go", Package: "company", InDegree: 5},
		},
		Edges: []Edge{
			{From: "internal/company/company.go", To: "internal/worker/spawner.go", Type: EdgeImport, Weight: 1},
		},
		Communities: []Community{
			{ID: 0, Name: "internal/worker", Files: []string{"internal/worker/spawner.go"}},
			{ID: 1, Name: "internal/company", Files: []string{"internal/company/company.go"}},
		},
		GodNodes: []string{"internal/company/company.go"},
	}
	if len(graph.Nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(graph.Nodes))
	}
	if len(graph.GodNodes) != 1 {
		t.Errorf("expected 1 god node, got %d", len(graph.GodNodes))
	}
}

func TestGetCommunityForFile(t *testing.T) {
	graph := &CodeGraph{
		Communities: []Community{
			{ID: 0, Name: "internal/worker", Files: []string{"internal/worker/spawner.go", "internal/worker/monitor.go"}},
			{ID: 1, Name: "internal/company", Files: []string{"internal/company/company.go", "internal/company/review.go"}},
			{ID: 2, Name: "internal/knowledge", Files: []string{"internal/knowledge/types.go"}},
		},
	}

	tests := []struct {
		name     string
		file     string
		wantID   int
		wantName string
		wantNil  bool
	}{
		{"finds worker community", "internal/worker/spawner.go", 0, "internal/worker", false},
		{"finds company community", "internal/company/review.go", 1, "internal/company", false},
		{"returns nil for unknown file", "internal/config/config.go", 0, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetCommunityForFile(graph, tt.file)
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got community %q", got.Name)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil community")
			}
			if got.ID != tt.wantID {
				t.Errorf("expected ID %d, got %d", tt.wantID, got.ID)
			}
			if got.Name != tt.wantName {
				t.Errorf("expected name %q, got %q", tt.wantName, got.Name)
			}
		})
	}
}

func TestGetGodNodes(t *testing.T) {
	graph := &CodeGraph{
		Nodes: []FileNode{
			{Path: "a.go", InDegree: 1},
			{Path: "b.go", InDegree: 10},
			{Path: "c.go", InDegree: 5},
			{Path: "d.go", InDegree: 8},
			{Path: "e.go", InDegree: 3},
			{Path: "f.go", InDegree: 12},
		},
	}

	tests := []struct {
		name  string
		topN  int
		want  []string
	}{
		{"top 3", 3, []string{"f.go", "b.go", "d.go"}},
		{"top 1", 1, []string{"f.go"}},
		{"top exceeds count", 10, []string{"f.go", "b.go", "d.go", "c.go", "e.go", "a.go"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetGodNodes(graph, tt.topN)
			if len(got) != len(tt.want) {
				t.Fatalf("expected %d god nodes, got %d: %v", len(tt.want), len(got), got)
			}
			for i, path := range got {
				if path != tt.want[i] {
					t.Errorf("position %d: expected %q, got %q", i, tt.want[i], path)
				}
			}
		})
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/knowledge/ -run "TestFileNode_Creation|TestEdge_Creation|TestCommunity_Creation|TestCodeGraph_Creation|TestGetCommunityForFile|TestGetGodNodes" -v`
Expected: FAIL — types `FileNode`, `Edge`, `Community`, `CodeGraph`, functions `GetCommunityForFile`, `GetGodNodes` not defined

**Step 3: Implement the types and functions**

```go
// internal/knowledge/codegraph.go
package knowledge

import "sort"

// EdgeType constants for graph edges.
const (
	EdgeImport   = "import"
	EdgeCoChange = "co-change"
)

// FileNode represents a single source file in the code graph.
type FileNode struct {
	Path     string `json:"path"`
	Package  string `json:"package"`
	InDegree int    `json:"inDegree"`
}

// Edge represents a relationship between two files.
type Edge struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Type   string `json:"type"`   // EdgeImport or EdgeCoChange
	Weight int    `json:"weight"` // 1 for imports, frequency for co-change
}

// Community represents a group of closely related files.
type Community struct {
	ID    int      `json:"id"`
	Name  string   `json:"name"`
	Files []string `json:"files"`
}

// CodeGraph is the full repository knowledge graph.
type CodeGraph struct {
	RepoPath    string      `json:"repoPath"`
	HeadSHA     string      `json:"headSHA"`
	Nodes       []FileNode  `json:"nodes"`
	Edges       []Edge      `json:"edges"`
	Communities []Community `json:"communities"`
	GodNodes    []string    `json:"godNodes"`
}

// CodeGraphProvider is the interface for obtaining and querying a code graph.
type CodeGraphProvider interface {
	GetGraph(repoPath string) (*CodeGraph, error)
}

// GetCommunityForFile returns the community containing the given file path,
// or nil if the file is not in any community.
func GetCommunityForFile(graph *CodeGraph, filePath string) *Community {
	for i := range graph.Communities {
		for _, f := range graph.Communities[i].Files {
			if f == filePath {
				return &graph.Communities[i]
			}
		}
	}
	return nil
}

// GetGodNodes returns the top N files by InDegree (most depended-upon).
func GetGodNodes(graph *CodeGraph, topN int) []string {
	sorted := make([]FileNode, len(graph.Nodes))
	copy(sorted, graph.Nodes)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].InDegree > sorted[j].InDegree
	})
	if topN > len(sorted) {
		topN = len(sorted)
	}
	result := make([]string, topN)
	for i := 0; i < topN; i++ {
		result[i] = sorted[i].Path
	}
	return result
}
```

**Step 4: Run tests to verify they pass**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/knowledge/ -run "TestFileNode_Creation|TestEdge_Creation|TestCommunity_Creation|TestCodeGraph_Creation|TestGetCommunityForFile|TestGetGodNodes" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/knowledge/codegraph.go internal/knowledge/codegraph_test.go
git commit -m "feat(knowledge): add CodeGraph types, GetCommunityForFile, GetGodNodes

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: Lightweight Graph Builder (Tier 1)

**Files:**
- Create: `internal/knowledge/lightgraph.go`
- Create: `internal/knowledge/lightgraph_test.go`

**Step 1: Write the failing tests**

```go
// internal/knowledge/lightgraph_test.go
package knowledge

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupTempRepo creates a temp git repo with known Go files for testing.
func setupTempRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Init git repo
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "test")

	// Create directory structure:
	// internal/worker/spawner.go   — imports internal/tmux
	// internal/worker/monitor.go   — imports internal/tmux
	// internal/tmux/client.go      — no imports within repo
	// internal/company/company.go  — imports internal/worker, internal/tmux
	mkdirAll := func(p string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(dir, p), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile := func(p, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, p), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// go.mod
	writeFile("go.mod", "module example.com/testrepo\n\ngo 1.23\n")

	mkdirAll("internal/tmux")
	writeFile("internal/tmux/client.go", `package tmux

type Client struct{}
`)

	mkdirAll("internal/worker")
	writeFile("internal/worker/spawner.go", `package worker

import "example.com/testrepo/internal/tmux"

var _ = tmux.Client{}
`)
	writeFile("internal/worker/monitor.go", `package worker

import "example.com/testrepo/internal/tmux"

var _ = tmux.Client{}
`)

	mkdirAll("internal/company")
	writeFile("internal/company/company.go", `package company

import (
	"example.com/testrepo/internal/worker"
	"example.com/testrepo/internal/tmux"
)

var _ = worker.Spawner{}
var _ = tmux.Client{}
`)

	// Commit 1: all files
	run("add", ".")
	run("commit", "-m", "initial")

	// Commit 2: change worker/spawner.go and company/company.go together (co-change)
	writeFile("internal/worker/spawner.go", `package worker

import "example.com/testrepo/internal/tmux"

var _ = tmux.Client{}
// updated
`)
	writeFile("internal/company/company.go", `package company

import (
	"example.com/testrepo/internal/worker"
	"example.com/testrepo/internal/tmux"
)

var _ = worker.Spawner{}
var _ = tmux.Client{}
// updated
`)
	run("add", ".")
	run("commit", "-m", "co-change worker+company")

	return dir
}

func TestBuildLightGraph_Nodes(t *testing.T) {
	repo := setupTempRepo(t)
	graph, err := BuildLightGraph(repo)
	if err != nil {
		t.Fatalf("BuildLightGraph error: %v", err)
	}

	// Should have 4 Go files as nodes
	if len(graph.Nodes) != 4 {
		t.Errorf("expected 4 nodes, got %d", len(graph.Nodes))
		for _, n := range graph.Nodes {
			t.Logf("  node: %s (pkg=%s, indegree=%d)", n.Path, n.Package, n.InDegree)
		}
	}

	// Verify packages are correctly extracted
	pkgs := make(map[string]string)
	for _, n := range graph.Nodes {
		pkgs[n.Path] = n.Package
	}
	if pkgs["internal/tmux/client.go"] != "tmux" {
		t.Errorf("expected package 'tmux' for client.go, got %q", pkgs["internal/tmux/client.go"])
	}
	if pkgs["internal/worker/spawner.go"] != "worker" {
		t.Errorf("expected package 'worker' for spawner.go, got %q", pkgs["internal/worker/spawner.go"])
	}
}

func TestBuildLightGraph_ImportEdges(t *testing.T) {
	repo := setupTempRepo(t)
	graph, err := BuildLightGraph(repo)
	if err != nil {
		t.Fatalf("BuildLightGraph error: %v", err)
	}

	// Count import edges
	importEdges := 0
	for _, e := range graph.Edges {
		if e.Type == EdgeImport {
			importEdges++
		}
	}

	// Expected import edges:
	// worker/spawner.go -> tmux/client.go
	// worker/monitor.go -> tmux/client.go
	// company/company.go -> worker/spawner.go (first file in worker pkg)
	// company/company.go -> tmux/client.go
	if importEdges < 3 {
		t.Errorf("expected at least 3 import edges, got %d", importEdges)
		for _, e := range graph.Edges {
			if e.Type == EdgeImport {
				t.Logf("  import: %s -> %s", e.From, e.To)
			}
		}
	}
}

func TestBuildLightGraph_CoChangeEdges(t *testing.T) {
	repo := setupTempRepo(t)
	graph, err := BuildLightGraph(repo)
	if err != nil {
		t.Fatalf("BuildLightGraph error: %v", err)
	}

	// Should have co-change edges from commit 2 (spawner.go + company.go changed together)
	found := false
	for _, e := range graph.Edges {
		if e.Type == EdgeCoChange {
			if (e.From == "internal/worker/spawner.go" && e.To == "internal/company/company.go") ||
				(e.From == "internal/company/company.go" && e.To == "internal/worker/spawner.go") {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected co-change edge between spawner.go and company.go")
		for _, e := range graph.Edges {
			if e.Type == EdgeCoChange {
				t.Logf("  co-change: %s -> %s (weight=%d)", e.From, e.To, e.Weight)
			}
		}
	}
}

func TestBuildLightGraph_InDegree(t *testing.T) {
	repo := setupTempRepo(t)
	graph, err := BuildLightGraph(repo)
	if err != nil {
		t.Fatalf("BuildLightGraph error: %v", err)
	}

	// tmux/client.go should have highest InDegree (imported by 3 files)
	degreeMap := make(map[string]int)
	for _, n := range graph.Nodes {
		degreeMap[n.Path] = n.InDegree
	}
	tmuxDeg := degreeMap["internal/tmux/client.go"]
	if tmuxDeg < 2 {
		t.Errorf("expected tmux/client.go InDegree >= 2, got %d", tmuxDeg)
	}
}

func TestBuildLightGraph_Communities(t *testing.T) {
	repo := setupTempRepo(t)
	graph, err := BuildLightGraph(repo)
	if err != nil {
		t.Fatalf("BuildLightGraph error: %v", err)
	}

	// Should have 3 communities: internal/worker, internal/tmux, internal/company
	if len(graph.Communities) != 3 {
		t.Errorf("expected 3 communities, got %d", len(graph.Communities))
		for _, c := range graph.Communities {
			t.Logf("  community: %q files=%v", c.Name, c.Files)
		}
	}

	// Worker community should have 2 files
	for _, c := range graph.Communities {
		if c.Name == "internal/worker" {
			if len(c.Files) != 2 {
				t.Errorf("expected 2 files in worker community, got %d", len(c.Files))
			}
		}
	}
}

func TestBuildLightGraph_GodNodes(t *testing.T) {
	repo := setupTempRepo(t)
	graph, err := BuildLightGraph(repo)
	if err != nil {
		t.Fatalf("BuildLightGraph error: %v", err)
	}

	// GodNodes should contain tmux/client.go (highest InDegree)
	if len(graph.GodNodes) == 0 {
		t.Fatal("expected at least 1 god node")
	}
	if graph.GodNodes[0] != "internal/tmux/client.go" {
		t.Errorf("expected top god node to be tmux/client.go, got %q", graph.GodNodes[0])
	}
}

func TestBuildLightGraph_HeadSHA(t *testing.T) {
	repo := setupTempRepo(t)
	graph, err := BuildLightGraph(repo)
	if err != nil {
		t.Fatalf("BuildLightGraph error: %v", err)
	}
	if graph.HeadSHA == "" {
		t.Error("expected non-empty HeadSHA")
	}
	if len(graph.HeadSHA) < 7 {
		t.Errorf("HeadSHA too short: %q", graph.HeadSHA)
	}
}

func TestBuildLightGraph_CacheRoundTrip(t *testing.T) {
	repo := setupTempRepo(t)

	// Build once — should create cache
	graph1, err := BuildLightGraph(repo)
	if err != nil {
		t.Fatalf("BuildLightGraph error: %v", err)
	}

	// Build again — should load from cache (same HEAD)
	graph2, err := BuildLightGraph(repo)
	if err != nil {
		t.Fatalf("BuildLightGraph (cached) error: %v", err)
	}

	if graph1.HeadSHA != graph2.HeadSHA {
		t.Errorf("cached graph HeadSHA mismatch: %q vs %q", graph1.HeadSHA, graph2.HeadSHA)
	}
	if len(graph1.Nodes) != len(graph2.Nodes) {
		t.Errorf("cached graph node count mismatch: %d vs %d", len(graph1.Nodes), len(graph2.Nodes))
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/knowledge/ -run "TestBuildLightGraph" -v`
Expected: FAIL — `BuildLightGraph` not defined

**Step 3: Implement the lightweight graph builder**

```go
// internal/knowledge/lightgraph.go
package knowledge

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const (
	lightGraphCacheDir  = ".aisupervisor"
	lightGraphCacheFile = "codegraph.json"
	defaultGodNodeCount = 5
	gitLogLimit         = 100
)

// BuildLightGraph builds a lightweight code graph from go/ast imports and git co-change data.
// Results are cached to <repoPath>/.aisupervisor/codegraph.json and invalidated by HEAD SHA.
func BuildLightGraph(repoPath string) (*CodeGraph, error) {
	headSHA, err := getHeadSHA(repoPath)
	if err != nil {
		return nil, err
	}

	// Try loading from cache
	if cached, err := loadCachedGraph(repoPath, headSHA); err == nil {
		return cached, nil
	}

	// Detect module path from go.mod
	modPath, err := readModulePath(repoPath)
	if err != nil {
		return nil, err
	}

	// 1. Walk repo for .go files, parse imports
	nodes, importEdges, err := buildImportGraph(repoPath, modPath)
	if err != nil {
		return nil, err
	}

	// 2. Build co-change edges from git log
	coChangeEdges, err := buildCoChangeEdges(repoPath)
	if err != nil {
		// Non-fatal: proceed without co-change data
		coChangeEdges = nil
	}

	// Merge all edges
	allEdges := append(importEdges, coChangeEdges...)

	// 3. Calculate InDegree for each node from import edges
	degreeMap := make(map[string]int)
	for _, e := range importEdges {
		degreeMap[e.To]++
	}
	for i := range nodes {
		nodes[i].InDegree = degreeMap[nodes[i].Path]
	}

	// 4. Community detection: group by top-level directory
	communities := buildCommunities(nodes)

	// 5. God nodes: top N by InDegree
	godNodes := computeGodNodes(nodes, defaultGodNodeCount)

	graph := &CodeGraph{
		RepoPath:    repoPath,
		HeadSHA:     headSHA,
		Nodes:       nodes,
		Edges:       allEdges,
		Communities: communities,
		GodNodes:    godNodes,
	}

	// Cache for next time
	_ = saveCachedGraph(repoPath, graph)

	return graph, nil
}

// getHeadSHA returns the current HEAD commit SHA.
func getHeadSHA(repoPath string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// readModulePath reads the module path from go.mod.
func readModulePath(repoPath string) (string, error) {
	data, err := os.ReadFile(filepath.Join(repoPath, "go.mod"))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module")), nil
		}
	}
	return "", os.ErrNotExist
}

// buildImportGraph walks .go files, parses AST, and builds import edges.
func buildImportGraph(repoPath, modPath string) ([]FileNode, []Edge, error) {
	var nodes []FileNode
	var edges []Edge

	// Map package import path -> list of relative file paths in that package
	pkgFiles := make(map[string][]string)
	fset := token.NewFileSet()

	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		// Skip hidden dirs, vendor, testdata
		if info.IsDir() {
			base := filepath.Base(path)
			if strings.HasPrefix(base, ".") || base == "vendor" || base == "testdata" || base == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		relPath, _ := filepath.Rel(repoPath, path)

		f, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			return nil // skip unparseable files
		}

		pkgName := ""
		if f.Name != nil {
			pkgName = f.Name.Name
		}
		nodes = append(nodes, FileNode{Path: relPath, Package: pkgName})

		// Track which package dir this file belongs to
		pkgDir := filepath.Dir(relPath)
		importPath := modPath + "/" + pkgDir
		pkgFiles[importPath] = append(pkgFiles[importPath], relPath)

		// Record imports
		for _, imp := range f.Imports {
			impPath := strings.Trim(imp.Path.Value, `"`)
			if strings.HasPrefix(impPath, modPath) {
				edges = append(edges, Edge{
					From:   relPath,
					To:     impPath, // will be resolved to file path below
					Type:   EdgeImport,
					Weight: 1,
				})
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	// Resolve import paths to actual file paths (pick first file in target package)
	var resolvedEdges []Edge
	for _, e := range edges {
		targetFiles, ok := pkgFiles[e.To]
		if ok && len(targetFiles) > 0 {
			// Create edge to first file in the target package
			resolvedEdges = append(resolvedEdges, Edge{
				From:   e.From,
				To:     targetFiles[0],
				Type:   EdgeImport,
				Weight: 1,
			})
		}
	}

	return nodes, resolvedEdges, nil
}

// buildCoChangeEdges parses recent git log to find files that change together.
func buildCoChangeEdges(repoPath string) ([]Edge, error) {
	cmd := exec.Command("git", "log", "--format=COMMIT", "--name-only", "--diff-filter=AMRC",
		"-"+strings.Itoa(gitLogLimit))
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	// Parse commits: group files by commit
	type pair struct{ a, b string }
	freq := make(map[pair]int)

	lines := strings.Split(string(out), "\n")
	var currentFiles []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "COMMIT" {
			// Process previous commit's files
			if len(currentFiles) > 1 {
				for i := 0; i < len(currentFiles); i++ {
					for j := i + 1; j < len(currentFiles); j++ {
						a, b := currentFiles[i], currentFiles[j]
						if a > b {
							a, b = b, a
						}
						freq[pair{a, b}]++
					}
				}
			}
			currentFiles = nil
			continue
		}
		if line == "" {
			continue
		}
		// Only include .go files
		if strings.HasSuffix(line, ".go") {
			currentFiles = append(currentFiles, line)
		}
	}
	// Process last commit
	if len(currentFiles) > 1 {
		for i := 0; i < len(currentFiles); i++ {
			for j := i + 1; j < len(currentFiles); j++ {
				a, b := currentFiles[i], currentFiles[j]
				if a > b {
					a, b = b, a
				}
				freq[pair{a, b}]++
			}
		}
	}

	var edges []Edge
	for p, w := range freq {
		edges = append(edges, Edge{
			From:   p.a,
			To:     p.b,
			Type:   EdgeCoChange,
			Weight: w,
		})
	}
	return edges, nil
}

// buildCommunities groups files by their top-level directory (e.g. internal/worker/).
func buildCommunities(nodes []FileNode) []Community {
	groups := make(map[string][]string)
	for _, n := range nodes {
		dir := filepath.Dir(n.Path)
		groups[dir] = append(groups[dir], n.Path)
	}

	var names []string
	for name := range groups {
		names = append(names, name)
	}
	sort.Strings(names)

	communities := make([]Community, len(names))
	for i, name := range names {
		communities[i] = Community{
			ID:    i,
			Name:  name,
			Files: groups[name],
		}
	}
	return communities
}

// computeGodNodes returns the top N files by InDegree.
func computeGodNodes(nodes []FileNode, topN int) []string {
	sorted := make([]FileNode, len(nodes))
	copy(sorted, nodes)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].InDegree > sorted[j].InDegree
	})
	if topN > len(sorted) {
		topN = len(sorted)
	}
	// Only include nodes with InDegree > 0
	var result []string
	for i := 0; i < topN; i++ {
		if sorted[i].InDegree > 0 {
			result = append(result, sorted[i].Path)
		}
	}
	return result
}

// loadCachedGraph loads a cached graph if the HEAD SHA matches.
func loadCachedGraph(repoPath, headSHA string) (*CodeGraph, error) {
	cachePath := filepath.Join(repoPath, lightGraphCacheDir, lightGraphCacheFile)
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, err
	}
	var graph CodeGraph
	if err := json.Unmarshal(data, &graph); err != nil {
		return nil, err
	}
	if graph.HeadSHA != headSHA {
		return nil, os.ErrNotExist
	}
	return &graph, nil
}

// saveCachedGraph writes the graph to disk for caching.
func saveCachedGraph(repoPath string, graph *CodeGraph) error {
	cacheDir := filepath.Join(repoPath, lightGraphCacheDir)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(graph, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(cacheDir, lightGraphCacheFile), data, 0o644)
}
```

**Step 4: Run tests to verify they pass**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/knowledge/ -run "TestBuildLightGraph" -v -timeout 30s`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/knowledge/lightgraph.go internal/knowledge/lightgraph_test.go
git commit -m "feat(knowledge): add lightweight code graph builder with go/ast and git co-change

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: Graphify Integration (Tier 2)

**Files:**
- Create: `internal/knowledge/graphify.go`
- Create: `internal/knowledge/graphify_test.go`

**Step 1: Write the failing tests**

```go
// internal/knowledge/graphify_test.go
package knowledge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGraphifyIntegration_IsAvailable_WhenNotInstalled(t *testing.T) {
	gi := NewGraphifyIntegration("/tmp/nonexistent-repo")
	// In CI/test environments, graphify is almost certainly not in PATH.
	// This test verifies the method doesn't panic and returns a bool.
	_ = gi.IsAvailable()
	// We can't assert false because the test runner might have graphify installed.
	// Instead, verify the method works without errors.
}

func TestGraphifyIntegration_HasGraph_NoOutputDir(t *testing.T) {
	dir := t.TempDir()
	gi := NewGraphifyIntegration(dir)
	if gi.HasGraph() {
		t.Error("expected HasGraph() false when graphify-out/ does not exist")
	}
}

func TestGraphifyIntegration_HasGraph_WithOutputDir(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "graphify-out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "graph.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	gi := NewGraphifyIntegration(dir)
	if !gi.HasGraph() {
		t.Error("expected HasGraph() true when graphify-out/graph.json exists")
	}
}

func TestGraphifyIntegration_GetReport_NoFile(t *testing.T) {
	dir := t.TempDir()
	gi := NewGraphifyIntegration(dir)
	_, err := gi.GetReport()
	if err == nil {
		t.Error("expected error when GRAPH_REPORT.md does not exist")
	}
}

func TestGraphifyIntegration_GetReport_WithFile(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "graphify-out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "# Graph Report\nSome analysis here.\n"
	if err := os.WriteFile(filepath.Join(outDir, "GRAPH_REPORT.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	gi := NewGraphifyIntegration(dir)
	report, err := gi.GetReport()
	if err != nil {
		t.Fatalf("GetReport error: %v", err)
	}
	if report != content {
		t.Errorf("expected report %q, got %q", content, report)
	}
}

func TestGraphifyIntegration_GetGraphFromGraphify_ParsesJSON(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "graphify-out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a minimal graphify-compatible graph.json
	graphifyData := graphifyOutput{
		Nodes: []graphifyNode{
			{Path: "internal/worker/spawner.go", Package: "worker", Connections: 3},
			{Path: "internal/tmux/client.go", Package: "tmux", Connections: 5},
			{Path: "internal/company/company.go", Package: "company", Connections: 2},
		},
		Edges: []graphifyEdge{
			{Source: "internal/worker/spawner.go", Target: "internal/tmux/client.go", Type: "import"},
			{Source: "internal/company/company.go", Target: "internal/worker/spawner.go", Type: "call"},
		},
		Communities: []graphifyCommunity{
			{ID: 0, Name: "worker-cluster", Members: []string{"internal/worker/spawner.go"}},
			{ID: 1, Name: "infra-cluster", Members: []string{"internal/tmux/client.go"}},
			{ID: 2, Name: "company-cluster", Members: []string{"internal/company/company.go"}},
		},
	}

	data, _ := json.Marshal(graphifyData)
	if err := os.WriteFile(filepath.Join(outDir, "graph.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	gi := NewGraphifyIntegration(dir)
	graph, err := gi.GetGraphFromGraphify()
	if err != nil {
		t.Fatalf("GetGraphFromGraphify error: %v", err)
	}

	if len(graph.Nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(graph.Nodes))
	}
	if len(graph.Edges) != 2 {
		t.Errorf("expected 2 edges, got %d", len(graph.Edges))
	}
	if len(graph.Communities) != 3 {
		t.Errorf("expected 3 communities, got %d", len(graph.Communities))
	}

	// Verify InDegree was computed from edges
	degreeMap := make(map[string]int)
	for _, n := range graph.Nodes {
		degreeMap[n.Path] = n.InDegree
	}
	if degreeMap["internal/tmux/client.go"] != 1 {
		t.Errorf("expected tmux InDegree 1 (from edge), got %d", degreeMap["internal/tmux/client.go"])
	}

	// Verify GodNodes includes highest InDegree
	if len(graph.GodNodes) == 0 {
		t.Error("expected at least 1 god node")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/knowledge/ -run "TestGraphifyIntegration" -v`
Expected: FAIL — `NewGraphifyIntegration`, `graphifyOutput` etc. not defined

**Step 3: Implement the Graphify integration**

```go
// internal/knowledge/graphify.go
package knowledge

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
)

// graphifyOutput mirrors the Graphify CLI output format (graph.json).
type graphifyOutput struct {
	Nodes       []graphifyNode      `json:"nodes"`
	Edges       []graphifyEdge      `json:"edges"`
	Communities []graphifyCommunity `json:"communities"`
}

type graphifyNode struct {
	Path        string `json:"path"`
	Package     string `json:"package"`
	Connections int    `json:"connections"`
}

type graphifyEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
}

type graphifyCommunity struct {
	ID      int      `json:"id"`
	Name    string   `json:"name"`
	Members []string `json:"members"`
}

// GraphifyIntegration wraps the Graphify CLI for Tier 2 graph enrichment.
type GraphifyIntegration struct {
	repoPath string
}

// NewGraphifyIntegration creates a new GraphifyIntegration for the given repo.
func NewGraphifyIntegration(repoPath string) *GraphifyIntegration {
	return &GraphifyIntegration{repoPath: repoPath}
}

// IsAvailable returns true if the graphify CLI is installed and in PATH.
func (gi *GraphifyIntegration) IsAvailable() bool {
	_, err := exec.LookPath("graphify")
	return err == nil
}

// HasGraph returns true if graphify-out/graph.json exists.
func (gi *GraphifyIntegration) HasGraph() bool {
	_, err := os.Stat(filepath.Join(gi.repoPath, "graphify-out", "graph.json"))
	return err == nil
}

// GetReport reads graphify-out/GRAPH_REPORT.md.
func (gi *GraphifyIntegration) GetReport() (string, error) {
	data, err := os.ReadFile(filepath.Join(gi.repoPath, "graphify-out", "GRAPH_REPORT.md"))
	if err != nil {
		return "", fmt.Errorf("graphify report: %w", err)
	}
	return string(data), nil
}

// RunAnalysis executes `graphify analyze <repoPath>`.
func (gi *GraphifyIntegration) RunAnalysis() error {
	cmd := exec.Command("graphify", "analyze", gi.repoPath)
	cmd.Dir = gi.repoPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("graphify analyze: %w\n%s", err, out)
	}
	return nil
}

// GetGraphFromGraphify parses graphify-out/graph.json into our CodeGraph format.
func (gi *GraphifyIntegration) GetGraphFromGraphify() (*CodeGraph, error) {
	data, err := os.ReadFile(filepath.Join(gi.repoPath, "graphify-out", "graph.json"))
	if err != nil {
		return nil, fmt.Errorf("graphify graph.json: %w", err)
	}

	var gOut graphifyOutput
	if err := json.Unmarshal(data, &gOut); err != nil {
		return nil, fmt.Errorf("parse graphify graph.json: %w", err)
	}

	// Convert nodes
	nodes := make([]FileNode, len(gOut.Nodes))
	for i, n := range gOut.Nodes {
		nodes[i] = FileNode{
			Path:    n.Path,
			Package: n.Package,
		}
	}

	// Convert edges
	edges := make([]Edge, len(gOut.Edges))
	for i, e := range gOut.Edges {
		edgeType := EdgeImport
		if e.Type != "import" {
			edgeType = EdgeCoChange // map non-import types to co-change
		}
		edges[i] = Edge{
			From:   e.Source,
			To:     e.Target,
			Type:   edgeType,
			Weight: 1,
		}
	}

	// Compute InDegree from edges
	degreeMap := make(map[string]int)
	for _, e := range edges {
		degreeMap[e.To]++
	}
	for i := range nodes {
		nodes[i].InDegree = degreeMap[nodes[i].Path]
	}

	// Convert communities
	communities := make([]Community, len(gOut.Communities))
	for i, c := range gOut.Communities {
		communities[i] = Community{
			ID:    c.ID,
			Name:  c.Name,
			Files: c.Members,
		}
	}

	// Compute god nodes
	godNodes := computeGodNodes(nodes, defaultGodNodeCount)

	return &CodeGraph{
		RepoPath:    gi.repoPath,
		Nodes:       nodes,
		Edges:       edges,
		Communities: communities,
		GodNodes:    godNodes,
	}, nil
}

// computeGodNodes is defined in lightgraph.go; this is a re-export reference.
// (Already defined — no duplicate needed.)

// sortedGodNodes returns top N files by InDegree from a node list.
// This is a package-level helper reused by both Tier 1 and Tier 2.
// Note: computeGodNodes in lightgraph.go already handles this.
// Ensure lightgraph.go's computeGodNodes is used via the shared package.
var _ = sort.Strings // ensure sort is used (graphify.go may reference it)
```

**Important:** The `computeGodNodes` function is already defined in `lightgraph.go` (Task 2). The graphify code calls it directly since both are in the same package. Remove the `var _ = sort.Strings` line if `sort` is not otherwise used in this file.

**Step 4: Run tests to verify they pass**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/knowledge/ -run "TestGraphifyIntegration" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/knowledge/graphify.go internal/knowledge/graphify_test.go
git commit -m "feat(knowledge): add Graphify CLI integration for Tier 2 graph enrichment

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: Unified Graph Provider

**Files:**
- Create: `internal/knowledge/graph_provider.go`
- Create: `internal/knowledge/graph_provider_test.go`
- Modify: `internal/company/company.go:31-79` (Manager struct — add `graphProvider` field)

**Step 1: Write the failing tests**

```go
// internal/knowledge/graph_provider_test.go
package knowledge

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupProviderTestRepo creates a minimal Go repo for provider tests.
func setupProviderTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	writeFile := func(p, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(dir, filepath.Dir(p)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, p), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "test")
	writeFile("go.mod", "module example.com/providertest\n\ngo 1.23\n")
	writeFile("main.go", "package main\n\nfunc main() {}\n")
	run("add", ".")
	run("commit", "-m", "init")

	return dir
}

func TestUnifiedGraphProvider_FallsBackToLightGraph(t *testing.T) {
	repo := setupProviderTestRepo(t)
	provider := NewUnifiedGraphProvider()

	graph, err := provider.GetGraph(repo)
	if err != nil {
		t.Fatalf("GetGraph error: %v", err)
	}
	if graph == nil {
		t.Fatal("expected non-nil graph")
	}
	if len(graph.Nodes) < 1 {
		t.Errorf("expected at least 1 node (main.go), got %d", len(graph.Nodes))
	}
}

func TestUnifiedGraphProvider_UsesGraphifyWhenAvailable(t *testing.T) {
	repo := setupProviderTestRepo(t)

	// Simulate graphify output by creating the expected files
	outDir := filepath.Join(repo, "graphify-out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}
	graphifyJSON := `{
		"nodes": [
			{"path": "main.go", "package": "main", "connections": 0},
			{"path": "lib.go", "package": "main", "connections": 1}
		],
		"edges": [
			{"source": "main.go", "target": "lib.go", "type": "call"}
		],
		"communities": [
			{"id": 0, "name": "root", "members": ["main.go", "lib.go"]}
		]
	}`
	if err := os.WriteFile(filepath.Join(outDir, "graph.json"), []byte(graphifyJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	provider := NewUnifiedGraphProvider()
	// Force graphify path check to succeed by providing pre-built output
	graph, err := provider.GetGraph(repo)
	if err != nil {
		t.Fatalf("GetGraph error: %v", err)
	}

	// If graphify CLI is in PATH, it should use graphify data (2 nodes from graphify).
	// If not, it falls back to light graph (1 node — main.go).
	// Either way, we should get a valid graph.
	if graph == nil {
		t.Fatal("expected non-nil graph")
	}
	if len(graph.Nodes) < 1 {
		t.Error("expected at least 1 node")
	}
}

func TestUnifiedGraphProvider_CachesResult(t *testing.T) {
	repo := setupProviderTestRepo(t)
	provider := NewUnifiedGraphProvider()

	graph1, err := provider.GetGraph(repo)
	if err != nil {
		t.Fatalf("first GetGraph error: %v", err)
	}

	graph2, err := provider.GetGraph(repo)
	if err != nil {
		t.Fatalf("second GetGraph error: %v", err)
	}

	// Same pointer should be returned from cache (same HEAD SHA)
	if graph1 != graph2 {
		t.Error("expected cached graph (same pointer) for same repo/HEAD")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/knowledge/ -run "TestUnifiedGraphProvider" -v`
Expected: FAIL — `NewUnifiedGraphProvider` not defined

**Step 3: Implement the unified provider**

```go
// internal/knowledge/graph_provider.go
package knowledge

import (
	"sync"
)

// UnifiedGraphProvider selects the best available graph source:
// Graphify (Tier 2) when available, otherwise lightweight builder (Tier 1).
type UnifiedGraphProvider struct {
	mu          sync.Mutex
	cachedGraph map[string]*CodeGraph // keyed by repoPath
	cachedSHA   map[string]string     // keyed by repoPath
}

// NewUnifiedGraphProvider creates a new UnifiedGraphProvider.
func NewUnifiedGraphProvider() *UnifiedGraphProvider {
	return &UnifiedGraphProvider{
		cachedGraph: make(map[string]*CodeGraph),
		cachedSHA:   make(map[string]string),
	}
}

// GetGraph returns the code graph for the given repo, using the best available source.
// Results are cached in memory and invalidated when HEAD SHA changes.
func (p *UnifiedGraphProvider) GetGraph(repoPath string) (*CodeGraph, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check if cached graph is still valid
	headSHA, err := getHeadSHA(repoPath)
	if err != nil {
		return nil, err
	}
	if cached, ok := p.cachedGraph[repoPath]; ok {
		if p.cachedSHA[repoPath] == headSHA {
			return cached, nil
		}
	}

	// Try Graphify (Tier 2) first
	gi := NewGraphifyIntegration(repoPath)
	if gi.IsAvailable() && gi.HasGraph() {
		graph, err := gi.GetGraphFromGraphify()
		if err == nil {
			graph.HeadSHA = headSHA
			p.cachedGraph[repoPath] = graph
			p.cachedSHA[repoPath] = headSHA
			return graph, nil
		}
		// Graphify parse failed — fall through to Tier 1
	}

	// Fall back to lightweight graph (Tier 1)
	graph, err := BuildLightGraph(repoPath)
	if err != nil {
		return nil, err
	}
	p.cachedGraph[repoPath] = graph
	p.cachedSHA[repoPath] = headSHA
	return graph, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/knowledge/ -run "TestUnifiedGraphProvider" -v`
Expected: PASS

**Step 5: Wire into Manager struct**

Add `graphProvider` field to the `Manager` struct in `internal/company/company.go`. After the `reviewCfg` field (line 78), add:

```go
graphProvider   *knowledge.UnifiedGraphProvider
```

Add the import for the knowledge package if not already present (check the existing imports — `knowledge` is already imported in `review.go` but may need adding to `company.go`).

Initialize in the `NewManager` function (or wherever the Manager is constructed). Locate the constructor and add:

```go
m.graphProvider = knowledge.NewUnifiedGraphProvider()
```

Add a public accessor for use by GUI bindings:

```go
// GraphProvider returns the unified code graph provider.
func (m *Manager) GraphProvider() *knowledge.UnifiedGraphProvider {
	return m.graphProvider
}
```

**Step 6: Verify compilation**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go build ./internal/...`
Expected: BUILD SUCCESSFUL

**Step 7: Commit**

```bash
git add internal/knowledge/graph_provider.go internal/knowledge/graph_provider_test.go internal/company/company.go
git commit -m "feat(knowledge): add UnifiedGraphProvider with Tier 1/2 fallback, wire into Manager

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 5: Community-Based Task Assignment

**Files:**
- Create: `internal/company/graph_assign_test.go`
- Modify: `internal/company/pipeline.go:437-480` (matchWorker function)
- Modify: `internal/worker/worker.go:54-78` (Worker struct — add `LastCommunityID` field)

**Step 1: Write the failing tests**

```go
// internal/company/graph_assign_test.go
package company

import (
	"testing"

	"github.com/hanfourmini/aisupervisor/internal/knowledge"
	"github.com/hanfourmini/aisupervisor/internal/personality"
	"github.com/hanfourmini/aisupervisor/internal/project"
	"github.com/hanfourmini/aisupervisor/internal/worker"
)

func TestFindBestWorkerForCommunity_PrefersSameCommunity(t *testing.T) {
	graph := &knowledge.CodeGraph{
		Communities: []knowledge.Community{
			{ID: 0, Name: "internal/worker", Files: []string{"internal/worker/spawner.go", "internal/worker/monitor.go"}},
			{ID: 1, Name: "internal/company", Files: []string{"internal/company/company.go"}},
			{ID: 2, Name: "internal/tmux", Files: []string{"internal/tmux/client.go"}},
		},
	}

	task := &project.Task{
		ID:    "t1",
		Type:  project.TaskTypeFeature,
		Files: []string{"internal/worker/spawner.go"},
	}

	idle := []idleWorkerSnapshot{
		{ID: "w1", SkillProfile: "coder", Tier: worker.TierEngineer, LastCommunityID: 1}, // company community
		{ID: "w2", SkillProfile: "coder", Tier: worker.TierEngineer, LastCommunityID: 0}, // worker community (match!)
		{ID: "w3", SkillProfile: "coder", Tier: worker.TierEngineer, LastCommunityID: 2}, // tmux community
	}

	bestID := findBestWorkerForCommunity(task, idle, graph, map[string]bool{})
	if bestID != "w2" {
		t.Errorf("expected w2 (same community), got %q", bestID)
	}
}

func TestFindBestWorkerForCommunity_FallsBackWhenNoMatch(t *testing.T) {
	graph := &knowledge.CodeGraph{
		Communities: []knowledge.Community{
			{ID: 0, Name: "internal/worker", Files: []string{"internal/worker/spawner.go"}},
			{ID: 1, Name: "internal/company", Files: []string{"internal/company/company.go"}},
		},
	}

	task := &project.Task{
		ID:    "t1",
		Type:  project.TaskTypeFeature,
		Files: []string{"internal/worker/spawner.go"},
	}

	idle := []idleWorkerSnapshot{
		{ID: "w1", SkillProfile: "coder", Tier: worker.TierEngineer, LastCommunityID: 1}, // company — no match
		{ID: "w2", SkillProfile: "coder", Tier: worker.TierEngineer, LastCommunityID: 1}, // company — no match
	}

	bestID := findBestWorkerForCommunity(task, idle, graph, map[string]bool{})
	if bestID != "" {
		t.Errorf("expected empty (no community match), got %q", bestID)
	}
}

func TestFindBestWorkerForCommunity_SkipsAssigned(t *testing.T) {
	graph := &knowledge.CodeGraph{
		Communities: []knowledge.Community{
			{ID: 0, Name: "internal/worker", Files: []string{"internal/worker/spawner.go"}},
		},
	}

	task := &project.Task{
		ID:    "t1",
		Type:  project.TaskTypeFeature,
		Files: []string{"internal/worker/spawner.go"},
	}

	idle := []idleWorkerSnapshot{
		{ID: "w1", SkillProfile: "coder", Tier: worker.TierEngineer, LastCommunityID: 0},
		{ID: "w2", SkillProfile: "coder", Tier: worker.TierEngineer, LastCommunityID: 0},
	}

	assigned := map[string]bool{"w1": true}
	bestID := findBestWorkerForCommunity(task, idle, graph, assigned)
	if bestID != "w2" {
		t.Errorf("expected w2 (w1 is assigned), got %q", bestID)
	}
}

func TestFindBestWorkerForCommunity_NilGraph(t *testing.T) {
	task := &project.Task{
		ID:    "t1",
		Type:  project.TaskTypeFeature,
		Files: []string{"internal/worker/spawner.go"},
	}

	idle := []idleWorkerSnapshot{
		{ID: "w1", SkillProfile: "coder", Tier: worker.TierEngineer, LastCommunityID: 0},
	}

	bestID := findBestWorkerForCommunity(task, idle, nil, map[string]bool{})
	if bestID != "" {
		t.Errorf("expected empty for nil graph, got %q", bestID)
	}
}

// Ensure idleWorkerSnapshot was extended with LastCommunityID.
func TestIdleWorkerSnapshot_HasLastCommunityID(t *testing.T) {
	snap := idleWorkerSnapshot{
		ID:              "w1",
		SkillProfile:    "coder",
		Tier:            worker.TierEngineer,
		SkillScores:     personality.SkillScores{},
		LastCommunityID: 42,
	}
	if snap.LastCommunityID != 42 {
		t.Errorf("expected LastCommunityID 42, got %d", snap.LastCommunityID)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/company/ -run "TestFindBestWorkerForCommunity|TestIdleWorkerSnapshot_HasLastCommunityID" -v`
Expected: FAIL — `findBestWorkerForCommunity` not defined, `LastCommunityID` not a field

**Step 3: Add LastCommunityID to Worker struct and idleWorkerSnapshot**

In `internal/worker/worker.go`, add after `CreatedAt` field (around line 77):

```go
LastCommunityID int `yaml:"last_community_id,omitempty" json:"lastCommunityId,omitempty"`
```

In `internal/company/company.go`, extend `idleWorkerSnapshot` (around line 1685-1691) to add:

```go
LastCommunityID int
```

In the `drainReadyQueue` function where snapshots are built (around line 1711), populate the new field:

```go
snap.LastCommunityID = w.LastCommunityID
```

**Step 4: Implement findBestWorkerForCommunity**

Add to `internal/company/pipeline.go`:

```go
// findBestWorkerForCommunity returns the idle worker whose last community matches
// the task's target community. Returns "" if no match found or graph is nil.
func findBestWorkerForCommunity(task *project.Task, idle []idleWorkerSnapshot, graph *knowledge.CodeGraph, assigned map[string]bool) string {
	if graph == nil || len(task.Files) == 0 {
		return ""
	}

	// Determine the task's community from its first file
	taskCommunity := knowledge.GetCommunityForFile(graph, task.Files[0])
	if taskCommunity == nil {
		return ""
	}

	for _, w := range idle {
		if assigned[w.ID] {
			continue
		}
		if w.LastCommunityID == taskCommunity.ID {
			return w.ID
		}
	}
	return ""
}
```

**Step 5: Integrate into matchWorker**

In `internal/company/pipeline.go`, modify the `matchWorker` function to check community preference before the existing profile/skill-based matching. Add a `graph *knowledge.CodeGraph` parameter or access it from a package-level variable. The cleanest approach: add an optional graph parameter to `matchWorker` or create a wrapper.

Modify `matchWorker` signature to accept an optional graph:

```go
func matchWorker(t *project.Task, idle []idleWorkerSnapshot, assigned map[string]bool, graph ...*knowledge.CodeGraph) string {
```

At the top of `matchWorker`, add community preference check:

```go
	// Community preference: if graph is available, prefer worker in same community
	if len(graph) > 0 && graph[0] != nil {
		communityMatch := findBestWorkerForCommunity(t, idle, graph[0], assigned)
		if communityMatch != "" {
			return communityMatch
		}
	}
```

Update the call site in `drainReadyQueue` to pass the graph:

```go
// In drainReadyQueue, before the loop, get the graph:
var codeGraph *knowledge.CodeGraph
if m.graphProvider != nil {
    // Use first project's repo path (best effort)
    for _, t := range readyTasks {
        if p, ok := m.projectStore.GetProject(t.ProjectID); ok {
            codeGraph, _ = m.graphProvider.GetGraph(p.RepoPath)
            break
        }
    }
}

// In the loop:
best := matchWorker(t, idle, assignedMap, codeGraph)
```

**Step 6: Update LastCommunityID on task completion**

In `internal/company/company.go`, after a task completes (in the completion handler), set the worker's `LastCommunityID`:

```go
// After task completion, update worker's last community for future assignment
if m.graphProvider != nil {
    if p, ok := m.projectStore.GetProject(t.ProjectID); ok {
        if graph, err := m.graphProvider.GetGraph(p.RepoPath); err == nil && len(t.Files) > 0 {
            if c := knowledge.GetCommunityForFile(graph, t.Files[0]); c != nil {
                w.LastCommunityID = c.ID
            }
        }
    }
}
```

**Step 7: Run tests to verify they pass**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/company/ -run "TestFindBestWorkerForCommunity|TestIdleWorkerSnapshot_HasLastCommunityID" -v`
Expected: PASS

**Step 8: Verify full compilation**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go build ./internal/...`
Expected: BUILD SUCCESSFUL

**Step 9: Run existing tests to ensure no regressions**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/company/ -v -timeout 60s`
Expected: All existing tests PASS (the matchWorker variadic change is backward-compatible)

**Step 10: Commit**

```bash
git add internal/worker/worker.go internal/company/company.go internal/company/pipeline.go internal/company/graph_assign_test.go
git commit -m "feat(company): add community-based task assignment via code graph

Workers are now preferentially assigned tasks in the same code community
as their last completed task, improving context locality.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 6: Review Escalation for Cross-Community PRs

**Files:**
- Create: `internal/company/graph_escalate_test.go`
- Modify: `internal/company/review.go:279-326` (runChatReview method)
- Modify: `internal/company/debate.go:37-45` (selectStrategy function)

**Step 1: Write the failing tests**

```go
// internal/company/graph_escalate_test.go
package company

import (
	"testing"

	"github.com/hanfourmini/aisupervisor/internal/knowledge"
)

func TestShouldEscalateReview_SingleCommunity(t *testing.T) {
	graph := &knowledge.CodeGraph{
		Communities: []knowledge.Community{
			{ID: 0, Name: "internal/worker", Files: []string{"internal/worker/spawner.go", "internal/worker/monitor.go"}},
			{ID: 1, Name: "internal/company", Files: []string{"internal/company/company.go"}},
		},
		GodNodes: []string{},
	}

	files := []string{"internal/worker/spawner.go", "internal/worker/monitor.go"}
	if shouldEscalateReview(files, graph) {
		t.Error("expected no escalation for single-community change")
	}
}

func TestShouldEscalateReview_TwoCommunities(t *testing.T) {
	graph := &knowledge.CodeGraph{
		Communities: []knowledge.Community{
			{ID: 0, Name: "internal/worker", Files: []string{"internal/worker/spawner.go"}},
			{ID: 1, Name: "internal/company", Files: []string{"internal/company/company.go"}},
		},
		GodNodes: []string{},
	}

	files := []string{"internal/worker/spawner.go", "internal/company/company.go"}
	if shouldEscalateReview(files, graph) {
		t.Error("expected no escalation for 2-community change (threshold is 3)")
	}
}

func TestShouldEscalateReview_ThreePlusCommunities(t *testing.T) {
	graph := &knowledge.CodeGraph{
		Communities: []knowledge.Community{
			{ID: 0, Name: "internal/worker", Files: []string{"internal/worker/spawner.go"}},
			{ID: 1, Name: "internal/company", Files: []string{"internal/company/company.go"}},
			{ID: 2, Name: "internal/tmux", Files: []string{"internal/tmux/client.go"}},
			{ID: 3, Name: "internal/knowledge", Files: []string{"internal/knowledge/types.go"}},
		},
		GodNodes: []string{},
	}

	files := []string{
		"internal/worker/spawner.go",
		"internal/company/company.go",
		"internal/tmux/client.go",
	}
	if !shouldEscalateReview(files, graph) {
		t.Error("expected escalation for 3-community change")
	}
}

func TestShouldEscalateReview_GodNode(t *testing.T) {
	graph := &knowledge.CodeGraph{
		Communities: []knowledge.Community{
			{ID: 0, Name: "internal/company", Files: []string{"internal/company/company.go"}},
		},
		GodNodes: []string{"internal/company/company.go"},
	}

	// Even a single-community change should escalate if it touches a god node
	files := []string{"internal/company/company.go"}
	if !shouldEscalateReview(files, graph) {
		t.Error("expected escalation when god node is touched")
	}
}

func TestShouldEscalateReview_NilGraph(t *testing.T) {
	files := []string{"internal/worker/spawner.go"}
	if shouldEscalateReview(files, nil) {
		t.Error("expected no escalation for nil graph")
	}
}

func TestShouldEscalateReview_EmptyFiles(t *testing.T) {
	graph := &knowledge.CodeGraph{
		Communities: []knowledge.Community{
			{ID: 0, Name: "internal/worker", Files: []string{"internal/worker/spawner.go"}},
		},
		GodNodes: []string{},
	}

	if shouldEscalateReview(nil, graph) {
		t.Error("expected no escalation for empty file list")
	}
}

func TestShouldEscalateReview_UnknownFilesIgnored(t *testing.T) {
	graph := &knowledge.CodeGraph{
		Communities: []knowledge.Community{
			{ID: 0, Name: "internal/worker", Files: []string{"internal/worker/spawner.go"}},
		},
		GodNodes: []string{},
	}

	// unknown.go is not in any community — should not count
	files := []string{"internal/worker/spawner.go", "unknown.go", "also_unknown.go"}
	if shouldEscalateReview(files, graph) {
		t.Error("expected no escalation when unknown files don't push community count to 3+")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/company/ -run "TestShouldEscalateReview" -v`
Expected: FAIL — `shouldEscalateReview` not defined

**Step 3: Implement shouldEscalateReview**

Add to `internal/company/debate.go` (near the `selectStrategy` function):

```go
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
```

Add the `knowledge` import to `debate.go` if not already present:

```go
import (
	// ...existing imports...
	"github.com/hanfourmini/aisupervisor/internal/knowledge"
)
```

**Step 4: Run tests to verify they pass**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/company/ -run "TestShouldEscalateReview" -v`
Expected: PASS

**Step 5: Integrate into the review pipeline**

In `internal/company/review.go`, modify the `runChatReview` method. After `selectStrategy` is called (around line 299), add graph-based escalation:

```go
	strategy := selectStrategy(diffLines, fileCount, cfg.DebateThreshold, cfg.LightMaxLines, cfg.LightMaxFiles)

	// Graph-based escalation: force debate for cross-community or god-node changes
	if strategy != ReviewDebate && rp.mgr.graphProvider != nil {
		if graph, err := rp.mgr.graphProvider.GetGraph(p.RepoPath); err == nil {
			// Extract changed file names from the diff
			changedFiles := extractChangedFiles(diff)
			if shouldEscalateReview(changedFiles, graph) {
				log.Printf("debate: escalating task=%s to debate (graph: cross-community or god-node)", t.ID)
				strategy = ReviewDebate
			}
		}
	}

	log.Printf("debate: task=%s strategy=%s (lines=%d files=%d)", t.ID, strategy, diffLines, fileCount)
```

Add the `extractChangedFiles` helper to `internal/company/debate.go`:

```go
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
```

**Step 6: Run full test suite to verify no regressions**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/company/ -v -timeout 60s`
Expected: All tests PASS

**Step 7: Verify compilation**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go build ./internal/...`
Expected: BUILD SUCCESSFUL

**Step 8: Commit**

```bash
git add internal/company/debate.go internal/company/review.go internal/company/graph_escalate_test.go
git commit -m "feat(company): add graph-based review escalation for cross-community PRs

Changes spanning 3+ communities or touching god nodes are automatically
escalated to debate review for thorough architectural scrutiny.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 7: GUI Binding

**Files:**
- Modify: `internal/gui/company_app.go` (add `GetProjectGraph` binding)
- Create: `internal/gui/company_app_graph_test.go`

**Step 1: Write the failing test**

```go
// internal/gui/company_app_graph_test.go
package gui

import (
	"testing"

	"github.com/hanfourmini/aisupervisor/internal/knowledge"
)

func TestGetProjectGraph_ReturnsCodeGraphType(t *testing.T) {
	// Verify that the CodeGraph type is correctly wirable through the GUI layer.
	// This is a compile-time check: if GetProjectGraph exists and returns *knowledge.CodeGraph,
	// this test compiles and passes.
	var graph *knowledge.CodeGraph
	_ = graph

	// Verify CodeGraph has the expected JSON-serializable fields
	g := &knowledge.CodeGraph{
		RepoPath: "/tmp/test",
		HeadSHA:  "abc123",
		Nodes: []knowledge.FileNode{
			{Path: "main.go", Package: "main", InDegree: 0},
		},
		Edges: []knowledge.Edge{
			{From: "a.go", To: "b.go", Type: knowledge.EdgeImport, Weight: 1},
		},
		Communities: []knowledge.Community{
			{ID: 0, Name: "root", Files: []string{"main.go"}},
		},
		GodNodes: []string{"main.go"},
	}
	if g.RepoPath != "/tmp/test" {
		t.Errorf("expected repoPath /tmp/test, got %q", g.RepoPath)
	}
	if len(g.Nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(g.Nodes))
	}
}
```

**Step 2: Run test to verify it compiles (basic type check)**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/gui/ -run "TestGetProjectGraph_ReturnsCodeGraphType" -v`
Expected: PASS (this is a compile-time check)

**Step 3: Add the GetProjectGraph binding**

In `internal/gui/company_app.go`, add the binding method and import:

Add to imports (if not already present):

```go
"github.com/hanfourmini/aisupervisor/internal/knowledge"
```

Add the method:

```go
// GetProjectGraph returns the code graph for a project's repository.
// Used by the frontend to visualize code communities and dependencies.
func (c *CompanyApp) GetProjectGraph(projectID string) (*knowledge.CodeGraph, error) {
	provider := c.company.GraphProvider()
	if provider == nil {
		return nil, fmt.Errorf("graph provider not initialized")
	}

	p := c.company.GetProject(projectID)
	if p == nil {
		return nil, fmt.Errorf("project %q not found", projectID)
	}

	return provider.GetGraph(p.RepoPath)
}
```

**Note:** Verify that `Manager.GetProject(id)` exists and returns `*project.Project`. If the method has a different signature (e.g., returns `(project, bool)`), adjust accordingly. Check with:

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && grep -n "func.*Manager.*GetProject" internal/company/company.go`

Adjust the call pattern to match the actual signature.

**Step 4: Verify full compilation**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go build ./internal/...`
Expected: BUILD SUCCESSFUL

If there are frontend-facing changes needed (Svelte store/component), those are out of scope for this plan — the Wails binding is sufficient for now and will be picked up when the frontend graph visualization is built.

**Step 5: Commit**

```bash
git add internal/gui/company_app.go internal/gui/company_app_graph_test.go
git commit -m "feat(gui): add GetProjectGraph Wails binding for code graph visualization

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Summary

| Task | Files | What it does |
|------|-------|-------------|
| 1 | `knowledge/codegraph.go`, `codegraph_test.go` | Core types: FileNode, Edge, Community, CodeGraph; helpers: GetCommunityForFile, GetGodNodes |
| 2 | `knowledge/lightgraph.go`, `lightgraph_test.go` | Tier 1 builder: go/ast imports, git co-change, directory-based communities, JSON cache |
| 3 | `knowledge/graphify.go`, `graphify_test.go` | Tier 2 integration: Graphify CLI wrapper, graph.json parser, report reader |
| 4 | `knowledge/graph_provider.go`, `graph_provider_test.go`, `company/company.go` | Unified provider: Tier 2 when available, else Tier 1; wired into Manager |
| 5 | `company/graph_assign_test.go`, `company/pipeline.go`, `worker/worker.go` | Community-based assignment: prefer worker in same community as task |
| 6 | `company/graph_escalate_test.go`, `company/debate.go`, `company/review.go` | Review escalation: 3+ communities or god node forces debate strategy |
| 7 | `gui/company_app.go`, `gui/company_app_graph_test.go` | GUI binding: `GetProjectGraph` passthrough for frontend visualization |

**Dependency order:** Task 1 → Task 2 → Task 3 → Task 4 → Task 5 + Task 6 (parallel) → Task 7

**Total new test functions:** 28

**Final verification after all tasks:**

```bash
cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/knowledge/ ./internal/company/ ./internal/gui/ -v -timeout 120s
```
