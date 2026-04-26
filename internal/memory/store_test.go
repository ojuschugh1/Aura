package memory

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ojuschugh1/aura/internal/db"
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

// newTestStore returns a Store backed by a fresh test database.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	return New(openTestDB(t))
}

// --- Add --------------------------------------------------------------------

func TestAdd_InsertsNewEntry(t *testing.T) {
	s := newTestStore(t)

	entry, err := s.Add("db.host", "localhost:5432", "cli", "sess-1")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	if entry.Key != "db.host" {
		t.Errorf("Key = %q, want %q", entry.Key, "db.host")
	}
	if entry.Value != "localhost:5432" {
		t.Errorf("Value = %q, want %q", entry.Value, "localhost:5432")
	}
	if entry.SourceTool != "cli" {
		t.Errorf("SourceTool = %q, want %q", entry.SourceTool, "cli")
	}
	if entry.SessionID != "sess-1" {
		t.Errorf("SessionID = %q, want %q", entry.SessionID, "sess-1")
	}
	if entry.ID == 0 {
		t.Error("expected non-zero ID")
	}
}

func TestAdd_SetsContentHash(t *testing.T) {
	s := newTestStore(t)

	entry, err := s.Add("key1", "hello world", "cli", "sess-1")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	want := fmt.Sprintf("%x", sha256.Sum256([]byte("hello world")))
	if entry.ContentHash != want {
		t.Errorf("ContentHash = %q, want %q", entry.ContentHash, want)
	}
}

func TestAdd_SetsTimestamps(t *testing.T) {
	s := newTestStore(t)

	before := time.Now().UTC().Add(-time.Second)
	entry, err := s.Add("key1", "val", "cli", "sess-1")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	after := time.Now().UTC().Add(time.Second)

	if entry.CreatedAt.Before(before) || entry.CreatedAt.After(after) {
		t.Errorf("CreatedAt %v not in expected range [%v, %v]", entry.CreatedAt, before, after)
	}
	if entry.UpdatedAt.Before(before) || entry.UpdatedAt.After(after) {
		t.Errorf("UpdatedAt %v not in expected range [%v, %v]", entry.UpdatedAt, before, after)
	}
}

// --- Get --------------------------------------------------------------------

func TestGet_RetrievesExistingEntry(t *testing.T) {
	s := newTestStore(t)

	_, err := s.Add("api.key", "secret-123", "mcp", "sess-1")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	got, err := s.Get("api.key")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if got.Key != "api.key" {
		t.Errorf("Key = %q, want %q", got.Key, "api.key")
	}
	if got.Value != "secret-123" {
		t.Errorf("Value = %q, want %q", got.Value, "secret-123")
	}
}

func TestGet_NonExistentKeyReturnsError(t *testing.T) {
	s := newTestStore(t)

	_, err := s.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent key, got nil")
	}
}

// --- Upsert -----------------------------------------------------------------

func TestAdd_UpsertUpdatesValueAndTimestamp(t *testing.T) {
	s := newTestStore(t)

	original, err := s.Add("config.port", "8080", "cli", "sess-1")
	if err != nil {
		t.Fatalf("Add original: %v", err)
	}

	// Small delay to ensure distinct timestamps.
	time.Sleep(10 * time.Millisecond)

	updated, err := s.Add("config.port", "9090", "mcp", "sess-2")
	if err != nil {
		t.Fatalf("Add upsert: %v", err)
	}

	if updated.Value != "9090" {
		t.Errorf("Value = %q, want %q", updated.Value, "9090")
	}
	if updated.SourceTool != "mcp" {
		t.Errorf("SourceTool = %q, want %q", updated.SourceTool, "mcp")
	}
	if updated.ContentHash == original.ContentHash {
		t.Error("expected ContentHash to change after upsert")
	}
}

func TestAdd_UpsertPreservesCreatedAt(t *testing.T) {
	s := newTestStore(t)

	original, err := s.Add("key1", "val1", "cli", "sess-1")
	if err != nil {
		t.Fatalf("Add original: %v", err)
	}
	originalCreatedAt := original.CreatedAt

	time.Sleep(10 * time.Millisecond)

	updated, err := s.Add("key1", "val2", "mcp", "sess-2")
	if err != nil {
		t.Fatalf("Add upsert: %v", err)
	}

	if !updated.CreatedAt.Equal(originalCreatedAt) {
		t.Errorf("CreatedAt changed: original=%v, updated=%v", originalCreatedAt, updated.CreatedAt)
	}
}

func TestAdd_UpsertUpdatesUpdatedAt(t *testing.T) {
	s := newTestStore(t)

	original, err := s.Add("key1", "val1", "cli", "sess-1")
	if err != nil {
		t.Fatalf("Add original: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	updated, err := s.Add("key1", "val2", "cli", "sess-1")
	if err != nil {
		t.Fatalf("Add upsert: %v", err)
	}

	if !updated.UpdatedAt.After(original.UpdatedAt) {
		t.Errorf("UpdatedAt not advanced: original=%v, updated=%v", original.UpdatedAt, updated.UpdatedAt)
	}
}

// --- List -------------------------------------------------------------------

func TestList_EmptyStoreReturnsEmpty(t *testing.T) {
	s := newTestStore(t)

	entries, err := s.List(ListFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestList_ReturnsAllEntries(t *testing.T) {
	s := newTestStore(t)

	for i := 0; i < 3; i++ {
		key := fmt.Sprintf("key-%d", i)
		if _, err := s.Add(key, "val", "cli", "sess-1"); err != nil {
			t.Fatalf("Add %s: %v", key, err)
		}
	}

	entries, err := s.List(ListFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}
}

func TestList_TruncatesLongValues(t *testing.T) {
	s := newTestStore(t)

	longValue := strings.Repeat("a", 200)
	if _, err := s.Add("long", longValue, "cli", "sess-1"); err != nil {
		t.Fatalf("Add: %v", err)
	}

	entries, err := s.List(ListFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	// Value should be truncated to 80 chars + "…".
	if len(entries[0].Value) > 84 { // 80 + up to 3 bytes for "…"
		t.Errorf("expected truncated value, got length %d", len(entries[0].Value))
	}
	if !strings.HasSuffix(entries[0].Value, "…") {
		t.Errorf("expected truncated value to end with '…', got %q", entries[0].Value[len(entries[0].Value)-3:])
	}
}

func TestList_ShortValuesNotTruncated(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.Add("short", "hello", "cli", "sess-1"); err != nil {
		t.Fatalf("Add: %v", err)
	}

	entries, err := s.List(ListFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if entries[0].Value != "hello" {
		t.Errorf("Value = %q, want %q", entries[0].Value, "hello")
	}
}

func TestList_FilterByAgent(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.Add("k1", "v1", "cli", "sess-1"); err != nil {
		t.Fatalf("Add k1: %v", err)
	}
	if _, err := s.Add("k2", "v2", "mcp", "sess-1"); err != nil {
		t.Fatalf("Add k2: %v", err)
	}
	if _, err := s.Add("k3", "v3", "cli", "sess-1"); err != nil {
		t.Fatalf("Add k3: %v", err)
	}

	entries, err := s.List(ListFilter{Agent: "cli"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries for agent=cli, got %d", len(entries))
	}
	for _, e := range entries {
		if e.SourceTool != "cli" {
			t.Errorf("expected SourceTool=cli, got %q", e.SourceTool)
		}
	}
}

func TestList_FilterByAgentNoMatch(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.Add("k1", "v1", "cli", "sess-1"); err != nil {
		t.Fatalf("Add: %v", err)
	}

	entries, err := s.List(ListFilter{Agent: "nonexistent"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

// --- Delete -----------------------------------------------------------------

func TestDelete_RemovesEntry(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.Add("to-delete", "val", "cli", "sess-1"); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if err := s.Delete("to-delete"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := s.Get("to-delete")
	if err == nil {
		t.Fatal("expected error after deleting key, got nil")
	}
}

func TestDelete_NonExistentKeyReturnsError(t *testing.T) {
	s := newTestStore(t)

	err := s.Delete("nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent key, got nil")
	}
}

func TestDelete_DoesNotAffectOtherEntries(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.Add("keep", "val1", "cli", "sess-1"); err != nil {
		t.Fatalf("Add keep: %v", err)
	}
	if _, err := s.Add("remove", "val2", "cli", "sess-1"); err != nil {
		t.Fatalf("Add remove: %v", err)
	}

	if err := s.Delete("remove"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	got, err := s.Get("keep")
	if err != nil {
		t.Fatalf("Get keep: %v", err)
	}
	if got.Value != "val1" {
		t.Errorf("Value = %q, want %q", got.Value, "val1")
	}
}

// --- Round-trip Add → Get (Req 2.9) -----------------------------------------

func TestRoundTrip_AddThenGetReturnsIdenticalValue(t *testing.T) {
	s := newTestStore(t)

	values := []string{
		"simple value",
		"value with special chars: <>&\"'",
		strings.Repeat("x", 1000),
		"",
		"multi\nline\nvalue",
	}

	for i, val := range values {
		key := fmt.Sprintf("rt-key-%d", i)
		if _, err := s.Add(key, val, "cli", "sess-1"); err != nil {
			t.Fatalf("Add %s: %v", key, err)
		}

		got, err := s.Get(key)
		if err != nil {
			t.Fatalf("Get %s: %v", key, err)
		}
		if got.Value != val {
			t.Errorf("round-trip mismatch for key %q: got %q, want %q", key, got.Value, val)
		}
	}
}

// --- Export / Import (Req 18.5) ---------------------------------------------

func TestExport_WritesJSONFile(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.Add("e1", "val1", "cli", "sess-1"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := s.Add("e2", "val2", "mcp", "sess-2"); err != nil {
		t.Fatalf("Add: %v", err)
	}

	exportPath := filepath.Join(t.TempDir(), "export.json")
	if err := s.Export(exportPath); err != nil {
		t.Fatalf("Export: %v", err)
	}

	data, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	// Verify it's valid JSON.
	var entries []map[string]interface{}
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("exported file is not valid JSON: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries in export, got %d", len(entries))
	}
}

func TestImport_ReadsEntriesIntoStore(t *testing.T) {
	// Create a store, add entries, export them.
	s1 := newTestStore(t)
	if _, err := s1.Add("imp1", "value-one", "cli", "sess-1"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := s1.Add("imp2", "value-two", "mcp", "sess-2"); err != nil {
		t.Fatalf("Add: %v", err)
	}

	exportPath := filepath.Join(t.TempDir(), "export.json")
	if err := s1.Export(exportPath); err != nil {
		t.Fatalf("Export: %v", err)
	}

	// Import into a fresh store.
	s2 := newTestStore(t)
	count, err := s2.Import(exportPath)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if count != 2 {
		t.Errorf("Import count = %d, want 2", count)
	}

	// Verify entries exist in the new store.
	got1, err := s2.Get("imp1")
	if err != nil {
		t.Fatalf("Get imp1: %v", err)
	}
	if got1.Value != "value-one" {
		t.Errorf("imp1 Value = %q, want %q", got1.Value, "value-one")
	}

	got2, err := s2.Get("imp2")
	if err != nil {
		t.Fatalf("Get imp2: %v", err)
	}
	if got2.Value != "value-two" {
		t.Errorf("imp2 Value = %q, want %q", got2.Value, "value-two")
	}
}

func TestExportImport_RoundTripProducesIdenticalContent(t *testing.T) {
	s1 := newTestStore(t)

	// Add several entries with varied data.
	testData := []struct {
		key, value, source, session string
	}{
		{"db.host", "localhost:5432", "cli", "sess-1"},
		{"api.url", "https://example.com/api", "mcp", "sess-1"},
		{"config.debug", "true", "cursor", "sess-2"},
	}
	for _, td := range testData {
		if _, err := s1.Add(td.key, td.value, td.source, td.session); err != nil {
			t.Fatalf("Add %s: %v", td.key, err)
		}
	}

	// Export from s1.
	exportPath := filepath.Join(t.TempDir(), "roundtrip.json")
	if err := s1.Export(exportPath); err != nil {
		t.Fatalf("Export: %v", err)
	}

	// Import into fresh s2.
	s2 := newTestStore(t)
	if _, err := s2.Import(exportPath); err != nil {
		t.Fatalf("Import: %v", err)
	}

	// Verify each entry matches by key and value.
	for _, td := range testData {
		got, err := s2.Get(td.key)
		if err != nil {
			t.Fatalf("Get %s from imported store: %v", td.key, err)
		}
		if got.Value != td.value {
			t.Errorf("key %q: Value = %q, want %q", td.key, got.Value, td.value)
		}
		if got.SourceTool != td.source {
			t.Errorf("key %q: SourceTool = %q, want %q", td.key, got.SourceTool, td.source)
		}
		if got.SessionID != td.session {
			t.Errorf("key %q: SessionID = %q, want %q", td.key, got.SessionID, td.session)
		}
	}
}

func TestImport_OverwritesExistingKeys(t *testing.T) {
	s := newTestStore(t)

	// Pre-populate with an entry.
	if _, err := s.Add("shared-key", "old-value", "cli", "sess-1"); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Create an export file with the same key but different value.
	exportPath := filepath.Join(t.TempDir(), "overwrite.json")
	content := `[{"key":"shared-key","value":"new-value","source_tool":"mcp","session_id":"sess-2"}]`
	if err := os.WriteFile(exportPath, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := s.Import(exportPath); err != nil {
		t.Fatalf("Import: %v", err)
	}

	got, err := s.Get("shared-key")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Value != "new-value" {
		t.Errorf("Value = %q, want %q (import should overwrite)", got.Value, "new-value")
	}
}

func TestImport_InvalidFileReturnsError(t *testing.T) {
	s := newTestStore(t)

	_, err := s.Import("/no/such/file.json")
	if err == nil {
		t.Fatal("expected error for non-existent import file, got nil")
	}
}

func TestImport_InvalidJSONReturnsError(t *testing.T) {
	s := newTestStore(t)

	badPath := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(badPath, []byte("not json"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := s.Import(badPath)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}
