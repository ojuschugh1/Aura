package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// LockInfo holds the PID and port of a running Aura daemon instance.
type LockInfo struct {
	PID  int `json:"pid"`
	Port int `json:"port"`
}

// LockFilePath returns the full path to the lock file inside dir.
func LockFilePath(dir string) string {
	return filepath.Join(dir, "aura.lock")
}

// WriteLockFile writes the daemon PID and port as JSON to the lock file.
func WriteLockFile(dir string, pid int, port int) error {
	info := LockInfo{PID: pid, Port: port}
	data, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("marshal lock info: %w", err)
	}
	return os.WriteFile(LockFilePath(dir), data, 0644)
}

// ReadLockFile reads and parses the lock file from dir.
func ReadLockFile(dir string) (*LockInfo, error) {
	data, err := os.ReadFile(LockFilePath(dir))
	if err != nil {
		return nil, fmt.Errorf("read lock file: %w", err)
	}
	var info LockInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("parse lock file: %w", err)
	}
	return &info, nil
}

// RemoveLockFile removes the lock file from dir.
func RemoveLockFile(dir string) error {
	return os.Remove(LockFilePath(dir))
}

// IsStale returns true if the process identified by info.PID is no longer running.
func IsStale(info *LockInfo) bool {
	proc, err := os.FindProcess(info.PID)
	if err != nil {
		return true
	}
	// Signal 0 checks if the process exists without sending a real signal.
	err = proc.Signal(syscall.Signal(0))
	return err != nil
}
