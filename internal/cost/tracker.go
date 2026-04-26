package cost

import (
	"database/sql"
	"fmt"
	"time"
)

// Tracker records token usage and cost per MCP interaction.
type Tracker struct {
	db *sql.DB
}

// New creates a Tracker backed by the given database.
func New(db *sql.DB) *Tracker {
	return &Tracker{db: db}
}

// Record inserts a cost record for a single MCP interaction.
func (t *Tracker) Record(sessionID, sourceTool, model string, inputTokens, outputTokens, originalTokens, compressedTokens int) error {
	costUSD := CalcCost(model, inputTokens, outputTokens)
	savedUSD := 0.0
	if originalTokens > 0 && compressedTokens > 0 && originalTokens > compressedTokens {
		savedUSD = CalcCost(model, originalTokens-compressedTokens, 0)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := t.db.Exec(`
		INSERT INTO cost_records
			(session_id, source_tool, model, input_tokens, output_tokens, original_tokens, compressed_tokens, cost_usd, saved_usd, recorded_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sessionID, sourceTool, model, inputTokens, outputTokens, originalTokens, compressedTokens, costUSD, savedUSD, now,
	)
	if err != nil {
		return fmt.Errorf("record cost: %w", err)
	}
	return nil
}
