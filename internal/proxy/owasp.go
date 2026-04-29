package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// OWASPScorer maps agent actions to the OWASP Agentic Top 10 risks
// and generates a compliance score. This is detection and scoring,
// not enforcement — it tells you what risks your agent is exposed to.
type OWASPScorer struct {
	mu       sync.Mutex
	findings []OWASPFinding
}

// OWASPFinding is a single detected risk event.
type OWASPFinding struct {
	Timestamp time.Time `json:"timestamp"`
	Risk      string    `json:"risk"`       // OWASP risk ID (ASI01–ASI10)
	RiskName  string    `json:"risk_name"`  // human-readable name
	Severity  string    `json:"severity"`   // critical, high, medium, low
	Tool      string    `json:"tool"`       // the tool call that triggered it
	Detail    string    `json:"detail"`     // what specifically was detected
}

// OWASPReport is the compliance summary.
type OWASPReport struct {
	Score       int             `json:"score"`       // 0–10 (10 = fully compliant)
	TotalCalls  int             `json:"total_calls"`
	Findings    []OWASPFinding  `json:"findings"`
	RiskCounts  map[string]int  `json:"risk_counts"` // risk ID → count
	GeneratedAt time.Time       `json:"generated_at"`
}

// OWASP Agentic Top 10 risk IDs.
const (
	ASI01 = "ASI01" // Excessive Agency
	ASI02 = "ASI02" // Prompt Injection via Tools
	ASI03 = "ASI03" // Tool Misuse
	ASI04 = "ASI04" // Uncontrolled Cascading
	ASI05 = "ASI05" // Memory Poisoning
	ASI06 = "ASI06" // Identity & Access Abuse
	ASI07 = "ASI07" // Insufficient Logging
	ASI08 = "ASI08" // Rogue Agent Behavior
	ASI09 = "ASI09" // Insecure Communication
	ASI10 = "ASI10" // Resource Exhaustion
)

var riskNames = map[string]string{
	ASI01: "Excessive Agency",
	ASI02: "Prompt Injection via Tools",
	ASI03: "Tool Misuse",
	ASI04: "Uncontrolled Cascading",
	ASI05: "Memory Poisoning",
	ASI06: "Identity & Access Abuse",
	ASI07: "Insufficient Logging",
	ASI08: "Rogue Agent Behavior",
	ASI09: "Insecure Communication",
	ASI10: "Resource Exhaustion",
}

// NewOWASPScorer creates a scorer.
func NewOWASPScorer() *OWASPScorer {
	return &OWASPScorer{}
}

// Hook returns a ProxyHook that scores every call. It never blocks —
// it only records findings for the report.
func (s *OWASPScorer) Hook() ProxyHook {
	return func(ctx context.Context, call *CallRecord) error {
		s.analyze(call)
		return nil // never block, only observe
	}
}

// Report generates the current OWASP compliance report.
func (s *OWASPScorer) Report(totalCalls int) *OWASPReport {
	s.mu.Lock()
	defer s.mu.Unlock()

	report := &OWASPReport{
		TotalCalls:  totalCalls,
		Findings:    append([]OWASPFinding(nil), s.findings...),
		RiskCounts:  make(map[string]int),
		GeneratedAt: time.Now(),
	}

	for _, f := range s.findings {
		report.RiskCounts[f.Risk]++
	}

	// Score: start at 10, subtract for each risk category with findings.
	report.Score = 10
	for range report.RiskCounts {
		report.Score--
	}
	if report.Score < 0 {
		report.Score = 0
	}

	return report
}

func (s *OWASPScorer) analyze(call *CallRecord) {
	var findings []OWASPFinding

	// ASI01: Excessive Agency — agent calling destructive tools.
	if isDestructiveTool(call.Tool) {
		findings = append(findings, OWASPFinding{
			Timestamp: call.Timestamp,
			Risk:      ASI01,
			RiskName:  riskNames[ASI01],
			Severity:  "high",
			Tool:      call.Tool,
			Detail:    fmt.Sprintf("Agent called destructive tool %q without explicit approval", call.Tool),
		})
	}

	// ASI03: Tool Misuse — unusual parameter patterns.
	if hasShellInjection(call.Params) {
		findings = append(findings, OWASPFinding{
			Timestamp: call.Timestamp,
			Risk:      ASI03,
			RiskName:  riskNames[ASI03],
			Severity:  "critical",
			Tool:      call.Tool,
			Detail:    "Potential shell injection detected in tool parameters",
		})
	}

	// ASI05: Memory Poisoning — writing contradictory or suspicious memory.
	if call.Tool == "memory_write" {
		if hasSuspiciousContent(call.Params) {
			findings = append(findings, OWASPFinding{
				Timestamp: call.Timestamp,
				Risk:      ASI05,
				RiskName:  riskNames[ASI05],
				Severity:  "medium",
				Tool:      call.Tool,
				Detail:    "Memory write contains potentially poisoned content (instruction-like text)",
			})
		}
	}

	// ASI06: Identity Abuse — tool calls without session/agent identification.
	if call.Params != nil {
		if _, hasSession := call.Params["session_id"]; !hasSession {
			if _, hasAgent := call.Params["agent"]; !hasAgent {
				findings = append(findings, OWASPFinding{
					Timestamp: call.Timestamp,
					Risk:      ASI06,
					RiskName:  riskNames[ASI06],
					Severity:  "low",
					Tool:      call.Tool,
					Detail:    "Tool call missing session_id and agent identification",
				})
			}
		}
	}

	// ASI10: Resource Exhaustion — very large payloads.
	paramSize := estimateSize(call.Params)
	if paramSize > 100000 {
		findings = append(findings, OWASPFinding{
			Timestamp: call.Timestamp,
			Risk:      ASI10,
			RiskName:  riskNames[ASI10],
			Severity:  "medium",
			Tool:      call.Tool,
			Detail:    fmt.Sprintf("Unusually large payload (%d bytes) — potential resource exhaustion", paramSize),
		})
	}

	if len(findings) > 0 {
		s.mu.Lock()
		s.findings = append(s.findings, findings...)
		// Cap findings to prevent unbounded growth.
		if len(s.findings) > 5000 {
			s.findings = s.findings[len(s.findings)-5000:]
		}
		s.mu.Unlock()
	}
}

func isDestructiveTool(tool string) bool {
	destructive := []string{
		"file_delete", "git_push", "git_force_push",
		"shell_exec", "run_command", "execute",
		"database_drop", "database_delete",
	}
	lower := strings.ToLower(tool)
	for _, d := range destructive {
		if strings.Contains(lower, d) {
			return true
		}
	}
	return false
}

func hasShellInjection(params map[string]interface{}) bool {
	dangerous := []string{"; rm ", "| rm ", "&& rm ", "; curl ", "| curl ", "$(", "`"}
	for _, v := range params {
		s, ok := v.(string)
		if !ok {
			continue
		}
		lower := strings.ToLower(s)
		for _, d := range dangerous {
			if strings.Contains(lower, d) {
				return true
			}
		}
	}
	return false
}

func hasSuspiciousContent(params map[string]interface{}) bool {
	// Detect instruction-like text in memory values (potential poisoning).
	suspicious := []string{
		"ignore previous", "disregard", "you are now",
		"system prompt", "override", "jailbreak",
		"pretend you are", "act as if",
	}
	for _, v := range params {
		s, ok := v.(string)
		if !ok {
			continue
		}
		lower := strings.ToLower(s)
		for _, p := range suspicious {
			if strings.Contains(lower, p) {
				return true
			}
		}
	}
	return false
}

func estimateSize(params map[string]interface{}) int {
	b, _ := json.Marshal(params)
	return len(b)
}
