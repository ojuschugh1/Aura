package trace

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/ojuschugh1/aura/internal/db"
	"github.com/ojuschugh1/aura/pkg/types"
)

// Recorder writes trace entries to a JSONL file and tracks metadata in SQLite.
type Recorder struct {
	db          *sql.DB
	tracesDir   string
	sessionID   string
	file        *os.File
	mu          sync.Mutex
	actionCount int
	httpCount   int
	startedAt   time.Time
}

// NewRecorder creates a Recorder for the given session, opening the JSONL file and inserting a traces row.
func NewRecorder(database *sql.DB, tracesDir, sessionID string) (*Recorder, error) {
	if err := os.MkdirAll(tracesDir, 0o755); err != nil {
		return nil, fmt.Errorf("create traces dir: %w", err)
	}

	filePath := fmt.Sprintf("%s/%s.jsonl", tracesDir, sessionID)
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open trace file: %w", err)
	}

	// Ensure a session row exists before inserting the trace (FK constraint).
	_, err = database.Exec(
		`INSERT OR IGNORE INTO sessions (id, started_at, status) VALUES (?, ?, 'active')`,
		sessionID, db.TimeNow(),
	)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("upsert session: %w", err)
	}

	_, err = database.Exec(
		`INSERT INTO traces (id, session_id, file_path, action_count, http_count, size_bytes)
		 VALUES (?, ?, ?, 0, 0, 0)`,
		sessionID, sessionID, filePath,
	)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("insert trace row: %w", err)
	}

	return &Recorder{
		db:        database,
		tracesDir: tracesDir,
		sessionID: sessionID,
		file:      f,
		startedAt: time.Now(),
	}, nil
}

// Record appends a TraceEntry as a JSON line and updates the traces table counters.
func (r *Recorder) Record(entry types.TraceEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	line, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal entry: %w", err)
	}
	line = append(line, '\n')

	if _, err := r.file.Write(line); err != nil {
		return fmt.Errorf("write trace entry: %w", err)
	}

	r.actionCount++
	if entry.Request != nil || entry.Response != nil {
		r.httpCount++
	}

	info, err := r.file.Stat()
	if err != nil {
		return fmt.Errorf("stat trace file: %w", err)
	}

	_, err = r.db.Exec(
		`UPDATE traces SET action_count=?, http_count=?, size_bytes=? WHERE id=?`,
		r.actionCount, r.httpCount, info.Size(), r.sessionID,
	)
	return err
}

// Close flushes and closes the file, then writes final duration and counts to the traces table.
func (r *Recorder) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	durationMs := time.Since(r.startedAt).Milliseconds()

	var sizeBytes int64
	if info, err := r.file.Stat(); err == nil {
		sizeBytes = info.Size()
	}

	if err := r.file.Close(); err != nil {
		return fmt.Errorf("close trace file: %w", err)
	}

	_, err := r.db.Exec(
		`UPDATE traces SET duration_ms=?, action_count=?, http_count=?, size_bytes=? WHERE id=?`,
		durationMs, r.actionCount, r.httpCount, sizeBytes, r.sessionID,
	)
	return err
}

// Pin sets pinned=1 in the traces table for the given session.
func Pin(database *sql.DB, tracesDir, sessionID string) error {
	_, err := database.Exec(`UPDATE traces SET pinned=1 WHERE id=?`, sessionID)
	return err
}
