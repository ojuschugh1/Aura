package wiki

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ojuschugh1/aura/internal/autocapture"
	"github.com/ojuschugh1/aura/internal/memory"
)

// AutoLearner runs inside the Aura daemon and automatically learns from
// everything happening on the machine — session transcripts, memory writes,
// tool output. No IDE hooks, no manual steps. Just install Aura and it learns.
type AutoLearner struct {
	engine       *Engine
	memStore     *memory.Store
	capture      *autocapture.CaptureEngine
	db           *sql.DB
	dir          string
	tracesDir    string
	stop         chan struct{}
	metabolismCfg MetabolismConfig
}

// AutoLearnConfig controls the auto-learning behavior.
type AutoLearnConfig struct {
	MetabolismInterval time.Duration // how often to run metabolism (default: 6h)
	SyncInterval       time.Duration // how often to sync memory → wiki (default: 5m)
}

// DefaultAutoLearnConfig returns sensible defaults.
func DefaultAutoLearnConfig() AutoLearnConfig {
	return AutoLearnConfig{
		MetabolismInterval: 6 * time.Hour,
		SyncInterval:       5 * time.Minute,
	}
}

// NewAutoLearner creates an AutoLearner wired into the daemon's subsystems.
func NewAutoLearner(engine *Engine, memStore *memory.Store, capture *autocapture.CaptureEngine, db *sql.DB, dir string) *AutoLearner {
	return &AutoLearner{
		engine:        engine,
		memStore:      memStore,
		capture:       capture,
		db:            db,
		dir:           dir,
		tracesDir:     filepath.Join(dir, "traces"),
		stop:          make(chan struct{}),
		metabolismCfg: DefaultMetabolismConfig(),
	}
}

// Start begins the background auto-learning loops. Call Stop() to shut down.
func (al *AutoLearner) Start(cfg AutoLearnConfig) {
	slog.Info("auto-learner started",
		"metabolism_interval", cfg.MetabolismInterval,
		"sync_interval", cfg.SyncInterval)

	go al.metabolismLoop(cfg.MetabolismInterval)
	go al.memorySyncLoop(cfg.SyncInterval)
}

// Stop shuts down all background loops.
func (al *AutoLearner) Stop() {
	close(al.stop)
	slog.Info("auto-learner stopped")
}

// OnSessionEnd is called by the session manager when a session completes.
// It ingests the session's transcript and decisions into the wiki.
func (al *AutoLearner) OnSessionEnd(sessionID string) {
	slog.Info("auto-learn: session ended, ingesting", "session_id", sessionID)

	// 1. Ingest the session transcript as a wiki source.
	transcriptPath := filepath.Join(al.tracesDir, sessionID+".jsonl")
	if data, err := os.ReadFile(transcriptPath); err == nil && len(data) > 0 {
		title := fmt.Sprintf("Session transcript — %s", sessionID[:8])
		_, err := al.engine.Ingest(title, string(data), "jsonl", transcriptPath)
		if err != nil {
			slog.Warn("auto-learn: transcript ingest failed", "err", err)
		} else {
			slog.Info("auto-learn: transcript ingested", "session_id", sessionID)
		}
	}

	// 2. Create a session summary page from memory entries written during this session.
	al.createSessionSummary(sessionID)

	// 3. Record in audit chain.
	_ = al.engine.Audit().Record("session-"+sessionID[:8], "session-end", "auto-learner",
		fmt.Sprintf("Session %s ended, auto-ingested", sessionID[:8]))
}

// OnToolResult is called after any MCP tool produces output.
// It automatically feeds the result into the wiki via the appropriate adapter.
func (al *AutoLearner) OnToolResult(toolName, sessionID string, resultJSON []byte) {
	if al.engine == nil || len(resultJSON) == 0 {
		return
	}

	// Only auto-feed tools that produce meaningful knowledge.
	switch toolName {
	case "verify_session":
		// Auto-feed verification results.
		_, err := al.engine.IngestToolJSON("claimcheck", resultJSON)
		if err != nil {
			slog.Debug("auto-learn: verify feed failed", "err", err)
		}
	case "scan_deps":
		_, err := al.engine.IngestToolJSON("ghostdep", resultJSON)
		if err != nil {
			slog.Debug("auto-learn: scan feed failed", "err", err)
		}
	case "compact_context":
		_, err := al.engine.IngestToolJSON("sqz", resultJSON)
		if err != nil {
			slog.Debug("auto-learn: compact feed failed", "err", err)
		}
	case "cost_summary":
		_, err := al.engine.IngestToolJSON("cost", resultJSON)
		if err != nil {
			slog.Debug("auto-learn: cost feed failed", "err", err)
		}
	default:
		// Skip tools that don't produce wiki-worthy output.
	}
}

// metabolismLoop runs the knowledge lifecycle on a timer.
func (al *AutoLearner) metabolismLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-al.stop:
			return
		case <-ticker.C:
			result, err := al.engine.Metabolize(al.metabolismCfg)
			if err != nil {
				slog.Warn("auto-learn: metabolism failed", "err", err)
				continue
			}
			slog.Info("auto-learn: metabolism complete",
				"decayed", result.PagesDecayed,
				"consolidated", result.PagesConsolidated,
				"archived", result.PagesArchived,
				"pressure_alerts", len(result.PressureAlerts))
		}
	}
}

// memorySyncLoop periodically syncs important memory entries into the wiki.
// Memory entries tagged with specific patterns (decisions, architecture, etc.)
// get promoted to wiki pages so they're searchable and cross-referenced.
func (al *AutoLearner) memorySyncLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	synced := make(map[string]bool) // track what we've already synced

	for {
		select {
		case <-al.stop:
			return
		case <-ticker.C:
			al.syncMemoryToWiki(synced)
		}
	}
}

// syncMemoryToWiki promotes memory entries to wiki pages.
func (al *AutoLearner) syncMemoryToWiki(synced map[string]bool) {
	entries, err := al.memStore.List(memory.ListFilter{})
	if err != nil {
		return
	}

	promotionKeywords := []string{
		"architecture", "decision", "stack", "design", "auth",
		"database", "deploy", "api", "config", "migration",
	}

	for _, entry := range entries {
		if synced[entry.Key] {
			continue
		}

		// Check if this entry is worth promoting to the wiki.
		keyLower := strings.ToLower(entry.Key)
		valueLower := strings.ToLower(entry.Value)
		shouldPromote := false

		for _, kw := range promotionKeywords {
			if strings.Contains(keyLower, kw) || strings.Contains(valueLower, kw) {
				shouldPromote = true
				break
			}
		}

		// Also promote entries from auto-capture (they're decisions).
		if entry.SourceTool == "auto-capture" {
			shouldPromote = true
		}

		if !shouldPromote {
			continue
		}

		slug := slugify("memory-" + entry.Key)
		existing, _ := al.engine.Store().GetPage(slug)
		if existing != nil {
			// Update if the value changed.
			if !strings.Contains(existing.Content, entry.Value) {
				newContent := existing.Content + fmt.Sprintf("\n\n### Updated %s\n\n%s",
					entry.UpdatedAt.Format("2006-01-02 15:04"), entry.Value)
				_, _ = al.engine.Store().UpdatePage(slug, newContent, existing.Tags, existing.SourceIDs, existing.LinksSlugs)
			}
		} else {
			content := fmt.Sprintf("# %s\n\n%s\n\n*Source: %s, synced from memory*",
				entry.Key, entry.Value, entry.SourceTool)
			_, _ = al.engine.Store().CreatePage(slug, entry.Key, content, "entity",
				[]string{"auto-synced", "memory"}, nil, nil)
		}

		synced[entry.Key] = true
	}
}

// createSessionSummary builds a wiki page summarizing what happened in a session.
func (al *AutoLearner) createSessionSummary(sessionID string) {
	// Get memory entries from this session.
	entries, err := al.memStore.List(memory.ListFilter{})
	if err != nil {
		return
	}

	var sessionEntries []string
	for _, e := range entries {
		if e.SessionID == sessionID {
			sessionEntries = append(sessionEntries, fmt.Sprintf("- **%s:** %s", e.Key, e.Value))
		}
	}

	if len(sessionEntries) == 0 {
		return // nothing to summarize
	}

	slug := slugify("session-" + sessionID[:8])
	content := fmt.Sprintf("# Session Summary: %s\n\n*Ended: %s*\n\n## Context Stored\n\n%s",
		sessionID[:8],
		time.Now().Format("2006-01-02 15:04"),
		strings.Join(sessionEntries, "\n"))

	_, err = al.engine.Store().CreatePage(slug, "Session: "+sessionID[:8], content, "synthesis",
		[]string{"session", "auto-summary"}, nil, nil)
	if err != nil {
		slog.Debug("auto-learn: session summary failed", "err", err)
	} else {
		_ = al.engine.Audit().Record(slug, "create", "auto-learner", "Auto-generated session summary")
		slog.Info("auto-learn: session summary created", "session_id", sessionID[:8])
	}
}
