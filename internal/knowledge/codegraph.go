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
