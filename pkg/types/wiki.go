package types

import "time"

// WikiPage represents a single page in the LLM-maintained wiki.
type WikiPage struct {
	ID          int64     `json:"id"`
	Slug        string    `json:"slug"`
	Title       string    `json:"title"`
	Content     string    `json:"content"`
	Category    string    `json:"category"`    // entity, concept, source, synthesis, comparison
	Tags        []string  `json:"tags,omitempty"`
	SourceIDs   []int64   `json:"source_ids,omitempty"` // raw sources that contributed
	LinksSlugs  []string  `json:"links,omitempty"`      // outbound cross-references
	Vitality    float64   `json:"vitality"`              // 0.0–1.0, decays over time
	AccessTier  string    `json:"access_tier"`           // public, team, private
	QueryCount  int       `json:"query_count"`           // times this page was read/queried
	LastQueried *time.Time `json:"last_queried,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// WikiSource represents an immutable raw source document ingested into the wiki.
type WikiSource struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Content     string    `json:"content"`
	ContentHash string    `json:"content_hash"`
	Format      string    `json:"format"` // markdown, text, jsonl, url
	Origin      string    `json:"origin"` // file path or URL
	IngestedAt  time.Time `json:"ingested_at"`
}

// WikiLogEntry records a chronological event in the wiki's evolution.
type WikiLogEntry struct {
	ID        int64     `json:"id"`
	Action    string    `json:"action"` // ingest, query, lint, update, create
	Summary   string    `json:"summary"`
	PageSlugs []string  `json:"page_slugs,omitempty"` // pages affected
	SourceID  *int64    `json:"source_id,omitempty"`   // source involved (for ingest)
	Timestamp time.Time `json:"timestamp"`
}

// WikiIndex is the in-memory catalog of all wiki pages for fast lookup.
type WikiIndex struct {
	Entries []WikiIndexEntry `json:"entries"`
}

// WikiIndexEntry is a single row in the wiki index.
type WikiIndexEntry struct {
	Slug     string `json:"slug"`
	Title    string `json:"title"`
	Category string `json:"category"`
	Summary  string `json:"summary"` // first 120 chars of content
	Links    int    `json:"links"`   // number of outbound links
}

// WikiLintResult reports health issues found during a wiki lint pass.
type WikiLintResult struct {
	Orphans        []string            `json:"orphans"`         // pages with no inbound links
	Contradictions []WikiContradiction `json:"contradictions"`  // conflicting claims across pages
	Stale          []string            `json:"stale"`           // pages not updated in 30+ days
	MissingPages   []string            `json:"missing_pages"`   // slugs referenced but not existing
	Suggestions    []WikiSuggestion    `json:"suggestions"`     // actionable recommendations
	TotalPages     int                 `json:"total_pages"`
	TotalSources   int                 `json:"total_sources"`
	HealthScore    float64             `json:"health_score"`    // 0.0–1.0
}

// WikiContradiction flags two pages with potentially conflicting content.
type WikiContradiction struct {
	PageA   string `json:"page_a"`
	PageB   string `json:"page_b"`
	Snippet string `json:"snippet"` // the conflicting text
}

// WikiSuggestion is an actionable recommendation from the lint pass.
type WikiSuggestion struct {
	Type    string `json:"type"`    // "create_page", "add_source", "investigate", "split_page", "add_links"
	Target  string `json:"target"`  // the slug or topic this applies to
	Message string `json:"message"` // human-readable recommendation
}

// WikiPressure tracks accumulated contradictory evidence against a page.
// When pressure from multiple sources exceeds a threshold, the system
// recommends revising the established belief.
type WikiPressure struct {
	ID           int64     `json:"id"`
	TargetSlug   string    `json:"target_slug"`   // the page under pressure
	SourceSlug   string    `json:"source_slug"`   // the page providing contradicting evidence
	Evidence     string    `json:"evidence"`       // description of the contradiction
	PressureType string    `json:"pressure_type"`  // "contradiction", "superseded", "disputed"
	Resolved     bool      `json:"resolved"`
	CreatedAt    time.Time `json:"created_at"`
}

// WikiAuditEntry is a single immutable record in the audit chain.
// Each entry's hash includes the previous entry's hash, forming a
// tamper-evident chain similar to a blockchain.
type WikiAuditEntry struct {
	ID        int64     `json:"id"`
	PageSlug  string    `json:"page_slug"`
	Action    string    `json:"action"`    // "create", "update", "delete", "consolidate", "decay"
	Agent     string    `json:"agent"`     // who performed the action
	PrevHash  string    `json:"prev_hash"` // hash of the previous audit entry
	EntryHash string    `json:"entry_hash"` // SHA-256 of (prev_hash + slug + action + summary + timestamp)
	Summary   string    `json:"summary"`
	CreatedAt time.Time `json:"created_at"`
}

// MetabolismResult holds the outcome of a metabolism cycle.
type MetabolismResult struct {
	PagesDecayed      int                `json:"pages_decayed"`
	PagesConsolidated int                `json:"pages_consolidated"`
	PagesArchived     int                `json:"pages_archived"`
	PressureAlerts    []PressureAlert    `json:"pressure_alerts,omitempty"`
	Suggestions       []WikiSuggestion   `json:"suggestions,omitempty"`
}

// PressureAlert is raised when accumulated contradictions exceed the threshold.
type PressureAlert struct {
	TargetSlug    string   `json:"target_slug"`
	PressureCount int      `json:"pressure_count"`
	Sources       []string `json:"sources"`
	Message       string   `json:"message"`
}
