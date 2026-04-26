package db

import (
	"database/sql"
	"fmt"
)

// migration holds a single DDL statement to execute.
type migration struct {
	name string
	sql  string
}

var migrations = []migration{
	{"memory_entries", `
CREATE TABLE IF NOT EXISTS memory_entries (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    key           TEXT    NOT NULL UNIQUE,
    value         TEXT    NOT NULL,
    source_tool   TEXT    NOT NULL DEFAULT 'cli',
    session_id    TEXT    NOT NULL,
    tags          TEXT,
    created_at    TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at    TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    content_hash  TEXT    NOT NULL
)`},
	{"idx_memory_key", "CREATE INDEX IF NOT EXISTS idx_memory_key ON memory_entries(key)"},
	{"idx_memory_source", "CREATE INDEX IF NOT EXISTS idx_memory_source ON memory_entries(source_tool)"},
	{"idx_memory_session", "CREATE INDEX IF NOT EXISTS idx_memory_session ON memory_entries(session_id)"},
	{"idx_memory_hash", "CREATE INDEX IF NOT EXISTS idx_memory_hash ON memory_entries(content_hash)"},

	{"sessions", `
CREATE TABLE IF NOT EXISTS sessions (
    id          TEXT    PRIMARY KEY,
    started_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    ended_at    TEXT,
    status      TEXT    NOT NULL DEFAULT 'active',
    tools       TEXT
)`},

	{"cost_records", `
CREATE TABLE IF NOT EXISTS cost_records (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id      TEXT    NOT NULL REFERENCES sessions(id),
    source_tool     TEXT    NOT NULL,
    model           TEXT    NOT NULL,
    input_tokens    INTEGER NOT NULL DEFAULT 0,
    output_tokens   INTEGER NOT NULL DEFAULT 0,
    original_tokens INTEGER,
    compressed_tokens INTEGER,
    cost_usd        REAL    NOT NULL DEFAULT 0.0,
    saved_usd       REAL    NOT NULL DEFAULT 0.0,
    recorded_at     TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
)`},
	{"idx_cost_session", "CREATE INDEX IF NOT EXISTS idx_cost_session ON cost_records(session_id)"},
	{"idx_cost_recorded", "CREATE INDEX IF NOT EXISTS idx_cost_recorded ON cost_records(recorded_at)"},

	{"escrow_actions", `
CREATE TABLE IF NOT EXISTS escrow_actions (
    id          TEXT    PRIMARY KEY,
    session_id  TEXT    NOT NULL REFERENCES sessions(id),
    action_type TEXT    NOT NULL,
    target      TEXT    NOT NULL,
    params      TEXT,
    agent       TEXT    NOT NULL,
    status      TEXT    NOT NULL DEFAULT 'pending',
    description TEXT,
    decided_at  TEXT,
    decided_by  TEXT,
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
)`},
	{"idx_escrow_session", "CREATE INDEX IF NOT EXISTS idx_escrow_session ON escrow_actions(session_id)"},
	{"idx_escrow_status", "CREATE INDEX IF NOT EXISTS idx_escrow_status ON escrow_actions(status)"},

	{"traces", `
CREATE TABLE IF NOT EXISTS traces (
    id          TEXT    PRIMARY KEY,
    session_id  TEXT    NOT NULL REFERENCES sessions(id),
    file_path   TEXT    NOT NULL,
    duration_ms INTEGER,
    action_count INTEGER NOT NULL DEFAULT 0,
    http_count  INTEGER NOT NULL DEFAULT 0,
    size_bytes  INTEGER NOT NULL DEFAULT 0,
    pinned      INTEGER NOT NULL DEFAULT 0,
    expires_at  TEXT,
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
)`},
	{"idx_traces_session", "CREATE INDEX IF NOT EXISTS idx_traces_session ON traces(session_id)"},
	{"idx_traces_expires", "CREATE INDEX IF NOT EXISTS idx_traces_expires ON traces(expires_at)"},
	{"idx_traces_pinned", "CREATE INDEX IF NOT EXISTS idx_traces_pinned ON traces(pinned)"},

	{"doom_loop_actions", `
CREATE TABLE IF NOT EXISTS doom_loop_actions (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id  TEXT    NOT NULL REFERENCES sessions(id),
    action_type TEXT    NOT NULL,
    target      TEXT    NOT NULL,
    params_hash TEXT    NOT NULL,
    outcome     TEXT    NOT NULL,
    recorded_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
)`},
	{"idx_doom_session", "CREATE INDEX IF NOT EXISTS idx_doom_session ON doom_loop_actions(session_id)"},
	{"idx_doom_fingerprint", "CREATE INDEX IF NOT EXISTS idx_doom_fingerprint ON doom_loop_actions(action_type, target, params_hash)"},

	{"dedup_cache", `
CREATE TABLE IF NOT EXISTS dedup_cache (
    content_hash TEXT    PRIMARY KEY,
    token_count  INTEGER NOT NULL,
    first_seen   TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    last_seen    TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    hit_count    INTEGER NOT NULL DEFAULT 1
)`},

	{"agent_activity_log", `
CREATE TABLE IF NOT EXISTS agent_activity_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id  TEXT    NOT NULL REFERENCES sessions(id),
    agent       TEXT    NOT NULL,
    operation   TEXT    NOT NULL,
    key         TEXT    NOT NULL,
    recorded_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
)`},
	{"idx_activity_session", "CREATE INDEX IF NOT EXISTS idx_activity_session ON agent_activity_log(session_id)"},
	{"idx_activity_agent", "CREATE INDEX IF NOT EXISTS idx_activity_agent ON agent_activity_log(agent)"},

	{"routing_decisions", `
CREATE TABLE IF NOT EXISTS routing_decisions (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id      TEXT    NOT NULL REFERENCES sessions(id),
    task_hash       TEXT    NOT NULL,
    classification  TEXT    NOT NULL,
    selected_model  TEXT    NOT NULL,
    reasoning       TEXT,
    latency_ms      INTEGER,
    decided_at      TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
)`},
	{"idx_routing_session", "CREATE INDEX IF NOT EXISTS idx_routing_session ON routing_decisions(session_id)"},
}

// RunMigrations executes all schema migrations. All statements are idempotent.
func RunMigrations(db *sql.DB) error {
	for _, m := range migrations {
		if _, err := db.Exec(m.sql); err != nil {
			return fmt.Errorf("migration %s: %w", m.name, err)
		}
	}
	return nil
}
