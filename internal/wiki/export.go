package wiki

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ojuschugh1/aura/pkg/types"
)

// ExportResult summarises a markdown export operation.
type ExportResult struct {
	Dir        string `json:"dir"`
	PagesCount int    `json:"pages_count"`
	IndexFile  string `json:"index_file"`
	LogFile    string `json:"log_file"`
}

// ExportMarkdown writes the entire wiki to a directory of .md files with
// YAML frontmatter and [[wikilinks]], compatible with Obsidian and similar tools.
func (e *Engine) ExportMarkdown(outDir string) (*ExportResult, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, fmt.Errorf("create export dir: %w", err)
	}

	pages, err := e.store.ListPages("")
	if err != nil {
		return nil, fmt.Errorf("list pages: %w", err)
	}

	// Write each page as a markdown file with YAML frontmatter.
	for _, p := range pages {
		md := renderPageMarkdown(p)
		path := filepath.Join(outDir, p.Slug+".md")
		if err := os.WriteFile(path, []byte(md), 0o644); err != nil {
			return nil, fmt.Errorf("write page %q: %w", p.Slug, err)
		}
	}

	// Write index.md — organized by category.
	indexContent := renderIndexMarkdown(pages)
	indexPath := filepath.Join(outDir, "index.md")
	if err := os.WriteFile(indexPath, []byte(indexContent), 0o644); err != nil {
		return nil, fmt.Errorf("write index: %w", err)
	}

	// Write log.md — chronological record.
	logEntries, _ := e.store.RecentLog(500)
	logContent := renderLogMarkdown(logEntries)
	logPath := filepath.Join(outDir, "log.md")
	if err := os.WriteFile(logPath, []byte(logContent), 0o644); err != nil {
		return nil, fmt.Errorf("write log: %w", err)
	}

	_ = e.store.AppendLog("export",
		fmt.Sprintf("Exported %d pages to %s", len(pages), outDir), nil, nil)

	return &ExportResult{
		Dir:        outDir,
		PagesCount: len(pages),
		IndexFile:  indexPath,
		LogFile:    logPath,
	}, nil
}

// renderPageMarkdown produces a full markdown file with YAML frontmatter.
func renderPageMarkdown(p *types.WikiPage) string {
	var b strings.Builder

	// YAML frontmatter.
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("title: %q\n", p.Title))
	b.WriteString(fmt.Sprintf("slug: %s\n", p.Slug))
	b.WriteString(fmt.Sprintf("category: %s\n", p.Category))
	if len(p.Tags) > 0 {
		b.WriteString("tags:\n")
		for _, t := range p.Tags {
			b.WriteString(fmt.Sprintf("  - %s\n", t))
		}
	}
	if len(p.SourceIDs) > 0 {
		b.WriteString(fmt.Sprintf("source_count: %d\n", len(p.SourceIDs)))
	}
	if len(p.LinksSlugs) > 0 {
		b.WriteString(fmt.Sprintf("link_count: %d\n", len(p.LinksSlugs)))
	}
	b.WriteString(fmt.Sprintf("created: %s\n", p.CreatedAt.Format("2006-01-02")))
	b.WriteString(fmt.Sprintf("updated: %s\n", p.UpdatedAt.Format("2006-01-02")))
	b.WriteString("---\n\n")

	// Convert [[slug]] references in content to Obsidian-style wikilinks.
	content := p.Content
	for _, link := range p.LinksSlugs {
		// Ensure links appear as [[slug]] in the content.
		if !strings.Contains(content, "[["+link+"]]") {
			content += fmt.Sprintf("\n\n## Related\n\n- [[%s]]", link)
			break // only add the Related section once, then append all
		}
	}

	// If there are links not yet in the content, add a Related section.
	var missingLinks []string
	for _, link := range p.LinksSlugs {
		if !strings.Contains(content, "[["+link+"]]") {
			missingLinks = append(missingLinks, link)
		}
	}
	if len(missingLinks) > 0 {
		if !strings.Contains(content, "## Related") {
			content += "\n\n## Related\n"
		}
		for _, link := range missingLinks {
			content += fmt.Sprintf("\n- [[%s]]", link)
		}
	}

	b.WriteString(content)
	b.WriteString("\n")
	return b.String()
}

// renderIndexMarkdown produces an index.md organized by category.
func renderIndexMarkdown(pages []*types.WikiPage) string {
	var b strings.Builder
	b.WriteString("---\ntitle: Wiki Index\ncategory: index\n---\n\n")
	b.WriteString("# Wiki Index\n\n")
	b.WriteString(fmt.Sprintf("*%d pages total — generated %s*\n\n",
		len(pages), time.Now().Format("2006-01-02 15:04")))

	// Group by category.
	categories := make(map[string][]*types.WikiPage)
	var order []string
	for _, p := range pages {
		if _, exists := categories[p.Category]; !exists {
			order = append(order, p.Category)
		}
		categories[p.Category] = append(categories[p.Category], p)
	}

	for _, cat := range order {
		b.WriteString(fmt.Sprintf("## %s\n\n", strings.Title(cat)))
		for _, p := range categories[cat] {
			summary := p.Content
			if len(summary) > 80 {
				summary = summary[:80] + "…"
			}
			// Strip newlines from summary for clean display.
			summary = strings.ReplaceAll(summary, "\n", " ")
			b.WriteString(fmt.Sprintf("- [[%s]] — %s\n", p.Slug, summary))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// renderLogMarkdown produces a log.md with parseable entries.
func renderLogMarkdown(entries []*types.WikiLogEntry) string {
	var b strings.Builder
	b.WriteString("---\ntitle: Wiki Log\ncategory: log\n---\n\n")
	b.WriteString("# Wiki Log\n\n")
	b.WriteString("*Chronological record of wiki operations.*\n\n")

	// Entries are in reverse chronological order from the DB; reverse for the log file.
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		b.WriteString(fmt.Sprintf("## [%s] %s | %s\n\n",
			e.Timestamp.Format("2006-01-02"), e.Action, e.Summary))
		if len(e.PageSlugs) > 0 {
			b.WriteString("Pages affected: ")
			var links []string
			for _, s := range e.PageSlugs {
				links = append(links, "[["+s+"]]")
			}
			b.WriteString(strings.Join(links, ", "))
			b.WriteString("\n\n")
		}
	}

	return b.String()
}
