package router

import (
	"database/sql"
	"fmt"
	"time"
)

// ModelMap maps complexity levels to model names.
type ModelMap struct {
	Low    string
	Medium string
	High   string
}

// DefaultModelMap returns sensible defaults.
func DefaultModelMap() ModelMap {
	return ModelMap{
		Low:    "gpt-4o-mini",
		Medium: "claude-sonnet",
		High:   "claude-opus",
	}
}

// Decision is the output of a routing decision.
type Decision struct {
	Model          string `json:"model"`
	Classification string `json:"classification"`
	Reasoning      string `json:"reasoning"`
}

// Router routes tasks to models based on complexity and budget.
type Router struct {
	db      *sql.DB
	models  ModelMap
	cfg     ClassifyConfig
	budget  *BudgetTracker
}

// New creates a Router with the given database, model map, and classify config.
func New(db *sql.DB, models ModelMap, cfg ClassifyConfig, budget *BudgetTracker) *Router {
	return &Router{db: db, models: models, cfg: cfg, budget: budget}
}

// Route classifies content and returns the appropriate model.
func (r *Router) Route(sessionID, content string) (*Decision, error) {
	start := time.Now()
	classification := Classify(content, r.cfg)

	model := r.modelForClass(classification)

	// Check budget ceiling.
	if r.budget != nil {
		if err := r.budget.Check(model); err != nil {
			return nil, fmt.Errorf("routing blocked: %w", err)
		}
	}

	reasoning := fmt.Sprintf("classified as %s (%d tokens)", classification, estimateTokens(content))
	d := &Decision{Model: model, Classification: classification, Reasoning: reasoning}

	// Persist the routing decision.
	if r.db != nil {
		latencyMs := time.Since(start).Milliseconds()
		_, _ = r.db.Exec(`
			INSERT INTO routing_decisions (session_id, task_hash, classification, selected_model, reasoning, latency_ms)
			VALUES (?, ?, ?, ?, ?, ?)`,
			sessionID, hashContent(content), classification, model, reasoning, latencyMs,
		)
	}
	return d, nil
}

func (r *Router) modelForClass(class string) string {
	switch class {
	case Low:
		return r.models.Low
	case Medium:
		return r.models.Medium
	default:
		return r.models.High
	}
}

// hashContent returns a short hash of content for dedup tracking.
func hashContent(content string) string {
	if len(content) > 64 {
		content = content[:64]
	}
	return fmt.Sprintf("%x", []byte(content))
}
