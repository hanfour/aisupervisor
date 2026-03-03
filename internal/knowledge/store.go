package knowledge

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Store manages knowledge entries with YAML persistence.
type Store struct {
	mu      sync.RWMutex
	dataDir string
	entries map[string]*Entry // keyed by ID
}

type entriesFile struct {
	Entries []*Entry `yaml:"entries"`
}

// NewStore creates a new knowledge store.
func NewStore(dataDir string) *Store {
	return &Store{
		dataDir: dataDir,
		entries: make(map[string]*Entry),
	}
}

// Add inserts a new knowledge entry.
func (s *Store) Add(entry *Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[entry.ID] = entry
	return s.save()
}

// Get retrieves an entry by ID.
func (s *Store) Get(id string) (*Entry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.entries[id]
	return e, ok
}

// Delete removes an entry by ID.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, id)
	return s.save()
}

// SearchByDomain returns all valid entries matching the given domain.
func (s *Store) SearchByDomain(domain string, minConfidence float64) []*Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Entry
	for _, e := range s.entries {
		if e.Domain == domain && e.IsValid(minConfidence) {
			result = append(result, e)
		}
	}
	return result
}

// SearchByTags returns all valid entries matching any of the given tags.
func (s *Store) SearchByTags(tags []string, minConfidence float64) []*Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tagSet := make(map[string]bool, len(tags))
	for _, t := range tags {
		tagSet[strings.ToLower(t)] = true
	}

	var result []*Entry
	for _, e := range s.entries {
		if !e.IsValid(minConfidence) {
			continue
		}
		for _, et := range e.Tags {
			if tagSet[strings.ToLower(et)] {
				result = append(result, e)
				break
			}
		}
	}
	return result
}

// Search returns entries matching domain and/or tags.
func (s *Store) Search(domain string, tags []string, minConfidence float64) []*Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tagSet := make(map[string]bool, len(tags))
	for _, t := range tags {
		tagSet[strings.ToLower(t)] = true
	}

	var result []*Entry
	for _, e := range s.entries {
		if !e.IsValid(minConfidence) {
			continue
		}

		domainMatch := domain == "" || e.Domain == domain
		tagMatch := len(tagSet) == 0
		if !tagMatch {
			for _, et := range e.Tags {
				if tagSet[strings.ToLower(et)] {
					tagMatch = true
					break
				}
			}
		}

		if domainMatch && tagMatch {
			result = append(result, e)
		}
	}
	return result
}

// ListAll returns all entries.
func (s *Store) ListAll() []*Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*Entry, 0, len(s.entries))
	for _, e := range s.entries {
		result = append(result, e)
	}
	return result
}

// Load reads entries from the YAML file.
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.dataDir, "knowledge.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var f entriesFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return err
	}
	for _, e := range f.Entries {
		s.entries[e.ID] = e
	}
	return nil
}

func (s *Store) save() error {
	path := filepath.Join(s.dataDir, "knowledge.yaml")
	if err := os.MkdirAll(s.dataDir, 0o755); err != nil {
		return err
	}

	entries := make([]*Entry, 0, len(s.entries))
	for _, e := range s.entries {
		entries = append(entries, e)
	}

	f := entriesFile{Entries: entries}
	data, err := yaml.Marshal(&f)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
