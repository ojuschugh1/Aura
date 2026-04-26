package types

import "time"

// WikiPage represents a single page in the LLM-maintained wiki.
type WikiPage struct {
	ID         int64     `json:"id"`
	Slug       string    `json:"slug"`
	Title      string    `json:"title"`
	Content    string    `json:"content"`
	Category   string    `json:"category"` // entity, concept, source, synthesis, comparison
	Tags       []string  `json:"tags,omitempty"`
	SourceIDs  []int64   `json:"source_ids,omitempty"` // raw sources that contributed
	LinksSlugs []string  `json:"links,omitempty"`      // outbound cross-references
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
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
