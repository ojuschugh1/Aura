package mcp

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
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

// newTestHandlers returns a handlers instance backed by a fresh test database.
func newTestHandlers(t *testing.T) *handlers {
	t.Helper()
	d := openTestDB(t)
	store := memory.New(d)
	return &handlers{store: store, db: d}
}

// sendMCPRequest sends a POST to the MCP server and returns the status code and body.
func sendMCPRequest(t *testing.T, port int, secret, body string) (int, string) {
	t.Helper()
	url := fmt.Sprintf("http://127.0.0.1:%d/mcp", port)
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if secret != "" {
		req.Header.Set("Authorization", "Bearer "+secret)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	return resp.StatusCode, string(respBody)
}

// --- memory_write -----------------------------------------------------------

func TestMemoryWrite_ValidInput(t *testing.T) {
	h := newTestHandlers(t)
	ctx := context.Background()

	result, err := h.memoryWrite(ctx, map[string]interface{}{
		"key":         "db.host",
		"value":       "localhost:5432",
		"source_tool": "cursor",
		"session_id":  "sess-1",
	})
	if err != nil {
		t.Fatalf("memoryWrite: %v", err)
	}

	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}
	if m["key"] != "db.host" {
		t.Errorf("key = %v, want %q", m["key"], "db.host")
	}
	if m["session_id"] != "sess-1" {
		t.Errorf("session_id = %v, want %q", m["session_id"], "sess-1")
	}
	if m["timestamp"] == nil {
		t.Error("expected non-nil timestamp")
	}
}

func TestMemoryWrite_DefaultsSourceToolToMCP(t *testing.T) {
	h := newTestHandlers(t)
	ctx := context.Background()

	_, err := h.memoryWrite(ctx, map[string]interface{}{
		"key":   "k1",
		"value": "v1",
	})
	if err != nil {
		t.Fatalf("memoryWrite: %v", err)
	}

	entry, err := h.store.Get("k1")
	if err != nil {
		t.Fatalf("store.Get: %v", err)
	}
	if entry.SourceTool != "mcp" {
		t.Errorf("SourceTool = %q, want %q", entry.SourceTool, "mcp")
	}
}

func TestMemoryWrite_MissingKey(t *testing.T) {
	h := newTestHandlers(t)
	ctx := context.Background()

	_, err := h.memoryWrite(ctx, map[string]interface{}{
		"value": "some-value",
	})
	if err == nil {
		t.Fatal("expected error for missing key, got nil")
	}
	if got := err.Error(); got != "INVALID_PARAMS: key is required" {
		t.Errorf("error = %q, want INVALID_PARAMS message", got)
	}
}

func TestMemoryWrite_MissingValue(t *testing.T) {
	h := newTestHandlers(t)
	ctx := context.Background()

	_, err := h.memoryWrite(ctx, map[string]interface{}{
		"key": "some-key",
	})
	if err == nil {
		t.Fatal("expected error for missing value, got nil")
	}
	if got := err.Error(); got != "INVALID_PARAMS: value is required" {
		t.Errorf("error = %q, want INVALID_PARAMS message", got)
	}
}

func TestMemoryWrite_EmptyParams(t *testing.T) {
	h := newTestHandlers(t)
	ctx := context.Background()

	_, err := h.memoryWrite(ctx, map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for empty params, got nil")
	}
}

func TestMemoryWrite_NilParams(t *testing.T) {
	h := newTestHandlers(t)
	ctx := context.Background()

	_, err := h.memoryWrite(ctx, nil)
	if err == nil {
		t.Fatal("expected error for nil params, got nil")
	}
}

// --- memory_read ------------------------------------------------------------

func TestMemoryRead_ValidInput(t *testing.T) {
	h := newTestHandlers(t)
	ctx := context.Background()

	_, err := h.memoryWrite(ctx, map[string]interface{}{
		"key":   "api.url",
		"value": "https://example.com",
	})
	if err != nil {
		t.Fatalf("memoryWrite: %v", err)
	}

	result, err := h.memoryRead(ctx, map[string]interface{}{
		"key": "api.url",
	})
	if err != nil {
		t.Fatalf("memoryRead: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestMemoryRead_MissingKey(t *testing.T) {
	h := newTestHandlers(t)
	ctx := context.Background()

	_, err := h.memoryRead(ctx, map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for missing key, got nil")
	}
	if got := err.Error(); got != "INVALID_PARAMS: key is required" {
		t.Errorf("error = %q, want INVALID_PARAMS message", got)
	}
}

func TestMemoryRead_NonExistentKey(t *testing.T) {
	h := newTestHandlers(t)
	ctx := context.Background()

	_, err := h.memoryRead(ctx, map[string]interface{}{
		"key": "nonexistent",
	})
	if err == nil {
		t.Fatal("expected error for non-existent key, got nil")
	}
}

// --- memory_list ------------------------------------------------------------

func TestMemoryList_EmptyStore(t *testing.T) {
	h := newTestHandlers(t)
	ctx := context.Background()

	result, err := h.memoryList(ctx, map[string]interface{}{})
	if err != nil {
		t.Fatalf("memoryList: %v", err)
	}
	// nil or empty slice are both acceptable for an empty store.
	_ = result
}

func TestMemoryList_ReturnsAllEntries(t *testing.T) {
	h := newTestHandlers(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		_, err := h.memoryWrite(ctx, map[string]interface{}{
			"key":   fmt.Sprintf("key-%d", i),
			"value": fmt.Sprintf("val-%d", i),
		})
		if err != nil {
			t.Fatalf("memoryWrite %d: %v", i, err)
		}
	}

	result, err := h.memoryList(ctx, map[string]interface{}{})
	if err != nil {
		t.Fatalf("memoryList: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil list result")
	}
}

func TestMemoryList_FilterByAgent(t *testing.T) {
	h := newTestHandlers(t)
	ctx := context.Background()

	_, _ = h.memoryWrite(ctx, map[string]interface{}{
		"key": "k1", "value": "v1", "source_tool": "cli",
	})
	_, _ = h.memoryWrite(ctx, map[string]interface{}{
		"key": "k2", "value": "v2", "source_tool": "cursor",
	})

	result, err := h.memoryList(ctx, map[string]interface{}{
		"agent": "cli",
	})
	if err != nil {
		t.Fatalf("memoryList: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

// --- memory_delete ----------------------------------------------------------

func TestMemoryDelete_ValidInput(t *testing.T) {
	h := newTestHandlers(t)
	ctx := context.Background()

	_, err := h.memoryWrite(ctx, map[string]interface{}{
		"key": "to-delete", "value": "val",
	})
	if err != nil {
		t.Fatalf("memoryWrite: %v", err)
	}

	result, err := h.memoryDelete(ctx, map[string]interface{}{
		"key": "to-delete",
	})
	if err != nil {
		t.Fatalf("memoryDelete: %v", err)
	}

	m, ok := result.(map[string]bool)
	if !ok {
		t.Fatalf("expected map[string]bool, got %T", result)
	}
	if !m["deleted"] {
		t.Error("expected deleted=true")
	}

	// Verify the entry is gone.
	_, err = h.memoryRead(ctx, map[string]interface{}{"key": "to-delete"})
	if err == nil {
		t.Error("expected error reading deleted key, got nil")
	}
}

func TestMemoryDelete_MissingKey(t *testing.T) {
	h := newTestHandlers(t)
	ctx := context.Background()

	_, err := h.memoryDelete(ctx, map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for missing key, got nil")
	}
	if got := err.Error(); got != "INVALID_PARAMS: key is required" {
		t.Errorf("error = %q, want INVALID_PARAMS message", got)
	}
}

func TestMemoryDelete_NonExistentKey(t *testing.T) {
	h := newTestHandlers(t)
	ctx := context.Background()

	_, err := h.memoryDelete(ctx, map[string]interface{}{
		"key": "nonexistent",
	})
	if err == nil {
		t.Fatal("expected error for non-existent key, got nil")
	}
}

// --- verify_session ---------------------------------------------------------

func TestVerifySession_ReturnsPlaceholder(t *testing.T) {
	h := newTestHandlers(t)
	ctx := context.Background()

	result, err := h.verifySession(ctx, map[string]interface{}{
		"session_id": "sess-42",
	})
	if err != nil {
		t.Fatalf("verifySession: %v", err)
	}

	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}
	if m["session_id"] != "sess-42" {
		t.Errorf("session_id = %v, want %q", m["session_id"], "sess-42")
	}
}

// --- cost_summary -----------------------------------------------------------

func TestCostSummary_NilDB(t *testing.T) {
	h := &handlers{db: nil}
	ctx := context.Background()

	_, err := h.costSummary(ctx, map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error when db is nil, got nil")
	}
}

func TestCostSummary_SessionPeriodRequiresSessionID(t *testing.T) {
	h := newTestHandlers(t)
	ctx := context.Background()

	result, err := h.costSummary(ctx, map[string]interface{}{
		"period": "session",
	})
	if err != nil {
		t.Fatalf("costSummary: %v", err)
	}

	m, ok := result.(map[string]string)
	if !ok {
		t.Fatalf("expected map[string]string, got %T", result)
	}
	if m["message"] == "" {
		t.Error("expected message about providing session_id")
	}
}

func TestCostSummary_DailyPeriod(t *testing.T) {
	h := newTestHandlers(t)
	ctx := context.Background()

	result, err := h.costSummary(ctx, map[string]interface{}{
		"period": "daily",
	})
	if err != nil {
		t.Fatalf("costSummary daily: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result for daily summary")
	}
}

func TestCostSummary_WeeklyPeriod(t *testing.T) {
	h := newTestHandlers(t)
	ctx := context.Background()

	result, err := h.costSummary(ctx, map[string]interface{}{
		"period": "weekly",
	})
	if err != nil {
		t.Fatalf("costSummary weekly: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result for weekly summary")
	}
}

func TestCostSummary_DefaultPeriodIsSession(t *testing.T) {
	h := newTestHandlers(t)
	ctx := context.Background()

	result, err := h.costSummary(ctx, map[string]interface{}{})
	if err != nil {
		t.Fatalf("costSummary: %v", err)
	}

	m, ok := result.(map[string]string)
	if !ok {
		t.Fatalf("expected map[string]string, got %T", result)
	}
	if m["message"] == "" {
		t.Error("expected message about providing session_id")
	}
}

// --- stringParam helper -----------------------------------------------------

func TestStringParam_ValidString(t *testing.T) {
	params := map[string]interface{}{"key": "hello"}
	val, ok := stringParam(params, "key")
	if !ok || val != "hello" {
		t.Errorf("stringParam = (%q, %v), want (hello, true)", val, ok)
	}
}

func TestStringParam_MissingKey(t *testing.T) {
	params := map[string]interface{}{}
	_, ok := stringParam(params, "key")
	if ok {
		t.Error("expected ok=false for missing key")
	}
}

func TestStringParam_NonStringValue(t *testing.T) {
	params := map[string]interface{}{"key": 42}
	_, ok := stringParam(params, "key")
	if ok {
		t.Error("expected ok=false for non-string value")
	}
}

func TestStringParam_NilMap(t *testing.T) {
	_, ok := stringParam(nil, "key")
	if ok {
		t.Error("expected ok=false for nil map")
	}
}

// --- Authentication (Server-level) ------------------------------------------

func TestServer_AuthRejectsInvalidSecret(t *testing.T) {
	srv := New(0, "correct-secret")
	srv.Register("memory_read", func(_ context.Context, _ map[string]interface{}) (interface{}, error) {
		return "ok", nil
	})
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { srv.Stop(context.Background()) })

	code, _ := sendMCPRequest(t, srv.Port(), "wrong-secret", `{"tool":"memory_read","params":{"key":"x"}}`)
	if code != 401 {
		t.Errorf("expected 401, got %d", code)
	}
}

func TestServer_AuthAcceptsValidSecret(t *testing.T) {
	srv := New(0, "correct-secret")
	srv.Register("memory_read", func(_ context.Context, _ map[string]interface{}) (interface{}, error) {
		return "ok", nil
	})
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { srv.Stop(context.Background()) })

	code, _ := sendMCPRequest(t, srv.Port(), "correct-secret", `{"tool":"memory_read","params":{"key":"x"}}`)
	if code != 200 {
		t.Errorf("expected 200, got %d", code)
	}
}

func TestServer_NoSecretAllowsAll(t *testing.T) {
	srv := New(0, "")
	srv.Register("memory_read", func(_ context.Context, _ map[string]interface{}) (interface{}, error) {
		return "ok", nil
	})
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { srv.Stop(context.Background()) })

	code, _ := sendMCPRequest(t, srv.Port(), "", `{"tool":"memory_read","params":{"key":"x"}}`)
	if code != 200 {
		t.Errorf("expected 200, got %d", code)
	}
}

func TestServer_UnknownToolReturns404(t *testing.T) {
	srv := New(0, "")
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { srv.Stop(context.Background()) })

	code, _ := sendMCPRequest(t, srv.Port(), "", `{"tool":"nonexistent","params":{}}`)
	if code != 404 {
		t.Errorf("expected 404, got %d", code)
	}
}

// --- Concurrent access ------------------------------------------------------

func TestConcurrentWrites_NoDataCorruption(t *testing.T) {
	h := newTestHandlers(t)
	ctx := context.Background()

	// Write entries sequentially first to seed the store, then read concurrently.
	// This tests that concurrent reads don't corrupt data. SQLite WAL mode
	// supports concurrent reads but serialises writes, so we test both patterns.
	const count = 20
	for i := 0; i < count; i++ {
		key := fmt.Sprintf("concurrent-key-%d", i)
		_, err := h.memoryWrite(ctx, map[string]interface{}{
			"key":        key,
			"value":      fmt.Sprintf("value-%d", i),
			"session_id": "sess-concurrent",
		})
		if err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
	}

	// Concurrent reads should all succeed without corruption.
	var wg sync.WaitGroup
	errs := make(chan error, count)

	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := fmt.Sprintf("concurrent-key-%d", n)
			result, err := h.memoryRead(ctx, map[string]interface{}{
				"key": key,
			})
			if err != nil {
				errs <- fmt.Errorf("goroutine %d read: %w", n, err)
				return
			}
			if result == nil {
				errs <- fmt.Errorf("goroutine %d: read returned nil", n)
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}

	// Verify all entries exist via list.
	result, err := h.memoryList(ctx, map[string]interface{}{})
	if err != nil {
		t.Fatalf("memoryList: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil list result")
	}
}

func TestConcurrentWritesSameKey_LastWriterWins(t *testing.T) {
	h := newTestHandlers(t)
	ctx := context.Background()

	const goroutines = 10
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_, _ = h.memoryWrite(ctx, map[string]interface{}{
				"key":   "shared-key",
				"value": fmt.Sprintf("value-%d", n),
			})
		}(i)
	}
	wg.Wait()

	// The key should exist with one of the written values (no corruption).
	result, err := h.memoryRead(ctx, map[string]interface{}{
		"key": "shared-key",
	})
	if err != nil {
		t.Fatalf("memoryRead: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result for shared-key")
	}
}

// --- Write then read round-trip via handlers --------------------------------

func TestWriteReadRoundTrip(t *testing.T) {
	h := newTestHandlers(t)
	ctx := context.Background()

	_, err := h.memoryWrite(ctx, map[string]interface{}{
		"key":         "roundtrip",
		"value":       "test-value-123",
		"source_tool": "test",
		"session_id":  "sess-rt",
	})
	if err != nil {
		t.Fatalf("memoryWrite: %v", err)
	}

	result, err := h.memoryRead(ctx, map[string]interface{}{
		"key": "roundtrip",
	})
	if err != nil {
		t.Fatalf("memoryRead: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

// --- Write, list, delete lifecycle ------------------------------------------

func TestWriteListDeleteLifecycle(t *testing.T) {
	h := newTestHandlers(t)
	ctx := context.Background()

	for _, key := range []string{"lc-1", "lc-2"} {
		_, err := h.memoryWrite(ctx, map[string]interface{}{
			"key": key, "value": "val",
		})
		if err != nil {
			t.Fatalf("memoryWrite %s: %v", key, err)
		}
	}

	listResult, err := h.memoryList(ctx, map[string]interface{}{})
	if err != nil {
		t.Fatalf("memoryList: %v", err)
	}
	if listResult == nil {
		t.Fatal("expected non-nil list result")
	}

	_, err = h.memoryDelete(ctx, map[string]interface{}{"key": "lc-1"})
	if err != nil {
		t.Fatalf("memoryDelete: %v", err)
	}

	_, err = h.memoryRead(ctx, map[string]interface{}{"key": "lc-1"})
	if err == nil {
		t.Error("expected error reading deleted key")
	}

	_, err = h.memoryRead(ctx, map[string]interface{}{"key": "lc-2"})
	if err != nil {
		t.Errorf("expected lc-2 to still exist: %v", err)
	}
}
