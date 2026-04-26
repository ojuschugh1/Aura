package codebase

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/ojuschugh1/aura/internal/db"
	"github.com/ojuschugh1/aura/internal/memory"
)

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

func TestStoreResult_StoresAllKeys(t *testing.T) {
	store := memory.New(openTestDB(t))

	result := &ScanResult{
		Languages:    []string{"Go", "JavaScript"},
		EntryPoints:  []string{"cmd/main.go"},
		Packages:     []string{"internal", "pkg"},
		Dependencies: []string{"cobra", "viper"},
		FileCount:    42,
		TotalLines:   1500,
	}

	n, err := StoreResult(store, result, "sess-1")
	if err != nil {
		t.Fatalf("StoreResult: %v", err)
	}
	if n != 5 {
		t.Errorf("expected 5 stored entries, got %d", n)
	}

	// Verify each key.
	keys := []string{
		"aura.project.languages",
		"aura.project.entry_points",
		"aura.project.packages",
		"aura.project.dependencies",
		"aura.project.stats",
	}
	for _, key := range keys {
		entry, err := store.Get(key)
		if err != nil {
			t.Errorf("Get(%q): %v", key, err)
			continue
		}
		if entry.SourceTool != "aura-awareness" {
			t.Errorf("key %q: SourceTool = %q, want %q", key, entry.SourceTool, "aura-awareness")
		}
		if entry.Confidence != 1.0 {
			t.Errorf("key %q: Confidence = %f, want 1.0", key, entry.Confidence)
		}
	}
}

func TestStoreResult_EmptyResult(t *testing.T) {
	store := memory.New(openTestDB(t))

	result := &ScanResult{}

	n, err := StoreResult(store, result, "")
	if err != nil {
		t.Fatalf("StoreResult: %v", err)
	}
	if n != 5 {
		t.Errorf("expected 5 stored entries even for empty result, got %d", n)
	}

	entry, err := store.Get("aura.project.stats")
	if err != nil {
		t.Fatalf("Get aura.project.stats: %v", err)
	}
	if entry.Value != "0 files, 0 lines" {
		t.Errorf("stats = %q, want %q", entry.Value, "0 files, 0 lines")
	}
}

func TestReconcileCodebase_UpdatesEntries(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), "package main\nfunc main() {}\n")

	store := memory.New(openTestDB(t))

	n, err := ReconcileCodebase(store, dir, "")
	if err != nil {
		t.Fatalf("ReconcileCodebase: %v", err)
	}
	if n != 5 {
		t.Errorf("expected 5 stored entries, got %d", n)
	}

	entry, err := store.Get("aura.project.languages")
	if err != nil {
		t.Fatalf("Get aura.project.languages: %v", err)
	}
	if entry.Value != "Go" {
		t.Errorf("languages = %q, want %q", entry.Value, "Go")
	}
}
