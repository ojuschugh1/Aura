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
	return s.AddWithMeta(key, value, sourceTool, sessionID, 1.0, nil)
}

// AddWithMeta inserts or updates a memory entry with confidence and tags.
func (s *Store) AddWithMeta(key, value, sourceTool, sessionID string, confidence float64, tags []string) (*types.MemoryEntry, error) {
	hash := contentHash(value)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tagsStr := joinTags(tags)

	_, err := s.db.Exec(`
		INSERT INTO memory_entries (key, value, source_tool, session_id, content_hash, confidence, tags, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET
			value        = excluded.value,
			source_tool  = excluded.source_tool,
			session_id   = excluded.session_id,
			content_hash = excluded.content_hash,
			confidence   = excluded.confidence,
			tags         = COALESCE(excluded.tags, tags),
			updated_at   = excluded.updated_at`,
		key, value, sourceTool, sessionID, hash, confidence, tagsStr, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("upsert memory entry: %w", err)
	}
	return s.Get(key)
}

// Get retrieves the most recent entry by key.
func (s *Store) Get(key string) (*types.MemoryEntry, error) {
	row := s.db.QueryRow(`
		SELECT id, key, value, source_tool, session_id, tags, confidence, created_at, updated_at, content_hash
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
	query := `SELECT id, key, value, source_tool, session_id, tags, confidence, created_at, updated_at, content_hash
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

// --- Edge operations (knowledge graph) --------------------------------------

// AddEdge creates or updates a directed edge between two memory keys.
func (s *Store) AddEdge(fromKey, toKey, relation, sourceTool, sessionID string, confidence float64) (*types.MemoryEdge, error) {
	if relation == "" {
		relation = "related-to"
	}
	if sourceTool == "" {
		sourceTool = "cli"
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)

	_, err := s.db.Exec(`
		INSERT INTO memory_edges (from_key, to_key, relation, confidence, source_tool, session_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(from_key, to_key, relation) DO UPDATE SET
			confidence  = excluded.confidence,
			source_tool = excluded.source_tool,
			session_id  = excluded.session_id`,
		fromKey, toKey, relation, confidence, sourceTool, sessionID, now,
	)
	if err != nil {
		return nil, fmt.Errorf("upsert edge: %w", err)
	}

	row := s.db.QueryRow(`
		SELECT id, from_key, to_key, relation, confidence, source_tool, session_id, created_at
		FROM memory_edges WHERE from_key = ? AND to_key = ? AND relation = ?`,
		fromKey, toKey, relation,
	)
	return scanEdge(row)
}

// GetEdges returns all edges from or to a key.
func (s *Store) GetEdges(key string) ([]*types.MemoryEdge, error) {
	rows, err := s.db.Query(`
		SELECT id, from_key, to_key, relation, confidence, source_tool, session_id, created_at
		FROM memory_edges WHERE from_key = ? OR to_key = ?
		ORDER BY created_at DESC`, key, key,
	)
	if err != nil {
		return nil, fmt.Errorf("get edges: %w", err)
	}
	defer rows.Close()

	var edges []*types.MemoryEdge
	for rows.Next() {
		e, err := scanEdge(rows)
		if err != nil {
			return nil, err
		}
		edges = append(edges, e)
	}
	return edges, rows.Err()
}

// GetRelated returns all memory entries connected to a key via edges.
func (s *Store) GetRelated(key string) ([]*types.MemoryEntry, error) {
	rows, err := s.db.Query(`
		SELECT DISTINCT m.id, m.key, m.value, m.source_tool, m.session_id, m.tags, m.confidence, m.created_at, m.updated_at, m.content_hash
		FROM memory_entries m
		INNER JOIN memory_edges e ON (e.to_key = m.key AND e.from_key = ?) OR (e.from_key = m.key AND e.to_key = ?)
		ORDER BY m.updated_at DESC`, key, key,
	)
	if err != nil {
		return nil, fmt.Errorf("get related: %w", err)
	}
	defer rows.Close()

	var entries []*types.MemoryEntry
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// DeleteEdge removes a specific edge.
func (s *Store) DeleteEdge(fromKey, toKey, relation string) error {
	if relation == "" {
		relation = "related-to"
	}
	res, err := s.db.Exec(`DELETE FROM memory_edges WHERE from_key = ? AND to_key = ? AND relation = ?`,
		fromKey, toKey, relation)
	if err != nil {
		return fmt.Errorf("delete edge: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("edge %q -> %q (%s) not found", fromKey, toKey, relation)
	}
	return nil
}

// --- Search operations (Feature 5) ------------------------------------------

// Search performs full-text search across keys, values, and tags using LIKE.
func (s *Store) Search(query string) ([]*types.MemoryEntry, error) {
	pattern := "%" + query + "%"
	rows, err := s.db.Query(`
		SELECT id, key, value, source_tool, session_id, tags, confidence, created_at, updated_at, content_hash
		FROM memory_entries
		WHERE key LIKE ? OR value LIKE ? OR tags LIKE ?
		ORDER BY
			CASE WHEN key LIKE ? THEN 0 ELSE 1 END,
			updated_at DESC`,
		pattern, pattern, pattern, pattern,
	)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer rows.Close()

	var entries []*types.MemoryEntry
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// SearchByTag returns entries that have a specific tag.
func (s *Store) SearchByTag(tag string) ([]*types.MemoryEntry, error) {
	// Tags are stored as JSON arrays, so we search for the quoted tag string.
	pattern := `%"` + tag + `"%`
	rows, err := s.db.Query(`
		SELECT id, key, value, source_tool, session_id, tags, confidence, created_at, updated_at, content_hash
		FROM memory_entries
		WHERE tags LIKE ?
		ORDER BY updated_at DESC`, pattern,
	)
	if err != nil {
		return nil, fmt.Errorf("search by tag: %w", err)
	}
	defer rows.Close()

	var entries []*types.MemoryEntry
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// AddTags appends tags to an existing entry.
func (s *Store) AddTags(key string, newTags []string) (*types.MemoryEntry, error) {
	entry, err := s.Get(key)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	for _, t := range entry.Tags {
		seen[t] = true
	}
	for _, t := range newTags {
		if !seen[t] {
			entry.Tags = append(entry.Tags, t)
			seen[t] = true
		}
	}
	tagsStr := joinTags(entry.Tags)
	_, err = s.db.Exec(`UPDATE memory_entries SET tags = ?, updated_at = ? WHERE key = ?`,
		tagsStr, time.Now().UTC().Format(time.RFC3339Nano), key)
	if err != nil {
		return nil, fmt.Errorf("add tags: %w", err)
	}
	return s.Get(key)
}

// AllEdges returns every edge in the store.
func (s *Store) AllEdges() ([]*types.MemoryEdge, error) {
	rows, err := s.db.Query(`
		SELECT id, from_key, to_key, relation, confidence, source_tool, session_id, created_at
		FROM memory_edges ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("all edges: %w", err)
	}
	defer rows.Close()

	var edges []*types.MemoryEdge
	for rows.Next() {
		e, err := scanEdge(rows)
		if err != nil {
			return nil, err
		}
		edges = append(edges, e)
	}
	return edges, rows.Err()
}

// --- Internal helpers -------------------------------------------------------

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
		e                    types.MemoryEntry
		tagsJSON             sql.NullString
		createdAt, updatedAt string
	)
	err := scanner.Scan(
		&e.ID, &e.Key, &e.Value, &e.SourceTool, &e.SessionID,
		&tagsJSON, &e.Confidence, &createdAt, &updatedAt, &e.ContentHash,
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

// scanEdge scans a single memory_edges row.
func scanEdge(scanner interface {
	Scan(...any) error
}) (*types.MemoryEdge, error) {
	var (
		e         types.MemoryEdge
		createdAt string
	)
	err := scanner.Scan(
		&e.ID, &e.FromKey, &e.ToKey, &e.Relation, &e.Confidence,
		&e.SourceTool, &e.SessionID, &createdAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan memory edge: %w", err)
	}
	if t, err := time.Parse(time.RFC3339Nano, createdAt); err == nil {
		e.CreatedAt = t
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
