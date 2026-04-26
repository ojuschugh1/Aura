package wiki

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"time"

	"github.com/ojuschugh1/aura/pkg/types"
)

// AuditChain provides a tamper-evident log of all wiki mutations.
// Each entry's hash includes the previous entry's hash, forming a chain.
// If any entry is modified or deleted, the chain breaks — detectable
// by VerifyChain().
type AuditChain struct {
	db *sql.DB
}

// NewAuditChain creates an AuditChain backed by the given database.
func NewAuditChain(db *sql.DB) *AuditChain {
	return &AuditChain{db: db}
}

// Record appends a new entry to the audit chain. The entry's hash is
// computed from: prev_hash + slug + action + agent + summary + timestamp.
func (a *AuditChain) Record(slug, action, agent, summary string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)

	// Get the hash of the most recent entry (the chain tip).
	prevHash := a.lastHash()

	// Compute this entry's hash.
	payload := fmt.Sprintf("%s|%s|%s|%s|%s|%s", prevHash, slug, action, agent, summary, now)
	entryHash := fmt.Sprintf("%x", sha256.Sum256([]byte(payload)))

	_, err := a.db.Exec(`
		INSERT INTO wiki_audit (page_slug, action, agent, prev_hash, entry_hash, summary, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		slug, action, agent, prevHash, entryHash, summary, now,
	)
	if err != nil {
		return fmt.Errorf("audit record: %w", err)
	}
	return nil
}

// History returns the audit trail for a specific page, most recent first.
func (a *AuditChain) History(slug string, limit int) ([]*types.WikiAuditEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := a.db.Query(`
		SELECT id, page_slug, action, agent, prev_hash, entry_hash, summary, created_at
		FROM wiki_audit WHERE page_slug = ?
		ORDER BY id DESC LIMIT ?`, slug, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("audit history: %w", err)
	}
	defer rows.Close()
	return scanAuditRows(rows)
}

// FullHistory returns the entire audit chain, ordered by ID ascending.
func (a *AuditChain) FullHistory(limit int) ([]*types.WikiAuditEntry, error) {
	if limit <= 0 {
		limit = 500
	}
	rows, err := a.db.Query(`
		SELECT id, page_slug, action, agent, prev_hash, entry_hash, summary, created_at
		FROM wiki_audit ORDER BY id ASC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("audit full history: %w", err)
	}
	defer rows.Close()
	return scanAuditRows(rows)
}

// VerifyChain walks the entire audit chain and checks that each entry's
// hash correctly chains to the previous entry. Returns the number of
// verified entries and any broken link found.
type ChainVerification struct {
	TotalEntries  int    `json:"total_entries"`
	Verified      int    `json:"verified"`
	Intact        bool   `json:"intact"`
	BrokenAt      *int64 `json:"broken_at,omitempty"`       // ID of the first broken entry
	BrokenMessage string `json:"broken_message,omitempty"`
}

func (a *AuditChain) VerifyChain() (*ChainVerification, error) {
	entries, err := a.FullHistory(0)
	if err != nil {
		return nil, err
	}

	result := &ChainVerification{
		TotalEntries: len(entries),
		Intact:       true,
	}

	prevHash := ""
	for _, e := range entries {
		if e.PrevHash != prevHash {
			result.Intact = false
			result.BrokenAt = &e.ID
			result.BrokenMessage = fmt.Sprintf("entry #%d: expected prev_hash %q, got %q", e.ID, prevHash, e.PrevHash)
			break
		}
		result.Verified++
		prevHash = e.EntryHash
	}

	return result, nil
}

// lastHash returns the entry_hash of the most recent audit entry, or "" if empty.
func (a *AuditChain) lastHash() string {
	var hash sql.NullString
	a.db.QueryRow(`SELECT entry_hash FROM wiki_audit ORDER BY id DESC LIMIT 1`).Scan(&hash)
	if hash.Valid {
		return hash.String
	}
	return ""
}

func scanAuditRows(rows *sql.Rows) ([]*types.WikiAuditEntry, error) {
	var entries []*types.WikiAuditEntry
	for rows.Next() {
		var e types.WikiAuditEntry
		var createdAt string
		if err := rows.Scan(&e.ID, &e.PageSlug, &e.Action, &e.Agent,
			&e.PrevHash, &e.EntryHash, &e.Summary, &createdAt); err != nil {
			continue
		}
		if t, err := time.Parse(time.RFC3339Nano, createdAt); err == nil {
			e.CreatedAt = t
		}
		entries = append(entries, &e)
	}
	return entries, nil
}
