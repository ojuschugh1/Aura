package wiki

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher monitors a directory and auto-ingests new or changed files
// into the wiki. Only processes text-based files (.md, .txt, .jsonl).
// No LLM calls — all extraction is heuristic-based.
type Watcher struct {
	engine  *Engine
	dir     string
	watcher *fsnotify.Watcher
	stop    chan struct{}
	mu      sync.Mutex
	count   int
}

// supportedExts are the file extensions the watcher will auto-ingest.
var supportedExts = map[string]string{
	".md":       "markdown",
	".markdown": "markdown",
	".txt":      "text",
	".jsonl":    "jsonl",
}

// NewWatcher creates a Watcher that monitors dir for file changes.
func NewWatcher(engine *Engine, dir string) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create watcher: %w", err)
	}

	// Watch the directory and all immediate subdirectories.
	if err := fsw.Add(dir); err != nil {
		fsw.Close()
		return nil, fmt.Errorf("watch %s: %w", dir, err)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.IsDir() {
			subdir := filepath.Join(dir, e.Name())
			_ = fsw.Add(subdir) // best-effort on subdirs
		}
	}

	return &Watcher{
		engine:  engine,
		dir:     dir,
		watcher: fsw,
		stop:    make(chan struct{}),
	}, nil
}

// Start begins watching for file changes. It blocks until Stop is called.
// Each file change triggers an auto-ingest with a 500ms debounce.
func (w *Watcher) Start(onIngest func(path string, result *IngestResult)) error {
	slog.Info("wiki watch started", "dir", w.dir)

	// Debounce: collect events for 500ms before processing.
	pending := make(map[string]time.Time)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	defer w.watcher.Close()

	for {
		select {
		case <-w.stop:
			slog.Info("wiki watch stopped", "dir", w.dir, "ingested", w.count)
			return nil

		case event, ok := <-w.watcher.Events:
			if !ok {
				return nil
			}
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
				ext := strings.ToLower(filepath.Ext(event.Name))
				if _, supported := supportedExts[ext]; supported {
					pending[event.Name] = time.Now()
				}
			}

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return nil
			}
			slog.Warn("wiki watch error", "err", err)

		case <-ticker.C:
			// Process debounced events.
			now := time.Now()
			for path, ts := range pending {
				if now.Sub(ts) < 500*time.Millisecond {
					continue // still debouncing
				}
				delete(pending, path)
				w.processFile(path, onIngest)
			}
		}
	}
}

// Stop signals the watcher to shut down.
func (w *Watcher) Stop() {
	close(w.stop)
}

// Count returns the number of files auto-ingested so far.
func (w *Watcher) Count() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.count
}

func (w *Watcher) processFile(path string, onIngest func(string, *IngestResult)) {
	ext := strings.ToLower(filepath.Ext(path))
	format, ok := supportedExts[ext]
	if !ok {
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		slog.Warn("wiki watch: read failed", "path", path, "err", err)
		return
	}

	title := filepath.Base(path)
	result, err := w.engine.Ingest(title, string(data), format, path)
	if err != nil {
		slog.Warn("wiki watch: ingest failed", "path", path, "err", err)
		return
	}

	w.mu.Lock()
	w.count++
	w.mu.Unlock()

	if onIngest != nil {
		onIngest(path, result)
	}

	if result.Duplicate {
		slog.Debug("wiki watch: duplicate skipped", "path", path)
	} else {
		slog.Info("wiki watch: ingested",
			"path", path,
			"pages_created", len(result.PagesCreated),
			"pages_updated", len(result.PagesUpdated))
	}
}
