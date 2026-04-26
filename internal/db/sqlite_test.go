package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

// helper opens a temp SQLite database using the package Open function.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open(%s): %v", dbPath, err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// queryPragma is a small helper that returns a single string pragma value.
func queryPragma(t *testing.T, db *sql.DB, pragma string) string {
	t.Helper()
	var val string
	if err := db.QueryRow(pragma).Scan(&val); err != nil {
		t.Fatalf("query %s: %v", pragma, err)
	}
	return val
}

// --- Open / pragma tests ---------------------------------------------------

func TestOpen_WALModeEnabled(t *testing.T) {
	db := openTestDB(t)
	mode := queryPragma(t, db, "PRAGMA journal_mode")
	if mode != "wal" {
		t.Errorf("expected journal_mode=wal, got %q", mode)
	}
}

func TestOpen_ForeignKeysEnabled(t *testing.T) {
	db := openTestDB(t)
	val := queryPragma(t, db, "PRAGMA foreign_keys")
	if val != "1" {
		t.Errorf("expected foreign_keys=1, got %q", val)
	}
}

func TestOpen_BusyTimeoutSet(t *testing.T) {
	db := openTestDB(t)
	val := queryPragma(t, db, "PRAGMA busy_timeout")
	if val != "5000" {
		t.Errorf("expected busy_timeout=5000, got %q", val)
	}
}

func TestOpen_CreatesFileOnDisk(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "new.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("expected database file to exist on disk")
	}
}

func TestOpen_InvalidPath(t *testing.T) {
	// Attempt to open a path inside a non-existent directory.
	db, err := Open("/no/such/dir/test.db")
	if err == nil {
		db.Close()
		t.Fatal("expected error for invalid path, got nil")
	}
}
