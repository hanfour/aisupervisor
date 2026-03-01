package context

import (
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// Store defines the interface for session context persistence.
type Store interface {
	Get(sessionID string) (*SessionContext, error)
	GetOrCreate(sessionID string, maxDecisions, maxActivities int) (*SessionContext, error)
	Save(sc *SessionContext) error
	Delete(sessionID string) error
}

// FileStore persists one YAML file per session under a base directory.
type FileStore struct {
	dir   string
	cache map[string]*SessionContext
	mu    sync.Mutex
}

// NewFileStore creates a FileStore, ensuring the directory exists.
func NewFileStore(dir string) (*FileStore, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &FileStore{
		dir:   dir,
		cache: make(map[string]*SessionContext),
	}, nil
}

func (fs *FileStore) path(sessionID string) string {
	return filepath.Join(fs.dir, sessionID+".yaml")
}

// Get loads a session context from disk. Returns nil if not found.
func (fs *FileStore) Get(sessionID string) (*SessionContext, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if sc, ok := fs.cache[sessionID]; ok {
		return sc, nil
	}

	sc, err := fs.load(sessionID)
	if err != nil {
		return nil, err
	}
	if sc != nil {
		fs.cache[sessionID] = sc
	}
	return sc, nil
}

// GetOrCreate returns an existing context or creates a new one.
func (fs *FileStore) GetOrCreate(sessionID string, maxDecisions, maxActivities int) (*SessionContext, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if sc, ok := fs.cache[sessionID]; ok {
		return sc, nil
	}

	sc, err := fs.load(sessionID)
	if err != nil {
		return nil, err
	}
	if sc != nil {
		sc.maxDecisions = maxDecisions
		sc.maxActivities = maxActivities
		fs.cache[sessionID] = sc
		return sc, nil
	}

	sc = NewSessionContext(sessionID, maxDecisions, maxActivities)
	fs.cache[sessionID] = sc
	return sc, nil
}

// Save persists the session context to disk.
func (fs *FileStore) Save(sc *SessionContext) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	fs.cache[sc.SessionID] = sc

	snap := sc.Snapshot()
	data, err := yaml.Marshal(&snap)
	if err != nil {
		return err
	}
	return os.WriteFile(fs.path(sc.SessionID), data, 0o644)
}

// Delete removes the session context file and cache entry.
func (fs *FileStore) Delete(sessionID string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	delete(fs.cache, sessionID)
	err := os.Remove(fs.path(sessionID))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (fs *FileStore) load(sessionID string) (*SessionContext, error) {
	data, err := os.ReadFile(fs.path(sessionID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var sc SessionContext
	if err := yaml.Unmarshal(data, &sc); err != nil {
		return nil, err
	}
	return &sc, nil
}
