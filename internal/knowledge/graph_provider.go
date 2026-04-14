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
