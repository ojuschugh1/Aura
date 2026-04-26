package types

import "time"

// MemoryEntry represents a single unit of stored context in the memory store.
type MemoryEntry struct {
	ID          int64     `json:"id"`
	Key         string    `json:"key"`
	Value       string    `json:"value"`
	SourceTool  string    `json:"source_tool"`
	SessionID   string    `json:"session_id"`
	Tags        []string  `json:"tags,omitempty"`
	Confidence  float64   `json:"confidence"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	ContentHash string    `json:"content_hash"`
}

// MemoryEdge represents a directed relationship between two memory entries.
type MemoryEdge struct {
	ID         int64     `json:"id"`
	FromKey    string    `json:"from_key"`
	ToKey      string    `json:"to_key"`
	Relation   string    `json:"relation"`
	Confidence float64   `json:"confidence"`
	SourceTool string    `json:"source_tool"`
	SessionID  string    `json:"session_id"`
	CreatedAt  time.Time `json:"created_at"`
}
