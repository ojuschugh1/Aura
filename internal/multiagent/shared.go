package multiagent

import (
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/ojuschugh1/aura/internal/memory"
	"github.com/ojuschugh1/aura/pkg/types"
)

// SharedMemory wraps the memory store with multi-agent coordination.
// All connected MCP clients share the same underlying store; last-writer-wins.
type SharedMemory struct {
	mu    sync.RWMutex
	store *memory.Store
	db    *sql.DB
}

// New creates a SharedMemory backed by the given store and database.
func New(store *memory.Store, db *sql.DB) *SharedMemory {
	return &SharedMemory{store: store, db: db}
}

// Write stores a memory entry and logs the activity. Propagation is immediate
// (SQLite WAL ensures all readers see the write within the 100ms busy_timeout).
func (sm *SharedMemory) Write(key, value, agent, sessionID string) (*types.MemoryEntry, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Check for a concurrent write conflict (same key written within 100ms by another agent).
	existing, _ := sm.store.Get(key)
	if existing != nil && time.Since(existing.UpdatedAt) < 100*time.Millisecond && existing.SourceTool != agent {
		slog.Warn("multi-agent write conflict",
			"key", key,
			"existing_agent", existing.SourceTool,
			"new_agent", agent,
			"existing_value", existing.Value,
			"new_value", value,
		)
	}

	entry, err := sm.store.Add(key, value, agent, sessionID)
	if err != nil {
		return nil, err
	}
	if err := logActivity(sm.db, sessionID, agent, "write", key); err != nil {
		slog.Warn("activity log failed", "err", err)
	}
	return entry, nil
}

// Read retrieves a memory entry and logs the read activity.
func (sm *SharedMemory) Read(key, agent, sessionID string) (*types.MemoryEntry, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	entry, err := sm.store.Get(key)
	if err != nil {
		return nil, err
	}
	if err := logActivity(sm.db, sessionID, agent, "read", key); err != nil {
		slog.Warn("activity log failed", "err", err)
	}
	return entry, nil
}

// Delete removes a memory entry and logs the activity.
func (sm *SharedMemory) Delete(key, agent, sessionID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if err := sm.store.Delete(key); err != nil {
		return err
	}
	if err := logActivity(sm.db, sessionID, agent, "delete", key); err != nil {
		slog.Warn("activity log failed", "err", err)
	}
	return nil
}

// logActivity inserts a row into agent_activity_log.
func logActivity(db *sql.DB, sessionID, agent, operation, key string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := db.Exec(`
		INSERT INTO agent_activity_log (session_id, agent, operation, key, recorded_at)
		VALUES (?, ?, ?, ?, ?)`,
		sessionID, agent, operation, key, now,
	)
	if err != nil {
		return fmt.Errorf("log activity: %w", err)
	}
	return nil
}
