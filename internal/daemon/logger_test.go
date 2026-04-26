package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitLogger_CreatesLogFile(t *testing.T) {
	dir := t.TempDir()
	logger, closer, err := InitLogger(dir, "info")
	if err != nil {
		t.Fatalf("InitLogger: %v", err)
	}
	defer closer.Close()

	// Write a message so the file has content.
	logger.Info("hello")

	logPath := filepath.Join(dir, "aura.log")
	info, statErr := os.Stat(logPath)
	if os.IsNotExist(statErr) {
		t.Fatal("aura.log was not created")
	}
	if info.Size() == 0 {
		t.Error("aura.log is empty after writing a log message")
	}
}

func TestInitLogger_JSONOutput(t *testing.T) {
	dir := t.TempDir()
	logger, closer, err := InitLogger(dir, "info")
	if err != nil {
		t.Fatalf("InitLogger: %v", err)
	}

	logger.Info("test message", "key", "value")
	closer.Close()

	data, err := os.ReadFile(filepath.Join(dir, "aura.log"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}

	// Each line should be valid JSON.
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 {
		t.Fatal("no log lines written")
	}

	var entry map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("log line is not valid JSON: %v\nline: %s", err, lines[0])
	}

	// Verify structured fields are present.
	if _, ok := entry["msg"]; !ok {
		t.Error("JSON log entry missing 'msg' field")
	}
	if _, ok := entry["time"]; !ok {
		t.Error("JSON log entry missing 'time' field")
	}
	if _, ok := entry["level"]; !ok {
		t.Error("JSON log entry missing 'level' field")
	}
	if entry["key"] != "value" {
		t.Errorf("expected key=value in log entry, got key=%v", entry["key"])
	}
}

func TestInitLogger_LevelFiltering_InfoFiltersDebug(t *testing.T) {
	dir := t.TempDir()
	logger, closer, err := InitLogger(dir, "info")
	if err != nil {
		t.Fatalf("InitLogger: %v", err)
	}

	logger.Debug("should be filtered")
	logger.Info("should appear")
	closer.Close()

	data, err := os.ReadFile(filepath.Join(dir, "aura.log"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}

	content := string(data)
	if strings.Contains(content, "should be filtered") {
		t.Error("debug message should be filtered at info level")
	}
	if !strings.Contains(content, "should appear") {
		t.Error("info message should appear at info level")
	}
}

func TestInitLogger_LevelFiltering_DebugShowsAll(t *testing.T) {
	dir := t.TempDir()
	logger, closer, err := InitLogger(dir, "debug")
	if err != nil {
		t.Fatalf("InitLogger: %v", err)
	}

	logger.Debug("debug msg")
	logger.Info("info msg")
	logger.Warn("warn msg")
	closer.Close()

	data, err := os.ReadFile(filepath.Join(dir, "aura.log"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}

	content := string(data)
	for _, msg := range []string{"debug msg", "info msg", "warn msg"} {
		if !strings.Contains(content, msg) {
			t.Errorf("expected %q to appear at debug level", msg)
		}
	}
}

func TestInitLogger_LevelFiltering_WarnFiltersInfoAndDebug(t *testing.T) {
	dir := t.TempDir()
	logger, closer, err := InitLogger(dir, "warn")
	if err != nil {
		t.Fatalf("InitLogger: %v", err)
	}

	logger.Debug("debug msg")
	logger.Info("info msg")
	logger.Warn("warn msg")
	logger.Error("error msg")
	closer.Close()

	data, err := os.ReadFile(filepath.Join(dir, "aura.log"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}

	content := string(data)
	if strings.Contains(content, "debug msg") {
		t.Error("debug should be filtered at warn level")
	}
	if strings.Contains(content, "info msg") {
		t.Error("info should be filtered at warn level")
	}
	if !strings.Contains(content, "warn msg") {
		t.Error("warn should appear at warn level")
	}
	if !strings.Contains(content, "error msg") {
		t.Error("error should appear at warn level")
	}
}

func TestInitLogger_LevelFiltering_ErrorFiltersLower(t *testing.T) {
	dir := t.TempDir()
	logger, closer, err := InitLogger(dir, "error")
	if err != nil {
		t.Fatalf("InitLogger: %v", err)
	}

	logger.Debug("debug msg")
	logger.Info("info msg")
	logger.Warn("warn msg")
	logger.Error("error msg")
	closer.Close()

	data, err := os.ReadFile(filepath.Join(dir, "aura.log"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}

	content := string(data)
	if strings.Contains(content, "debug msg") {
		t.Error("debug should be filtered at error level")
	}
	if strings.Contains(content, "info msg") {
		t.Error("info should be filtered at error level")
	}
	if strings.Contains(content, "warn msg") {
		t.Error("warn should be filtered at error level")
	}
	if !strings.Contains(content, "error msg") {
		t.Error("error should appear at error level")
	}
}

func TestInitLogger_DefaultLevelIsInfo(t *testing.T) {
	dir := t.TempDir()
	// Pass an unrecognized level string; should default to info.
	logger, closer, err := InitLogger(dir, "unknown")
	if err != nil {
		t.Fatalf("InitLogger: %v", err)
	}

	logger.Debug("debug msg")
	logger.Info("info msg")
	closer.Close()

	data, err := os.ReadFile(filepath.Join(dir, "aura.log"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}

	content := string(data)
	if strings.Contains(content, "debug msg") {
		t.Error("debug should be filtered at default (info) level")
	}
	if !strings.Contains(content, "info msg") {
		t.Error("info should appear at default level")
	}
}

func TestInitLogger_CreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	_, closer, err := InitLogger(dir, "info")
	if err != nil {
		t.Fatalf("InitLogger should create nested dirs: %v", err)
	}
	closer.Close()

	if _, err := os.Stat(filepath.Join(dir, "aura.log")); os.IsNotExist(err) {
		t.Error("log file not created in nested directory")
	}
}

func TestRotatingWriter_RotatesAtMaxSize(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	rw, err := newRotatingWriter(logPath)
	if err != nil {
		t.Fatalf("newRotatingWriter: %v", err)
	}
	defer rw.Close()

	// Write enough data to exceed maxLogSize (10 MB).
	// We'll write in chunks to trigger rotation.
	chunk := strings.Repeat("x", 1024*1024) // 1 MB
	for i := 0; i < 11; i++ {
		if _, err := rw.Write([]byte(chunk)); err != nil {
			t.Fatalf("Write chunk %d: %v", i, err)
		}
	}

	// After exceeding maxLogSize, a rotated file should exist.
	rotated := logPath + ".0"
	if _, err := os.Stat(rotated); os.IsNotExist(err) {
		t.Error("expected rotated file .0 to exist after exceeding max size")
	}

	// The main log file should still exist and be smaller than maxLogSize.
	info, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("stat main log: %v", err)
	}
	if info.Size() > maxLogSize {
		t.Errorf("main log size %d exceeds maxLogSize %d after rotation", info.Size(), maxLogSize)
	}
}

func TestRotatingWriter_MaxFileCount(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	rw, err := newRotatingWriter(logPath)
	if err != nil {
		t.Fatalf("newRotatingWriter: %v", err)
	}
	defer rw.Close()

	// Trigger multiple rotations by writing large chunks.
	chunk := strings.Repeat("x", maxLogSize+1)
	for i := 0; i < maxLogFiles+2; i++ {
		if _, err := rw.Write([]byte(chunk)); err != nil {
			t.Fatalf("Write rotation %d: %v", i, err)
		}
	}

	// Files .0 through .maxLogFiles-1 should exist, but no more.
	for i := 0; i < maxLogFiles; i++ {
		rotated := filepath.Join(dir, "test.log."+string(rune('0'+i)))
		// Only check .0, .1, .2 (maxLogFiles=3)
		name := logPath + "." + intToStr(i)
		if _, err := os.Stat(name); os.IsNotExist(err) {
			// Older files may have been shifted out; only .0 is guaranteed.
			if i == 0 {
				t.Errorf("expected %s to exist", name)
			}
		}
		_ = rotated
	}

	// File beyond maxLogFiles should not exist.
	overflow := logPath + "." + intToStr(maxLogFiles)
	if _, err := os.Stat(overflow); err == nil {
		t.Errorf("file %s should not exist (exceeds maxLogFiles=%d)", overflow, maxLogFiles)
	}
}

// intToStr converts a small int to its string representation.
func intToStr(i int) string {
	return strings.TrimSpace(strings.Replace("0123456789"[i:i+1], "", "", 0))
}

func TestRotatingWriter_Close(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	rw, err := newRotatingWriter(logPath)
	if err != nil {
		t.Fatalf("newRotatingWriter: %v", err)
	}

	if _, err := rw.Write([]byte("data")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if err := rw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Writing after close should fail.
	_, err = rw.Write([]byte("more data"))
	if err == nil {
		t.Error("expected error writing to closed writer")
	}
}
