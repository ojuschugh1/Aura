package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLockFilePath(t *testing.T) {
	got := LockFilePath("/tmp/mydir")
	want := filepath.Join("/tmp/mydir", "aura.lock")
	if got != want {
		t.Errorf("LockFilePath = %q, want %q", got, want)
	}
}

func TestWriteAndReadLockFile(t *testing.T) {
	dir := t.TempDir()
	pid := 12345
	port := 7437

	if err := WriteLockFile(dir, pid, port); err != nil {
		t.Fatalf("WriteLockFile: %v", err)
	}

	// Verify the file exists on disk.
	lockPath := LockFilePath(dir)
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		t.Fatal("lock file was not created")
	}

	// Verify the raw JSON content has the correct fields.
	data, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("read lock file: %v", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("lock file is not valid JSON: %v", err)
	}

	// Read back through the API.
	info, err := ReadLockFile(dir)
	if err != nil {
		t.Fatalf("ReadLockFile: %v", err)
	}
	if info.PID != pid {
		t.Errorf("PID = %d, want %d", info.PID, pid)
	}
	if info.Port != port {
		t.Errorf("Port = %d, want %d", info.Port, port)
	}
}

func TestReadLockFile_NoFile(t *testing.T) {
	dir := t.TempDir()
	_, err := ReadLockFile(dir)
	if err == nil {
		t.Fatal("expected error when lock file does not exist")
	}
}

func TestReadLockFile_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	lockPath := LockFilePath(dir)
	if err := os.WriteFile(lockPath, []byte("not json"), 0644); err != nil {
		t.Fatalf("write invalid lock file: %v", err)
	}
	_, err := ReadLockFile(dir)
	if err == nil {
		t.Fatal("expected error for invalid JSON lock file")
	}
}

func TestRemoveLockFile(t *testing.T) {
	dir := t.TempDir()
	if err := WriteLockFile(dir, 1, 7437); err != nil {
		t.Fatalf("WriteLockFile: %v", err)
	}

	if err := RemoveLockFile(dir); err != nil {
		t.Fatalf("RemoveLockFile: %v", err)
	}

	if _, err := os.Stat(LockFilePath(dir)); !os.IsNotExist(err) {
		t.Error("lock file still exists after removal")
	}
}

func TestIsStale_CurrentProcess(t *testing.T) {
	// The current process PID should not be stale.
	info := &LockInfo{PID: os.Getpid(), Port: 7437}
	if IsStale(info) {
		t.Error("current process PID reported as stale")
	}
}

func TestIsStale_DeadPID(t *testing.T) {
	// Use a very high PID that is almost certainly not running.
	info := &LockInfo{PID: 999999999, Port: 7437}
	if !IsStale(info) {
		t.Error("expected dead PID to be reported as stale")
	}
}

func TestWriteLockFile_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()

	// Write first lock file.
	if err := WriteLockFile(dir, 100, 7000); err != nil {
		t.Fatalf("first WriteLockFile: %v", err)
	}

	// Overwrite with new values.
	if err := WriteLockFile(dir, 200, 8000); err != nil {
		t.Fatalf("second WriteLockFile: %v", err)
	}

	info, err := ReadLockFile(dir)
	if err != nil {
		t.Fatalf("ReadLockFile: %v", err)
	}
	if info.PID != 200 {
		t.Errorf("PID = %d, want 200", info.PID)
	}
	if info.Port != 8000 {
		t.Errorf("Port = %d, want 8000", info.Port)
	}
}
