package autocapture

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

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

// newTestStore returns a memory.Store backed by a fresh test database.
func newTestStore(t *testing.T) *memory.Store {
	t.Helper()
	return memory.New(openTestDB(t))
}

// writeJSONL writes JSONL content to a temp file and returns the path.
func writeJSONL(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "transcript.jsonl")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return p
}

// --- ProcessTranscript: JSONL captures decisions ----------------------------

func TestProcessTranscript_CapturesDecisionsFromJSONL(t *testing.T) {
	store := newTestStore(t)
	engine := NewCaptureEngine(store, DefaultCaptureConfig())

	jsonl := `{"role":"assistant","content":"We decided to use PostgreSQL for the database.","timestamp":"2024-01-01T00:00:00Z"}
{"role":"user","content":"Sounds good.","timestamp":"2024-01-01T00:00:01Z"}
{"role":"assistant","content":"Let's use Redis for caching.","timestamp":"2024-01-01T00:00:02Z"}`

	path := writeJSONL(t, jsonl)

	count, err := engine.ProcessTranscript("sess-1", path)
	if err != nil {
		t.Fatalf("ProcessTranscript: %v", err)
	}
	if count != 2 {
		t.Errorf("captured = %d, want 2", count)
	}

	// Verify entries are in the store with source_tool = "auto-capture".
	entries, err := store.List(memory.ListFilter{Agent: "auto-capture"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("store entries = %d, want 2", len(entries))
	}
	for _, e := range entries {
		if e.SourceTool != "auto-capture" {
			t.Errorf("SourceTool = %q, want %q", e.SourceTool, "auto-capture")
		}
		if e.SessionID != "sess-1" {
			t.Errorf("SessionID = %q, want %q", e.SessionID, "sess-1")
		}
	}
}

// --- MinConfidence filtering ------------------------------------------------

func TestProcessTranscript_MinConfidenceFiltering(t *testing.T) {
	store := newTestStore(t)
	cfg := CaptureConfig{
		Enabled:       true,
		MinConfidence: 0.85, // only strong patterns (0.9) should pass
	}
	engine := NewCaptureEngine(store, cfg)

	// "We decided" is strong (0.9), "Let's use" is weak (0.7).
	jsonl := `{"role":"assistant","content":"We decided to use Go for the backend.","timestamp":"2024-01-01T00:00:00Z"}
{"role":"assistant","content":"Let's use React for the frontend.","timestamp":"2024-01-01T00:00:01Z"}`

	path := writeJSONL(t, jsonl)

	count, err := engine.ProcessTranscript("sess-1", path)
	if err != nil {
		t.Fatalf("ProcessTranscript: %v", err)
	}
	if count != 1 {
		t.Errorf("captured = %d, want 1 (only strong pattern)", count)
	}

	// Verify only the strong-confidence entry was stored.
	entries, err := store.List(memory.ListFilter{Agent: "auto-capture"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("store entries = %d, want 1", len(entries))
	}
}

// --- Disabled engine returns 0 ----------------------------------------------

func TestProcessTranscript_DisabledEngineReturnsZero(t *testing.T) {
	store := newTestStore(t)
	cfg := CaptureConfig{Enabled: false}
	engine := NewCaptureEngine(store, cfg)

	jsonl := `{"role":"assistant","content":"We decided to use PostgreSQL.","timestamp":"2024-01-01T00:00:00Z"}`
	path := writeJSONL(t, jsonl)

	count, err := engine.ProcessTranscript("sess-1", path)
	if err != nil {
		t.Fatalf("ProcessTranscript: %v", err)
	}
	if count != 0 {
		t.Errorf("captured = %d, want 0 (engine disabled)", count)
	}

	// Store should be empty.
	entries, err := store.List(memory.ListFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("store entries = %d, want 0", len(entries))
	}
}

// --- Deduplication against existing entries ---------------------------------

func TestProcessTranscript_DeduplicatesAgainstExistingEntries(t *testing.T) {
	store := newTestStore(t)
	engine := NewCaptureEngine(store, DefaultCaptureConfig())

	// Pre-populate the store with an entry that has the same key as a decision.
	// "use postgresql for the database" is the topic extracted from
	// "We decided to use PostgreSQL for the database."
	_, err := store.Add("use postgresql for the database", "old value", "cli", "sess-0")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	jsonl := `{"role":"assistant","content":"We decided to use PostgreSQL for the database.","timestamp":"2024-01-01T00:00:00Z"}`
	path := writeJSONL(t, jsonl)

	count, err := engine.ProcessTranscript("sess-1", path)
	if err != nil {
		t.Fatalf("ProcessTranscript: %v", err)
	}
	if count != 1 {
		t.Errorf("captured = %d, want 1 (upsert)", count)
	}

	// The existing entry should be updated, not duplicated.
	entry, err := store.Get("use postgresql for the database")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	// Value should be the new decision text, not the old value.
	if entry.Value == "old value" {
		t.Error("expected entry to be updated with new decision text")
	}
	// Source tool should now be auto-capture.
	if entry.SourceTool != "auto-capture" {
		t.Errorf("SourceTool = %q, want %q", entry.SourceTool, "auto-capture")
	}

	// Total entries should still be 1 (upsert, not insert).
	entries, err := store.List(memory.ListFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("total entries = %d, want 1 (dedup)", len(entries))
	}
}

// --- ProcessText works with raw text ----------------------------------------

func TestProcessText_CapturesDecisionsFromRawText(t *testing.T) {
	store := newTestStore(t)
	engine := NewCaptureEngine(store, DefaultCaptureConfig())

	text := `# Session Notes

We decided to use Go for the backend.
The approach is to use microservices.
Some unrelated text here.
I chose PostgreSQL for persistence.`

	count, err := engine.ProcessText("sess-1", text)
	if err != nil {
		t.Fatalf("ProcessText: %v", err)
	}
	if count != 3 {
		t.Errorf("captured = %d, want 3", count)
	}

	entries, err := store.List(memory.ListFilter{Agent: "auto-capture"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("store entries = %d, want 3", len(entries))
	}
}

func TestProcessText_DisabledEngineReturnsZero(t *testing.T) {
	store := newTestStore(t)
	cfg := CaptureConfig{Enabled: false}
	engine := NewCaptureEngine(store, cfg)

	count, err := engine.ProcessText("sess-1", "We decided to use Go.")
	if err != nil {
		t.Fatalf("ProcessText: %v", err)
	}
	if count != 0 {
		t.Errorf("captured = %d, want 0 (engine disabled)", count)
	}
}

func TestProcessText_EmptyTextReturnsZero(t *testing.T) {
	store := newTestStore(t)
	engine := NewCaptureEngine(store, DefaultCaptureConfig())

	count, err := engine.ProcessText("sess-1", "")
	if err != nil {
		t.Fatalf("ProcessText: %v", err)
	}
	if count != 0 {
		t.Errorf("captured = %d, want 0", count)
	}
}

func TestProcessText_NoDecisionsReturnsZero(t *testing.T) {
	store := newTestStore(t)
	engine := NewCaptureEngine(store, DefaultCaptureConfig())

	count, err := engine.ProcessText("sess-1", "This is just regular text with no decisions.")
	if err != nil {
		t.Fatalf("ProcessText: %v", err)
	}
	if count != 0 {
		t.Errorf("captured = %d, want 0", count)
	}
}

// --- Batch deduplication ----------------------------------------------------

func TestProcessText_DeduplicatesWithinBatch(t *testing.T) {
	store := newTestStore(t)
	engine := NewCaptureEngine(store, DefaultCaptureConfig())

	// Same decision repeated with identical wording — produces the same key.
	text := `We decided to use PostgreSQL for the database.
We decided to use PostgreSQL for the database.`

	count, err := engine.ProcessText("sess-1", text)
	if err != nil {
		t.Fatalf("ProcessText: %v", err)
	}
	// Both lines produce the same key, so only 1 should be captured.
	if count != 1 {
		t.Errorf("captured = %d, want 1 (batch dedup)", count)
	}
}

// --- ConcatenateEntries helper ----------------------------------------------

func TestConcatenateEntries_OnlyAssistantAndMessage(t *testing.T) {
	entries := []struct {
		role    string
		content string
	}{
		{"user", "user text"},
		{"assistant", "assistant text"},
		{"system", "system text"},
		{"message", "message text"},
	}

	// Build verify.TranscriptEntry slice manually via the helper.
	// We test the exported concatenation logic indirectly through ProcessTranscript,
	// but also test the internal helper directly.
	var verifyEntries []struct {
		Role    string
		Content string
	}
	for _, e := range entries {
		verifyEntries = append(verifyEntries, struct {
			Role    string
			Content string
		}{e.role, e.content})
	}

	// Since concatenateEntries is unexported, we test it through ProcessTranscript
	// with a JSONL file that has mixed roles.
	store := newTestStore(t)
	engine := NewCaptureEngine(store, DefaultCaptureConfig())

	jsonl := `{"role":"user","content":"We decided to use Python.","timestamp":"2024-01-01T00:00:00Z"}
{"role":"assistant","content":"We decided to use Go for the backend.","timestamp":"2024-01-01T00:00:01Z"}
{"role":"system","content":"We decided to use Java.","timestamp":"2024-01-01T00:00:02Z"}`

	path := writeJSONL(t, jsonl)

	count, err := engine.ProcessTranscript("sess-1", path)
	if err != nil {
		t.Fatalf("ProcessTranscript: %v", err)
	}
	// Only the assistant message should be processed.
	if count != 1 {
		t.Errorf("captured = %d, want 1 (only assistant role)", count)
	}
}
