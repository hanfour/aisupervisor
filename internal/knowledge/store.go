package knowledge

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type Store struct {
	baseDir string
	mu      sync.RWMutex
}

func NewStore(baseDir string) *Store {
	return &Store{baseDir: baseDir}
}

func (s *Store) Add(e Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if e.ID == "" {
		e.ID = fmt.Sprintf("k%d", time.Now().UnixNano())
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now()
	}

	entries, err := s.readFile(s.filePath(e.ProjectID, e.WorkerID))
	if err != nil {
		entries = nil
	}
	entries = append(entries, e)
	return s.writeFile(s.filePath(e.ProjectID, e.WorkerID), entries)
}

func (s *Store) GetForWorker(workerID, projectID string) ([]Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.readFile(s.filePath(projectID, workerID))
}

func (s *Store) GetShared(projectID string) ([]Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.readFile(s.filePath(projectID, ""))
}

func (s *Store) GetAll(workerID, projectID string) ([]Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	shared, _ := s.readFile(s.filePath(projectID, ""))
	worker, _ := s.readFile(s.filePath(projectID, workerID))
	all := append(shared, worker...)
	sort.Slice(all, func(i, j int) bool {
		return all[i].Relevance > all[j].Relevance
	})
	return all, nil
}

func (s *Store) Delete(entryID, projectID, workerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.filePath(projectID, workerID)
	entries, err := s.readFile(path)
	if err != nil {
		return err
	}
	filtered := make([]Entry, 0, len(entries))
	for _, e := range entries {
		if e.ID != entryID {
			filtered = append(filtered, e)
		}
	}
	return s.writeFile(path, filtered)
}

func (s *Store) filePath(projectID, workerID string) string {
	if workerID == "" {
		return filepath.Join(s.baseDir, "projects", projectID, "shared.yaml")
	}
	return filepath.Join(s.baseDir, "projects", projectID, "workers", workerID+".yaml")
}

func (s *Store) readFile(path string) ([]Entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var entries []Entry
	if err := yaml.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("unmarshal %s: %w", path, err)
	}
	return entries, nil
}

func (s *Store) writeFile(path string, entries []Entry) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	data, err := yaml.Marshal(entries)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}
