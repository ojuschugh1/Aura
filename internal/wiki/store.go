package wiki

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ojuschugh1/aura/pkg/types"
)

// Store provides CRUD operations on wiki pages, sources, and the log.
type Store struct {
	db *sql.DB
}

// NewStore creates a Store backed by the given database.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// --- Sources ---

// IngestSource inserts a new raw source. Returns the source with its ID and
// whether it was a duplicate (already existed with the same content hash).
// If a source with the same content hash already exists, returns it without re-inserting.
func (s *Store) IngestSource(title, content, format, origin string) (*types.WikiSource, bool, error) {
	hash := contentHash(content)

	// Check for duplicate.
	var existingID int64
	err := s.db.QueryRow(`SELECT id FROM wiki_sources WHERE content_hash = ?`, hash).Scan(&existingID)
	if err == nil {
		src, err := s.GetSource(existingID)
		return src, true, err
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.Exec(`
		INSERT INTO wiki_sources (title, content, content_hash, format, origin, ingested_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		title, content, hash, format, origin, now,
	)
	if err != nil {
		return nil, false, fmt.Errorf("ingest source: %w", err)
	}
	id, _ := res.LastInsertId()
	src, err := s.GetSource(id)
	return src, false, err
}

// GetSource retrieves a source by ID.
func (s *Store) GetSource(id int64) (*types.WikiSource, error) {
	row := s.db.QueryRow(`
		SELECT id, title, content, content_hash, format, origin, ingested_at
		FROM wiki_sources WHERE id = ?`, id,
	)
	return scanSource(row)
}

// ListSources returns all sources ordered by ingestion time descending.
func (s *Store) ListSources() ([]*types.WikiSource, error) {
	rows, err := s.db.Query(`
		SELECT id, title, content, content_hash, format, origin, ingested_at
		FROM wiki_sources ORDER BY ingested_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list sources: %w", err)
	}
	defer rows.Close()

	var sources []*types.WikiSource
	for rows.Next() {
		src, err := scanSource(rows)
		if err != nil {
			return nil, err
		}
		sources = append(sources, src)
	}
	return sources, rows.Err()
}

// --- Pages ---

// CreatePage inserts a new wiki page.
func (s *Store) CreatePage(slug, title, content, category string, tags []string, sourceIDs []int64, links []string) (*types.WikiPage, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tagsJSON := joinStrings(tags)
	sourceIDsJSON := joinInt64s(sourceIDs)
	linksJSON := joinStrings(links)

	_, err := s.db.Exec(`
		INSERT INTO wiki_pages (slug, title, content, category, tags, source_ids, links, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		slug, title, content, category, tagsJSON, sourceIDsJSON, linksJSON, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("create page: %w", err)
	}
	return s.GetPage(slug)
}

// UpdatePage updates an existing wiki page's content, tags, source IDs, and links.
func (s *Store) UpdatePage(slug, content string, tags []string, sourceIDs []int64, links []string) (*types.WikiPage, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tagsJSON := joinStrings(tags)
	sourceIDsJSON := joinInt64s(sourceIDs)
	linksJSON := joinStrings(links)

	res, err := s.db.Exec(`
		UPDATE wiki_pages SET content=?, tags=?, source_ids=?, links=?, updated_at=?
		WHERE slug=?`,
		content, tagsJSON, sourceIDsJSON, linksJSON, now, slug,
	)
	if err != nil {
		return nil, fmt.Errorf("update page: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, fmt.Errorf("page %q not found", slug)
	}
	return s.GetPage(slug)
}

// GetPage retrieves a wiki page by slug.
func (s *Store) GetPage(slug string) (*types.WikiPage, error) {
	row := s.db.QueryRow(`
		SELECT id, slug, title, content, category, tags, source_ids, links, created_at, updated_at,
		       vitality, access_tier, query_count, last_queried
		FROM wiki_pages WHERE slug = ?`, slug,
	)
	return scanPage(row)
}

// DeletePage removes a wiki page by slug.
func (s *Store) DeletePage(slug string) error {
	res, err := s.db.Exec(`DELETE FROM wiki_pages WHERE slug = ?`, slug)
	if err != nil {
		return fmt.Errorf("delete page: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("page %q not found", slug)
	}
	return nil
}

// ListPages returns all pages, optionally filtered by category.
func (s *Store) ListPages(category string) ([]*types.WikiPage, error) {
	query := `SELECT id, slug, title, content, category, tags, source_ids, links, created_at, updated_at,
	                 vitality, access_tier, query_count, last_queried
	          FROM wiki_pages`
	var args []any
	if category != "" {
		query += " WHERE category = ?"
		args = append(args, category)
	}
	query += " ORDER BY updated_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list pages: %w", err)
	}
	defer rows.Close()

	var pages []*types.WikiPage
	for rows.Next() {
		p, err := scanPage(rows)
		if err != nil {
			return nil, err
		}
		pages = append(pages, p)
	}
	return pages, rows.Err()
}

// SearchPages performs a case-insensitive search across page titles and content.
func (s *Store) SearchPages(query string) ([]*types.WikiPage, error) {
	pattern := "%" + strings.ToLower(query) + "%"
	rows, err := s.db.Query(`
		SELECT id, slug, title, content, category, tags, source_ids, links, created_at, updated_at,
		       vitality, access_tier, query_count, last_queried
		FROM wiki_pages
		WHERE LOWER(title) LIKE ? OR LOWER(content) LIKE ?
		ORDER BY updated_at DESC`,
		pattern, pattern,
	)
	if err != nil {
		return nil, fmt.Errorf("search pages: %w", err)
	}
	defer rows.Close()

	var pages []*types.WikiPage
	for rows.Next() {
		p, err := scanPage(rows)
		if err != nil {
			return nil, err
		}
		pages = append(pages, p)
	}
	return pages, rows.Err()
}

// PageCount returns the total number of wiki pages.
func (s *Store) PageCount() int {
	var count int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM wiki_pages`).Scan(&count)
	return count
}

// SourceCount returns the total number of wiki sources.
func (s *Store) SourceCount() int {
	var count int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM wiki_sources`).Scan(&count)
	return count
}

// --- Log ---

// AppendLog adds a chronological entry to the wiki log.
func (s *Store) AppendLog(action, summary string, pageSlugs []string, sourceID *int64) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	slugsJSON := joinStrings(pageSlugs)

	_, err := s.db.Exec(`
		INSERT INTO wiki_log (action, summary, page_slugs, source_id, timestamp)
		VALUES (?, ?, ?, ?, ?)`,
		action, summary, slugsJSON, sourceID, now,
	)
	if err != nil {
		return fmt.Errorf("append log: %w", err)
	}
	return nil
}

// RecentLog returns the last N log entries.
func (s *Store) RecentLog(limit int) ([]*types.WikiLogEntry, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(`
		SELECT id, action, summary, page_slugs, source_id, timestamp
		FROM wiki_log ORDER BY timestamp DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("recent log: %w", err)
	}
	defer rows.Close()

	var entries []*types.WikiLogEntry
	for rows.Next() {
		e, err := scanLogEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// --- Index ---

// BuildIndex generates the wiki index from all pages.
func (s *Store) BuildIndex() (*types.WikiIndex, error) {
	pages, err := s.ListPages("")
	if err != nil {
		return nil, err
	}

	idx := &types.WikiIndex{Entries: make([]types.WikiIndexEntry, 0, len(pages))}
	for _, p := range pages {
		summary := p.Content
		if len(summary) > 120 {
			summary = summary[:120] + "…"
		}
		idx.Entries = append(idx.Entries, types.WikiIndexEntry{
			Slug:     p.Slug,
			Title:    p.Title,
			Category: p.Category,
			Summary:  summary,
			Links:    len(p.LinksSlugs),
		})
	}
	return idx, nil
}

// --- Helpers ---

func contentHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", sum)
}

func joinStrings(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	b, _ := json.Marshal(ss)
	return string(b)
}

func joinInt64s(ids []int64) string {
	if len(ids) == 0 {
		return ""
	}
	b, _ := json.Marshal(ids)
	return string(b)
}

func scanSource(scanner interface{ Scan(...any) error }) (*types.WikiSource, error) {
	var (
		src        types.WikiSource
		ingestedAt string
	)
	err := scanner.Scan(&src.ID, &src.Title, &src.Content, &src.ContentHash,
		&src.Format, &src.Origin, &ingestedAt)
	if err != nil {
		return nil, fmt.Errorf("scan source: %w", err)
	}
	if t, err := time.Parse(time.RFC3339Nano, ingestedAt); err == nil {
		src.IngestedAt = t
	}
	return &src, nil
}

func scanPage(scanner interface{ Scan(...any) error }) (*types.WikiPage, error) {
	var (
		p                        types.WikiPage
		tagsJSON, sourceIDsJSON  sql.NullString
		linksJSON                sql.NullString
		createdAt, updatedAt     string
		vitality                 sql.NullFloat64
		accessTier               sql.NullString
		queryCount               sql.NullInt64
		lastQueried              sql.NullString
	)
	err := scanner.Scan(&p.ID, &p.Slug, &p.Title, &p.Content, &p.Category,
		&tagsJSON, &sourceIDsJSON, &linksJSON, &createdAt, &updatedAt,
		&vitality, &accessTier, &queryCount, &lastQueried)
	if err != nil {
		return nil, fmt.Errorf("scan page: %w", err)
	}
	if t, err := time.Parse(time.RFC3339Nano, createdAt); err == nil {
		p.CreatedAt = t
	}
	if t, err := time.Parse(time.RFC3339Nano, updatedAt); err == nil {
		p.UpdatedAt = t
	}
	if tagsJSON.Valid && tagsJSON.String != "" {
		_ = json.Unmarshal([]byte(tagsJSON.String), &p.Tags)
	}
	if sourceIDsJSON.Valid && sourceIDsJSON.String != "" {
		_ = json.Unmarshal([]byte(sourceIDsJSON.String), &p.SourceIDs)
	}
	if linksJSON.Valid && linksJSON.String != "" {
		_ = json.Unmarshal([]byte(linksJSON.String), &p.LinksSlugs)
	}
	if vitality.Valid {
		p.Vitality = vitality.Float64
	} else {
		p.Vitality = 1.0
	}
	if accessTier.Valid && accessTier.String != "" {
		p.AccessTier = accessTier.String
	} else {
		p.AccessTier = "public"
	}
	if queryCount.Valid {
		p.QueryCount = int(queryCount.Int64)
	}
	if lastQueried.Valid && lastQueried.String != "" {
		if t, err := time.Parse(time.RFC3339Nano, lastQueried.String); err == nil {
			p.LastQueried = &t
		}
	}
	return &p, nil
}

func scanLogEntry(scanner interface{ Scan(...any) error }) (*types.WikiLogEntry, error) {
	var (
		e         types.WikiLogEntry
		slugsJSON sql.NullString
		sourceID  sql.NullInt64
		ts        string
	)
	err := scanner.Scan(&e.ID, &e.Action, &e.Summary, &slugsJSON, &sourceID, &ts)
	if err != nil {
		return nil, fmt.Errorf("scan log entry: %w", err)
	}
	if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
		e.Timestamp = t
	}
	if slugsJSON.Valid && slugsJSON.String != "" {
		_ = json.Unmarshal([]byte(slugsJSON.String), &e.PageSlugs)
	}
	if sourceID.Valid {
		id := sourceID.Int64
		e.SourceID = &id
	}
	return &e, nil
}
