package types

import "time"

// EscrowAction represents a destructive action intercepted and held pending developer approval.
type EscrowAction struct {
	ID          string                 `json:"id"`
	SessionID   string                 `json:"session_id"`
	ActionType  string                 `json:"action_type"`
	Target      string                 `json:"target"`
	Params      map[string]interface{} `json:"params,omitempty"`
	Agent       string                 `json:"agent"`
	Status      string                 `json:"status"`
	Description string                 `json:"description,omitempty"`
	DecidedAt   *time.Time             `json:"decided_at,omitempty"`
	DecidedBy   string                 `json:"decided_by,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
}
