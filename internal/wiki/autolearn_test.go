package wiki

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ojuschugh1/aura/internal/autocapture"
	auradb "github.com/ojuschugh1/aura/internal/db"
	"github.com/ojuschugh1/aura/internal/memory"
)

func TestAutoLearnerMemorySync(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	engine := NewEngine(store)
	memStore := memory.New(db)
	captureEngine := autocapture.NewCaptureEngine(memStore, autocapture.DefaultCaptureConfig())

	dir := t.TempDir()
	al := NewAutoLearner(engine, memStore, captureEngine, db, dir)

	// Add memory entries that should be promoted to wiki.
	_, _ = memStore.Add("architecture", "event sourcing with CQRS", "cli", "sess-1")
	_, _ = memStore.Add("database", "PostgreSQL 16 with pgvector", "cli", "sess-1")
	_, _ = memStore.Add("random-key", "not important enough to promote", "cli", "sess-1")

	// Run sync.
	synced := make(map[string]bool)
	al.syncMemoryToWiki(synced)

	// Architecture and database entries should be promoted.
	page, err := store.GetPage("memory-architecture")
	if err != nil {
		t.Fatalf("expected architecture page: %v", err)
	}
	if !strings.Contains(page.Content, "event sourcing") {
		t.Error("expected architecture content in wiki page")
	}

	page, err = store.GetPage("memory-database")
	if err != nil {
		t.Fatalf("expected database page: %v", err)
	}
	if !strings.Contains(page.Content, "PostgreSQL") {
		t.Error("expected database content in wiki page")
	}

	// Random key should NOT be promoted.
	_, err = store.GetPage("memory-random-key")
	if err == nil {
		t.Error("random-key should not be promoted to wiki")
	}

	// Synced map should track what was synced.
	if !synced["architecture"] || !synced["database"] {
		t.Error("synced map should track promoted entries")
	}
}

func TestAutoLearnerSessionSummary(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	engine := NewEngine(store)
	memStore := memory.New(db)
	captureEngine := autocapture.NewCaptureEngine(memStore, autocapture.DefaultCaptureConfig())

	dir := t.TempDir()
	al := NewAutoLearner(engine, memStore, captureEngine, db, dir)

	sessionID := "abcdef12-3456-7890-abcd-ef1234567890"

	// Add memory entries for this session.
	_, _ = memStore.Add("auth-decision", "JWT with refresh tokens", "claude", sessionID)
	_, _ = memStore.Add("db-choice", "PostgreSQL", "claude", sessionID)

	// Create session summary.
	al.createSessionSummary(sessionID)

	// Verify the summary page was created.
	page, err := store.GetPage("session-abcdef12")
	if err != nil {
		t.Fatalf("expected session summary page: %v", err)
	}
	if !strings.Contains(page.Content, "JWT") {
		t.Error("expected session content in summary")
	}
	if page.Category != "synthesis" {
		t.Errorf("category = %q, want %q", page.Category, "synthesis")
	}

	// Verify audit entry was created.
	history, _ := engine.Audit().History("session-abcdef12", 5)
	if len(history) == 0 {
		t.Error("expected audit entry for session summary")
	}
}

func TestAutoLearnerStartStop(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	engine := NewEngine(store)
	memStore := memory.New(db)
	captureEngine := autocapture.NewCaptureEngine(memStore, autocapture.DefaultCaptureConfig())

	dir := t.TempDir()
	al := NewAutoLearner(engine, memStore, captureEngine, db, dir)

	// Start with very short intervals for testing.
	cfg := AutoLearnConfig{
		MetabolismInterval: 100 * time.Millisecond,
		SyncInterval:       100 * time.Millisecond,
	}
	al.Start(cfg)

	// Add a memory entry that should get synced.
	_, _ = memStore.Add("architecture", "microservices", "cli", "test-sess")

	// Wait for at least one sync cycle.
	time.Sleep(300 * time.Millisecond)

	al.Stop()

	// Verify the memory entry was synced to wiki.
	page, err := store.GetPage("memory-architecture")
	if err != nil {
		t.Fatalf("expected synced page after auto-learn: %v", err)
	}
	if !strings.Contains(page.Content, "microservices") {
		t.Error("expected synced content")
	}
}

func TestAutoLearnerOnToolResult(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	engine := NewEngine(store)
	memStore := memory.New(db)
	captureEngine := autocapture.NewCaptureEngine(memStore, autocapture.DefaultCaptureConfig())

	dir := t.TempDir()
	al := NewAutoLearner(engine, memStore, captureEngine, db, dir)

	// Simulate a scan_deps tool result.
	scanResult := []byte(`{"phantoms": [{"import": "axios", "file": "src/api.js"}], "high_risk": []}`)
	al.OnToolResult("scan_deps", "sess-1", scanResult)

	// Should have created a tool page.
	pages, _ := store.ListPages("")
	found := false
	for _, p := range pages {
		if strings.Contains(p.Slug, "ghostdep") || p.Category == "tool" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected tool page after OnToolResult")
	}
}

func TestAutoLearnerDaemonIntegration(t *testing.T) {
	// This test verifies the auto-learner can be created with a real DB
	// and all subsystems wired together — the same way the daemon does it.
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	database, err := auradb.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()
	if err := auradb.RunMigrations(database); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	memStore := memory.New(database)
	captureEngine := autocapture.NewCaptureEngine(memStore, autocapture.DefaultCaptureConfig())
	wikiStore := NewStore(database)
	wikiEngine := NewEngine(wikiStore)

	al := NewAutoLearner(wikiEngine, memStore, captureEngine, database, dir)

	// Should not panic.
	al.Start(AutoLearnConfig{
		MetabolismInterval: time.Hour, // long interval so it doesn't fire during test
		SyncInterval:       time.Hour,
	})
	al.Stop()
}
