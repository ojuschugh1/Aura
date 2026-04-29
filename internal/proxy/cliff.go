package proxy

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// CliffDetector monitors context window usage and warns when an agent
// is approaching the "context cliff" — the point where accumulated context
// exceeds effective reasoning capacity and the agent starts making
// confidently wrong decisions.
type CliffDetector struct {
	mu             sync.Mutex
	sessions       map[string]*SessionContext
	maxTokens      int     // model's context window size
	warningPct     float64 // warn at this percentage (default: 0.75)
	criticalPct    float64 // critical at this percentage (default: 0.90)
	onWarning      func(session string, usage float64, suggestion string)
}

// SessionContext tracks token usage for a single agent session.
type SessionContext struct {
	SessionID    string    `json:"session_id"`
	TokensUsed   int       `json:"tokens_used"`
	CallCount    int       `json:"call_count"`
	StartedAt    time.Time `json:"started_at"`
	LastCallAt   time.Time `json:"last_call_at"`
	WarningsSent int       `json:"warnings_sent"`
	Status       string    `json:"status"` // "ok", "warning", "critical"
}

// CliffConfig controls the cliff detector behavior.
type CliffConfig struct {
	MaxTokens   int     // context window size (default: 200000 for Claude)
	WarningPct  float64 // warn at this % (default: 0.75)
	CriticalPct float64 // critical at this % (default: 0.90)
}

// DefaultCliffConfig returns sensible defaults for Claude Sonnet.
func DefaultCliffConfig() CliffConfig {
	return CliffConfig{
		MaxTokens:   200000,
		WarningPct:  0.75,
		CriticalPct: 0.90,
	}
}

// NewCliffDetector creates a cliff detector.
func NewCliffDetector(cfg CliffConfig) *CliffDetector {
	return &CliffDetector{
		sessions:    make(map[string]*SessionContext),
		maxTokens:   cfg.MaxTokens,
		warningPct:  cfg.WarningPct,
		criticalPct: cfg.CriticalPct,
	}
}

// OnWarning registers a callback for cliff warnings.
func (d *CliffDetector) OnWarning(fn func(session string, usage float64, suggestion string)) {
	d.onWarning = fn
}

// Hook returns a ProxyHook that tracks token usage per session.
func (d *CliffDetector) Hook() ProxyHook {
	return func(ctx context.Context, call *CallRecord) error {
		sessionID := extractSessionID(call.Params)
		if sessionID == "" {
			sessionID = "default"
		}

		d.mu.Lock()
		sess, ok := d.sessions[sessionID]
		if !ok {
			sess = &SessionContext{
				SessionID: sessionID,
				StartedAt: time.Now(),
				Status:    "ok",
			}
			d.sessions[sessionID] = sess
		}

		// Estimate tokens from the call.
		tokensIn := call.TokensIn
		tokensOut := call.TokensOut
		if tokensIn == 0 {
			tokensIn = estimateSize(call.Params) / 4
		}
		sess.TokensUsed += tokensIn + tokensOut
		sess.CallCount++
		sess.LastCallAt = time.Now()

		usage := float64(sess.TokensUsed) / float64(d.maxTokens)
		d.mu.Unlock()

		// Check thresholds.
		if usage >= d.criticalPct && sess.Status != "critical" {
			sess.Status = "critical"
			sess.WarningsSent++
			suggestion := fmt.Sprintf(
				"CRITICAL: Session %s has used %.0f%% of context window (%d/%d tokens). "+
					"Agent reasoning quality is likely degraded. Recommend: "+
					"1) Save current context with 'aura memory add' "+
					"2) Run 'aura compact' to compress "+
					"3) Start a new session",
				sessionID, usage*100, sess.TokensUsed, d.maxTokens)
			if d.onWarning != nil {
				d.onWarning(sessionID, usage, suggestion)
			}
		} else if usage >= d.warningPct && sess.Status == "ok" {
			sess.Status = "warning"
			sess.WarningsSent++
			suggestion := fmt.Sprintf(
				"WARNING: Session %s has used %.0f%% of context window (%d/%d tokens). "+
					"Consider compressing context or saving important decisions to memory.",
				sessionID, usage*100, sess.TokensUsed, d.maxTokens)
			if d.onWarning != nil {
				d.onWarning(sessionID, usage, suggestion)
			}
		}

		return nil // never block
	}
}

// GetSession returns the context tracking for a session.
func (d *CliffDetector) GetSession(sessionID string) *SessionContext {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.sessions[sessionID]
}

// GetAllSessions returns all tracked sessions.
func (d *CliffDetector) GetAllSessions() []*SessionContext {
	d.mu.Lock()
	defer d.mu.Unlock()
	var result []*SessionContext
	for _, s := range d.sessions {
		result = append(result, s)
	}
	return result
}

// ResetSession clears token tracking for a session (call on session restart).
func (d *CliffDetector) ResetSession(sessionID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.sessions, sessionID)
}

func extractSessionID(params map[string]interface{}) string {
	if params == nil {
		return ""
	}
	if v, ok := params["session_id"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
