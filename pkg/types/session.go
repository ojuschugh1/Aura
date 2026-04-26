package types

import "time"

// Session represents a bounded interaction between a developer and one or more AI agents.
type Session struct {
	ID        string     `json:"id"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
	Status    string     `json:"status"`
	Tools     []string   `json:"tools,omitempty"`
}
