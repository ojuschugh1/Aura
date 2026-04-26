package daemon

import (
	"os"
	"testing"
)

func TestIsDaemonProcess_Default(t *testing.T) {
	// Without the env var set, should return false.
	os.Unsetenv(EnvDaemon)
	if IsDaemonProcess() {
		t.Error("expected IsDaemonProcess() = false when env var is unset")
	}
}

func TestIsDaemonProcess_Set(t *testing.T) {
	t.Setenv(EnvDaemon, "1")
	if !IsDaemonProcess() {
		t.Error("expected IsDaemonProcess() = true when AURA_DAEMON=1")
	}
}

func TestIsDaemonProcess_WrongValue(t *testing.T) {
	t.Setenv(EnvDaemon, "yes")
	if IsDaemonProcess() {
		t.Error("expected IsDaemonProcess() = false when AURA_DAEMON=yes (not '1')")
	}
}

func TestStart_DuplicateInstanceDetection(t *testing.T) {
	dir := t.TempDir()

	// Write a lock file with the current process PID (not stale).
	if err := WriteLockFile(dir, os.Getpid(), 7437); err != nil {
		t.Fatalf("WriteLockFile: %v", err)
	}

	// Start should refuse because a non-stale lock file exists.
	err := Start(dir, 7437)
	if err == nil {
		t.Fatal("expected error for duplicate instance, got nil")
	}
}

func TestStart_AllowsStartAfterStaleLock(t *testing.T) {
	dir := t.TempDir()

	// Write a lock file with a dead PID.
	if err := WriteLockFile(dir, 999999999, 7437); err != nil {
		t.Fatalf("WriteLockFile: %v", err)
	}

	// Start should proceed past the stale lock check.
	// It will fail later (e.g., resolving executable), but the stale check passes.
	err := Start(dir, 7437)
	// We don't check for nil because the fork will likely fail in test,
	// but the error should NOT be about "daemon already running".
	if err != nil && err.Error() == "daemon already running (pid 999999999)" {
		t.Error("Start should not reject a stale lock file")
	}
}

func TestStop_NoLockFile(t *testing.T) {
	dir := t.TempDir()
	err := Stop(dir)
	if err == nil {
		t.Fatal("expected error when no lock file exists")
	}
}

func TestStop_StaleLockFile(t *testing.T) {
	dir := t.TempDir()

	// Write a stale lock file.
	if err := WriteLockFile(dir, 999999999, 7437); err != nil {
		t.Fatalf("WriteLockFile: %v", err)
	}

	err := Stop(dir)
	if err == nil {
		t.Fatal("expected error for stale lock file")
	}

	// The stale lock file should be cleaned up.
	if _, readErr := ReadLockFile(dir); readErr == nil {
		t.Error("stale lock file should have been removed")
	}
}

func TestStatus_NoDaemon(t *testing.T) {
	dir := t.TempDir()
	si, err := Status(dir)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if si.Running {
		t.Error("expected Running=false when no lock file exists")
	}
}

func TestStatus_StaleLockCleanup(t *testing.T) {
	dir := t.TempDir()

	// Write a stale lock file.
	if err := WriteLockFile(dir, 999999999, 7437); err != nil {
		t.Fatalf("WriteLockFile: %v", err)
	}

	si, err := Status(dir)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if si.Running {
		t.Error("expected Running=false for stale lock")
	}

	// Lock file should be cleaned up.
	if _, readErr := os.Stat(LockFilePath(dir)); !os.IsNotExist(readErr) {
		t.Error("stale lock file should have been removed by Status")
	}
}

func TestStatus_RunningDaemon(t *testing.T) {
	dir := t.TempDir()

	// Write a lock file with the current process PID (alive).
	pid := os.Getpid()
	port := 8080
	if err := WriteLockFile(dir, pid, port); err != nil {
		t.Fatalf("WriteLockFile: %v", err)
	}

	si, err := Status(dir)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !si.Running {
		t.Error("expected Running=true for live PID")
	}
	if si.PID != pid {
		t.Errorf("PID = %d, want %d", si.PID, pid)
	}
	if si.Port != port {
		t.Errorf("Port = %d, want %d", si.Port, port)
	}
}
