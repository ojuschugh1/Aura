package memory

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ojuschugh1/aura/pkg/types"
)

// Store provides CRUD operations on the memory_entries table.
type Store struct {
	db *sql.DB
}

// New creates a Store backed by the given database.
func New(db *sql.DB) *Store {
	return &Store{db: db}
}

// Add inserts or updates a memory entry (upsert semantics).
// On update: value, updated_at, source_tool, and content_hash are refreshed; created_at is preserved.
func (s *Store) Add(key, value, sourceTool, sessionID string) (*types.MemoryEntry, error) {
	hash := contentHash(value)
	now := time.Now().UTC().Format(time.RFC3339Nano)

	_, err := s.db.Exec(`
		INSERT INTO memory_entries (key, value, source_tool, session_id, content_hash, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET
			value        = excluded.value,
			source_tool  = excluded.source_tool,
			session_id   = excluded.session_id,
			content_hash = excluded.content_hash,
			updated_at   = excluded.updated_at`,
		key, value, sourceTool, sessionID, hash, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("upsert memory entry: %w", err)
	}
	return s.Get(key)
}

// Get retrieves the most recent entry by key.
func (s *Store) Get(key string) (*types.MemoryEntry, error) {
	row := s.db.QueryRow(`
		SELECT id, key, value, source_tool, session_id, tags, created_at, updated_at, content_hash
		FROM memory_entries WHERE key = ?`, key,
	)
	return scanEntry(row)
}

// ListFilter controls optional filtering for List.
type ListFilter struct {
	Agent string // filter by source_tool
}

// List returns all entries, optionally filtered, with values truncated to 80 chars.
func (s *Store) List(f ListFilter) ([]*types.MemoryEntry, error) {
	query := `SELECT id, key, value, source_tool, session_id, tags, created_at, updated_at, content_hash
	          FROM memory_entries`
	var args []any
	if f.Agent != "" {
		query += " WHERE source_tool = ?"
		args = append(args, f.Agent)
	}
	query += " ORDER BY updated_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list memory entries: %w", err)
	}
	defer rows.Close()

	var entries []*types.MemoryEntry
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, err
		}
		// Truncate value for listing.
		if len(e.Value) > 80 {
			e.Value = e.Value[:80] + "…"
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// Delete removes an entry by key. Returns an error if the key does not exist.
func (s *Store) Delete(key string) error {
	res, err := s.db.Exec(`DELETE FROM memory_entries WHERE key = ?`, key)
	if err != nil {
		return fmt.Errorf("delete memory entry: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("key %q not found", key)
	}
	return nil
}

// contentHash returns the SHA-256 hex digest of value.
func contentHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", sum)
}

// scanEntry scans a single memory_entries row.
func scanEntry(scanner interface {
	Scan(...any) error
}) (*types.MemoryEntry, error) {
	var (
		e                     types.MemoryEntry
		tagsJSON              sql.NullString
		createdAt, updatedAt  string
	)
	err := scanner.Scan(
		&e.ID, &e.Key, &e.Value, &e.SourceTool, &e.SessionID,
		&tagsJSON, &createdAt, &updatedAt, &e.ContentHash,
	)
	if err != nil {
		return nil, fmt.Errorf("scan memory entry: %w", err)
	}

	if t, err := time.Parse(time.RFC3339Nano, createdAt); err == nil {
		e.CreatedAt = t
	}
	if t, err := time.Parse(time.RFC3339Nano, updatedAt); err == nil {
		e.UpdatedAt = t
	}
	if tagsJSON.Valid && tagsJSON.String != "" {
		_ = json.Unmarshal([]byte(tagsJSON.String), &e.Tags)
	}
	return &e, nil
}

// truncate shortens s to maxLen runes, appending "…" if truncated.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "…"
}

// joinTags serialises a tag slice to a JSON array string.
func joinTags(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	b, _ := json.Marshal(tags)
	return strings.TrimSpace(string(b))
}
