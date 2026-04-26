package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	auradb "github.com/ojuschugh1/aura/internal/db"
	"github.com/ojuschugh1/aura/internal/wiki"
	"github.com/spf13/cobra"
)

// NewWikiCmd returns the `aura wiki` command with all subcommands.
func NewWikiCmd(auraDir *string, jsonOut *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wiki",
		Short: "LLM-maintained knowledge base — ingest, query, lint, browse",
		Long: `The wiki is a persistent, compounding knowledge base maintained by your AI tools.
Sources are ingested once and compiled into interlinked pages. Knowledge accumulates
instead of being re-derived on every query.`,
	}
	cmd.AddCommand(newWikiIngestCmd(auraDir, jsonOut))
	cmd.AddCommand(newWikiQueryCmd(auraDir, jsonOut))
	cmd.AddCommand(newWikiLintCmd(auraDir, jsonOut))
	cmd.AddCommand(newWikiLsCmd(auraDir, jsonOut))
	cmd.AddCommand(newWikiShowCmd(auraDir, jsonOut))
	cmd.AddCommand(newWikiSearchCmd(auraDir, jsonOut))
	cmd.AddCommand(newWikiLogCmd(auraDir, jsonOut))
	cmd.AddCommand(newWikiIndexCmd(auraDir, jsonOut))
	cmd.AddCommand(newWikiRmCmd(auraDir))
	cmd.AddCommand(newWikiSourcesCmd(auraDir, jsonOut))
	cmd.AddCommand(newWikiExportCmd(auraDir, jsonOut))
	cmd.AddCommand(newWikiGraphCmd(auraDir, jsonOut))
	cmd.AddCommand(newWikiFeedCmd(auraDir, jsonOut))
	cmd.AddCommand(newWikiSchemaCmd(auraDir))
	cmd.AddCommand(newWikiFilterCmd(auraDir, jsonOut))
	return cmd
}

func openWikiEngine(auraDir string) (*wiki.Engine, func(), error) {
	dbPath := filepath.Join(auraDir, "aura.db")
	database, err := auradb.Open(dbPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open db: %w", err)
	}
	if err := auradb.RunMigrations(database); err != nil {
		database.Close()
		return nil, nil, fmt.Errorf("migrate db: %w", err)
	}
	store := wiki.NewStore(database)
	engine := wiki.NewEngine(store)
	return engine, func() { database.Close() }, nil
}

func newWikiIngestCmd(auraDir *string, jsonOut *bool) *cobra.Command {
	var title, format, dir string
	cmd := &cobra.Command{
		Use:   "ingest [file-or-text]",
		Short: "Ingest a source into the wiki (file path, inline text, or --dir for batch)",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			engine, close, err := openWikiEngine(*auraDir)
			if err != nil {
				return err
			}
			defer close()

			// Batch ingest mode.
			if dir != "" {
				result, err := engine.BatchIngest(dir)
				if err != nil {
					return err
				}
				if *jsonOut {
					return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "batch ingest from %s:\n", dir)
				fmt.Fprintf(cmd.OutOrStdout(), "  total:      %d files\n", result.Total)
				fmt.Fprintf(cmd.OutOrStdout(), "  ingested:   %d\n", result.Ingested)
				fmt.Fprintf(cmd.OutOrStdout(), "  duplicates: %d\n", result.Duplicates)
				fmt.Fprintf(cmd.OutOrStdout(), "  errors:     %d\n", result.Errors)
				for _, e := range result.ErrorDetails {
					fmt.Fprintf(cmd.OutOrStdout(), "  error: %s\n", e)
				}
				return nil
			}

			if len(args) == 0 {
				return fmt.Errorf("provide a file path, URL, inline text, or use --dir for batch ingest")
			}

			input := strings.Join(args, " ")

			// Check if input is a URL.
			if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
				result, err := engine.FetchAndIngest(input, title)
				if err != nil {
					return err
				}
				if *jsonOut {
					return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
				}
				if result.Duplicate {
					fmt.Fprintf(cmd.OutOrStdout(), "duplicate: source already ingested (id %d)\n", result.SourceID)
					return nil
				}
				fmt.Fprintf(cmd.OutOrStdout(), "ingested: %s (source #%d)\n", result.SourceTitle, result.SourceID)
				if len(result.PagesCreated) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "created:  %s\n", strings.Join(result.PagesCreated, ", "))
				}
				if len(result.PagesUpdated) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "updated:  %s\n", strings.Join(result.PagesUpdated, ", "))
				}
				return nil
			}

			origin := ""
			content := input

			// Check if input is a file path.
			if info, err := os.Stat(input); err == nil && !info.IsDir() {
				data, err := os.ReadFile(input)
				if err != nil {
					return fmt.Errorf("read file: %w", err)
				}
				content = string(data)
				origin = input
				if title == "" {
					title = filepath.Base(input)
				}
				if format == "" {
					ext := strings.ToLower(filepath.Ext(input))
					switch ext {
					case ".md", ".markdown":
						format = "markdown"
					case ".txt":
						format = "text"
					case ".jsonl":
						format = "jsonl"
					default:
						format = "text"
					}
				}
			} else {
				if title == "" {
					// Use first 50 chars as title.
					title = content
					if len(title) > 50 {
						title = title[:50] + "…"
					}
				}
				if format == "" {
					format = "text"
				}
			}

			result, err := engine.Ingest(title, content, format, origin)
			if err != nil {
				return err
			}

			if *jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
			}

			if result.Duplicate {
				fmt.Fprintf(cmd.OutOrStdout(), "duplicate: source already ingested (id %d)\n", result.SourceID)
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "ingested: %s (source #%d)\n", result.SourceTitle, result.SourceID)
			if len(result.PagesCreated) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "created:  %s\n", strings.Join(result.PagesCreated, ", "))
			}
			if len(result.PagesUpdated) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "updated:  %s\n", strings.Join(result.PagesUpdated, ", "))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "Source title (default: filename or first 50 chars)")
	cmd.Flags().StringVar(&format, "format", "", "Source format: markdown, text, jsonl (auto-detected)")
	cmd.Flags().StringVar(&dir, "dir", "", "Batch ingest all supported files from a directory")
	return cmd
}

func newWikiQueryCmd(auraDir *string, jsonOut *bool) *cobra.Command {
	var save bool
	cmd := &cobra.Command{
		Use:   "query <search-terms>",
		Short: "Search the wiki and get a synthesised answer",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			engine, close, err := openWikiEngine(*auraDir)
			if err != nil {
				return err
			}
			defer close()

			query := strings.Join(args, " ")
			result, err := engine.Query(query)
			if err != nil {
				return err
			}

			// File the answer back as a wiki page if --save is set.
			if save && result.PageCount > 0 {
				slug, err := engine.SaveQueryResult(result)
				if err != nil {
					return fmt.Errorf("save query result: %w", err)
				}
				result.SavedSlug = slug
			}

			if *jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
			}

			if result.PageCount == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no pages found matching %q\n", query)
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "found %d page(s):\n\n", result.PageCount)
			fmt.Fprintln(cmd.OutOrStdout(), result.Answer)

			if result.SavedSlug != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "\nsaved as wiki page: %s\n", result.SavedSlug)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&save, "save", false, "File the answer back into the wiki as a synthesis page")
	return cmd
}

func newWikiLintCmd(auraDir *string, jsonOut *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "lint",
		Short: "Health-check the wiki for orphans, stale pages, and missing references",
		RunE: func(cmd *cobra.Command, args []string) error {
			engine, close, err := openWikiEngine(*auraDir)
			if err != nil {
				return err
			}
			defer close()

			result, err := engine.Lint()
			if err != nil {
				return err
			}

			if *jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "pages:   %d\n", result.TotalPages)
			fmt.Fprintf(cmd.OutOrStdout(), "sources: %d\n", result.TotalSources)
			fmt.Fprintf(cmd.OutOrStdout(), "health:  %.0f%%\n", result.HealthScore*100)
			fmt.Fprintln(cmd.OutOrStdout())

			if len(result.Orphans) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "orphans (%d):\n", len(result.Orphans))
				for _, o := range result.Orphans {
					fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", o)
				}
			}
			if len(result.Stale) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "stale (%d):\n", len(result.Stale))
				for _, s := range result.Stale {
					fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", s)
				}
			}
			if len(result.MissingPages) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "missing references (%d):\n", len(result.MissingPages))
				for _, m := range result.MissingPages {
					fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", m)
				}
			}
			if len(result.Contradictions) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "contradictions (%d):\n", len(result.Contradictions))
				for _, c := range result.Contradictions {
					fmt.Fprintf(cmd.OutOrStdout(), "  - %s vs %s: %s\n", c.PageA, c.PageB, c.Snippet)
				}
			}
			if len(result.Orphans) == 0 && len(result.Stale) == 0 && len(result.MissingPages) == 0 && len(result.Contradictions) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no issues found")
			}

			if len(result.Suggestions) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "\nsuggestions (%d):\n", len(result.Suggestions))
				for _, s := range result.Suggestions {
					fmt.Fprintf(cmd.OutOrStdout(), "  [%s] %s\n", s.Type, s.Message)
				}
			}
			return nil
		},
	}
}

func newWikiLsCmd(auraDir *string, jsonOut *bool) *cobra.Command {
	var category string
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List all wiki pages",
		RunE: func(cmd *cobra.Command, args []string) error {
			engine, close, err := openWikiEngine(*auraDir)
			if err != nil {
				return err
			}
			defer close()

			pages, err := engine.Store().ListPages(category)
			if err != nil {
				return err
			}

			if *jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(pages)
			}

			if len(pages) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no wiki pages")
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-12s %-20s %s\n", "SLUG", "CATEGORY", "UPDATED", "TITLE")
			for _, p := range pages {
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-12s %-20s %s\n",
					truncateStr(p.Slug, 30), p.Category,
					p.UpdatedAt.Format("2006-01-02 15:04"), p.Title)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&category, "category", "", "Filter by category (entity, concept, source, synthesis)")
	return cmd
}

func newWikiShowCmd(auraDir *string, jsonOut *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "show <slug>",
		Short: "Show the full content of a wiki page",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			engine, close, err := openWikiEngine(*auraDir)
			if err != nil {
				return err
			}
			defer close()

			page, err := engine.Store().GetPage(args[0])
			if err != nil {
				return fmt.Errorf("page %q not found", args[0])
			}

			if *jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(page)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "slug:     %s\n", page.Slug)
			fmt.Fprintf(cmd.OutOrStdout(), "title:    %s\n", page.Title)
			fmt.Fprintf(cmd.OutOrStdout(), "category: %s\n", page.Category)
			if len(page.Tags) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "tags:     %s\n", strings.Join(page.Tags, ", "))
			}
			if len(page.LinksSlugs) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "links:    %s\n", strings.Join(page.LinksSlugs, ", "))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "updated:  %s\n", page.UpdatedAt.Format("2006-01-02 15:04:05"))
			fmt.Fprintln(cmd.OutOrStdout())
			fmt.Fprintln(cmd.OutOrStdout(), page.Content)
			return nil
		},
	}
}

func newWikiSearchCmd(auraDir *string, jsonOut *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "search <query>",
		Short: "Search wiki pages by title and content",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			engine, close, err := openWikiEngine(*auraDir)
			if err != nil {
				return err
			}
			defer close()

			query := strings.Join(args, " ")
			pages, err := engine.Store().SearchPages(query)
			if err != nil {
				return err
			}

			if *jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(pages)
			}

			if len(pages) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no pages matching %q\n", query)
				return nil
			}

			for _, p := range pages {
				excerpt := p.Content
				if len(excerpt) > 80 {
					excerpt = excerpt[:80] + "…"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-25s [%s] %s\n", p.Slug, p.Category, excerpt)
			}
			return nil
		},
	}
}

func newWikiLogCmd(auraDir *string, jsonOut *bool) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "log",
		Short: "Show the wiki activity log",
		RunE: func(cmd *cobra.Command, args []string) error {
			engine, close, err := openWikiEngine(*auraDir)
			if err != nil {
				return err
			}
			defer close()

			entries, err := engine.Store().RecentLog(limit)
			if err != nil {
				return err
			}

			if *jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(entries)
			}

			if len(entries) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no log entries")
				return nil
			}

			for _, e := range entries {
				fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s — %s\n",
					e.Timestamp.Format("2006-01-02 15:04"), e.Action, e.Summary)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Number of log entries to show")
	return cmd
}

func newWikiIndexCmd(auraDir *string, jsonOut *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "index",
		Short: "Show the wiki index (catalog of all pages)",
		RunE: func(cmd *cobra.Command, args []string) error {
			engine, close, err := openWikiEngine(*auraDir)
			if err != nil {
				return err
			}
			defer close()

			idx, err := engine.Store().BuildIndex()
			if err != nil {
				return err
			}

			if *jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(idx)
			}

			if len(idx.Entries) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "wiki is empty")
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-25s %-12s %5s %s\n", "SLUG", "CATEGORY", "LINKS", "SUMMARY")
			for _, e := range idx.Entries {
				summary := e.Summary
				if len(summary) > 50 {
					summary = summary[:50] + "…"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-25s %-12s %5d %s\n",
					truncateStr(e.Slug, 25), e.Category, e.Links, summary)
			}
			return nil
		},
	}
}

func newWikiRmCmd(auraDir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "rm <slug>",
		Short: "Delete a wiki page by slug",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			engine, close, err := openWikiEngine(*auraDir)
			if err != nil {
				return err
			}
			defer close()

			if err := engine.Store().DeletePage(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted: %s\n", args[0])
			return nil
		},
	}
}

func newWikiSourcesCmd(auraDir *string, jsonOut *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "sources",
		Short: "List all ingested raw sources",
		RunE: func(cmd *cobra.Command, args []string) error {
			engine, close, err := openWikiEngine(*auraDir)
			if err != nil {
				return err
			}
			defer close()

			sources, err := engine.Store().ListSources()
			if err != nil {
				return err
			}

			if *jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(sources)
			}

			if len(sources) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no sources ingested")
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%5s %-30s %-10s %-20s %s\n", "ID", "TITLE", "FORMAT", "INGESTED", "ORIGIN")
			for _, s := range sources {
				title := s.Title
				if len(title) > 30 {
					title = title[:27] + "…"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%5d %-30s %-10s %-20s %s\n",
					s.ID, title, s.Format,
					s.IngestedAt.Format("2006-01-02 15:04"), s.Origin)
			}
			return nil
		},
	}
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}

func newWikiExportCmd(auraDir *string, jsonOut *bool) *cobra.Command {
	var outDir string
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export the wiki as markdown files with YAML frontmatter (Obsidian-compatible)",
		RunE: func(cmd *cobra.Command, args []string) error {
			engine, close, err := openWikiEngine(*auraDir)
			if err != nil {
				return err
			}
			defer close()

			if outDir == "" {
				outDir = filepath.Join(*auraDir, "wiki-export")
			}

			result, err := engine.ExportMarkdown(outDir)
			if err != nil {
				return err
			}

			if *jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "exported %d pages to %s\n", result.PagesCount, result.Dir)
			fmt.Fprintf(cmd.OutOrStdout(), "  index: %s\n", result.IndexFile)
			fmt.Fprintf(cmd.OutOrStdout(), "  log:   %s\n", result.LogFile)
			fmt.Fprintln(cmd.OutOrStdout(), "\nopen in Obsidian to browse with graph view and Dataview queries")
			return nil
		},
	}
	cmd.Flags().StringVar(&outDir, "out", "", "Output directory (default: .aura/wiki-export)")
	return cmd
}

func newWikiGraphCmd(auraDir *string, jsonOut *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "graph",
		Short: "Show wiki connectivity stats — hubs, clusters, density",
		RunE: func(cmd *cobra.Command, args []string) error {
			engine, close, err := openWikiEngine(*auraDir)
			if err != nil {
				return err
			}
			defer close()

			stats, err := engine.Graph()
			if err != nil {
				return err
			}

			if *jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(stats)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "pages:    %d\n", stats.TotalPages)
			fmt.Fprintf(cmd.OutOrStdout(), "edges:    %d\n", stats.TotalEdges)
			fmt.Fprintf(cmd.OutOrStdout(), "density:  %.3f\n", stats.Density)
			fmt.Fprintf(cmd.OutOrStdout(), "avg links: %.1f\n", stats.AvgLinks)
			fmt.Fprintf(cmd.OutOrStdout(), "clusters: %d\n", len(stats.Clusters))
			fmt.Fprintln(cmd.OutOrStdout())

			// Categories.
			if len(stats.Categories) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "categories:")
				for cat, count := range stats.Categories {
					fmt.Fprintf(cmd.OutOrStdout(), "  %-15s %d\n", cat, count)
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}

			// Top hubs.
			if len(stats.Hubs) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "top hubs:")
				fmt.Fprintf(cmd.OutOrStdout(), "  %-30s %5s %5s %5s\n", "SLUG", "IN", "OUT", "TOTAL")
				for _, h := range stats.Hubs {
					fmt.Fprintf(cmd.OutOrStdout(), "  %-30s %5d %5d %5d\n",
						truncateStr(h.Slug, 30), h.Inbound, h.Outbound, h.Total)
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}

			// Clusters.
			if len(stats.Clusters) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "clusters:")
				for _, c := range stats.Clusters {
					preview := strings.Join(c.Pages, ", ")
					if len(preview) > 60 {
						preview = preview[:60] + "…"
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  #%d (%d pages): %s\n", c.ID, c.Size, preview)
				}
			}

			// Orphans.
			if len(stats.Orphans) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "\nisolated pages (%d):\n", len(stats.Orphans))
				for _, o := range stats.Orphans {
					fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", o)
				}
			}

			return nil
		},
	}
}

func newWikiFeedCmd(auraDir *string, jsonOut *bool) *cobra.Command {
	var tool string
	cmd := &cobra.Command{
		Use:   "feed [file]",
		Short: "Feed tool output into the wiki (sqz, ghostdep, claimcheck, etch, or generic JSON)",
		Long: `Ingest structured output from Aura's companion tools into the wiki.
Each tool has a dedicated adapter that creates well-structured pages
with cross-references and cumulative history.

Supported tools: sqz, ghostdep, claimcheck, etch
For other tools, use --tool <name> to ingest generic JSON.

Input can be a file path or piped via stdin.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			engine, close, err := openWikiEngine(*auraDir)
			if err != nil {
				return err
			}
			defer close()

			// Read input from file or stdin.
			var data []byte
			if len(args) > 0 {
				data, err = os.ReadFile(args[0])
				if err != nil {
					return fmt.Errorf("read file: %w", err)
				}
			} else {
				stat, _ := os.Stdin.Stat()
				if stat.Mode()&os.ModeCharDevice != 0 {
					return fmt.Errorf("provide a file path or pipe JSON via stdin\n\nExamples:\n  aura wiki feed scan-results.json --tool ghostdep\n  aura scan --json | aura wiki feed --tool ghostdep\n  aura verify --json | aura wiki feed --tool claimcheck")
				}
				data, err = readAllStdin()
				if err != nil {
					return fmt.Errorf("read stdin: %w", err)
				}
			}

			var result *wiki.ToolIngestResult

			switch strings.ToLower(tool) {
			case "sqz":
				var report wiki.SQZReport
				if err := json.Unmarshal(data, &report); err != nil {
					return fmt.Errorf("parse sqz report: %w", err)
				}
				result, err = engine.IngestSQZ(report)

			case "ghostdep":
				var report wiki.GhostDepReport
				if err := json.Unmarshal(data, &report); err != nil {
					return fmt.Errorf("parse ghostdep report: %w", err)
				}
				result, err = engine.IngestGhostDep(report)

			case "claimcheck":
				var report wiki.ClaimCheckReport
				if err := json.Unmarshal(data, &report); err != nil {
					return fmt.Errorf("parse claimcheck report: %w", err)
				}
				result, err = engine.IngestClaimCheck(report)

			case "etch":
				var report wiki.EtchReport
				if err := json.Unmarshal(data, &report); err != nil {
					return fmt.Errorf("parse etch report: %w", err)
				}
				result, err = engine.IngestEtch(report)

			default:
				if tool == "" {
					tool = "unknown"
				}
				result, err = engine.IngestToolJSON(tool, data)
			}

			if err != nil {
				return err
			}

			if *jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s\n", result.Tool, result.Summary)
			if len(result.PagesCreated) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "created: %s\n", strings.Join(result.PagesCreated, ", "))
			}
			if len(result.PagesUpdated) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "updated: %s\n", strings.Join(result.PagesUpdated, ", "))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&tool, "tool", "", "Tool name: sqz, ghostdep, claimcheck, etch (auto-detected if omitted)")
	return cmd
}

func newWikiSchemaCmd(auraDir *string) *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Generate a schema file (CLAUDE.md / AGENTS.md / .kiro steering) for LLM wiki maintenance",
		Long: `Generate a schema document that teaches an LLM how to maintain the wiki.
This is the "third layer" from Karpathy's LLM Wiki pattern — the config
that turns a generic chatbot into a disciplined wiki maintainer.

Output the schema to stdout, or redirect to a file:
  aura wiki schema --format claude > CLAUDE.md
  aura wiki schema --format kiro > .kiro/steering/wiki.md
  aura wiki schema --format codex > AGENTS.md`,
		RunE: func(cmd *cobra.Command, args []string) error {
			engine, close, err := openWikiEngine(*auraDir)
			if err != nil {
				return err
			}
			defer close()

			var schemaFormat wiki.SchemaFormat
			switch strings.ToLower(format) {
			case "claude":
				schemaFormat = wiki.SchemaClaudeCode
			case "cursor":
				schemaFormat = wiki.SchemaCursor
			case "kiro":
				schemaFormat = wiki.SchemaKiro
			case "codex":
				schemaFormat = wiki.SchemaCodex
			default:
				schemaFormat = wiki.SchemaGeneric
			}

			schema := engine.GenerateSchema(schemaFormat)
			fmt.Fprint(cmd.OutOrStdout(), schema)
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "generic", "Target format: claude, cursor, kiro, codex, generic")
	return cmd
}

func newWikiFilterCmd(auraDir *string, jsonOut *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "filter <expression>",
		Short: "Query pages by metadata (category, tags, dates, counts)",
		Long: `Structured queries over page metadata, like Dataview but from the CLI.

Syntax: field=value, field!=value, field>N, field<N, field contains value
Fields: category, slug, title, tags, source_count, link_count, created, updated
Join multiple filters with AND.

Examples:
  aura wiki filter "category=entity"
  aura wiki filter "tags contains api"
  aura wiki filter "category=entity AND link_count>3"
  aura wiki filter "updated>2026-04-01 AND category=source"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			engine, close, err := openWikiEngine(*auraDir)
			if err != nil {
				return err
			}
			defer close()

			filters, err := wiki.ParseFilters(args[0])
			if err != nil {
				return err
			}

			pages, err := engine.Store().ListPages("")
			if err != nil {
				return err
			}

			matched, err := wiki.FilterPages(pages, filters)
			if err != nil {
				return err
			}

			if *jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(matched)
			}

			if len(matched) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no pages matching %q\n", args[0])
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%d page(s) matching %q:\n\n", len(matched), args[0])
			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-12s %-20s %s\n", "SLUG", "CATEGORY", "UPDATED", "TITLE")
			for _, p := range matched {
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-12s %-20s %s\n",
					truncateStr(p.Slug, 30), p.Category,
					p.UpdatedAt.Format("2006-01-02 15:04"), p.Title)
			}
			return nil
		},
	}
}

func readAllStdin() ([]byte, error) {
	var buf strings.Builder
	scanner := make([]byte, 4096)
	for {
		n, err := os.Stdin.Read(scanner)
		if n > 0 {
			buf.Write(scanner[:n])
		}
		if err != nil {
			break
		}
	}
	return []byte(buf.String()), nil
}
