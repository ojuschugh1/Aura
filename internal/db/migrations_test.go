package db

import (
	"testing"
)

// expectedTables lists every table that RunMigrations must create.
var expectedTables = []string{
	"memory_entries",
	"sessions",
	"cost_records",
	"escrow_actions",
	"traces",
	"doom_loop_actions",
	"dedup_cache",
	"agent_activity_log",
	"routing_decisions",
}

func TestRunMigrations_CreatesAllTables(t *testing.T) {
	db := openTestDB(t)
	if err := RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	for _, table := range expectedTables {
		var name string
		err := db.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?",
			table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %q not found after migration: %v", table, err)
		}
	}
}

func TestRunMigrations_Idempotent(t *testing.T) {
	db := openTestDB(t)

	// Run migrations three times; none should error.
	for i := 0; i < 3; i++ {
		if err := RunMigrations(db); err != nil {
			t.Fatalf("RunMigrations (run %d): %v", i+1, err)
		}
	}

	// Verify tables still exist after repeated runs.
	for _, table := range expectedTables {
		var name string
		err := db.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?",
			table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %q missing after idempotent migrations: %v", table, err)
		}
	}
}

func TestRunMigrations_CreatesIndexes(t *testing.T) {
	db := openTestDB(t)
	if err := RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	// Spot-check a few indexes from different tables.
	indexes := []string{
		"idx_memory_key",
		"idx_memory_session",
		"idx_cost_session",
		"idx_escrow_status",
		"idx_traces_session",
		"idx_doom_fingerprint",
		"idx_activity_agent",
		"idx_routing_session",
	}
	for _, idx := range indexes {
		var name string
		err := db.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='index' AND name=?",
			idx,
		).Scan(&name)
		if err != nil {
			t.Errorf("index %q not found: %v", idx, err)
		}
	}
}

func TestRunMigrations_ForeignKeysEnforced(t *testing.T) {
	db := openTestDB(t)
	if err := RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	// Inserting a cost_record that references a non-existent session should fail
	// because foreign_keys=ON was set by Open.
	_, err := db.Exec(
		`INSERT INTO cost_records (session_id, source_tool, model, cost_usd, saved_usd)
		 VALUES ('nonexistent', 'test', 'gpt-4', 0.0, 0.0)`,
	)
	if err == nil {
		t.Error("expected foreign key violation for non-existent session_id, got nil")
	}
}

func TestRunMigrations_InsertAndQueryRoundTrip(t *testing.T) {
	db := openTestDB(t)
	if err := RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	// Insert a session, then a cost_record referencing it.
	_, err := db.Exec(`INSERT INTO sessions (id) VALUES ('sess-1')`)
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}

	_, err = db.Exec(
		`INSERT INTO cost_records (session_id, source_tool, model, cost_usd, saved_usd)
		 VALUES ('sess-1', 'cli', 'gpt-4', 0.01, 0.005)`,
	)
	if err != nil {
		t.Fatalf("insert cost_record: %v", err)
	}

	var costUSD float64
	err = db.QueryRow("SELECT cost_usd FROM cost_records WHERE session_id='sess-1'").Scan(&costUSD)
	if err != nil {
		t.Fatalf("query cost_record: %v", err)
	}
	if costUSD != 0.01 {
		t.Errorf("expected cost_usd=0.01, got %f", costUSD)
	}
}
