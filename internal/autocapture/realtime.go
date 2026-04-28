package autocapture

import (
	"log/slog"
	"strings"
	"sync"
	"time"
)

// RealtimeCapture watches MCP tool calls and auto-captures decisions from
// text content in params. This is what makes Aura truly zero-effort —
// users never need to type 'aura memory add'. The daemon captures
// everything automatically from the AI's own words.
type RealtimeCapture struct {
	engine    *CaptureEngine
	sessionID string
	mu        sync.Mutex
	// Dedup window: skip re-capturing the same text within this window.
	seenTexts map[string]time.Time
	// Minimum confidence to auto-capture (higher than batch mode since
	// we're firing on every call, we want fewer false positives).
	minConfidence float64
}

// NewRealtimeCapture creates a real-time capture middleware.
func NewRealtimeCapture(engine *CaptureEngine, sessionID string) *RealtimeCapture {
	return &RealtimeCapture{
		engine:        engine,
		sessionID:     sessionID,
		seenTexts:     make(map[string]time.Time),
		minConfidence: 0.7,
	}
}

// Middleware returns a function suitable for registration with mcp.Server.Use().
// It extracts text content from any tool call params and runs decision
// extraction on it in the background.
func (r *RealtimeCapture) Middleware() func(tool string, params map[string]interface{}, result interface{}) {
	return func(tool string, params map[string]interface{}, result interface{}) {
		// Skip tools that wouldn't contain decision text.
		if !isCaptureCandidate(tool) {
			return
		}

		// Extract all text-like fields from params and result.
		var texts []string
		texts = append(texts, extractTextFields(params)...)
		if result != nil {
			if m, ok := result.(map[string]interface{}); ok {
				texts = append(texts, extractTextFields(m)...)
			}
		}

		for _, text := range texts {
			r.captureIfNew(text)
		}

		// Periodic cleanup of old dedup entries.
		r.cleanupDedup()
	}
}

// isCaptureCandidate returns true for tools whose params likely contain
// decision text worth scanning. We skip read-only tools and metadata queries.
func isCaptureCandidate(tool string) bool {
	switch tool {
	// Tools that likely contain user/AI text:
	case "memory_write", "wiki_ingest", "wiki_write", "wiki_save_query",
		"compact_context", "wiki_query":
		return true
	// Explicitly skip read-only lookups:
	case "memory_read", "memory_list", "memory_delete",
		"wiki_read", "wiki_search", "wiki_index", "wiki_log",
		"wiki_lint", "wiki_graph", "wiki_filter",
		"cost_summary", "verify_session", "scan_deps",
		"check_action", "escrow_decide", "route_task":
		return false
	}
	return false
}

// extractTextFields pulls string values from a map that are long enough
// to plausibly contain a decision.
func extractTextFields(m map[string]interface{}) []string {
	var texts []string
	for key, val := range m {
		// Focus on common text field names.
		if !isTextField(key) {
			continue
		}
		s, ok := val.(string)
		if !ok || len(s) < 20 {
			continue
		}
		texts = append(texts, s)
	}
	return texts
}

// isTextField returns true for param names that typically hold decision text.
func isTextField(key string) bool {
	key = strings.ToLower(key)
	switch key {
	case "content", "value", "text", "query", "message",
		"title", "description", "body", "summary", "answer":
		return true
	}
	return false
}

// captureIfNew runs decision extraction on text, but only captures matches
// that haven't been seen in the dedup window.
func (r *RealtimeCapture) captureIfNew(text string) {
	if len(text) < 20 {
		return
	}

	matches := MatchDecisions(text)
	if len(matches) == 0 {
		return
	}

	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, m := range matches {
		if m.Confidence < r.minConfidence {
			continue
		}

		// Skip if we've captured this exact key recently (within 5 min).
		dedupKey := m.Key + "|" + m.Value
		if lastSeen, ok := r.seenTexts[dedupKey]; ok && now.Sub(lastSeen) < 5*time.Minute {
			continue
		}
		r.seenTexts[dedupKey] = now

		_, err := r.engine.store.AddWithMeta(m.Key, m.Value, "auto-capture", r.sessionID, m.Confidence, nil)
		if err != nil {
			slog.Debug("realtime capture store failed", "err", err)
			continue
		}
		slog.Info("auto-captured", "key", m.Key, "confidence", m.Confidence, "pattern", m.Pattern)
	}
}

// cleanupDedup removes entries older than 10 minutes from the dedup map.
func (r *RealtimeCapture) cleanupDedup() {
	// Only cleanup occasionally to avoid lock contention.
	if time.Now().Second()%30 != 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	cutoff := time.Now().Add(-10 * time.Minute)
	for k, t := range r.seenTexts {
		if t.Before(cutoff) {
			delete(r.seenTexts, k)
		}
	}
}
