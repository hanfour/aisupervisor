package session

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*MonitoredSession
	filePath string
}

type sessionsFile struct {
	Sessions []*MonitoredSession `yaml:"sessions"`
}

func NewManager(dataDir string) (*Manager, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}

	m := &Manager{
		sessions: make(map[string]*MonitoredSession),
		filePath: filepath.Join(dataDir, "sessions.yaml"),
	}

	if err := m.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	return m, nil
}

func (m *Manager) Add(s *MonitoredSession) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s.ID == "" {
		s.ID = fmt.Sprintf("s%d", time.Now().UnixMilli())
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now()
	}
	if s.Status == "" {
		s.Status = StatusActive
	}

	m.sessions[s.ID] = s
	return m.save()
}

func (m *Manager) Remove(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.sessions, id)
	return m.save()
}

func (m *Manager) Get(id string) (*MonitoredSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.sessions[id]
	return s, ok
}

func (m *Manager) List() []*MonitoredSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*MonitoredSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		result = append(result, s)
	}
	return result
}

func (m *Manager) Active() []*MonitoredSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*MonitoredSession
	for _, s := range m.sessions {
		if s.Status == StatusActive {
			result = append(result, s)
		}
	}
	return result
}

func (m *Manager) load() error {
	data, err := os.ReadFile(m.filePath)
	if err != nil {
		return err
	}

	var f sessionsFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return err
	}

	for _, s := range f.Sessions {
		m.sessions[s.ID] = s
	}
	return nil
}

func (m *Manager) save() error {
	f := sessionsFile{
		Sessions: make([]*MonitoredSession, 0, len(m.sessions)),
	}
	for _, s := range m.sessions {
		f.Sessions = append(f.Sessions, s)
	}

	data, err := yaml.Marshal(&f)
	if err != nil {
		return err
	}
	return os.WriteFile(m.filePath, data, 0o644)
}
