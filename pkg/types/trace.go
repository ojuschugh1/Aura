package types

import "time"

// TraceEntry records a single agent action within a session trace.
type TraceEntry struct {
	Timestamp  time.Time              `json:"timestamp"`
	ActionType string                 `json:"action_type"`
	Target     string                 `json:"target"`
	Agent      string                 `json:"agent"`
	Request    *HTTPCapture           `json:"request,omitempty"`
	Response   *HTTPCapture           `json:"response,omitempty"`
	Outcome    string                 `json:"outcome"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// HTTPCapture holds a captured HTTP request or response.
type HTTPCapture struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
	Status  int               `json:"status,omitempty"`
}
