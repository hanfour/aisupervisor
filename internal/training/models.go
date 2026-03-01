package training

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// ModelVersion tracks a single version in the fine-tune chain.
type ModelVersion struct {
	Version      string    `yaml:"version" json:"version"`
	BaseModel    string    `yaml:"base_model" json:"base_model"`
	ParentVer    string    `yaml:"parent_version,omitempty" json:"parent_version,omitempty"`
	TrainPairs   int       `yaml:"train_pairs" json:"train_pairs"`
	Method       string    `yaml:"method" json:"method"` // "sft" or "dpo"
	OllamaModel  string    `yaml:"ollama_model,omitempty" json:"ollama_model,omitempty"`
	BenchmarkScore float64 `yaml:"benchmark_score,omitempty" json:"benchmark_score,omitempty"`
	CreatedAt    time.Time `yaml:"created_at" json:"created_at"`
	Notes        string    `yaml:"notes,omitempty" json:"notes,omitempty"`
}

// ModelRegistry tracks the full version history of fine-tuned models.
type ModelRegistry struct {
	mu       sync.RWMutex
	path     string
	Versions []ModelVersion `yaml:"versions"`
}

func NewModelRegistry(dataDir string) (*ModelRegistry, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	r := &ModelRegistry{
		path: filepath.Join(dataDir, "model_versions.yaml"),
	}
	if err := r.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return r, nil
}

// Register adds a new model version to the registry.
func (r *ModelRegistry) Register(v ModelVersion) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if v.Version == "" {
		v.Version = fmt.Sprintf("v%d", len(r.Versions)+1)
	}
	if v.CreatedAt.IsZero() {
		v.CreatedAt = time.Now()
	}
	r.Versions = append(r.Versions, v)
	return r.save()
}

// Latest returns the most recent model version, or nil if none.
func (r *ModelRegistry) Latest() *ModelVersion {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.Versions) == 0 {
		return nil
	}
	v := r.Versions[len(r.Versions)-1]
	return &v
}

// List returns all versions.
func (r *ModelRegistry) List() []ModelVersion {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]ModelVersion, len(r.Versions))
	copy(result, r.Versions)
	return result
}

// Get retrieves a specific version by name.
func (r *ModelRegistry) Get(version string) (*ModelVersion, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, v := range r.Versions {
		if v.Version == version {
			return &v, true
		}
	}
	return nil, false
}

// VersionChain returns the lineage from a version back to the base model.
func (r *ModelRegistry) VersionChain(version string) []ModelVersion {
	r.mu.RLock()
	defer r.mu.RUnlock()

	versionMap := make(map[string]ModelVersion)
	for _, v := range r.Versions {
		versionMap[v.Version] = v
	}

	var chain []ModelVersion
	current := version
	for current != "" {
		v, ok := versionMap[current]
		if !ok {
			break
		}
		chain = append(chain, v)
		current = v.ParentVer
	}
	return chain
}

func (r *ModelRegistry) load() error {
	data, err := os.ReadFile(r.path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, r)
}

func (r *ModelRegistry) save() error {
	data, err := yaml.Marshal(r)
	if err != nil {
		return err
	}
	return os.WriteFile(r.path, data, 0o644)
}
