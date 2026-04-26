package trace

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"time"
)

const (
	DefaultTTLDays  = 14
	DefaultMaxMB    = 500
)

// PruneByTTL removes unpinned traces older than ttlDays days.
// Returns the number of pruned sessions.
func PruneByTTL(db *sql.DB, tracesDir string, ttlDays int) (int, error) {
	cutoff := time.Now().UTC().AddDate(0, 0, -ttlDays).Format(time.RFC3339Nano)

	rows, err := db.Query(
		`SELECT id, file_path FROM traces WHERE pinned=0 AND created_at < ?`, cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("query expired traces: %w", err)
	}
	defer rows.Close()

	return pruneRows(db, rows)
}

// PruneBySize removes the oldest unpinned traces until total size is under maxMB.
func PruneBySize(db *sql.DB, tracesDir string, maxMB int64) (int, error) {
	maxBytes := maxMB * 1024 * 1024

	var totalBytes int64
	_ = db.QueryRow(`SELECT COALESCE(SUM(size_bytes),0) FROM traces`).Scan(&totalBytes)
	if totalBytes <= maxBytes {
		return 0, nil
	}

	rows, err := db.Query(
		`SELECT id, file_path FROM traces WHERE pinned=0 ORDER BY created_at ASC`,
	)
	if err != nil {
		return 0, fmt.Errorf("query traces for size pruning: %w", err)
	}
	defer rows.Close()

	var pruned int
	for rows.Next() && totalBytes > maxBytes {
		var id, filePath string
		if err := rows.Scan(&id, &filePath); err != nil {
			continue
		}
		info, _ := os.Stat(filePath)
		if err := deleteTrace(db, id, filePath); err == nil {
			pruned++
			slog.Info("pruned trace (size limit)", "session_id", id)
			if info != nil {
				totalBytes -= info.Size()
			}
		}
	}
	return pruned, nil
}

// pruneRows deletes each trace row and its file, returning the count pruned.
func pruneRows(db *sql.DB, rows *sql.Rows) (int, error) {
	var pruned int
	for rows.Next() {
		var id, filePath string
		if err := rows.Scan(&id, &filePath); err != nil {
			continue
		}
		if err := deleteTrace(db, id, filePath); err == nil {
			pruned++
			slog.Info("pruned trace (TTL)", "session_id", id)
		}
	}
	return pruned, rows.Err()
}

// deleteTrace removes the trace file and its database row.
func deleteTrace(db *sql.DB, id, filePath string) error {
	_ = os.Remove(filePath)
	_, err := db.Exec(`DELETE FROM traces WHERE id=?`, id)
	return err
}
