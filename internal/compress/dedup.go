package compress

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"time"
)

// DedupCache manages the deduplication cache in SQLite.
type DedupCache struct {
	db *sql.DB
}

// NewDedupCache creates a DedupCache backed by the given database.
func NewDedupCache(db *sql.DB) *DedupCache {
	return &DedupCache{db: db}
}

// Seen returns true if the content hash is already in the cache, and updates last_seen/hit_count.
func (c *DedupCache) Seen(content string) (bool, error) {
	hash := hashContent(content)
	now := time.Now().UTC().Format(time.RFC3339Nano)

	res, err := c.db.Exec(`
		UPDATE dedup_cache SET last_seen=?, hit_count=hit_count+1 WHERE content_hash=?`,
		now, hash,
	)
	if err != nil {
		return false, fmt.Errorf("dedup update: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// Record inserts a new entry into the dedup cache.
func (c *DedupCache) Record(content string, tokenCount int) error {
	hash := hashContent(content)
	now := time.Now().UTC().Format(time.RFC3339Nano)

	_, err := c.db.Exec(`
		INSERT OR IGNORE INTO dedup_cache (content_hash, token_count, first_seen, last_seen, hit_count)
		VALUES (?, ?, ?, ?, 1)`,
		hash, tokenCount, now, now,
	)
	if err != nil {
		return fmt.Errorf("dedup insert: %w", err)
	}
	return nil
}

// hashContent returns the SHA-256 hex digest of content.
func hashContent(content string) string {
	sum := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", sum)
}
