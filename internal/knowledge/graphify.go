// internal/knowledge/graphify.go
package knowledge

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
