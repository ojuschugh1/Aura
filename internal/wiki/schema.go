package wiki

import (
	"fmt"
	"strings"
	"time"
)

// SchemaFormat controls which LLM tool the schema targets.
type SchemaFormat string

const (
	SchemaClaudeCode SchemaFormat = "claude"
	SchemaCursor     SchemaFormat = "cursor"
	SchemaKiro       SchemaFormat = "kiro"
	SchemaCodex      SchemaFormat = "codex"
	SchemaGeneric    SchemaFormat = "generic"
)

// GenerateSchema produces a schema document (CLAUDE.md, AGENTS.md, or .kiro
// steering file) that teaches the LLM how to maintain the wiki. This is the
// gist's "third layer" — the configuration that turns a generic chatbot into
// a disciplined wiki maintainer.
func (e *Engine) GenerateSchema(format SchemaFormat) string {
	stats := e.gatherStats()

	var b strings.Builder

	// Header varies by format.
	switch format {
	case SchemaClaudeCode:
		b.WriteString("# CLAUDE.md — Aura Wiki Schema\n\n")
	case SchemaCodex:
		b.WriteString("# AGENTS.md — Aura Wiki Schema\n\n")
	case SchemaKiro:
		b.WriteString("---\ninclusion: auto\n---\n\n# Aura Wiki Schema\n\n")
	default:
		b.WriteString("# Wiki Schema\n\n")
	}

	b.WriteString("This document defines how you (the LLM) should maintain the Aura wiki.\n")
	b.WriteString("Follow these conventions on every interaction.\n\n")

	// Current state.
	b.WriteString("## Current State\n\n")
	b.WriteString(fmt.Sprintf("- **Pages:** %d\n", stats.totalPages))
	b.WriteString(fmt.Sprintf("- **Sources:** %d\n", stats.totalSources))
	b.WriteString(fmt.Sprintf("- **Categories:** %s\n", stats.categories))
	b.WriteString(fmt.Sprintf("- **Last updated:** %s\n\n", time.Now().Format("2006-01-02")))

	// Architecture.
	b.WriteString("## Architecture\n\n")
	b.WriteString("The wiki has three layers:\n\n")
	b.WriteString("1. **Raw sources** — immutable documents stored in the `wiki_sources` table. Never modify these.\n")
	b.WriteString("2. **Wiki pages** — LLM-generated markdown in the `wiki_pages` table. You own this layer. Create, update, and cross-reference pages freely.\n")
	b.WriteString("3. **This schema** — conventions and workflows. Co-evolve this with the user as the wiki grows.\n\n")

	// Page conventions.
	b.WriteString("## Page Conventions\n\n")
	b.WriteString("### Categories\n\n")
	b.WriteString("Every page has exactly one category:\n\n")
	b.WriteString("| Category | Purpose | Example |\n")
	b.WriteString("|----------|---------|--------|\n")
	b.WriteString("| `source` | Summary of an ingested raw source | \"Design Doc Summary\" |\n")
	b.WriteString("| `entity` | A specific thing: person, tool, package, service | \"PostgreSQL\", \"auth-service\" |\n")
	b.WriteString("| `concept` | An idea, pattern, or principle | \"Event Sourcing\", \"Architecture Overview\" |\n")
	b.WriteString("| `synthesis` | An answer or analysis filed back from a query | \"Synthesis: auth comparison\" |\n")
	b.WriteString("| `tool` | Cumulative output from a tool (sqz, ghostdep, etc.) | \"SQZ Compression History\" |\n\n")

	// Slug conventions.
	b.WriteString("### Slugs\n\n")
	b.WriteString("- Lowercase, hyphens for spaces: `postgresql-database`\n")
	b.WriteString("- Max 80 characters\n")
	b.WriteString("- Prefix tool pages with `tool-`: `tool-sqz-compression`\n")
	b.WriteString("- Prefix dependency pages with `dep-`: `dep-axios`\n")
	b.WriteString("- Prefix endpoint pages with `endpoint-`: `endpoint-post-api-users`\n")
	b.WriteString("- Prefix API service pages with `api-`: `api-user-service`\n")
	b.WriteString("- Prefix synthesis pages with `synthesis-`: `synthesis-auth-comparison`\n\n")

	// Cross-references.
	b.WriteString("### Cross-References\n\n")
	b.WriteString("- Use `[[slug]]` wikilink syntax in page content\n")
	b.WriteString("- Always add a `## Sources` or `## Related` section at the bottom with links\n")
	b.WriteString("- When updating a page, check if new links should be added to related pages\n")
	b.WriteString("- The `links` field on each page tracks outbound references\n\n")

	// Workflows.
	b.WriteString("## Workflows\n\n")

	b.WriteString("### Ingest a New Source\n\n")
	b.WriteString("1. Call `wiki_ingest` with the source content\n")
	b.WriteString("2. Review the created/updated pages\n")
	b.WriteString("3. Check if any existing pages should be updated with new cross-references\n")
	b.WriteString("4. If the source contradicts existing pages, update both and note the contradiction\n\n")

	b.WriteString("### Answer a Question\n\n")
	b.WriteString("1. Call `wiki_index` to see all available pages\n")
	b.WriteString("2. Call `wiki_read` on relevant pages\n")
	b.WriteString("3. Synthesize an answer with citations to page slugs\n")
	b.WriteString("4. If the answer is valuable, call `wiki_save_query` to file it as a synthesis page\n\n")

	b.WriteString("### Maintain the Wiki\n\n")
	b.WriteString("1. Call `wiki_lint` periodically to find issues\n")
	b.WriteString("2. Fix orphan pages by adding inbound links from related pages\n")
	b.WriteString("3. Update stale pages with current information\n")
	b.WriteString("4. Create pages for missing references\n")
	b.WriteString("5. Resolve contradictions by updating the outdated page\n\n")

	// Available tools.
	b.WriteString("## Available MCP Tools\n\n")
	b.WriteString("| Tool | Purpose |\n")
	b.WriteString("|------|--------|\n")
	b.WriteString("| `wiki_ingest` | Ingest a new source (title, content, format, origin) |\n")
	b.WriteString("| `wiki_query` | Search pages and get a synthesized answer |\n")
	b.WriteString("| `wiki_read` | Read a specific page by slug |\n")
	b.WriteString("| `wiki_write` | Create or update a page (slug, title, content, category) |\n")
	b.WriteString("| `wiki_search` | Search pages by title and content |\n")
	b.WriteString("| `wiki_index` | Get the full page catalog |\n")
	b.WriteString("| `wiki_lint` | Health-check: orphans, stale, contradictions, missing |\n")
	b.WriteString("| `wiki_log` | Recent activity log |\n")
	b.WriteString("| `wiki_graph` | Connectivity stats: hubs, clusters, density |\n")
	b.WriteString("| `wiki_export` | Export as Obsidian-compatible markdown |\n")
	b.WriteString("| `wiki_save_query` | Run a query and save the answer as a page |\n")
	b.WriteString("| `wiki_feed_*` | Ingest tool output (sqz, ghostdep, claimcheck, etch) |\n\n")

	// Quality guidelines.
	b.WriteString("## Quality Guidelines\n\n")
	b.WriteString("- **Be specific.** \"PostgreSQL 16 with pgvector\" is better than \"a database.\"\n")
	b.WriteString("- **Note contradictions.** When new info conflicts with existing pages, flag it explicitly.\n")
	b.WriteString("- **Cite sources.** Every claim should trace back to a source page.\n")
	b.WriteString("- **Keep pages focused.** One topic per page. Split if a page grows beyond ~500 words.\n")
	b.WriteString("- **Update, don't duplicate.** Check if a page exists before creating a new one.\n")
	b.WriteString("- **Maintain links.** Every page should have at least one inbound link.\n\n")

	return b.String()
}

type schemaStats struct {
	totalPages   int
	totalSources int
	categories   string
}

func (e *Engine) gatherStats() schemaStats {
	pages, _ := e.store.ListPages("")
	cats := make(map[string]int)
	for _, p := range pages {
		cats[p.Category]++
	}
	var catParts []string
	for cat, count := range cats {
		catParts = append(catParts, fmt.Sprintf("%s (%d)", cat, count))
	}
	catStr := "none"
	if len(catParts) > 0 {
		catStr = strings.Join(catParts, ", ")
	}
	return schemaStats{
		totalPages:   e.store.PageCount(),
		totalSources: e.store.SourceCount(),
		categories:   catStr,
	}
}
