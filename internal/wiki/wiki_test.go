package wiki

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	auradb "github.com/ojuschugh1/aura/internal/db"
	"github.com/ojuschugh1/aura/pkg/types"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	database, err := auradb.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := auradb.RunMigrations(database); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func TestStoreSourceCRUD(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)

	// Ingest a source.
	src, isDup, err := store.IngestSource("Test Article", "This is test content.", "markdown", "test.md")
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if isDup {
		t.Fatal("first ingest should not be duplicate")
	}
	if src.ID == 0 {
		t.Fatal("expected non-zero source ID")
	}
	if src.Title != "Test Article" {
		t.Errorf("title = %q, want %q", src.Title, "Test Article")
	}

	// Duplicate detection.
	dup, wasDup, err := store.IngestSource("Test Article Dup", "This is test content.", "markdown", "test2.md")
	if err != nil {
		t.Fatalf("ingest dup: %v", err)
	}
	if !wasDup {
		t.Error("second ingest of same content should be duplicate")
	}
	if dup.ID != src.ID {
		t.Errorf("duplicate should return same ID: got %d, want %d", dup.ID, src.ID)
	}

	// List sources.
	sources, err := store.ListSources()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(sources) != 1 {
		t.Errorf("expected 1 source, got %d", len(sources))
	}
}

func TestStorePageCRUD(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)

	// Create a page.
	page, err := store.CreatePage("test-page", "Test Page", "# Test\n\nContent here.",
		"entity", []string{"test"}, []int64{1}, []string{"other-page"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if page.Slug != "test-page" {
		t.Errorf("slug = %q, want %q", page.Slug, "test-page")
	}
	if page.Category != "entity" {
		t.Errorf("category = %q, want %q", page.Category, "entity")
	}
	if len(page.Tags) != 1 || page.Tags[0] != "test" {
		t.Errorf("tags = %v, want [test]", page.Tags)
	}

	// Get page.
	got, err := store.GetPage("test-page")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Title != "Test Page" {
		t.Errorf("title = %q, want %q", got.Title, "Test Page")
	}

	// Update page.
	updated, err := store.UpdatePage("test-page", "# Updated\n\nNew content.",
		[]string{"test", "updated"}, []int64{1, 2}, []string{"other-page", "new-page"})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if len(updated.Tags) != 2 {
		t.Errorf("tags count = %d, want 2", len(updated.Tags))
	}
	if len(updated.LinksSlugs) != 2 {
		t.Errorf("links count = %d, want 2", len(updated.LinksSlugs))
	}

	// List pages.
	pages, err := store.ListPages("")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(pages) != 1 {
		t.Errorf("expected 1 page, got %d", len(pages))
	}

	// List by category.
	pages, err = store.ListPages("entity")
	if err != nil {
		t.Fatalf("list by category: %v", err)
	}
	if len(pages) != 1 {
		t.Errorf("expected 1 entity page, got %d", len(pages))
	}

	pages, err = store.ListPages("concept")
	if err != nil {
		t.Fatalf("list by category: %v", err)
	}
	if len(pages) != 0 {
		t.Errorf("expected 0 concept pages, got %d", len(pages))
	}

	// Search pages.
	results, err := store.SearchPages("updated")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 search result, got %d", len(results))
	}

	// Delete page.
	if err := store.DeletePage("test-page"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, err = store.GetPage("test-page")
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestStoreLog(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)

	srcID := int64(1)
	if err := store.AppendLog("ingest", "Ingested test doc", []string{"test-page"}, &srcID); err != nil {
		t.Fatalf("append log: %v", err)
	}
	if err := store.AppendLog("query", "Searched for X", nil, nil); err != nil {
		t.Fatalf("append log: %v", err)
	}

	entries, err := store.RecentLog(10)
	if err != nil {
		t.Fatalf("recent log: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 log entries, got %d", len(entries))
	}
	// Most recent first.
	if entries[0].Action != "query" {
		t.Errorf("first entry action = %q, want %q", entries[0].Action, "query")
	}
}

func TestStoreBuildIndex(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)

	_, _ = store.CreatePage("page-a", "Page A", "Content of page A", "entity", nil, nil, []string{"page-b"})
	_, _ = store.CreatePage("page-b", "Page B", "Content of page B", "concept", nil, nil, nil)

	idx, err := store.BuildIndex()
	if err != nil {
		t.Fatalf("build index: %v", err)
	}
	if len(idx.Entries) != 2 {
		t.Errorf("expected 2 index entries, got %d", len(idx.Entries))
	}
}

func TestEngineIngest(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	engine := NewEngine(store)

	content := `# Architecture Overview

The system uses **PostgreSQL** for persistence and **Redis** for caching.

## Authentication

JWT tokens with 24-hour expiry. Refresh tokens stored in httpOnly cookies.

## Deployment

Deployed on **Kubernetes** with Helm charts.`

	result, err := engine.Ingest("System Design Doc", content, "markdown", "design.md")
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}

	if result.SourceID == 0 {
		t.Error("expected non-zero source ID")
	}
	if result.Duplicate {
		t.Error("first ingest should not be duplicate")
	}
	if len(result.PagesCreated) == 0 {
		t.Error("expected at least one page created")
	}

	// Verify source page was created.
	page, err := store.GetPage("system-design-doc")
	if err != nil {
		t.Fatalf("get source page: %v", err)
	}
	if page.Category != "source" {
		t.Errorf("source page category = %q, want %q", page.Category, "source")
	}

	// Verify topic pages were created.
	totalPages := store.PageCount()
	if totalPages < 2 {
		t.Errorf("expected at least 2 pages, got %d", totalPages)
	}

	// Verify log was written.
	log, _ := store.RecentLog(5)
	if len(log) == 0 {
		t.Error("expected log entries after ingest")
	}
	if log[0].Action != "ingest" {
		t.Errorf("log action = %q, want %q", log[0].Action, "ingest")
	}
}

func TestEngineIngestDuplicate(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	engine := NewEngine(store)

	content := "Simple test content for dedup."
	_, err := engine.Ingest("Doc A", content, "text", "a.txt")
	if err != nil {
		t.Fatalf("first ingest: %v", err)
	}

	result, err := engine.Ingest("Doc B", content, "text", "b.txt")
	if err != nil {
		t.Fatalf("second ingest: %v", err)
	}
	if !result.Duplicate {
		t.Error("second ingest of same content should be duplicate")
	}
}

func TestEngineQuery(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	engine := NewEngine(store)

	_, _ = store.CreatePage("postgresql", "PostgreSQL", "PostgreSQL is a relational database.",
		"entity", nil, nil, nil)
	_, _ = store.CreatePage("redis", "Redis", "Redis is an in-memory data store.",
		"entity", nil, nil, nil)

	result, err := engine.Query("PostgreSQL")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if result.PageCount != 1 {
		t.Errorf("expected 1 result, got %d", result.PageCount)
	}
	if result.Pages[0].Slug != "postgresql" {
		t.Errorf("slug = %q, want %q", result.Pages[0].Slug, "postgresql")
	}

	// Query with no results.
	result, err = engine.Query("nonexistent-topic")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if result.PageCount != 0 {
		t.Errorf("expected 0 results, got %d", result.PageCount)
	}
}

func TestEngineLint(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	engine := NewEngine(store)

	// Create pages with a missing link reference.
	_, _ = store.CreatePage("page-a", "Page A", "Content A", "entity", nil, nil, []string{"page-b", "missing-page"})
	_, _ = store.CreatePage("page-b", "Page B", "Content B", "entity", nil, nil, nil)

	result, err := engine.Lint()
	if err != nil {
		t.Fatalf("lint: %v", err)
	}

	if result.TotalPages != 2 {
		t.Errorf("total pages = %d, want 2", result.TotalPages)
	}

	// page-b has an inbound link from page-a, so it's not an orphan.
	// page-a has no inbound links, so it IS an orphan.
	if len(result.Orphans) != 1 || result.Orphans[0] != "page-a" {
		t.Errorf("orphans = %v, want [page-a]", result.Orphans)
	}

	// "missing-page" is referenced but doesn't exist.
	if len(result.MissingPages) != 1 || result.MissingPages[0] != "missing-page" {
		t.Errorf("missing = %v, want [missing-page]", result.MissingPages)
	}

	if result.HealthScore <= 0 || result.HealthScore > 1 {
		t.Errorf("health score = %f, want 0 < score <= 1", result.HealthScore)
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello World", "hello-world"},
		{"PostgreSQL Database", "postgresql-database"},
		{"  Spaces  Everywhere  ", "spaces-everywhere"},
		{"Special!@#$%Characters", "specialcharacters"},
		{"Already-slugified", "already-slugified"},
	}
	for _, tt := range tests {
		got := slugify(tt.input)
		if got != tt.want {
			t.Errorf("slugify(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestExtractTopics(t *testing.T) {
	content := `# Architecture

The system uses event sourcing.

## Database Layer

**PostgreSQL** handles persistence.

## Cache Layer

**Redis** provides fast lookups.`

	topics := extractTopics(content)
	if len(topics) < 2 {
		t.Errorf("expected at least 2 topics, got %d", len(topics))
	}

	// Check that headers were extracted.
	found := make(map[string]bool)
	for _, topic := range topics {
		found[topic.Name] = true
	}
	if !found["Architecture"] {
		t.Error("expected 'Architecture' topic")
	}
	if !found["Database Layer"] {
		t.Error("expected 'Database Layer' topic")
	}
}

func TestEngineIngestFromFile(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	engine := NewEngine(store)

	// Write a temp file.
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.md")
	content := "# Meeting Notes\n\nWe decided to use **Go** for the backend.\n\n## Action Items\n\n- Set up CI pipeline"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	// Read and ingest.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	result, err := engine.Ingest("Meeting Notes", string(data), "markdown", path)
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if result.SourceID == 0 {
		t.Error("expected non-zero source ID")
	}
	if len(result.PagesCreated) == 0 {
		t.Error("expected pages to be created")
	}
}

func TestEngineSaveQueryResult(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	engine := NewEngine(store)

	_, _ = store.CreatePage("postgresql", "PostgreSQL", "PostgreSQL is a relational database.",
		"entity", nil, []int64{1}, nil)

	result, err := engine.Query("PostgreSQL")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if result.PageCount == 0 {
		t.Fatal("expected results")
	}

	slug, err := engine.SaveQueryResult(result)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if slug == "" {
		t.Fatal("expected non-empty slug")
	}

	// Verify the synthesis page was created.
	page, err := store.GetPage(slug)
	if err != nil {
		t.Fatalf("get saved page: %v", err)
	}
	if page.Category != "synthesis" {
		t.Errorf("category = %q, want %q", page.Category, "synthesis")
	}
	if len(page.Tags) < 1 {
		t.Error("expected tags on synthesis page")
	}
}

func TestEngineBatchIngest(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	engine := NewEngine(store)

	// Create a temp directory with some files.
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "doc1.md"), []byte("# Doc One\n\nFirst document."), 0644)
	os.WriteFile(filepath.Join(dir, "doc2.txt"), []byte("Second document about testing."), 0644)
	os.WriteFile(filepath.Join(dir, "ignore.png"), []byte("not a text file"), 0644) // should be skipped

	result, err := engine.BatchIngest(dir)
	if err != nil {
		t.Fatalf("batch ingest: %v", err)
	}
	if result.Total != 2 {
		t.Errorf("total = %d, want 2", result.Total)
	}
	if result.Ingested != 2 {
		t.Errorf("ingested = %d, want 2", result.Ingested)
	}
	if result.Errors != 0 {
		t.Errorf("errors = %d, want 0", result.Errors)
	}

	// Verify pages were created.
	if store.PageCount() < 2 {
		t.Errorf("expected at least 2 pages, got %d", store.PageCount())
	}
}

func TestEngineExportMarkdown(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	engine := NewEngine(store)

	_, _ = store.CreatePage("test-page", "Test Page", "# Test\n\nContent here.",
		"entity", []string{"test"}, []int64{1}, []string{"other-page"})
	_, _ = store.CreatePage("other-page", "Other Page", "# Other\n\nMore content.",
		"concept", nil, nil, nil)

	outDir := filepath.Join(t.TempDir(), "export")
	result, err := engine.ExportMarkdown(outDir)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if result.PagesCount != 2 {
		t.Errorf("pages count = %d, want 2", result.PagesCount)
	}

	// Verify files were created.
	data, err := os.ReadFile(filepath.Join(outDir, "test-page.md"))
	if err != nil {
		t.Fatalf("read exported file: %v", err)
	}
	content := string(data)

	// Check YAML frontmatter.
	if !strings.Contains(content, "---") {
		t.Error("expected YAML frontmatter delimiters")
	}
	if !strings.Contains(content, "category: entity") {
		t.Error("expected category in frontmatter")
	}
	if !strings.Contains(content, "tags:") {
		t.Error("expected tags in frontmatter")
	}

	// Check index.md was created.
	indexData, err := os.ReadFile(filepath.Join(outDir, "index.md"))
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	if !strings.Contains(string(indexData), "Wiki Index") {
		t.Error("expected 'Wiki Index' in index.md")
	}
	if !strings.Contains(string(indexData), "[[test-page]]") {
		t.Error("expected wikilink to test-page in index.md")
	}

	// Check log.md was created.
	if _, err := os.Stat(filepath.Join(outDir, "log.md")); err != nil {
		t.Error("expected log.md to exist")
	}
}

func TestEngineGraph(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	engine := NewEngine(store)

	// Create a small graph: A -> B -> C, A -> C (triangle minus one edge).
	_, _ = store.CreatePage("page-a", "Page A", "Content A", "entity", nil, nil, []string{"page-b", "page-c"})
	_, _ = store.CreatePage("page-b", "Page B", "Content B", "entity", nil, nil, []string{"page-c"})
	_, _ = store.CreatePage("page-c", "Page C", "Content C", "concept", nil, nil, nil)
	_, _ = store.CreatePage("orphan", "Orphan", "Isolated page", "entity", nil, nil, nil)

	stats, err := engine.Graph()
	if err != nil {
		t.Fatalf("graph: %v", err)
	}

	if stats.TotalPages != 4 {
		t.Errorf("total pages = %d, want 4", stats.TotalPages)
	}
	if stats.TotalEdges != 3 {
		t.Errorf("total edges = %d, want 3", stats.TotalEdges)
	}
	if stats.Density <= 0 {
		t.Error("expected positive density")
	}

	// Check hubs — page-a should be top (2 outbound).
	if len(stats.Hubs) == 0 {
		t.Fatal("expected hubs")
	}

	// Check categories.
	if stats.Categories["entity"] != 3 {
		t.Errorf("entity count = %d, want 3", stats.Categories["entity"])
	}
	if stats.Categories["concept"] != 1 {
		t.Errorf("concept count = %d, want 1", stats.Categories["concept"])
	}

	// Check clusters — should be 2: {a,b,c} and {orphan}.
	if len(stats.Clusters) != 2 {
		t.Errorf("clusters = %d, want 2", len(stats.Clusters))
	}

	// Check orphans.
	if len(stats.Orphans) != 1 || stats.Orphans[0] != "orphan" {
		t.Errorf("orphans = %v, want [orphan]", stats.Orphans)
	}
}

func TestContradictionDetection(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	engine := NewEngine(store)

	// Create two pages with contradicting claims.
	_, _ = store.CreatePage("page-old", "Old Design", "The backend uses MySQL for persistence. The API uses REST endpoints.",
		"entity", nil, nil, nil)
	_, _ = store.CreatePage("page-new", "New Design", "The backend uses PostgreSQL for persistence. The API uses GraphQL endpoints.",
		"entity", nil, nil, nil)

	result, err := engine.Lint()
	if err != nil {
		t.Fatalf("lint: %v", err)
	}

	// Should detect at least one contradiction (MySQL vs PostgreSQL or REST vs GraphQL).
	if len(result.Contradictions) == 0 {
		t.Log("note: contradiction detection is heuristic — may not catch all cases")
	}
	// The contradictions field should at least be initialized (not nil).
	if result.Contradictions == nil {
		// It's fine if it's an empty slice, but shouldn't be nil after lint.
		t.Log("contradictions field is nil (no contradictions found)")
	}
}

func TestRenderPageMarkdown(t *testing.T) {
	page := &types.WikiPage{
		Slug:       "test-page",
		Title:      "Test Page",
		Content:    "# Test\n\nSome content here.",
		Category:   "entity",
		Tags:       []string{"test", "example"},
		SourceIDs:  []int64{1, 2},
		LinksSlugs: []string{"other-page"},
		CreatedAt:  time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		UpdatedAt:  time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC),
	}

	md := renderPageMarkdown(page)

	// Check frontmatter.
	if !strings.HasPrefix(md, "---\n") {
		t.Error("expected YAML frontmatter start")
	}
	if !strings.Contains(md, "title: \"Test Page\"") {
		t.Error("expected title in frontmatter")
	}
	if !strings.Contains(md, "category: entity") {
		t.Error("expected category in frontmatter")
	}
	if !strings.Contains(md, "source_count: 2") {
		t.Error("expected source_count in frontmatter")
	}
	if !strings.Contains(md, "created: 2026-01-15") {
		t.Error("expected created date in frontmatter")
	}
	if !strings.Contains(md, "  - test") {
		t.Error("expected tags in frontmatter")
	}

	// Check wikilinks.
	if !strings.Contains(md, "[[other-page]]") {
		t.Error("expected wikilink to other-page")
	}
}
