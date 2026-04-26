package multiagent

import (
	"database/sql"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ojuschugh1/aura/internal/db"
	"github.com/ojuschugh1/aura/internal/memory"
)

// openTestDB creates an isolated SQLite database with all migrations applied.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	d, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	if err := db.RunMigrations(d); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

// insertSession creates a session row so foreign key constraints are satisfied.
func insertSession(t *testing.T, d *sql.DB, sessionID string) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := d.Exec(
		`INSERT INTO sessions (id, started_at, status) VALUES (?, ?, 'active')`,
		sessionID, now,
	)
	if err != nil {
		t.Fatalf("insert session %s: %v", sessionID, err)
	}
}

// newTestSharedMemory returns a SharedMemory backed by a fresh test database.
func newTestSharedMemory(t *testing.T) (*SharedMemory, *sql.DB) {
	t.Helper()
	d := openTestDB(t)
	store := memory.New(d)
	return New(store, d), d
}

// --- Write (Req 12.2) -------------------------------------------------------

func TestWrite_StoresEntry(t *testing.T) {
	sm, d := newTestSharedMemory(t)
	insertSession(t, d, "sess-1")

	entry, err := sm.Write("db.host", "localhost:5432", "agent-a", "sess-1")
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	if entry.Key != "db.host" {
		t.Errorf("Key = %q, want %q", entry.Key, "db.host")
	}
	if entry.Value != "localhost:5432" {
		t.Errorf("Value = %q, want %q", entry.Value, "localhost:5432")
	}
	if entry.SourceTool != "agent-a" {
		t.Errorf("SourceTool = %q, want %q", entry.SourceTool, "agent-a")
	}
}

func TestWrite_LastWriterWins(t *testing.T) {
	sm, d := newTestSharedMemory(t)
	insertSession(t, d, "sess-1")

	// Agent A writes first.
	if _, err := sm.Write("config.port", "8080", "agent-a", "sess-1"); err != nil {
		t.Fatalf("Write agent-a: %v", err)
	}

	// Agent B overwrites the same key.
	entry, err := sm.Write("config.port", "9090", "agent-b", "sess-1")
	if err != nil {
		t.Fatalf("Write agent-b: %v", err)
	}

	if entry.Value != "9090" {
		t.Errorf("Value = %q, want %q (last writer wins)", entry.Value, "9090")
	}
	if entry.SourceTool != "agent-b" {
		t.Errorf("SourceTool = %q, want %q", entry.SourceTool, "agent-b")
	}
}

// --- Concurrent writes (Req 12.2) -------------------------------------------

func TestWrite_ConcurrentWritesDoNotCorruptData(t *testing.T) {
	sm, d := newTestSharedMemory(t)
	insertSession(t, d, "sess-1")

	const numWriters = 10
	var wg sync.WaitGroup
	wg.Add(numWriters)

	// Each goroutine writes to a unique key concurrently.
	for i := 0; i < numWriters; i++ {
		go func(idx int) {
			defer wg.Done()
			key := "concurrent-key"
			val := string(rune('A' + idx))
			if _, err := sm.Write(key, val, "agent-"+val, "sess-1"); err != nil {
				t.Errorf("Write goroutine %d: %v", idx, err)
			}
		}(i)
	}
	wg.Wait()

	// The key should exist with one of the written values (last writer wins).
	entry, err := sm.Read("concurrent-key", "reader", "sess-1")
	if err != nil {
		t.Fatalf("Read after concurrent writes: %v", err)
	}
	if entry.Value == "" {
		t.Error("expected non-empty value after concurrent writes")
	}
}

func TestWrite_ConcurrentWritesToDistinctKeys(t *testing.T) {
	sm, d := newTestSharedMemory(t)
	insertSession(t, d, "sess-1")

	const numWriters = 10
	var wg sync.WaitGroup
	wg.Add(numWriters)

	for i := 0; i < numWriters; i++ {
		go func(idx int) {
			defer wg.Done()
			key := "key-" + string(rune('a'+idx))
			val := "val-" + string(rune('a'+idx))
			if _, err := sm.Write(key, val, "agent", "sess-1"); err != nil {
				t.Errorf("Write goroutine %d: %v", idx, err)
			}
		}(i)
	}
	wg.Wait()

	// All keys should be present.
	for i := 0; i < numWriters; i++ {
		key := "key-" + string(rune('a'+i))
		entry, err := sm.Read(key, "reader", "sess-1")
		if err != nil {
			t.Errorf("Read %s: %v", key, err)
			continue
		}
		want := "val-" + string(rune('a'+i))
		if entry.Value != want {
			t.Errorf("key %s: Value = %q, want %q", key, entry.Value, want)
		}
	}
}

// --- Read (Req 12.2) --------------------------------------------------------

func TestRead_RetrievesWrittenEntry(t *testing.T) {
	sm, d := newTestSharedMemory(t)
	insertSession(t, d, "sess-1")

	if _, err := sm.Write("api.url", "https://example.com", "agent-a", "sess-1"); err != nil {
		t.Fatalf("Write: %v", err)
	}

	entry, err := sm.Read("api.url", "agent-b", "sess-1")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if entry.Value != "https://example.com" {
		t.Errorf("Value = %q, want %q", entry.Value, "https://example.com")
	}
}

func TestRead_NonExistentKeyReturnsError(t *testing.T) {
	sm, d := newTestSharedMemory(t)
	insertSession(t, d, "sess-1")

	_, err := sm.Read("nonexistent", "agent-a", "sess-1")
	if err == nil {
		t.Fatal("expected error for non-existent key, got nil")
	}
}

// --- Delete -----------------------------------------------------------------

func TestDelete_RemovesEntry(t *testing.T) {
	sm, d := newTestSharedMemory(t)
	insertSession(t, d, "sess-1")

	if _, err := sm.Write("to-delete", "val", "agent-a", "sess-1"); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if err := sm.Delete("to-delete", "agent-a", "sess-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := sm.Read("to-delete", "agent-a", "sess-1")
	if err == nil {
		t.Fatal("expected error after deleting key, got nil")
	}
}

func TestDelete_NonExistentKeyReturnsError(t *testing.T) {
	sm, d := newTestSharedMemory(t)
	insertSession(t, d, "sess-1")

	err := sm.Delete("nonexistent", "agent-a", "sess-1")
	if err == nil {
		t.Fatal("expected error for non-existent key, got nil")
	}
}

// --- Propagation (Req 12.2) -------------------------------------------------

func TestWrite_PropagationWithin100ms(t *testing.T) {
	sm, d := newTestSharedMemory(t)
	insertSession(t, d, "sess-1")

	// Agent A writes.
	if _, err := sm.Write("fast-key", "fast-val", "agent-a", "sess-1"); err != nil {
		t.Fatalf("Write: %v", err)
	}

	start := time.Now()

	// Agent B reads immediately.
	entry, err := sm.Read("fast-key", "agent-b", "sess-1")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	elapsed := time.Since(start)
	if elapsed > 100*time.Millisecond {
		t.Errorf("propagation took %v, want ≤100ms", elapsed)
	}
	if entry.Value != "fast-val" {
		t.Errorf("Value = %q, want %q", entry.Value, "fast-val")
	}
}

// --- Activity audit (Req 12.6) ----------------------------------------------

func TestWrite_LogsWriteActivity(t *testing.T) {
	sm, d := newTestSharedMemory(t)
	insertSession(t, d, "sess-1")

	if _, err := sm.Write("audit-key", "audit-val", "agent-a", "sess-1"); err != nil {
		t.Fatalf("Write: %v", err)
	}

	entries, err := SessionActivity(d, "sess-1")
	if err != nil {
		t.Fatalf("SessionActivity: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 activity entry, got %d", len(entries))
	}
	if entries[0].Agent != "agent-a" {
		t.Errorf("Agent = %q, want %q", entries[0].Agent, "agent-a")
	}
	if entries[0].Operation != "write" {
		t.Errorf("Operation = %q, want %q", entries[0].Operation, "write")
	}
	if entries[0].Key != "audit-key" {
		t.Errorf("Key = %q, want %q", entries[0].Key, "audit-key")
	}
}

func TestRead_LogsReadActivity(t *testing.T) {
	sm, d := newTestSharedMemory(t)
	insertSession(t, d, "sess-1")

	if _, err := sm.Write("read-audit", "val", "agent-a", "sess-1"); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if _, err := sm.Read("read-audit", "agent-b", "sess-1"); err != nil {
		t.Fatalf("Read: %v", err)
	}

	entries, err := SessionActivity(d, "sess-1")
	if err != nil {
		t.Fatalf("SessionActivity: %v", err)
	}

	// Should have 2 entries: one write by agent-a, one read by agent-b.
	if len(entries) != 2 {
		t.Fatalf("expected 2 activity entries, got %d", len(entries))
	}

	if entries[0].Operation != "write" || entries[0].Agent != "agent-a" {
		t.Errorf("entry[0]: want write by agent-a, got %s by %s", entries[0].Operation, entries[0].Agent)
	}
	if entries[1].Operation != "read" || entries[1].Agent != "agent-b" {
		t.Errorf("entry[1]: want read by agent-b, got %s by %s", entries[1].Operation, entries[1].Agent)
	}
}

func TestDelete_LogsDeleteActivity(t *testing.T) {
	sm, d := newTestSharedMemory(t)
	insertSession(t, d, "sess-1")

	if _, err := sm.Write("del-audit", "val", "agent-a", "sess-1"); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if err := sm.Delete("del-audit", "agent-b", "sess-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	entries, err := SessionActivity(d, "sess-1")
	if err != nil {
		t.Fatalf("SessionActivity: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 activity entries, got %d", len(entries))
	}
	if entries[1].Operation != "delete" || entries[1].Agent != "agent-b" {
		t.Errorf("entry[1]: want delete by agent-b, got %s by %s", entries[1].Operation, entries[1].Agent)
	}
}

func TestSessionActivity_EmptyForUnknownSession(t *testing.T) {
	_, d := newTestSharedMemory(t)

	entries, err := SessionActivity(d, "no-such-session")
	if err != nil {
		t.Fatalf("SessionActivity: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for unknown session, got %d", len(entries))
	}
}

func TestSessionActivity_IsolatesBySessions(t *testing.T) {
	sm, d := newTestSharedMemory(t)
	insertSession(t, d, "sess-1")
	insertSession(t, d, "sess-2")

	if _, err := sm.Write("k1", "v1", "agent-a", "sess-1"); err != nil {
		t.Fatalf("Write sess-1: %v", err)
	}
	if _, err := sm.Write("k2", "v2", "agent-b", "sess-2"); err != nil {
		t.Fatalf("Write sess-2: %v", err)
	}

	entries1, err := SessionActivity(d, "sess-1")
	if err != nil {
		t.Fatalf("SessionActivity sess-1: %v", err)
	}
	if len(entries1) != 1 {
		t.Errorf("sess-1: expected 1 entry, got %d", len(entries1))
	}

	entries2, err := SessionActivity(d, "sess-2")
	if err != nil {
		t.Fatalf("SessionActivity sess-2: %v", err)
	}
	if len(entries2) != 1 {
		t.Errorf("sess-2: expected 1 entry, got %d", len(entries2))
	}
}

func TestSessionActivity_OrderedByTime(t *testing.T) {
	sm, d := newTestSharedMemory(t)
	insertSession(t, d, "sess-1")

	if _, err := sm.Write("k1", "v1", "agent-a", "sess-1"); err != nil {
		t.Fatalf("Write k1: %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	if _, err := sm.Write("k2", "v2", "agent-b", "sess-1"); err != nil {
		t.Fatalf("Write k2: %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	if _, err := sm.Read("k1", "agent-c", "sess-1"); err != nil {
		t.Fatalf("Read k1: %v", err)
	}

	entries, err := SessionActivity(d, "sess-1")
	if err != nil {
		t.Fatalf("SessionActivity: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	for i := 1; i < len(entries); i++ {
		if entries[i].RecordedAt.Before(entries[i-1].RecordedAt) {
			t.Errorf("entry[%d] recorded_at %v is before entry[%d] %v",
				i, entries[i].RecordedAt, i-1, entries[i-1].RecordedAt)
		}
	}
}

// --- Agent filtering (Req 12.4) ---------------------------------------------

func TestActivityLog_FiltersByAgent(t *testing.T) {
	sm, d := newTestSharedMemory(t)
	insertSession(t, d, "sess-1")

	// Multiple agents write entries.
	if _, err := sm.Write("k1", "v1", "agent-a", "sess-1"); err != nil {
		t.Fatalf("Write k1: %v", err)
	}
	if _, err := sm.Write("k2", "v2", "agent-b", "sess-1"); err != nil {
		t.Fatalf("Write k2: %v", err)
	}
	if _, err := sm.Write("k3", "v3", "agent-a", "sess-1"); err != nil {
		t.Fatalf("Write k3: %v", err)
	}

	// Query all activity, then filter by agent.
	all, err := SessionActivity(d, "sess-1")
	if err != nil {
		t.Fatalf("SessionActivity: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 total entries, got %d", len(all))
	}

	// Filter for agent-a entries.
	var agentAEntries []ActivityEntry
	for _, e := range all {
		if e.Agent == "agent-a" {
			agentAEntries = append(agentAEntries, e)
		}
	}
	if len(agentAEntries) != 2 {
		t.Errorf("expected 2 entries for agent-a, got %d", len(agentAEntries))
	}

	// Filter for agent-b entries.
	var agentBEntries []ActivityEntry
	for _, e := range all {
		if e.Agent == "agent-b" {
			agentBEntries = append(agentBEntries, e)
		}
	}
	if len(agentBEntries) != 1 {
		t.Errorf("expected 1 entry for agent-b, got %d", len(agentBEntries))
	}
}

// --- Conflict logging (Req 12.4) --------------------------------------------

func TestWrite_ConflictDetectedOnRapidOverwrite(t *testing.T) {
	sm, d := newTestSharedMemory(t)
	insertSession(t, d, "sess-1")

	// Agent A writes.
	if _, err := sm.Write("conflict-key", "val-a", "agent-a", "sess-1"); err != nil {
		t.Fatalf("Write agent-a: %v", err)
	}

	// Agent B writes the same key immediately (within 100ms window).
	entry, err := sm.Write("conflict-key", "val-b", "agent-b", "sess-1")
	if err != nil {
		t.Fatalf("Write agent-b: %v", err)
	}

	// Last writer wins — agent-b's value should be stored.
	if entry.Value != "val-b" {
		t.Errorf("Value = %q, want %q (last writer wins)", entry.Value, "val-b")
	}
	if entry.SourceTool != "agent-b" {
		t.Errorf("SourceTool = %q, want %q", entry.SourceTool, "agent-b")
	}

	// Both writes should be logged in the activity log.
	activities, err := SessionActivity(d, "sess-1")
	if err != nil {
		t.Fatalf("SessionActivity: %v", err)
	}
	if len(activities) != 2 {
		t.Fatalf("expected 2 activity entries (both writes logged), got %d", len(activities))
	}
}

func TestWrite_SameAgentOverwriteNoConflict(t *testing.T) {
	sm, d := newTestSharedMemory(t)
	insertSession(t, d, "sess-1")

	// Same agent writes twice — not a conflict.
	if _, err := sm.Write("same-agent-key", "val-1", "agent-a", "sess-1"); err != nil {
		t.Fatalf("Write first: %v", err)
	}

	entry, err := sm.Write("same-agent-key", "val-2", "agent-a", "sess-1")
	if err != nil {
		t.Fatalf("Write second: %v", err)
	}

	if entry.Value != "val-2" {
		t.Errorf("Value = %q, want %q", entry.Value, "val-2")
	}
}

// --- Multi-agent shared visibility (Req 12.2) --------------------------------

func TestSharedMemory_AllAgentsSeeAllEntries(t *testing.T) {
	sm, d := newTestSharedMemory(t)
	insertSession(t, d, "sess-1")

	// Agent A writes key-a, Agent B writes key-b.
	if _, err := sm.Write("key-a", "from-a", "agent-a", "sess-1"); err != nil {
		t.Fatalf("Write key-a: %v", err)
	}
	if _, err := sm.Write("key-b", "from-b", "agent-b", "sess-1"); err != nil {
		t.Fatalf("Write key-b: %v", err)
	}

	// Agent C can read both.
	entryA, err := sm.Read("key-a", "agent-c", "sess-1")
	if err != nil {
		t.Fatalf("Read key-a: %v", err)
	}
	if entryA.Value != "from-a" {
		t.Errorf("key-a Value = %q, want %q", entryA.Value, "from-a")
	}

	entryB, err := sm.Read("key-b", "agent-c", "sess-1")
	if err != nil {
		t.Fatalf("Read key-b: %v", err)
	}
	if entryB.Value != "from-b" {
		t.Errorf("key-b Value = %q, want %q", entryB.Value, "from-b")
	}
}

// --- Mixed operations audit trail -------------------------------------------

func TestAuditTrail_MixedOperations(t *testing.T) {
	sm, d := newTestSharedMemory(t)
	insertSession(t, d, "sess-1")

	// Write, read, delete sequence.
	if _, err := sm.Write("trail-key", "val", "agent-a", "sess-1"); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if _, err := sm.Read("trail-key", "agent-b", "sess-1"); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if err := sm.Delete("trail-key", "agent-c", "sess-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	entries, err := SessionActivity(d, "sess-1")
	if err != nil {
		t.Fatalf("SessionActivity: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("expected 3 activity entries, got %d", len(entries))
	}

	expected := []struct {
		agent, op, key string
	}{
		{"agent-a", "write", "trail-key"},
		{"agent-b", "read", "trail-key"},
		{"agent-c", "delete", "trail-key"},
	}

	for i, want := range expected {
		if entries[i].Agent != want.agent {
			t.Errorf("entry[%d] Agent = %q, want %q", i, entries[i].Agent, want.agent)
		}
		if entries[i].Operation != want.op {
			t.Errorf("entry[%d] Operation = %q, want %q", i, entries[i].Operation, want.op)
		}
		if entries[i].Key != want.key {
			t.Errorf("entry[%d] Key = %q, want %q", i, entries[i].Key, want.key)
		}
	}
}
