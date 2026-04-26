package trace

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ojuschugh1/aura/internal/db"
	"github.com/ojuschugh1/aura/pkg/types"
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

// =============================================================================
// Recorder tests
// =============================================================================

func TestNewRecorder_CreatesTraceFile(t *testing.T) {
	d := openTestDB(t)
	tracesDir := filepath.Join(t.TempDir(), "traces")
	sessionID := "sess-001"

	rec, err := NewRecorder(d, tracesDir, sessionID)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}
	defer rec.Close()

	expectedPath := filepath.Join(tracesDir, sessionID+".jsonl")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Fatalf("expected trace file %s to exist", expectedPath)
	}
}

func TestNewRecorder_InsertsTraceRow(t *testing.T) {
	d := openTestDB(t)
	tracesDir := filepath.Join(t.TempDir(), "traces")
	sessionID := "sess-002"

	rec, err := NewRecorder(d, tracesDir, sessionID)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}
	defer rec.Close()

	var count int
	if err := d.QueryRow(`SELECT COUNT(*) FROM traces WHERE id=?`, sessionID).Scan(&count); err != nil {
		t.Fatalf("query traces: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 trace row, got %d", count)
	}
}

func TestRecord_WritesJSONLineWithCorrectStructure(t *testing.T) {
	d := openTestDB(t)
	tracesDir := filepath.Join(t.TempDir(), "traces")
	sessionID := "sess-003"

	rec, err := NewRecorder(d, tracesDir, sessionID)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	entry := types.TraceEntry{
		Timestamp:  time.Now().UTC(),
		ActionType: "file_write",
		Target:     "src/main.go",
		Agent:      "coder",
		Outcome:    "success",
	}
	if err := rec.Record(entry); err != nil {
		t.Fatalf("Record: %v", err)
	}
	rec.Close()

	data, err := os.ReadFile(filepath.Join(tracesDir, sessionID+".jsonl"))
	if err != nil {
		t.Fatalf("read trace file: %v", err)
	}

	var decoded types.TraceEntry
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal trace line: %v", err)
	}
	if decoded.ActionType != "file_write" {
		t.Errorf("ActionType = %q, want %q", decoded.ActionType, "file_write")
	}
	if decoded.Target != "src/main.go" {
		t.Errorf("Target = %q, want %q", decoded.Target, "src/main.go")
	}
	if decoded.Agent != "coder" {
		t.Errorf("Agent = %q, want %q", decoded.Agent, "coder")
	}
	if decoded.Outcome != "success" {
		t.Errorf("Outcome = %q, want %q", decoded.Outcome, "success")
	}
}

func TestRecord_UpdatesActionCount(t *testing.T) {
	d := openTestDB(t)
	tracesDir := filepath.Join(t.TempDir(), "traces")
	sessionID := "sess-004"

	rec, err := NewRecorder(d, tracesDir, sessionID)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	for i := 0; i < 3; i++ {
		entry := types.TraceEntry{
			Timestamp:  time.Now().UTC(),
			ActionType: "file_write",
			Target:     fmt.Sprintf("file%d.go", i),
			Agent:      "coder",
			Outcome:    "success",
		}
		if err := rec.Record(entry); err != nil {
			t.Fatalf("Record %d: %v", i, err)
		}
	}
	rec.Close()

	var actionCount int
	if err := d.QueryRow(`SELECT action_count FROM traces WHERE id=?`, sessionID).Scan(&actionCount); err != nil {
		t.Fatalf("query action_count: %v", err)
	}
	if actionCount != 3 {
		t.Errorf("action_count = %d, want 3", actionCount)
	}
}

func TestRecord_CountsHTTPEntries(t *testing.T) {
	d := openTestDB(t)
	tracesDir := filepath.Join(t.TempDir(), "traces")
	sessionID := "sess-005"

	rec, err := NewRecorder(d, tracesDir, sessionID)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	// One non-HTTP entry.
	if err := rec.Record(types.TraceEntry{
		Timestamp:  time.Now().UTC(),
		ActionType: "file_write",
		Target:     "a.go",
		Agent:      "coder",
		Outcome:    "success",
	}); err != nil {
		t.Fatalf("Record non-HTTP: %v", err)
	}

	// One HTTP entry (has Request).
	if err := rec.Record(types.TraceEntry{
		Timestamp:  time.Now().UTC(),
		ActionType: "http_call",
		Target:     "https://api.example.com/data",
		Agent:      "fetcher",
		Outcome:    "success",
		Request:    &types.HTTPCapture{Method: "GET", URL: "https://api.example.com/data"},
	}); err != nil {
		t.Fatalf("Record HTTP: %v", err)
	}
	rec.Close()

	var httpCount int
	if err := d.QueryRow(`SELECT http_count FROM traces WHERE id=?`, sessionID).Scan(&httpCount); err != nil {
		t.Fatalf("query http_count: %v", err)
	}
	if httpCount != 1 {
		t.Errorf("http_count = %d, want 1", httpCount)
	}
}

func TestClose_WritesDurationAndSize(t *testing.T) {
	d := openTestDB(t)
	tracesDir := filepath.Join(t.TempDir(), "traces")
	sessionID := "sess-006"

	rec, err := NewRecorder(d, tracesDir, sessionID)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	if err := rec.Record(types.TraceEntry{
		Timestamp:  time.Now().UTC(),
		ActionType: "file_write",
		Target:     "x.go",
		Agent:      "coder",
		Outcome:    "success",
	}); err != nil {
		t.Fatalf("Record: %v", err)
	}

	if err := rec.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	var durationMs sql.NullInt64
	var sizeBytes int64
	if err := d.QueryRow(`SELECT duration_ms, size_bytes FROM traces WHERE id=?`, sessionID).Scan(&durationMs, &sizeBytes); err != nil {
		t.Fatalf("query trace: %v", err)
	}
	if !durationMs.Valid {
		t.Error("expected duration_ms to be set after Close")
	}
	if sizeBytes <= 0 {
		t.Errorf("expected size_bytes > 0, got %d", sizeBytes)
	}
}

func TestPin_SetsPinnedFlag(t *testing.T) {
	d := openTestDB(t)
	tracesDir := filepath.Join(t.TempDir(), "traces")
	sessionID := "sess-007"

	rec, err := NewRecorder(d, tracesDir, sessionID)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}
	rec.Close()

	if err := Pin(d, tracesDir, sessionID); err != nil {
		t.Fatalf("Pin: %v", err)
	}

	var pinned int
	if err := d.QueryRow(`SELECT pinned FROM traces WHERE id=?`, sessionID).Scan(&pinned); err != nil {
		t.Fatalf("query pinned: %v", err)
	}
	if pinned != 1 {
		t.Errorf("pinned = %d, want 1", pinned)
	}
}

// =============================================================================
// Search tests
// =============================================================================

func TestSearch_MatchesActionDescription(t *testing.T) {
	tracesDir := t.TempDir()

	// Write a trace file with a file_write action.
	content := `{"timestamp":"2024-01-01T00:00:00Z","action_type":"file_write","target":"src/main.go","agent":"coder","outcome":"success"}` + "\n"
	if err := os.WriteFile(filepath.Join(tracesDir, "sess-search-1.jsonl"), []byte(content), 0o644); err != nil {
		t.Fatalf("write trace: %v", err)
	}

	results, err := Search(tracesDir, "file_write")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].SessionID != "sess-search-1" {
		t.Errorf("SessionID = %q, want %q", results[0].SessionID, "sess-search-1")
	}
}

func TestSearch_MatchesFilePath(t *testing.T) {
	tracesDir := t.TempDir()

	content := `{"timestamp":"2024-01-01T00:00:00Z","action_type":"file_write","target":"internal/handler.go","agent":"coder","outcome":"success"}` + "\n"
	if err := os.WriteFile(filepath.Join(tracesDir, "sess-search-2.jsonl"), []byte(content), 0o644); err != nil {
		t.Fatalf("write trace: %v", err)
	}

	results, err := Search(tracesDir, "handler.go")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestSearch_MatchesHTTPURL(t *testing.T) {
	tracesDir := t.TempDir()

	content := `{"timestamp":"2024-01-01T00:00:00Z","action_type":"http_call","target":"https://api.example.com/users","agent":"fetcher","outcome":"success","request":{"method":"GET","url":"https://api.example.com/users"}}` + "\n"
	if err := os.WriteFile(filepath.Join(tracesDir, "sess-search-3.jsonl"), []byte(content), 0o644); err != nil {
		t.Fatalf("write trace: %v", err)
	}

	results, err := Search(tracesDir, "api.example.com")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestSearch_CaseInsensitive(t *testing.T) {
	tracesDir := t.TempDir()

	content := `{"timestamp":"2024-01-01T00:00:00Z","action_type":"FILE_WRITE","target":"README.md","agent":"coder","outcome":"success"}` + "\n"
	if err := os.WriteFile(filepath.Join(tracesDir, "sess-search-4.jsonl"), []byte(content), 0o644); err != nil {
		t.Fatalf("write trace: %v", err)
	}

	results, err := Search(tracesDir, "file_write")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result (case-insensitive), got %d", len(results))
	}
}

func TestSearch_NoResultsForNonMatchingQuery(t *testing.T) {
	tracesDir := t.TempDir()

	content := `{"timestamp":"2024-01-01T00:00:00Z","action_type":"file_write","target":"main.go","agent":"coder","outcome":"success"}` + "\n"
	if err := os.WriteFile(filepath.Join(tracesDir, "sess-search-5.jsonl"), []byte(content), 0o644); err != nil {
		t.Fatalf("write trace: %v", err)
	}

	results, err := Search(tracesDir, "nonexistent_query_xyz")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestSearch_SkipsNonJSONLFiles(t *testing.T) {
	tracesDir := t.TempDir()

	// Write a .txt file that contains the query — should be ignored.
	if err := os.WriteFile(filepath.Join(tracesDir, "notes.txt"), []byte("file_write"), 0o644); err != nil {
		t.Fatalf("write txt: %v", err)
	}
	// Write a matching .jsonl file.
	content := `{"action_type":"file_write"}` + "\n"
	if err := os.WriteFile(filepath.Join(tracesDir, "sess-search-6.jsonl"), []byte(content), 0o644); err != nil {
		t.Fatalf("write jsonl: %v", err)
	}

	results, err := Search(tracesDir, "file_write")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result (only .jsonl), got %d", len(results))
	}
}

func TestSearch_MultipleMatchingFiles(t *testing.T) {
	tracesDir := t.TempDir()

	for i := 0; i < 3; i++ {
		content := fmt.Sprintf(`{"action_type":"deploy","target":"service-%d"}`, i) + "\n"
		name := fmt.Sprintf("sess-multi-%d.jsonl", i)
		if err := os.WriteFile(filepath.Join(tracesDir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write trace %d: %v", i, err)
		}
	}

	results, err := Search(tracesDir, "deploy")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
}

// =============================================================================
// Prune tests
// =============================================================================

// insertTraceRow is a helper that inserts a trace row with a specific created_at timestamp.
func insertTraceRow(t *testing.T, d *sql.DB, tracesDir, sessionID string, createdAt time.Time, sizeBytes int64, pinned bool) {
	t.Helper()

	// Ensure session row exists (FK constraint).
	_, err := d.Exec(
		`INSERT OR IGNORE INTO sessions (id, started_at, status) VALUES (?, ?, 'active')`,
		sessionID, db.TimeNow(),
	)
	if err != nil {
		t.Fatalf("insert session %s: %v", sessionID, err)
	}

	pinnedInt := 0
	if pinned {
		pinnedInt = 1
	}

	filePath := filepath.Join(tracesDir, sessionID+".jsonl")
	_, err = d.Exec(
		`INSERT INTO traces (id, session_id, file_path, action_count, http_count, size_bytes, pinned, created_at)
		 VALUES (?, ?, ?, 0, 0, ?, ?, ?)`,
		sessionID, sessionID, filePath, sizeBytes, pinnedInt, createdAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		t.Fatalf("insert trace %s: %v", sessionID, err)
	}

	// Create the actual trace file so pruning can remove it.
	if err := os.MkdirAll(tracesDir, 0o755); err != nil {
		t.Fatalf("mkdir traces: %v", err)
	}
	if err := os.WriteFile(filePath, []byte(`{"action_type":"test"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write trace file %s: %v", sessionID, err)
	}
}

func TestPruneByTTL_RemovesExpiredTraces(t *testing.T) {
	d := openTestDB(t)
	tracesDir := filepath.Join(t.TempDir(), "traces")

	// Insert a trace that is 20 days old (older than default 14-day TTL).
	oldTime := time.Now().UTC().AddDate(0, 0, -20)
	insertTraceRow(t, d, tracesDir, "old-sess", oldTime, 100, false)

	pruned, err := PruneByTTL(d, tracesDir, DefaultTTLDays)
	if err != nil {
		t.Fatalf("PruneByTTL: %v", err)
	}
	if pruned != 1 {
		t.Errorf("pruned = %d, want 1", pruned)
	}

	// Verify the trace row is gone.
	var count int
	d.QueryRow(`SELECT COUNT(*) FROM traces WHERE id='old-sess'`).Scan(&count)
	if count != 0 {
		t.Errorf("expected trace row to be deleted, got count=%d", count)
	}

	// Verify the trace file is gone.
	if _, err := os.Stat(filepath.Join(tracesDir, "old-sess.jsonl")); !os.IsNotExist(err) {
		t.Error("expected trace file to be deleted")
	}
}

func TestPruneByTTL_PreservesRecentTraces(t *testing.T) {
	d := openTestDB(t)
	tracesDir := filepath.Join(t.TempDir(), "traces")

	// Insert a trace that is 2 days old (within 14-day TTL).
	recentTime := time.Now().UTC().AddDate(0, 0, -2)
	insertTraceRow(t, d, tracesDir, "recent-sess", recentTime, 100, false)

	pruned, err := PruneByTTL(d, tracesDir, DefaultTTLDays)
	if err != nil {
		t.Fatalf("PruneByTTL: %v", err)
	}
	if pruned != 0 {
		t.Errorf("pruned = %d, want 0 (recent trace should be preserved)", pruned)
	}

	var count int
	d.QueryRow(`SELECT COUNT(*) FROM traces WHERE id='recent-sess'`).Scan(&count)
	if count != 1 {
		t.Errorf("expected trace row to still exist, got count=%d", count)
	}
}

func TestPruneByTTL_PinnedTracesProtected(t *testing.T) {
	d := openTestDB(t)
	tracesDir := filepath.Join(t.TempDir(), "traces")

	// Insert a pinned trace that is 30 days old.
	oldTime := time.Now().UTC().AddDate(0, 0, -30)
	insertTraceRow(t, d, tracesDir, "pinned-old", oldTime, 100, true)

	pruned, err := PruneByTTL(d, tracesDir, DefaultTTLDays)
	if err != nil {
		t.Fatalf("PruneByTTL: %v", err)
	}
	if pruned != 0 {
		t.Errorf("pruned = %d, want 0 (pinned trace should be protected)", pruned)
	}

	var count int
	d.QueryRow(`SELECT COUNT(*) FROM traces WHERE id='pinned-old'`).Scan(&count)
	if count != 1 {
		t.Errorf("expected pinned trace to still exist, got count=%d", count)
	}
}

func TestPruneBySize_RemovesOldestUnpinnedWhenOverLimit(t *testing.T) {
	d := openTestDB(t)
	tracesDir := filepath.Join(t.TempDir(), "traces")

	// Insert 3 traces with known sizes. Total = 3 MB.
	// Use 1 MB limit so pruning must remove some.
	base := time.Now().UTC().AddDate(0, 0, -10)
	insertTraceRow(t, d, tracesDir, "size-old", base, 1*1024*1024, false)
	insertTraceRow(t, d, tracesDir, "size-mid", base.Add(time.Hour), 1*1024*1024, false)
	insertTraceRow(t, d, tracesDir, "size-new", base.Add(2*time.Hour), 1*1024*1024, false)

	pruned, err := PruneBySize(d, tracesDir, 1) // 1 MB limit
	if err != nil {
		t.Fatalf("PruneBySize: %v", err)
	}
	if pruned < 1 {
		t.Errorf("expected at least 1 pruned, got %d", pruned)
	}

	// The oldest should be pruned first.
	var countOld int
	d.QueryRow(`SELECT COUNT(*) FROM traces WHERE id='size-old'`).Scan(&countOld)
	if countOld != 0 {
		t.Error("expected oldest trace 'size-old' to be pruned")
	}
}

func TestPruneBySize_NoActionWhenUnderLimit(t *testing.T) {
	d := openTestDB(t)
	tracesDir := filepath.Join(t.TempDir(), "traces")

	insertTraceRow(t, d, tracesDir, "small-sess", time.Now().UTC(), 100, false)

	pruned, err := PruneBySize(d, tracesDir, DefaultMaxMB) // 500 MB limit
	if err != nil {
		t.Fatalf("PruneBySize: %v", err)
	}
	if pruned != 0 {
		t.Errorf("pruned = %d, want 0 (under size limit)", pruned)
	}
}

func TestPruneBySize_PinnedTracesProtected(t *testing.T) {
	d := openTestDB(t)
	tracesDir := filepath.Join(t.TempDir(), "traces")

	base := time.Now().UTC().AddDate(0, 0, -10)
	// Insert a pinned trace (oldest) and an unpinned trace.
	insertTraceRow(t, d, tracesDir, "pinned-big", base, 2*1024*1024, true)
	insertTraceRow(t, d, tracesDir, "unpinned-big", base.Add(time.Hour), 2*1024*1024, false)

	// Total = 4 MB, limit = 1 MB. Only unpinned should be pruned.
	pruned, err := PruneBySize(d, tracesDir, 1)
	if err != nil {
		t.Fatalf("PruneBySize: %v", err)
	}

	// Pinned trace must survive.
	var pinnedCount int
	d.QueryRow(`SELECT COUNT(*) FROM traces WHERE id='pinned-big'`).Scan(&pinnedCount)
	if pinnedCount != 1 {
		t.Error("expected pinned trace to survive size pruning")
	}

	// Unpinned trace should be pruned.
	var unpinnedCount int
	d.QueryRow(`SELECT COUNT(*) FROM traces WHERE id='unpinned-big'`).Scan(&unpinnedCount)
	if unpinnedCount != 0 {
		t.Error("expected unpinned trace to be pruned")
	}

	if pruned != 1 {
		t.Errorf("pruned = %d, want 1", pruned)
	}
}

func TestPruneByTTL_MixedPinnedAndUnpinned(t *testing.T) {
	d := openTestDB(t)
	tracesDir := filepath.Join(t.TempDir(), "traces")

	oldTime := time.Now().UTC().AddDate(0, 0, -20)

	// One old unpinned (should be pruned), one old pinned (should survive).
	insertTraceRow(t, d, tracesDir, "old-unpinned", oldTime, 100, false)
	insertTraceRow(t, d, tracesDir, "old-pinned", oldTime.Add(time.Minute), 100, true)

	pruned, err := PruneByTTL(d, tracesDir, DefaultTTLDays)
	if err != nil {
		t.Fatalf("PruneByTTL: %v", err)
	}
	if pruned != 1 {
		t.Errorf("pruned = %d, want 1", pruned)
	}

	var unpinnedCount int
	d.QueryRow(`SELECT COUNT(*) FROM traces WHERE id='old-unpinned'`).Scan(&unpinnedCount)
	if unpinnedCount != 0 {
		t.Error("expected old unpinned trace to be pruned")
	}

	var pinnedCount int
	d.QueryRow(`SELECT COUNT(*) FROM traces WHERE id='old-pinned'`).Scan(&pinnedCount)
	if pinnedCount != 1 {
		t.Error("expected old pinned trace to survive")
	}
}
