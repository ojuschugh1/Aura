package proxy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SessionReplay records full agent sessions from the proxy log and
// generates diff reports showing what actually changed vs what was claimed.
type SessionReplay struct {
	proxy *Proxy
}

// NewSessionReplay creates a replay engine backed by the proxy's call log.
func NewSessionReplay(proxy *Proxy) *SessionReplay {
	return &SessionReplay{proxy: proxy}
}

// ReplayReport summarizes a session's activity with diffs.
type ReplayReport struct {
	SessionID     string           `json:"session_id"`
	Duration      string           `json:"duration"`
	TotalCalls    int              `json:"total_calls"`
	BlockedCalls  int              `json:"blocked_calls"`
	ToolBreakdown map[string]int   `json:"tool_breakdown"`
	FileChanges   []FileChange     `json:"file_changes,omitempty"`
	Claims        []ClaimDiff      `json:"claims,omitempty"`
	Timeline      []TimelineEntry  `json:"timeline"`
	GeneratedAt   time.Time        `json:"generated_at"`
}

// FileChange tracks a file that was modified during the session.
type FileChange struct {
	Path      string `json:"path"`
	Action    string `json:"action"` // "created", "modified", "deleted"
	ClaimedBy string `json:"claimed_by,omitempty"` // which tool call claimed this
	Verified  bool   `json:"verified"`
}

// ClaimDiff compares what the agent said it did vs what actually happened.
type ClaimDiff struct {
	Claim    string `json:"claim"`
	Actual   string `json:"actual"`
	Match    bool   `json:"match"`
}

// TimelineEntry is a single event in the session timeline.
type TimelineEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Tool      string    `json:"tool"`
	Summary   string    `json:"summary"`
	Blocked   bool      `json:"blocked,omitempty"`
	LatencyMs int64     `json:"latency_ms"`
}

// GenerateReport creates a replay report for a session by analyzing
// the proxy call log.
func (sr *SessionReplay) GenerateReport(sessionID string) *ReplayReport {
	calls := sr.proxy.GetLog(0) // get all calls

	report := &ReplayReport{
		SessionID:     sessionID,
		ToolBreakdown: make(map[string]int),
		GeneratedAt:   time.Now(),
	}

	var sessionCalls []CallRecord
	var earliest, latest time.Time

	for _, c := range calls {
		// Match by session ID in params, or include all if sessionID is empty.
		callSession := extractSessionID(c.Params)
		if sessionID != "" && callSession != sessionID && callSession != "" {
			continue
		}

		sessionCalls = append(sessionCalls, c)
		report.TotalCalls++
		report.ToolBreakdown[c.Tool]++

		if c.Blocked {
			report.BlockedCalls++
		}

		if earliest.IsZero() || c.Timestamp.Before(earliest) {
			earliest = c.Timestamp
		}
		if c.Timestamp.After(latest) {
			latest = c.Timestamp
		}

		// Build timeline.
		summary := fmt.Sprintf("%s", c.Tool)
		if c.Blocked {
			summary += " [BLOCKED: " + c.BlockReason + "]"
		}
		if c.Error != "" {
			summary += " [ERROR]"
		}

		report.Timeline = append(report.Timeline, TimelineEntry{
			Timestamp: c.Timestamp,
			Tool:      c.Tool,
			Summary:   summary,
			Blocked:   c.Blocked,
			LatencyMs: c.LatencyMs,
		})

		// Detect file-related claims.
		if isFileOperation(c.Tool) {
			path := extractFilePath(c.Params)
			if path != "" {
				change := FileChange{
					Path:      path,
					Action:    inferAction(c.Tool),
					ClaimedBy: c.Tool,
					Verified:  fileExists(path),
				}
				report.FileChanges = append(report.FileChanges, change)

				report.Claims = append(report.Claims, ClaimDiff{
					Claim:  fmt.Sprintf("%s %s", c.Tool, path),
					Actual: verifyFileState(path),
					Match:  change.Verified,
				})
			}
		}
	}

	if !earliest.IsZero() && !latest.IsZero() {
		report.Duration = latest.Sub(earliest).Round(time.Second).String()
	}

	return report
}

// ExportReport writes the report as a readable markdown file.
func (sr *SessionReplay) ExportReport(report *ReplayReport, outPath string) error {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# Session Replay: %s\n\n", report.SessionID))
	b.WriteString(fmt.Sprintf("**Duration:** %s | **Calls:** %d | **Blocked:** %d\n\n",
		report.Duration, report.TotalCalls, report.BlockedCalls))

	// Tool breakdown.
	b.WriteString("## Tool Usage\n\n")
	for tool, count := range report.ToolBreakdown {
		b.WriteString(fmt.Sprintf("- `%s`: %d calls\n", tool, count))
	}

	// File changes.
	if len(report.FileChanges) > 0 {
		b.WriteString("\n## File Changes\n\n")
		for _, fc := range report.FileChanges {
			status := "✓"
			if !fc.Verified {
				status = "✗"
			}
			b.WriteString(fmt.Sprintf("- [%s] %s `%s`\n", status, fc.Action, fc.Path))
		}
	}

	// Claims.
	if len(report.Claims) > 0 {
		b.WriteString("\n## Claim Verification\n\n")
		for _, c := range report.Claims {
			status := "PASS"
			if !c.Match {
				status = "FAIL"
			}
			b.WriteString(fmt.Sprintf("- [%s] %s → %s\n", status, c.Claim, c.Actual))
		}
	}

	// Timeline.
	b.WriteString("\n## Timeline\n\n")
	for _, t := range report.Timeline {
		blocked := ""
		if t.Blocked {
			blocked = " 🚫"
		}
		b.WriteString(fmt.Sprintf("- `%s` %s (%dms)%s\n",
			t.Timestamp.Format("15:04:05"), t.Summary, t.LatencyMs, blocked))
	}

	return os.WriteFile(outPath, []byte(b.String()), 0644)
}

func isFileOperation(tool string) bool {
	lower := strings.ToLower(tool)
	return strings.Contains(lower, "file") || strings.Contains(lower, "write") ||
		strings.Contains(lower, "create") || strings.Contains(lower, "delete")
}

func extractFilePath(params map[string]interface{}) string {
	for _, key := range []string{"path", "file", "target", "filename"} {
		if v, ok := params[key]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}

func inferAction(tool string) string {
	lower := strings.ToLower(tool)
	switch {
	case strings.Contains(lower, "create"):
		return "created"
	case strings.Contains(lower, "delete"):
		return "deleted"
	default:
		return "modified"
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func verifyFileState(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return "file not found"
	}
	return fmt.Sprintf("exists (%d bytes, modified %s)",
		info.Size(), info.ModTime().Format("15:04:05"))
}

// ExportDir creates a directory with the full replay report.
func (sr *SessionReplay) ExportDir(sessionID, outDir string) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}
	report := sr.GenerateReport(sessionID)
	return sr.ExportReport(report, filepath.Join(outDir, "replay-"+sessionID+".md"))
}
