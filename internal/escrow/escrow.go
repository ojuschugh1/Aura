package escrow

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ojuschugh1/aura/pkg/types"
)

const defaultTimeoutMinutes = 5

// Store manages escrow action lifecycle in SQLite.
type Store struct {
	db      *sql.DB
	timeout time.Duration
}

// New creates an escrow Store with the given database.
func New(db *sql.DB) *Store {
	return &Store{db: db, timeout: defaultTimeoutMinutes * time.Minute}
}

// Create inserts a new pending EscrowAction and returns it.
func (s *Store) Create(sessionID, actionType, target, agent, description string, params map[string]interface{}) (*types.EscrowAction, error) {
	id := uuid.New().String()
	now := time.Now().UTC()

	paramsJSON := ""
	if params != nil {
		b, _ := json.Marshal(params)
		paramsJSON = string(b)
	}

	_, err := s.db.Exec(`
		INSERT INTO escrow_actions (id, session_id, action_type, target, params, agent, status, description, created_at)
		VALUES (?, ?, ?, ?, ?, ?, 'pending', ?, ?)`,
		id, sessionID, actionType, target, paramsJSON, agent, description, now.Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, fmt.Errorf("create escrow action: %w", err)
	}

	return &types.EscrowAction{
		ID: id, SessionID: sessionID, ActionType: actionType,
		Target: target, Params: params, Agent: agent,
		Status: "pending", Description: description, CreatedAt: now,
	}, nil
}

// Decide approves or denies a pending escrow action.
func (s *Store) Decide(id, decision, decidedBy string) error {
	if decision != "approve" && decision != "deny" {
		return fmt.Errorf("invalid decision %q: must be approve or deny", decision)
	}
	status := "approved"
	if decision == "deny" {
		status = "denied"
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.Exec(`
		UPDATE escrow_actions SET status=?, decided_at=?, decided_by=? WHERE id=? AND status='pending'`,
		status, now, decidedBy, id,
	)
	if err != nil {
		return fmt.Errorf("decide escrow: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("escrow action %s not found or already decided", id)
	}
	return nil
}

// TimeoutExpired auto-denies pending actions older than the configured timeout.
func (s *Store) TimeoutExpired() (int, error) {
	cutoff := time.Now().UTC().Add(-s.timeout).Format(time.RFC3339Nano)
	res, err := s.db.Exec(`
		UPDATE escrow_actions SET status='timeout', decided_at=?, decided_by='timeout'
		WHERE status='pending' AND created_at < ?`,
		time.Now().UTC().Format(time.RFC3339Nano), cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("timeout escrow: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// Get retrieves an escrow action by ID.
func (s *Store) Get(id string) (*types.EscrowAction, error) {
	row := s.db.QueryRow(`
		SELECT id, session_id, action_type, target, params, agent, status, description, decided_at, decided_by, created_at
		FROM escrow_actions WHERE id=?`, id,
	)
	return scanEscrow(row)
}

func scanEscrow(row interface{ Scan(...any) error }) (*types.EscrowAction, error) {
	var (
		e                    types.EscrowAction
		paramsJSON           sql.NullString
		decidedAt, decidedBy sql.NullString
		createdAt            string
	)
	err := row.Scan(&e.ID, &e.SessionID, &e.ActionType, &e.Target, &paramsJSON,
		&e.Agent, &e.Status, &e.Description, &decidedAt, &decidedBy, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("scan escrow: %w", err)
	}
	if paramsJSON.Valid && paramsJSON.String != "" {
		_ = json.Unmarshal([]byte(paramsJSON.String), &e.Params)
	}
	if decidedAt.Valid && decidedAt.String != "" {
		if t, err := time.Parse(time.RFC3339Nano, decidedAt.String); err == nil {
			e.DecidedAt = &t
		}
	}
	e.DecidedBy = decidedBy.String
	if t, err := time.Parse(time.RFC3339Nano, createdAt); err == nil {
		e.CreatedAt = t
	}
	return &e, nil
}
