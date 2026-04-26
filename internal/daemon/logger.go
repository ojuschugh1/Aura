package daemon

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

const (
	maxLogSize  = 10 * 1024 * 1024 // 10 MB
	maxLogFiles = 3
)

// rotatingWriter wraps a log file and rotates when it exceeds maxLogSize.
type rotatingWriter struct {
	mu   sync.Mutex
	path string
	file *os.File
	size int64
}

func newRotatingWriter(path string) (*rotatingWriter, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	info, _ := f.Stat()
	var sz int64
	if info != nil {
		sz = info.Size()
	}
	return &rotatingWriter{path: path, file: f, size: sz}, nil
}

func (w *rotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.size+int64(len(p)) > maxLogSize {
		w.rotate()
	}

	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
}

// rotate renames existing log files and opens a fresh one.
func (w *rotatingWriter) rotate() {
	w.file.Close()

	// Shift old files: .2 → drop, .1 → .2, .0 → .1, current → .0
	for i := maxLogFiles - 1; i > 0; i-- {
		older := fmt.Sprintf("%s.%d", w.path, i)
		newer := fmt.Sprintf("%s.%d", w.path, i-1)
		_ = os.Rename(newer, older)
	}
	_ = os.Rename(w.path, w.path+".0")

	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	w.file = f
	w.size = 0
}

func (w *rotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Close()
}

// InitLogger creates a slog.Logger writing JSON to .aura/aura.log with rotation.
// level must be one of "debug", "info", "warn", "error".
func InitLogger(dir, level string) (*slog.Logger, io.Closer, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, nil, fmt.Errorf("create log dir: %w", err)
	}

	logPath := filepath.Join(dir, "aura.log")
	rw, err := newRotatingWriter(logPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open log file: %w", err)
	}

	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(rw, &slog.HandlerOptions{Level: lvl})
	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger, rw, nil
}
