package router

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

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

// --- Classification ---------------------------------------------------------

func TestClassify_ShortSimpleText_Low(t *testing.T) {
	cfg := DefaultClassifyConfig()
	content := "fix the typo in readme"
	got := Classify(content, cfg)
	if got != Low {
		t.Errorf("Classify(%q) = %q, want %q", content, got, Low)
	}
}

func TestClassify_MediumLengthText_Medium(t *testing.T) {
	cfg := DefaultClassifyConfig()
	// Build content with ~800 words (above 500 low threshold, below 2000 medium threshold).
	words := make([]string, 800)
	for i := range words {
		words[i] = "refactor"
	}
	content := strings.Join(words, " ")

	got := Classify(content, cfg)
	if got != Medium {
		t.Errorf("Classify(800 words) = %q, want %q", got, Medium)
	}
}

func TestClassify_LongText_High(t *testing.T) {
	cfg := DefaultClassifyConfig()
	// Build content with ~2500 words (above 2000 medium threshold).
	words := make([]string, 2500)
	for i := range words {
		words[i] = "implement"
	}
	content := strings.Join(words, " ")

	got := Classify(content, cfg)
	if got != High {
		t.Errorf("Classify(2500 words) = %q, want %q", got, High)
	}
}

func TestClassify_ExactLowBoundary(t *testing.T) {
	cfg := DefaultClassifyConfig()
	// Exactly 500 words should still be low.
	words := make([]string, 500)
	for i := range words {
		words[i] = "word"
	}
	content := strings.Join(words, " ")

	got := Classify(content, cfg)
	if got != Low {
		t.Errorf("Classify(exactly 500 words) = %q, want %q", got, Low)
	}
}

func TestClassify_ExactMediumBoundary(t *testing.T) {
	cfg := DefaultClassifyConfig()
	// Exactly 2000 words should still be medium.
	words := make([]string, 2000)
	for i := range words {
		words[i] = "word"
	}
	content := strings.Join(words, " ")

	got := Classify(content, cfg)
	if got != Medium {
		t.Errorf("Classify(exactly 2000 words) = %q, want %q", got, Medium)
	}
}

func TestClassify_OnePastLowBoundary_Medium(t *testing.T) {
	cfg := DefaultClassifyConfig()
	// 501 words should be medium.
	words := make([]string, 501)
	for i := range words {
		words[i] = "word"
	}
	content := strings.Join(words, " ")

	got := Classify(content, cfg)
	if got != Medium {
		t.Errorf("Classify(501 words) = %q, want %q", got, Medium)
	}
}

func TestClassify_EmptyContent_Low(t *testing.T) {
	cfg := DefaultClassifyConfig()
	got := Classify("", cfg)
	if got != Low {
		t.Errorf("Classify(empty) = %q, want %q", got, Low)
	}
}

func TestClassify_CustomConfig(t *testing.T) {
	cfg := ClassifyConfig{LowMaxTokens: 10, MediumMaxTokens: 20}
	// 15 words → medium with custom config.
	words := make([]string, 15)
	for i := range words {
		words[i] = "test"
	}
	content := strings.Join(words, " ")

	got := Classify(content, cfg)
	if got != Medium {
		t.Errorf("Classify(15 words, custom cfg) = %q, want %q", got, Medium)
	}
}

func TestEstimateTokens_CountsWords(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"hello", 1},
		{"hello world", 2},
		{"  spaced   out  words  ", 3},
	}
	for _, tt := range tests {
		got := estimateTokens(tt.input)
		if got != tt.want {
			t.Errorf("estimateTokens(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

// --- Budget -----------------------------------------------------------------

func TestBudgetTracker_RecordAccumulates(t *testing.T) {
	bt := NewBudgetTracker(map[string]float64{"gpt-4o-mini": 10.0})

	bt.Record("gpt-4o-mini", 2.50)
	bt.Record("gpt-4o-mini", 3.00)

	// Should still be under ceiling.
	if err := bt.Check("gpt-4o-mini"); err != nil {
		t.Errorf("expected no error after $5.50 of $10 ceiling, got: %v", err)
	}
}

func TestBudgetTracker_CeilingReached(t *testing.T) {
	bt := NewBudgetTracker(map[string]float64{"gpt-4o-mini": 5.0})

	bt.Record("gpt-4o-mini", 5.0)

	err := bt.Check("gpt-4o-mini")
	if err == nil {
		t.Fatal("expected error when budget ceiling reached, got nil")
	}
	if !strings.Contains(err.Error(), "budget ceiling reached") {
		t.Errorf("error should mention budget ceiling, got: %v", err)
	}
}

func TestBudgetTracker_CeilingExceeded(t *testing.T) {
	bt := NewBudgetTracker(map[string]float64{"claude-opus": 1.0})

	bt.Record("claude-opus", 1.50)

	err := bt.Check("claude-opus")
	if err == nil {
		t.Fatal("expected error when budget exceeded, got nil")
	}
}

func TestBudgetTracker_NoCeilingConfigured(t *testing.T) {
	bt := NewBudgetTracker(map[string]float64{"gpt-4o-mini": 5.0})

	// Model with no ceiling should pass.
	if err := bt.Check("unknown-model"); err != nil {
		t.Errorf("expected no error for model without ceiling, got: %v", err)
	}
}

func TestBudgetTracker_IndependentModels(t *testing.T) {
	bt := NewBudgetTracker(map[string]float64{
		"gpt-4o-mini":  5.0,
		"claude-opus":  10.0,
	})

	bt.Record("gpt-4o-mini", 5.0)

	// gpt-4o-mini should be blocked.
	if err := bt.Check("gpt-4o-mini"); err == nil {
		t.Error("expected gpt-4o-mini to be blocked")
	}

	// claude-opus should still be available.
	if err := bt.Check("claude-opus"); err != nil {
		t.Errorf("expected claude-opus to be available, got: %v", err)
	}
}

func TestBudgetTracker_NotificationContainsModelAndCeiling(t *testing.T) {
	bt := NewBudgetTracker(map[string]float64{"gpt-4o-mini": 3.0})
	bt.Record("gpt-4o-mini", 3.0)

	err := bt.Check("gpt-4o-mini")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "gpt-4o-mini") {
		t.Errorf("error should contain model name, got: %v", err)
	}
	if !strings.Contains(err.Error(), "$3.00") {
		t.Errorf("error should contain ceiling amount, got: %v", err)
	}
}

// --- Routing ----------------------------------------------------------------

func TestRoute_LowComplexity_RoutesToCheapModel(t *testing.T) {
	d := openTestDB(t)
	models := DefaultModelMap()
	cfg := DefaultClassifyConfig()
	r := New(d, models, cfg, nil)

	// Insert a session so the FK constraint is satisfied.
	_, err := d.Exec(`INSERT INTO sessions (id) VALUES (?)`, "s1")
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}

	decision, err := r.Route("s1", "fix typo")
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if decision.Model != models.Low {
		t.Errorf("Route(short text).Model = %q, want %q", decision.Model, models.Low)
	}
	if decision.Classification != Low {
		t.Errorf("Route(short text).Classification = %q, want %q", decision.Classification, Low)
	}
}

func TestRoute_HighComplexity_RoutesToCapableModel(t *testing.T) {
	d := openTestDB(t)
	models := DefaultModelMap()
	cfg := DefaultClassifyConfig()
	r := New(d, models, cfg, nil)

	_, err := d.Exec(`INSERT INTO sessions (id) VALUES (?)`, "s1")
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}

	// Build content with 2500 words → high complexity.
	words := make([]string, 2500)
	for i := range words {
		words[i] = "implement"
	}
	content := strings.Join(words, " ")

	decision, err := r.Route("s1", content)
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if decision.Model != models.High {
		t.Errorf("Route(long text).Model = %q, want %q", decision.Model, models.High)
	}
	if decision.Classification != High {
		t.Errorf("Route(long text).Classification = %q, want %q", decision.Classification, High)
	}
}

func TestRoute_MediumComplexity_RoutesToMediumModel(t *testing.T) {
	d := openTestDB(t)
	models := DefaultModelMap()
	cfg := DefaultClassifyConfig()
	r := New(d, models, cfg, nil)

	_, err := d.Exec(`INSERT INTO sessions (id) VALUES (?)`, "s1")
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}

	words := make([]string, 800)
	for i := range words {
		words[i] = "refactor"
	}
	content := strings.Join(words, " ")

	decision, err := r.Route("s1", content)
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if decision.Model != models.Medium {
		t.Errorf("Route(medium text).Model = %q, want %q", decision.Model, models.Medium)
	}
}

func TestRoute_BudgetExhaustedModel_Blocked(t *testing.T) {
	d := openTestDB(t)
	models := DefaultModelMap()
	cfg := DefaultClassifyConfig()
	budget := NewBudgetTracker(map[string]float64{models.Low: 1.0})
	budget.Record(models.Low, 1.0) // exhaust the budget
	r := New(d, models, cfg, budget)

	_, err := d.Exec(`INSERT INTO sessions (id) VALUES (?)`, "s1")
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}

	_, err = r.Route("s1", "fix typo")
	if err == nil {
		t.Fatal("expected routing to be blocked when budget exhausted, got nil")
	}
	if !strings.Contains(err.Error(), "routing blocked") {
		t.Errorf("error should mention routing blocked, got: %v", err)
	}
}

func TestRoute_NoBudgetTracker_Succeeds(t *testing.T) {
	d := openTestDB(t)
	models := DefaultModelMap()
	cfg := DefaultClassifyConfig()
	r := New(d, models, cfg, nil) // nil budget

	_, err := d.Exec(`INSERT INTO sessions (id) VALUES (?)`, "s1")
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}

	decision, err := r.Route("s1", "fix typo")
	if err != nil {
		t.Fatalf("Route with nil budget: %v", err)
	}
	if decision.Model == "" {
		t.Error("expected a model to be selected")
	}
}

func TestRoute_PersistsDecisionToDB(t *testing.T) {
	d := openTestDB(t)
	models := DefaultModelMap()
	cfg := DefaultClassifyConfig()
	r := New(d, models, cfg, nil)

	_, err := d.Exec(`INSERT INTO sessions (id) VALUES (?)`, "s1")
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}

	_, err = r.Route("s1", "fix typo")
	if err != nil {
		t.Fatalf("Route: %v", err)
	}

	var count int
	err = d.QueryRow(`SELECT COUNT(*) FROM routing_decisions WHERE session_id = ?`, "s1").Scan(&count)
	if err != nil {
		t.Fatalf("query routing_decisions: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 routing decision row, got %d", count)
	}
}

func TestRoute_DecisionContainsReasoning(t *testing.T) {
	d := openTestDB(t)
	models := DefaultModelMap()
	cfg := DefaultClassifyConfig()
	r := New(d, models, cfg, nil)

	_, err := d.Exec(`INSERT INTO sessions (id) VALUES (?)`, "s1")
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}

	decision, err := r.Route("s1", "fix typo")
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if decision.Reasoning == "" {
		t.Error("expected non-empty reasoning in decision")
	}
	if !strings.Contains(decision.Reasoning, "classified as") {
		t.Errorf("reasoning should explain classification, got: %q", decision.Reasoning)
	}
}

// --- DefaultModelMap --------------------------------------------------------

func TestDefaultModelMap_HasAllLevels(t *testing.T) {
	m := DefaultModelMap()
	if m.Low == "" {
		t.Error("DefaultModelMap().Low is empty")
	}
	if m.Medium == "" {
		t.Error("DefaultModelMap().Medium is empty")
	}
	if m.High == "" {
		t.Error("DefaultModelMap().High is empty")
	}
}

// --- DefaultClassifyConfig --------------------------------------------------

func TestDefaultClassifyConfig_Thresholds(t *testing.T) {
	cfg := DefaultClassifyConfig()
	if cfg.LowMaxTokens <= 0 {
		t.Errorf("LowMaxTokens = %d, want > 0", cfg.LowMaxTokens)
	}
	if cfg.MediumMaxTokens <= cfg.LowMaxTokens {
		t.Errorf("MediumMaxTokens (%d) should be > LowMaxTokens (%d)", cfg.MediumMaxTokens, cfg.LowMaxTokens)
	}
}
