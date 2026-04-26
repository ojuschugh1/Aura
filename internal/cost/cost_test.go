package cost

import (
	"database/sql"
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/ojuschugh1/aura/internal/db"
)

// openTestDB creates an isolated SQLite database with all migrations applied.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	d, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	if err := db.RunMigrations(d); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

// createTestSession inserts a session row so cost_records FK is satisfied.
func createTestSession(t *testing.T, d *sql.DB, sessionID string) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := d.Exec(
		`INSERT INTO sessions (id, started_at, status) VALUES (?, ?, 'active')`,
		sessionID, now,
	)
	if err != nil {
		t.Fatalf("create test session %s: %v", sessionID, err)
	}
}

// almostEqual checks whether two float64 values are within a small tolerance.
func almostEqual(a, b, tolerance float64) bool {
	return math.Abs(a-b) < tolerance
}

// --- Pricing tests (Req 6.4) -----------------------------------------------

func TestCalcCost_ClaudeSonnet(t *testing.T) {
	// claude-sonnet: $3.00/1M input, $15.00/1M output
	got := CalcCost("claude-sonnet", 1_000_000, 1_000_000)
	want := 3.00 + 15.00
	if !almostEqual(got, want, 0.001) {
		t.Errorf("CalcCost(claude-sonnet, 1M, 1M) = %f, want %f", got, want)
	}
}

func TestCalcCost_ClaudeHaiku(t *testing.T) {
	// claude-haiku: $0.25/1M input, $1.25/1M output
	got := CalcCost("claude-haiku", 1_000_000, 1_000_000)
	want := 0.25 + 1.25
	if !almostEqual(got, want, 0.001) {
		t.Errorf("CalcCost(claude-haiku, 1M, 1M) = %f, want %f", got, want)
	}
}

func TestCalcCost_ClaudeOpus(t *testing.T) {
	// claude-opus: $15.00/1M input, $75.00/1M output
	got := CalcCost("claude-opus", 1_000_000, 1_000_000)
	want := 15.00 + 75.00
	if !almostEqual(got, want, 0.001) {
		t.Errorf("CalcCost(claude-opus, 1M, 1M) = %f, want %f", got, want)
	}
}

func TestCalcCost_GPT4o(t *testing.T) {
	// gpt-4o: $2.50/1M input, $10.00/1M output
	got := CalcCost("gpt-4o", 1_000_000, 1_000_000)
	want := 2.50 + 10.00
	if !almostEqual(got, want, 0.001) {
		t.Errorf("CalcCost(gpt-4o, 1M, 1M) = %f, want %f", got, want)
	}
}

func TestCalcCost_GPT4oMini(t *testing.T) {
	// gpt-4o-mini: $0.15/1M input, $0.60/1M output
	got := CalcCost("gpt-4o-mini", 1_000_000, 1_000_000)
	want := 0.15 + 0.60
	if !almostEqual(got, want, 0.001) {
		t.Errorf("CalcCost(gpt-4o-mini, 1M, 1M) = %f, want %f", got, want)
	}
}

func TestCalcCost_SmallTokenCounts(t *testing.T) {
	// 500 input tokens of claude-sonnet: (500/1M)*3.00 = 0.0015
	// 200 output tokens: (200/1M)*15.00 = 0.003
	got := CalcCost("claude-sonnet", 500, 200)
	want := 0.0015 + 0.003
	if !almostEqual(got, want, 0.000001) {
		t.Errorf("CalcCost(claude-sonnet, 500, 200) = %f, want %f", got, want)
	}
}

func TestCalcCost_ZeroTokens(t *testing.T) {
	got := CalcCost("claude-sonnet", 0, 0)
	if got != 0 {
		t.Errorf("CalcCost with zero tokens = %f, want 0", got)
	}
}

func TestCalcCost_UnknownModel(t *testing.T) {
	got := CalcCost("unknown-model", 1000, 1000)
	if got != 0 {
		t.Errorf("CalcCost(unknown-model) = %f, want 0", got)
	}
}

func TestCalcCost_InputOnlyTokens(t *testing.T) {
	got := CalcCost("gpt-4o", 1_000_000, 0)
	want := 2.50
	if !almostEqual(got, want, 0.001) {
		t.Errorf("CalcCost(gpt-4o, 1M input, 0 output) = %f, want %f", got, want)
	}
}

func TestCalcCost_OutputOnlyTokens(t *testing.T) {
	got := CalcCost("gpt-4o", 0, 1_000_000)
	want := 10.00
	if !almostEqual(got, want, 0.001) {
		t.Errorf("CalcCost(gpt-4o, 0 input, 1M output) = %f, want %f", got, want)
	}
}

// --- Tracker tests (Req 6.4, 6.5) ------------------------------------------

func TestTracker_RecordInsertsCostRecord(t *testing.T) {
	d := openTestDB(t)
	createTestSession(t, d, "sess-1")
	tr := New(d)

	err := tr.Record("sess-1", "code-review", "claude-sonnet", 1000, 500, 0, 0)
	if err != nil {
		t.Fatalf("Record: %v", err)
	}

	// Verify the record was inserted.
	var count int
	if err := d.QueryRow("SELECT COUNT(*) FROM cost_records WHERE session_id = ?", "sess-1").Scan(&count); err != nil {
		t.Fatalf("query count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 cost record, got %d", count)
	}
}

func TestTracker_RecordCalculatesCostCorrectly(t *testing.T) {
	d := openTestDB(t)
	createTestSession(t, d, "sess-1")
	tr := New(d)

	// 1000 input + 500 output of claude-sonnet
	// cost = (1000/1M)*3.00 + (500/1M)*15.00 = 0.003 + 0.0075 = 0.0105
	err := tr.Record("sess-1", "code-review", "claude-sonnet", 1000, 500, 0, 0)
	if err != nil {
		t.Fatalf("Record: %v", err)
	}

	var costUSD float64
	if err := d.QueryRow("SELECT cost_usd FROM cost_records WHERE session_id = ?", "sess-1").Scan(&costUSD); err != nil {
		t.Fatalf("query cost_usd: %v", err)
	}
	want := 0.0105
	if !almostEqual(costUSD, want, 0.0001) {
		t.Errorf("cost_usd = %f, want %f", costUSD, want)
	}
}

func TestTracker_RecordAttributesSourceToolAndSession(t *testing.T) {
	d := openTestDB(t)
	createTestSession(t, d, "sess-42")
	tr := New(d)

	err := tr.Record("sess-42", "file-edit", "gpt-4o", 100, 50, 0, 0)
	if err != nil {
		t.Fatalf("Record: %v", err)
	}

	var sessionID, sourceTool, model string
	err = d.QueryRow(
		"SELECT session_id, source_tool, model FROM cost_records WHERE session_id = ?",
		"sess-42",
	).Scan(&sessionID, &sourceTool, &model)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if sessionID != "sess-42" {
		t.Errorf("session_id = %q, want %q", sessionID, "sess-42")
	}
	if sourceTool != "file-edit" {
		t.Errorf("source_tool = %q, want %q", sourceTool, "file-edit")
	}
	if model != "gpt-4o" {
		t.Errorf("model = %q, want %q", model, "gpt-4o")
	}
}

func TestTracker_RecordMultipleInteractions(t *testing.T) {
	d := openTestDB(t)
	createTestSession(t, d, "sess-1")
	tr := New(d)

	for i := 0; i < 5; i++ {
		if err := tr.Record("sess-1", "tool", "claude-haiku", 100, 50, 0, 0); err != nil {
			t.Fatalf("Record %d: %v", i, err)
		}
	}

	var count int
	if err := d.QueryRow("SELECT COUNT(*) FROM cost_records WHERE session_id = ?", "sess-1").Scan(&count); err != nil {
		t.Fatalf("query count: %v", err)
	}
	if count != 5 {
		t.Errorf("expected 5 cost records, got %d", count)
	}
}

// --- Tracker savings tests (Req 6.6) ----------------------------------------

func TestTracker_RecordCalculatesSavings(t *testing.T) {
	d := openTestDB(t)
	createTestSession(t, d, "sess-1")
	tr := New(d)

	// original=10000, compressed=6000 → saved 4000 input tokens
	// saved_usd = CalcCost("claude-sonnet", 4000, 0) = (4000/1M)*3.00 = 0.012
	err := tr.Record("sess-1", "compress", "claude-sonnet", 6000, 500, 10000, 6000)
	if err != nil {
		t.Fatalf("Record: %v", err)
	}

	var savedUSD float64
	if err := d.QueryRow("SELECT saved_usd FROM cost_records WHERE session_id = ?", "sess-1").Scan(&savedUSD); err != nil {
		t.Fatalf("query saved_usd: %v", err)
	}
	want := 0.012
	if !almostEqual(savedUSD, want, 0.0001) {
		t.Errorf("saved_usd = %f, want %f", savedUSD, want)
	}
}

func TestTracker_RecordNoSavingsWhenNoCompression(t *testing.T) {
	d := openTestDB(t)
	createTestSession(t, d, "sess-1")
	tr := New(d)

	err := tr.Record("sess-1", "tool", "claude-sonnet", 1000, 500, 0, 0)
	if err != nil {
		t.Fatalf("Record: %v", err)
	}

	var savedUSD float64
	if err := d.QueryRow("SELECT saved_usd FROM cost_records WHERE session_id = ?", "sess-1").Scan(&savedUSD); err != nil {
		t.Fatalf("query saved_usd: %v", err)
	}
	if savedUSD != 0 {
		t.Errorf("saved_usd = %f, want 0 (no compression)", savedUSD)
	}
}

func TestTracker_RecordNoSavingsWhenCompressedEqualsOriginal(t *testing.T) {
	d := openTestDB(t)
	createTestSession(t, d, "sess-1")
	tr := New(d)

	// original == compressed → no savings
	err := tr.Record("sess-1", "tool", "claude-sonnet", 1000, 500, 5000, 5000)
	if err != nil {
		t.Fatalf("Record: %v", err)
	}

	var savedUSD float64
	if err := d.QueryRow("SELECT saved_usd FROM cost_records WHERE session_id = ?", "sess-1").Scan(&savedUSD); err != nil {
		t.Fatalf("query saved_usd: %v", err)
	}
	if savedUSD != 0 {
		t.Errorf("saved_usd = %f, want 0 (no actual savings)", savedUSD)
	}
}

// --- Report: SessionSummary tests (Req 6.5) ---------------------------------

func TestSessionSummary_CorrectTotals(t *testing.T) {
	d := openTestDB(t)
	createTestSession(t, d, "sess-1")
	tr := New(d)

	// Record two interactions in the same session.
	if err := tr.Record("sess-1", "tool-a", "claude-sonnet", 1000, 500, 0, 0); err != nil {
		t.Fatalf("Record 1: %v", err)
	}
	if err := tr.Record("sess-1", "tool-b", "gpt-4o", 2000, 1000, 0, 0); err != nil {
		t.Fatalf("Record 2: %v", err)
	}

	summary, err := SessionSummary(d, "sess-1")
	if err != nil {
		t.Fatalf("SessionSummary: %v", err)
	}

	if summary.Period != "session" {
		t.Errorf("Period = %q, want %q", summary.Period, "session")
	}
	if summary.InputTokens != 3000 {
		t.Errorf("InputTokens = %d, want 3000", summary.InputTokens)
	}
	if summary.OutputTokens != 1500 {
		t.Errorf("OutputTokens = %d, want 1500", summary.OutputTokens)
	}
	if summary.TotalTokens != 4500 {
		t.Errorf("TotalTokens = %d, want 4500", summary.TotalTokens)
	}

	// cost = CalcCost("claude-sonnet", 1000, 500) + CalcCost("gpt-4o", 2000, 1000)
	//      = 0.0105 + 0.015 = 0.0255
	wantCost := CalcCost("claude-sonnet", 1000, 500) + CalcCost("gpt-4o", 2000, 1000)
	if !almostEqual(summary.CostUSD, wantCost, 0.0001) {
		t.Errorf("CostUSD = %f, want %f", summary.CostUSD, wantCost)
	}
}

func TestSessionSummary_EmptySession(t *testing.T) {
	d := openTestDB(t)
	createTestSession(t, d, "sess-empty")

	summary, err := SessionSummary(d, "sess-empty")
	if err != nil {
		t.Fatalf("SessionSummary: %v", err)
	}

	if summary.InputTokens != 0 || summary.OutputTokens != 0 || summary.TotalTokens != 0 {
		t.Errorf("expected all zero tokens for empty session, got input=%d output=%d total=%d",
			summary.InputTokens, summary.OutputTokens, summary.TotalTokens)
	}
	if summary.CostUSD != 0 {
		t.Errorf("CostUSD = %f, want 0", summary.CostUSD)
	}
}

func TestSessionSummary_IsolatesFromOtherSessions(t *testing.T) {
	d := openTestDB(t)
	createTestSession(t, d, "sess-a")
	createTestSession(t, d, "sess-b")
	tr := New(d)

	if err := tr.Record("sess-a", "tool", "claude-sonnet", 1000, 500, 0, 0); err != nil {
		t.Fatalf("Record sess-a: %v", err)
	}
	if err := tr.Record("sess-b", "tool", "gpt-4o", 2000, 1000, 0, 0); err != nil {
		t.Fatalf("Record sess-b: %v", err)
	}

	summary, err := SessionSummary(d, "sess-a")
	if err != nil {
		t.Fatalf("SessionSummary: %v", err)
	}

	if summary.InputTokens != 1000 {
		t.Errorf("InputTokens = %d, want 1000 (only sess-a)", summary.InputTokens)
	}
	if summary.OutputTokens != 500 {
		t.Errorf("OutputTokens = %d, want 500 (only sess-a)", summary.OutputTokens)
	}
}

func TestSessionSummary_IncludesCompressionSavings(t *testing.T) {
	d := openTestDB(t)
	createTestSession(t, d, "sess-1")
	tr := New(d)

	// original=10000, compressed=6000 → saved 4000 tokens
	if err := tr.Record("sess-1", "tool", "claude-sonnet", 6000, 500, 10000, 6000); err != nil {
		t.Fatalf("Record: %v", err)
	}

	summary, err := SessionSummary(d, "sess-1")
	if err != nil {
		t.Fatalf("SessionSummary: %v", err)
	}

	if summary.SavedTokens != 4000 {
		t.Errorf("SavedTokens = %d, want 4000", summary.SavedTokens)
	}
	wantSaved := CalcCost("claude-sonnet", 4000, 0)
	if !almostEqual(summary.SavedUSD, wantSaved, 0.0001) {
		t.Errorf("SavedUSD = %f, want %f", summary.SavedUSD, wantSaved)
	}
}

// --- Report: DailySummary tests (Req 6.5) -----------------------------------

func TestDailySummary_AggregatesRecordsFromToday(t *testing.T) {
	d := openTestDB(t)
	createTestSession(t, d, "sess-1")
	tr := New(d)

	// Records inserted now will have today's timestamp.
	if err := tr.Record("sess-1", "tool-a", "claude-sonnet", 1000, 500, 0, 0); err != nil {
		t.Fatalf("Record 1: %v", err)
	}
	if err := tr.Record("sess-1", "tool-b", "gpt-4o-mini", 2000, 800, 0, 0); err != nil {
		t.Fatalf("Record 2: %v", err)
	}

	summary, err := DailySummary(d)
	if err != nil {
		t.Fatalf("DailySummary: %v", err)
	}

	if summary.Period != "daily" {
		t.Errorf("Period = %q, want %q", summary.Period, "daily")
	}
	if summary.InputTokens != 3000 {
		t.Errorf("InputTokens = %d, want 3000", summary.InputTokens)
	}
	if summary.OutputTokens != 1300 {
		t.Errorf("OutputTokens = %d, want 1300", summary.OutputTokens)
	}
	if summary.TotalTokens != 4300 {
		t.Errorf("TotalTokens = %d, want 4300", summary.TotalTokens)
	}
}

func TestDailySummary_ExcludesOldRecords(t *testing.T) {
	d := openTestDB(t)
	createTestSession(t, d, "sess-1")

	// Insert a record with yesterday's timestamp directly.
	yesterday := time.Now().UTC().AddDate(0, 0, -1).Format(time.RFC3339Nano)
	_, err := d.Exec(`
		INSERT INTO cost_records
			(session_id, source_tool, model, input_tokens, output_tokens, cost_usd, saved_usd, recorded_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"sess-1", "tool", "claude-sonnet", 5000, 2000, 0.045, 0.0, yesterday,
	)
	if err != nil {
		t.Fatalf("insert old record: %v", err)
	}

	summary, err := DailySummary(d)
	if err != nil {
		t.Fatalf("DailySummary: %v", err)
	}

	if summary.InputTokens != 0 {
		t.Errorf("InputTokens = %d, want 0 (old record should be excluded)", summary.InputTokens)
	}
}

// --- Report: WeeklySummary tests (Req 6.5) ----------------------------------

func TestWeeklySummary_AggregatesRecordsFromThisWeek(t *testing.T) {
	d := openTestDB(t)
	createTestSession(t, d, "sess-1")
	tr := New(d)

	// Records inserted now are within the current week.
	if err := tr.Record("sess-1", "tool", "claude-haiku", 5000, 2000, 0, 0); err != nil {
		t.Fatalf("Record: %v", err)
	}

	summary, err := WeeklySummary(d)
	if err != nil {
		t.Fatalf("WeeklySummary: %v", err)
	}

	if summary.Period != "weekly" {
		t.Errorf("Period = %q, want %q", summary.Period, "weekly")
	}
	if summary.InputTokens != 5000 {
		t.Errorf("InputTokens = %d, want 5000", summary.InputTokens)
	}
	if summary.OutputTokens != 2000 {
		t.Errorf("OutputTokens = %d, want 2000", summary.OutputTokens)
	}
}

func TestWeeklySummary_ExcludesOldRecords(t *testing.T) {
	d := openTestDB(t)
	createTestSession(t, d, "sess-1")

	// Insert a record from 2 weeks ago.
	twoWeeksAgo := time.Now().UTC().AddDate(0, 0, -14).Format(time.RFC3339Nano)
	_, err := d.Exec(`
		INSERT INTO cost_records
			(session_id, source_tool, model, input_tokens, output_tokens, cost_usd, saved_usd, recorded_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"sess-1", "tool", "gpt-4o", 3000, 1000, 0.0175, 0.0, twoWeeksAgo,
	)
	if err != nil {
		t.Fatalf("insert old record: %v", err)
	}

	summary, err := WeeklySummary(d)
	if err != nil {
		t.Fatalf("WeeklySummary: %v", err)
	}

	if summary.InputTokens != 0 {
		t.Errorf("InputTokens = %d, want 0 (old record should be excluded)", summary.InputTokens)
	}
}

// --- Report: Savings in summaries (Req 6.6) ---------------------------------

func TestDailySummary_IncludesCompressionSavings(t *testing.T) {
	d := openTestDB(t)
	createTestSession(t, d, "sess-1")
	tr := New(d)

	// Two records with compression savings.
	if err := tr.Record("sess-1", "tool-a", "claude-sonnet", 6000, 500, 10000, 6000); err != nil {
		t.Fatalf("Record 1: %v", err)
	}
	if err := tr.Record("sess-1", "tool-b", "gpt-4o", 3000, 200, 5000, 3000); err != nil {
		t.Fatalf("Record 2: %v", err)
	}

	summary, err := DailySummary(d)
	if err != nil {
		t.Fatalf("DailySummary: %v", err)
	}

	// saved tokens: (10000-6000) + (5000-3000) = 4000 + 2000 = 6000
	if summary.SavedTokens != 6000 {
		t.Errorf("SavedTokens = %d, want 6000", summary.SavedTokens)
	}

	// saved_usd: CalcCost("claude-sonnet", 4000, 0) + CalcCost("gpt-4o", 2000, 0)
	wantSaved := CalcCost("claude-sonnet", 4000, 0) + CalcCost("gpt-4o", 2000, 0)
	if !almostEqual(summary.SavedUSD, wantSaved, 0.0001) {
		t.Errorf("SavedUSD = %f, want %f", summary.SavedUSD, wantSaved)
	}
}
