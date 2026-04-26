package cost

import (
	"database/sql"
	"fmt"
	"time"
)

// Summary holds aggregated cost data for a period.
type Summary struct {
	Period           string  `json:"period"`
	InputTokens      int64   `json:"input_tokens"`
	OutputTokens     int64   `json:"output_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	CostUSD          float64 `json:"cost_usd"`
	SavedUSD         float64 `json:"saved_usd"`
	SavedTokens      int64   `json:"saved_tokens"`
}

// SessionSummary returns cost totals for the given session ID.
func SessionSummary(db *sql.DB, sessionID string) (*Summary, error) {
	return querySummary(db, "session", "session_id = ?", sessionID)
}

// DailySummary returns cost totals for today (UTC).
func DailySummary(db *sql.DB) (*Summary, error) {
	today := time.Now().UTC().Format("2006-01-02")
	return querySummary(db, "daily", "recorded_at >= ?", today)
}

// WeeklySummary returns cost totals for the current week (Mon–Sun UTC).
func WeeklySummary(db *sql.DB) (*Summary, error) {
	now := time.Now().UTC()
	weekStart := now.AddDate(0, 0, -int(now.Weekday())).Format("2006-01-02")
	return querySummary(db, "weekly", "recorded_at >= ?", weekStart)
}

func querySummary(db *sql.DB, period, where, arg string) (*Summary, error) {
	row := db.QueryRow(fmt.Sprintf(`
		SELECT
			COALESCE(SUM(input_tokens), 0),
			COALESCE(SUM(output_tokens), 0),
			COALESCE(SUM(cost_usd), 0),
			COALESCE(SUM(saved_usd), 0),
			COALESCE(SUM(COALESCE(original_tokens,0) - COALESCE(compressed_tokens,0)), 0)
		FROM cost_records WHERE %s`, where), arg,
	)

	var s Summary
	s.Period = period
	if err := row.Scan(&s.InputTokens, &s.OutputTokens, &s.CostUSD, &s.SavedUSD, &s.SavedTokens); err != nil {
		return nil, fmt.Errorf("query cost summary: %w", err)
	}
	s.TotalTokens = s.InputTokens + s.OutputTokens
	return &s, nil
}
