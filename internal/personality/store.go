package personality

import (
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// Store manages in-memory personality profiles and relationships with YAML persistence.
type Store struct {
	mu            sync.RWMutex
	dataDir       string
	profiles      map[string]*CharacterProfile
	relationships map[string]*Relationship
}

type profilesFile struct {
	Personalities map[string]*CharacterProfile `yaml:"personalities"`
}

type relationshipsFile struct {
	Relationships []*Relationship `yaml:"relationships"`
}

// NewStore creates a new Store that persists data in the given directory.
func NewStore(dataDir string) *Store {
	return &Store{
		dataDir:       dataDir,
		profiles:      make(map[string]*CharacterProfile),
		relationships: make(map[string]*Relationship),
	}
}

// SetProfile adds or replaces a character profile.
func (s *Store) SetProfile(p *CharacterProfile) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.profiles[p.WorkerID] = p
}

// GetProfile returns the profile for the given worker, or nil if not found.
func (s *Store) GetProfile(workerID string) *CharacterProfile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.profiles[workerID]
}

// DeleteProfile removes a profile by worker ID.
func (s *Store) DeleteProfile(workerID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.profiles, workerID)
}

// ListProfiles returns all stored profiles.
func (s *Store) ListProfiles() []*CharacterProfile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*CharacterProfile, 0, len(s.profiles))
	for _, p := range s.profiles {
		result = append(result, p)
	}
	return result
}

// SetRelationship adds or replaces a relationship.
func (s *Store) SetRelationship(r *Relationship) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := RelationshipKey(r.WorkerA, r.WorkerB)
	s.relationships[key] = r
}

// GetRelationship returns the relationship between two workers, or nil if not found.
func (s *Store) GetRelationship(a, b string) *Relationship {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.relationships[RelationshipKey(a, b)]
}

// GetOrCreateRelationship returns the existing relationship or creates a new one.
func (s *Store) GetOrCreateRelationship(a, b string) *Relationship {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := RelationshipKey(a, b)
	if r, ok := s.relationships[key]; ok {
		return r
	}
	r := NewRelationship(a, b)
	s.relationships[key] = r
	return r
}

// ListRelationships returns all stored relationships.
func (s *Store) ListRelationships() []*Relationship {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*Relationship, 0, len(s.relationships))
	for _, r := range s.relationships {
		result = append(result, r)
	}
	return result
}

// GetWorkerRelationships returns all relationships involving the given worker.
func (s *Store) GetWorkerRelationships(workerID string) []*Relationship {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*Relationship
	for _, r := range s.relationships {
		if r.WorkerA == workerID || r.WorkerB == workerID {
			result = append(result, r)
		}
	}
	return result
}

// Save writes all profiles and relationships to YAML files in the data directory.
func (s *Store) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pf := profilesFile{Personalities: s.profiles}
	if err := s.writeYAML("personalities.yaml", pf); err != nil {
		return err
	}

	rels := make([]*Relationship, 0, len(s.relationships))
	for _, r := range s.relationships {
		rels = append(rels, r)
	}
	rf := relationshipsFile{Relationships: rels}
	return s.writeYAML("relationships.yaml", rf)
}

// Load reads profiles and relationships from YAML files in the data directory.
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var pf profilesFile
	if err := s.readYAML("personalities.yaml", &pf); err != nil && !os.IsNotExist(err) {
		return err
	}
	if pf.Personalities != nil {
		s.profiles = pf.Personalities
	}

	var rf relationshipsFile
	if err := s.readYAML("relationships.yaml", &rf); err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, r := range rf.Relationships {
		key := RelationshipKey(r.WorkerA, r.WorkerB)
		s.relationships[key] = r
	}

	return nil
}

func (s *Store) writeYAML(filename string, data interface{}) error {
	path := filepath.Join(s.dataDir, filename)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := yaml.NewEncoder(f)
	enc.SetIndent(2)
	return enc.Encode(data)
}

func (s *Store) readYAML(filename string, out interface{}) error {
	path := filepath.Join(s.dataDir, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, out)
}
