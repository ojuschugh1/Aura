package session

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ojuschugh1/aura/pkg/types"
)

// create inserts a new session row and returns the new Session.
func create(db *sql.DB) (*types.Session, error) {
	id := uuid.New().String()
	now := time.Now().UTC()

	_, err := db.Exec(
		`INSERT INTO sessions (id, started_at, status) VALUES (?, ?, 'active')`,
		id, now.Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, fmt.Errorf("insert session: %w", err)
	}
	return &types.Session{ID: id, StartedAt: now, Status: "active"}, nil
}

// get retrieves a session by ID.
func get(db *sql.DB, id string) (*types.Session, error) {
	row := db.QueryRow(
		`SELECT id, started_at, ended_at, status, tools FROM sessions WHERE id = ?`, id,
	)
	return scanSession(row)
}

// end marks a session as completed and sets ended_at.
func end(db *sql.DB, id string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := db.Exec(
		`UPDATE sessions SET status='completed', ended_at=? WHERE id=?`, now, id,
	)
	if err != nil {
		return fmt.Errorf("end session: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("session %s not found", id)
	}
	return nil
}

// list returns all sessions ordered by started_at descending.
func list(db *sql.DB) ([]*types.Session, error) {
	rows, err := db.Query(
		`SELECT id, started_at, ended_at, status, tools FROM sessions ORDER BY started_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*types.Session
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

// scanSession scans a single session row from either *sql.Row or *sql.Rows.
func scanSession(scanner interface {
	Scan(...any) error
}) (*types.Session, error) {
	var (
		id, startedAt, status string
		endedAt               sql.NullString
		toolsJSON             sql.NullString
	)
	if err := scanner.Scan(&id, &startedAt, &endedAt, &status, &toolsJSON); err != nil {
		return nil, fmt.Errorf("scan session: %w", err)
	}

	s := &types.Session{ID: id, Status: status}

	if t, err := time.Parse(time.RFC3339Nano, startedAt); err == nil {
		s.StartedAt = t
	}
	if endedAt.Valid && endedAt.String != "" {
		if t, err := time.Parse(time.RFC3339Nano, endedAt.String); err == nil {
			s.EndedAt = &t
		}
	}
	if toolsJSON.Valid && toolsJSON.String != "" {
		_ = json.Unmarshal([]byte(toolsJSON.String), &s.Tools)
	}
	return s, nil
}
