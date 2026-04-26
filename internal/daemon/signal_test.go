package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecoverAndLog_NoPanic(t *testing.T) {
	dir := t.TempDir()
	err := RecoverAndLog(dir, func() error {
		return nil
	})
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestRecoverAndLog_FunctionError(t *testing.T) {
	dir := t.TempDir()
	want := fmt.Errorf("something broke")
	err := RecoverAndLog(dir, func() error {
		return want
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != want {
		t.Errorf("error = %v, want %v", err, want)
	}
}

func TestRecoverAndLog_Panic(t *testing.T) {
	dir := t.TempDir()
	err := RecoverAndLog(dir, func() error {
		panic("test panic")
	})
	if err == nil {
		t.Fatal("expected error from panic recovery, got nil")
	}
	if !strings.Contains(err.Error(), "test panic") {
		t.Errorf("error should contain panic message, got %q", err.Error())
	}

	// Verify crash log was written.
	crashPath := filepath.Join(dir, "crash.log")
	data, readErr := os.ReadFile(crashPath)
	if readErr != nil {
		t.Fatalf("crash.log not written: %v", readErr)
	}
	if !strings.Contains(string(data), "test panic") {
		t.Errorf("crash.log should contain panic message, got %q", string(data))
	}
}

func TestWriteCrashLog(t *testing.T) {
	dir := t.TempDir()
	msg := "unexpected failure"
	writeCrashLog(dir, msg)

	data, err := os.ReadFile(filepath.Join(dir, "crash.log"))
	if err != nil {
		t.Fatalf("read crash.log: %v", err)
	}
	if string(data) != msg {
		t.Errorf("crash.log = %q, want %q", string(data), msg)
	}
}
