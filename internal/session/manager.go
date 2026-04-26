package session

import (
	"database/sql"
	"sync"

	"github.com/ojuschugh1/aura/pkg/types"
)

// Manager manages session lifecycle backed by SQLite.
type Manager struct {
	mu      sync.RWMutex
	db      *sql.DB
	current *types.Session
}

// New creates a Manager using the provided database connection.
func New(db *sql.DB) *Manager {
	return &Manager{db: db}
}

// Create starts a new session and sets it as current.
func (m *Manager) Create() (*types.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, err := create(m.db)
	if err != nil {
		return nil, err
	}
	m.current = s
	return s, nil
}

// Current returns the active session, or nil if none exists.
func (m *Manager) Current() *types.Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current
}

// Get retrieves a session by ID.
func (m *Manager) Get(id string) (*types.Session, error) {
	return get(m.db, id)
}

// End marks the session with the given ID as completed.
func (m *Manager) End(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := end(m.db, id); err != nil {
		return err
	}
	if m.current != nil && m.current.ID == id {
		m.current = nil
	}
	return nil
}

// List returns all sessions ordered by start time descending.
func (m *Manager) List() ([]*types.Session, error) {
	return list(m.db)
}
