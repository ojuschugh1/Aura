package wiki

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ojuschugh1/aura/pkg/types"
)

// Engine orchestrates wiki operations: ingest, query, lint.
type Engine struct {
	store *Store
}

// NewEngine creates an Engine backed by the given store.
func NewEngine(store *Store) *Engine {
	return &Engine{store: store}
}

// Store returns the underlying wiki store for direct access.
func (e *Engine) Store() *Store {
	return e.store
}

// BatchIngestResult summarises a batch ingestion of multiple files.
type BatchIngestResult struct {
	Total        int              `json:"total"`
	Ingested     int              `json:"ingested"`
	Duplicates   int              `json:"duplicates"`
	Errors       int              `json:"errors"`
	Results      []*IngestResult  `json:"results"`
	ErrorDetails []string         `json:"error_details,omitempty"`
}

// BatchIngest processes all supported files in a directory.
func (e *Engine) BatchIngest(dir string) (*BatchIngestResult, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read directory: %w", err)
	}

	result := &BatchIngestResult{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".md" && ext != ".markdown" && ext != ".txt" && ext != ".jsonl" {
			continue
		}

		result.Total++
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			result.Errors++
			result.ErrorDetails = append(result.ErrorDetails, fmt.Sprintf("%s: %v", entry.Name(), err))
			continue
		}

		format := "text"
		switch ext {
		case ".md", ".markdown":
			format = "markdown"
		case ".jsonl":
			format = "jsonl"
		}

		ir, err := e.Ingest(entry.Name(), string(data), format, path)
		if err != nil {
			result.Errors++
			result.ErrorDetails = append(result.ErrorDetails, fmt.Sprintf("%s: %v", entry.Name(), err))
			continue
		}

		result.Results = append(result.Results, ir)
		if ir.Duplicate {
			result.Duplicates++
		} else {
			result.Ingested++
		}
	}

	_ = e.store.AppendLog("batch-ingest",
		fmt.Sprintf("Batch ingested from %s: %d files, %d new, %d duplicates, %d errors",
			dir, result.Total, result.Ingested, result.Duplicates, result.Errors),
		nil, nil)

	return result, nil
}

// IngestResult summarises what happened during source ingestion.
type IngestResult struct {
	SourceID     int64    `json:"source_id"`
	SourceTitle  string   `json:"source_title"`
	PagesCreated []string `json:"pages_created"`
	PagesUpdated []string `json:"pages_updated"`
	Duplicate    bool     `json:"duplicate"`
}

// Ingest processes a raw source: stores it, extracts key topics, creates/updates
// wiki pages, updates the index, and appends to the log.
// No LLM calls — extraction is regex/heuristic-based (same philosophy as auto-capture).
func (e *Engine) Ingest(title, content, format, origin string) (*IngestResult, error) {
	// Store the raw source.
	src, isDuplicate, err := e.store.IngestSource(title, content, format, origin)
	if err != nil {
		return nil, fmt.Errorf("ingest source: %w", err)
	}

	result := &IngestResult{
		SourceID:    src.ID,
		SourceTitle: src.Title,
	}

	// Check if this was a duplicate.
	if isDuplicate {
		result.Duplicate = true
		return result, nil
	}

	// Create a source summary page.
	slug := slugify(title)
	summaryContent := buildSourceSummary(title, content, origin)
	existing, _ := e.store.GetPage(slug)
	if existing != nil {
		// Update existing page with new source info.
		newContent := existing.Content + "\n\n---\n\n" + summaryContent
		newSourceIDs := appendUnique(existing.SourceIDs, src.ID)
		_, err := e.store.UpdatePage(slug, newContent, existing.Tags, newSourceIDs, existing.LinksSlugs)
		if err != nil {
			return nil, fmt.Errorf("update page %q: %w", slug, err)
		}
		result.PagesUpdated = append(result.PagesUpdated, slug)
	} else {
		_, err := e.store.CreatePage(slug, title, summaryContent, "source",
			[]string{"auto-ingested"}, []int64{src.ID}, nil)
		if err != nil {
			return nil, fmt.Errorf("create page %q: %w", slug, err)
		}
		result.PagesCreated = append(result.PagesCreated, slug)
	}

	// Extract topics and create/update entity/concept pages.
	topics := extractTopics(content)
	for _, topic := range topics {
		topicSlug := slugify(topic.Name)
		if topicSlug == slug {
			continue // skip self-reference
		}

		existingTopic, _ := e.store.GetPage(topicSlug)
		if existingTopic != nil {
			// Append a reference from this source.
			ref := fmt.Sprintf("\n\n### From: %s\n\n%s", title, topic.Context)
			newContent := existingTopic.Content + ref
			newSourceIDs := appendUnique(existingTopic.SourceIDs, src.ID)
			newLinks := appendUniqueStr(existingTopic.LinksSlugs, slug)
			_, err := e.store.UpdatePage(topicSlug, newContent, existingTopic.Tags, newSourceIDs, newLinks)
			if err == nil {
				result.PagesUpdated = append(result.PagesUpdated, topicSlug)
			}
		} else {
			topicContent := fmt.Sprintf("# %s\n\n%s\n\n## Sources\n\n- [[%s]]",
				topic.Name, topic.Context, slug)
			_, err := e.store.CreatePage(topicSlug, topic.Name, topicContent, topic.Category,
				[]string{"auto-extracted"}, []int64{src.ID}, []string{slug})
			if err == nil {
				result.PagesCreated = append(result.PagesCreated, topicSlug)
			}
		}
	}

	// Update the source page's links to include extracted topics.
	if page, _ := e.store.GetPage(slug); page != nil {
		var topicSlugs []string
		for _, t := range topics {
			topicSlugs = append(topicSlugs, slugify(t.Name))
		}
		allLinks := appendUniqueStrs(page.LinksSlugs, topicSlugs)
		_, _ = e.store.UpdatePage(slug, page.Content, page.Tags, page.SourceIDs, allLinks)
	}

	// Log the ingestion.
	allSlugs := append(result.PagesCreated, result.PagesUpdated...)
	_ = e.store.AppendLog("ingest", fmt.Sprintf("Ingested: %s", title), allSlugs, &src.ID)

	return result, nil
}

// QueryResult holds the answer to a wiki query.
type QueryResult struct {
	Query        string             `json:"query"`
	Pages        []*types.WikiPage  `json:"pages"`
	Answer       string             `json:"answer"`
	PageCount    int                `json:"page_count"`
	SavedSlug    string             `json:"saved_slug,omitempty"` // set when answer is filed as a page
}

// Query searches the wiki for pages matching the query string.
// Returns matching pages and a synthesised answer built from their content.
func (e *Engine) Query(query string) (*QueryResult, error) {
	pages, err := e.store.SearchPages(query)
	if err != nil {
		return nil, fmt.Errorf("query wiki: %w", err)
	}

	result := &QueryResult{
		Query:     query,
		Pages:     pages,
		PageCount: len(pages),
	}

	// Build a synthesised answer from matching pages.
	if len(pages) > 0 {
		var parts []string
		for _, p := range pages {
			excerpt := p.Content
			if len(excerpt) > 300 {
				excerpt = excerpt[:300] + "…"
			}
			parts = append(parts, fmt.Sprintf("## %s\n\n%s", p.Title, excerpt))
		}
		result.Answer = strings.Join(parts, "\n\n---\n\n")
	} else {
		result.Answer = fmt.Sprintf("No wiki pages found matching %q.", query)
	}

	// Log the query.
	var slugs []string
	for _, p := range pages {
		slugs = append(slugs, p.Slug)
	}
	_ = e.store.AppendLog("query", fmt.Sprintf("Query: %s (%d results)", query, len(pages)), slugs, nil)

	return result, nil
}

// SaveQueryResult files a query answer back into the wiki as a new synthesis page.
// This implements the gist's insight: "good answers can be filed back into the wiki
// as new pages... these are valuable and shouldn't disappear into chat history."
func (e *Engine) SaveQueryResult(result *QueryResult) (string, error) {
	slug := slugify("synthesis-" + result.Query)
	title := "Synthesis: " + result.Query

	// Collect source page slugs as links.
	var links []string
	for _, p := range result.Pages {
		links = append(links, p.Slug)
	}

	// Collect source IDs from contributing pages.
	var sourceIDs []int64
	seen := make(map[int64]bool)
	for _, p := range result.Pages {
		for _, id := range p.SourceIDs {
			if !seen[id] {
				sourceIDs = append(sourceIDs, id)
				seen[id] = true
			}
		}
	}

	content := fmt.Sprintf("# %s\n\n*Query: %q — %d source pages*\n\n%s",
		title, result.Query, result.PageCount, result.Answer)

	existing, _ := e.store.GetPage(slug)
	if existing != nil {
		_, err := e.store.UpdatePage(slug, content, []string{"synthesis", "auto-saved"}, sourceIDs, links)
		if err != nil {
			return "", fmt.Errorf("update synthesis page: %w", err)
		}
	} else {
		_, err := e.store.CreatePage(slug, title, content, "synthesis",
			[]string{"synthesis", "auto-saved"}, sourceIDs, links)
		if err != nil {
			return "", fmt.Errorf("create synthesis page: %w", err)
		}
	}

	_ = e.store.AppendLog("save", fmt.Sprintf("Saved query answer: %s", result.Query), []string{slug}, nil)

	return slug, nil
}

// Lint performs a health check on the wiki and returns issues found.
func (e *Engine) Lint() (*types.WikiLintResult, error) {
	pages, err := e.store.ListPages("")
	if err != nil {
		return nil, fmt.Errorf("lint: list pages: %w", err)
	}

	result := &types.WikiLintResult{
		TotalPages:   e.store.PageCount(),
		TotalSources: e.store.SourceCount(),
	}

	// Build inbound link map.
	inbound := make(map[string]int)
	allSlugs := make(map[string]bool)
	for _, p := range pages {
		allSlugs[p.Slug] = true
	}

	for _, p := range pages {
		for _, link := range p.LinksSlugs {
			inbound[link]++
			if !allSlugs[link] {
				result.MissingPages = append(result.MissingPages, link)
			}
		}
	}

	// Find orphans (no inbound links, excluding the index itself).
	for _, p := range pages {
		if inbound[p.Slug] == 0 && p.Category != "index" {
			result.Orphans = append(result.Orphans, p.Slug)
		}
	}

	// Find stale pages (not updated in 30+ days).
	cutoff := time.Now().AddDate(0, 0, -30)
	for _, p := range pages {
		if p.UpdatedAt.Before(cutoff) {
			result.Stale = append(result.Stale, p.Slug)
		}
	}

	// Deduplicate missing pages.
	result.MissingPages = dedup(result.MissingPages)

	// Detect contradictions across pages.
	result.Contradictions = findContradictions(pages)

	// Calculate health score.
	if result.TotalPages > 0 {
		issues := len(result.Orphans) + len(result.Stale) + len(result.MissingPages) + len(result.Contradictions)
		result.HealthScore = 1.0 - float64(issues)/float64(result.TotalPages*4)
		if result.HealthScore < 0 {
			result.HealthScore = 0
		}
	}

	// Log the lint pass.
	_ = e.store.AppendLog("lint",
		fmt.Sprintf("Lint: %d pages, %d orphans, %d stale, %d missing, %d contradictions, score=%.2f",
			result.TotalPages, len(result.Orphans), len(result.Stale),
			len(result.MissingPages), len(result.Contradictions), result.HealthScore),
		nil, nil)

	return result, nil
}

// --- Topic extraction (heuristic, no LLM) ---

type extractedTopic struct {
	Name     string
	Context  string
	Category string // "entity" or "concept"
}

// extractTopics pulls out key topics from content using heuristic patterns.
func extractTopics(content string) []extractedTopic {
	var topics []extractedTopic
	seen := make(map[string]bool)

	// Extract markdown headers as topics.
	headerRe := regexp.MustCompile(`(?m)^#{1,3}\s+(.+)$`)
	for _, match := range headerRe.FindAllStringSubmatch(content, -1) {
		name := strings.TrimSpace(match[1])
		lower := strings.ToLower(name)
		if seen[lower] || len(name) < 3 || len(name) > 80 {
			continue
		}
		seen[lower] = true

		// Get surrounding context (up to 200 chars after the header).
		idx := strings.Index(content, match[0])
		contextEnd := idx + len(match[0]) + 200
		if contextEnd > len(content) {
			contextEnd = len(content)
		}
		context := strings.TrimSpace(content[idx+len(match[0]) : contextEnd])

		topics = append(topics, extractedTopic{
			Name:     name,
			Context:  context,
			Category: categorize(name),
		})
	}

	// Extract bold terms as potential entities.
	boldRe := regexp.MustCompile(`\*\*([^*]{3,50})\*\*`)
	for _, match := range boldRe.FindAllStringSubmatch(content, 20) {
		name := strings.TrimSpace(match[1])
		lower := strings.ToLower(name)
		if seen[lower] {
			continue
		}
		seen[lower] = true

		// Find the sentence containing this bold term.
		sentenceCtx := extractSentence(content, match[0])

		topics = append(topics, extractedTopic{
			Name:     name,
			Context:  sentenceCtx,
			Category: "entity",
		})
	}

	return topics
}

// categorize guesses whether a topic name is an entity or concept.
func categorize(name string) string {
	lower := strings.ToLower(name)
	conceptWords := []string{"overview", "summary", "introduction", "architecture",
		"design", "pattern", "principle", "concept", "theory", "approach",
		"strategy", "methodology", "comparison", "analysis"}
	for _, w := range conceptWords {
		if strings.Contains(lower, w) {
			return "concept"
		}
	}
	return "entity"
}

// extractSentence returns the sentence containing needle from text.
func extractSentence(text, needle string) string {
	idx := strings.Index(text, needle)
	if idx < 0 {
		return ""
	}
	// Walk backward to sentence start.
	start := idx
	for start > 0 && text[start-1] != '.' && text[start-1] != '\n' {
		start--
	}
	// Walk forward to sentence end.
	end := idx + len(needle)
	for end < len(text) && text[end] != '.' && text[end] != '\n' {
		end++
	}
	if end < len(text) {
		end++ // include the period
	}
	return strings.TrimSpace(text[start:end])
}

// --- Slug and utility helpers ---

// slugify converts a title to a URL-safe slug.
func slugify(title string) string {
	s := strings.ToLower(title)
	s = regexp.MustCompile(`[^a-z0-9\s-]`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`[\s]+`).ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 80 {
		s = s[:80]
	}
	return s
}

func buildSourceSummary(title, content, origin string) string {
	excerpt := content
	if len(excerpt) > 500 {
		excerpt = excerpt[:500] + "…"
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# %s\n\n", title))
	if origin != "" {
		b.WriteString(fmt.Sprintf("**Origin:** %s\n\n", origin))
	}
	b.WriteString("## Summary\n\n")
	b.WriteString(excerpt)
	return b.String()
}

func appendUnique(ids []int64, id int64) []int64 {
	for _, existing := range ids {
		if existing == id {
			return ids
		}
	}
	return append(ids, id)
}

func appendUniqueStr(ss []string, s string) []string {
	for _, existing := range ss {
		if existing == s {
			return ss
		}
	}
	return append(ss, s)
}

func appendUniqueStrs(existing, additions []string) []string {
	set := make(map[string]bool)
	for _, s := range existing {
		set[s] = true
	}
	result := make([]string, len(existing))
	copy(result, existing)
	for _, s := range additions {
		if !set[s] {
			result = append(result, s)
			set[s] = true
		}
	}
	return result
}

func dedup(ss []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
