package doomloop

import (
	"database/sql"
	"fmt"
	"sync"
	"time"
)

const repeatThreshold = 3 // alert after this many consecutive failures

// Alert is returned when a doom loop is detected.
type Alert struct {
	ActionType  string
	Target      string
	Repetitions int
	Suggestion  string
}

// Detector tracks agent actions per session and detects doom loops.
type Detector struct {
	mu       sync.Mutex
	db       *sql.DB
	counters map[string]int // fingerprint → consecutive failure count
}

// New creates a Detector backed by the given database.
func New(db *sql.DB) *Detector {
	return &Detector{db: db, counters: make(map[string]int)}
}

// Record records an action and returns an Alert if a doom loop is detected.
func (d *Detector) Record(sessionID string, a Action) (*Alert, error) {
	fp := Fingerprint(a)
	now := time.Now().UTC().Format(time.RFC3339Nano)

	_, err := d.db.Exec(`
		INSERT INTO doom_loop_actions (session_id, action_type, target, params_hash, outcome, recorded_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		sessionID, a.Type, a.Target, fp, a.Outcome, now,
	)
	if err != nil {
		return nil, fmt.Errorf("record doom loop action: %w", err)
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if a.Outcome == "failure" {
		d.counters[fp]++
	} else {
		// Reset on success or a different action.
		delete(d.counters, fp)
	}

	count := d.counters[fp]
	if count >= repeatThreshold {
		return &Alert{
			ActionType:  a.Type,
			Target:      a.Target,
			Repetitions: count,
			Suggestion:  suggestion(a),
		}, nil
	}
	return nil, nil
}

// Reset clears all counters for a session (call on session end).
func (d *Detector) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.counters = make(map[string]int)
}

// suggestion returns a human-readable hint based on the action type.
func suggestion(a Action) string {
	switch a.Type {
	case "package_install":
		return fmt.Sprintf("Agent has attempted to install %q %d+ times — check network or package name", a.Target, repeatThreshold)
	case "file_write":
		return fmt.Sprintf("Agent has attempted to write %q %d+ times — check file permissions", a.Target, repeatThreshold)
	case "shell":
		return fmt.Sprintf("Agent has attempted to run %q %d+ times — check command syntax or environment", a.Target, repeatThreshold)
	default:
		return fmt.Sprintf("Agent has repeated %q on %q %d+ times — consider intervening", a.Type, a.Target, repeatThreshold)
	}
}
