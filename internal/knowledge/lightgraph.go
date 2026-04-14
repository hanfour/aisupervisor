// internal/knowledge/lightgraph.go
package knowledge

import (
	"encoding/json"
	"fmt"
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
	cmd := exec.Command("git", "log", "--format=format:COMMIT", "--name-only", "--diff-filter=AMRC",
		fmt.Sprintf("-%d", gitLogLimit))
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
