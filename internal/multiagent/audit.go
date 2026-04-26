package multiagent

import (
	"database/sql"
	"fmt"
	"time"
)

// ActivityEntry is a single row from the agent_activity_log.
type ActivityEntry struct {
	Agent      string    `json:"agent"`
	Operation  string    `json:"operation"`
	Key        string    `json:"key"`
	RecordedAt time.Time `json:"recorded_at"`
}

// SessionActivity returns all activity log entries for the given session.
func SessionActivity(db *sql.DB, sessionID string) ([]ActivityEntry, error) {
	rows, err := db.Query(`
		SELECT agent, operation, key, recorded_at
		FROM agent_activity_log WHERE session_id=? ORDER BY recorded_at ASC`,
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("query activity log: %w", err)
	}
	defer rows.Close()

	var entries []ActivityEntry
	for rows.Next() {
		var e ActivityEntry
		var ts string
		if err := rows.Scan(&e.Agent, &e.Operation, &e.Key, &ts); err != nil {
			return nil, err
		}
		if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
			e.RecordedAt = t
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
