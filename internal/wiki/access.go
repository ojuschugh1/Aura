package wiki

import (
	"fmt"

	"github.com/ojuschugh1/aura/pkg/types"
)

// Access tiers control which agents can see which pages.
const (
	TierPublic  = "public"  // visible to all agents
	TierTeam    = "team"    // visible to agents with team or higher access
	TierPrivate = "private" // visible only to the owner agent
)

// ValidTiers is the set of allowed access tier values.
var ValidTiers = map[string]bool{
	TierPublic:  true,
	TierTeam:    true,
	TierPrivate: true,
}

// SetAccessTier updates the access tier for a page.
func (s *Store) SetAccessTier(slug, tier string) error {
	if !ValidTiers[tier] {
		return fmt.Errorf("invalid access tier %q: must be public, team, or private", tier)
	}
	res, err := s.db.Exec(`UPDATE wiki_pages SET access_tier = ? WHERE slug = ?`, tier, slug)
	if err != nil {
		return fmt.Errorf("set access tier: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("page %q not found", slug)
	}
	return nil
}

// ListPagesWithAccess returns pages filtered by both category and access tier.
// An agent with "public" access sees only public pages.
// An agent with "team" access sees public + team pages.
// An agent with "private" access (or empty = all) sees everything.
func (s *Store) ListPagesWithAccess(category, agentTier string) ([]*types.WikiPage, error) {
	query := `SELECT id, slug, title, content, category, tags, source_ids, links, created_at, updated_at,
	                 vitality, access_tier, query_count, last_queried
	          FROM wiki_pages WHERE 1=1`
	var args []any

	if category != "" {
		query += " AND category = ?"
		args = append(args, category)
	}

	switch agentTier {
	case TierPublic:
		query += " AND access_tier = 'public'"
	case TierTeam:
		query += " AND access_tier IN ('public', 'team')"
	case TierPrivate, "":
		// sees everything
	default:
		query += " AND access_tier = 'public'" // unknown tier = public only
	}

	query += " ORDER BY updated_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list pages with access: %w", err)
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

// FilterByAccess filters an existing page list by access tier.
func FilterByAccess(pages []*types.WikiPage, agentTier string) []*types.WikiPage {
	if agentTier == "" || agentTier == TierPrivate {
		return pages // full access
	}

	var filtered []*types.WikiPage
	for _, p := range pages {
		switch agentTier {
		case TierPublic:
			if p.AccessTier == TierPublic {
				filtered = append(filtered, p)
			}
		case TierTeam:
			if p.AccessTier == TierPublic || p.AccessTier == TierTeam {
				filtered = append(filtered, p)
			}
		}
	}
	return filtered
}
