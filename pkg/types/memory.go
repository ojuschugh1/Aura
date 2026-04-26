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
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	ContentHash string    `json:"content_hash"`
}
