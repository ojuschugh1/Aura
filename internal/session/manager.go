package session

import (
	"database/sql"
	"sync"

	"github.com/ojuschugh1/aura/pkg/types"
)

// Manager manages session lifecycle backed by SQLite.
type Manager struct {
	mu        sync.RWMutex
	db        *sql.DB
	current   *types.Session
	onEndHook func(sessionID string)
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

// SetOnEndHook registers a callback that fires after a session ends.
// The hook receives the session ID and runs outside the manager's lock.
func (m *Manager) SetOnEndHook(hook func(sessionID string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onEndHook = hook
}

// End marks the session with the given ID as completed.
// If an on-end hook is registered, it is called after the session is ended.
func (m *Manager) End(id string) error {
	m.mu.Lock()
	if err := end(m.db, id); err != nil {
		m.mu.Unlock()
		return err
	}
	if m.current != nil && m.current.ID == id {
		m.current = nil
	}
	hook := m.onEndHook
	m.mu.Unlock()

	if hook != nil {
		hook(id)
	}
	return nil
}

// List returns all sessions ordered by start time descending.
func (m *Manager) List() ([]*types.Session, error) {
	return list(m.db)
}
