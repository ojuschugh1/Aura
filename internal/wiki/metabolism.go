package wiki

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/ojuschugh1/aura/pkg/types"
)

// MetabolismConfig controls the knowledge lifecycle parameters.
type MetabolismConfig struct {
	DecayRate         float64 // vitality lost per day of inactivity (default: 0.01)
	DecayFloor        float64 // minimum vitality before archival suggestion (default: 0.2)
	ArchiveThreshold  float64 // vitality below this triggers archival (default: 0.1)
	PressureThreshold int     // contradictions needed to trigger revision alert (default: 3)
	ConsolidateMinSim float64 // minimum word overlap to suggest consolidation (default: 0.4)
}

// DefaultMetabolismConfig returns sensible defaults.
func DefaultMetabolismConfig() MetabolismConfig {
	return MetabolismConfig{
		DecayRate:         0.01,
		DecayFloor:        0.2,
		ArchiveThreshold:  0.1,
		PressureThreshold: 3,
		ConsolidateMinSim: 0.4,
	}
}

// Metabolize runs a full lifecycle pass over the wiki: decay vitality,
// detect consolidation candidates, check pressure thresholds, and
// suggest archival for near-dead pages. This is the "heartbeat" that
// keeps the wiki alive — knowledge that isn't accessed or refreshed
// gradually fades, while actively-used knowledge stays strong.
func (e *Engine) Metabolize(cfg MetabolismConfig) (*types.MetabolismResult, error) {
	pages, err := e.store.ListPages("")
	if err != nil {
		return nil, fmt.Errorf("metabolism: list pages: %w", err)
	}

	result := &types.MetabolismResult{}

	// Phase 1: Decay — reduce vitality of pages not recently accessed or updated.
	for _, p := range pages {
		newVitality := decayVitality(p, cfg)
		if newVitality != p.Vitality {
			e.store.db.Exec(`UPDATE wiki_pages SET vitality=? WHERE slug=?`, newVitality, p.Slug)
			result.PagesDecayed++

			if newVitality <= cfg.ArchiveThreshold {
				result.Suggestions = append(result.Suggestions, types.WikiSuggestion{
					Type:    "archive",
					Target:  p.Slug,
					Message: fmt.Sprintf("Page %q vitality is %.0f%% — consider archiving or refreshing with new sources.", p.Slug, newVitality*100),
				})
				result.PagesArchived++
			}
		}
	}

	// Phase 2: Pressure — check for accumulated contradictions.
	result.PressureAlerts = e.checkPressure(cfg.PressureThreshold)

	for _, alert := range result.PressureAlerts {
		result.Suggestions = append(result.Suggestions, types.WikiSuggestion{
			Type:    "revise",
			Target:  alert.TargetSlug,
			Message: alert.Message,
		})
	}

	// Phase 3: Consolidation — find pages with high content overlap.
	consolidations := findConsolidationCandidates(pages, cfg.ConsolidateMinSim)
	for _, c := range consolidations {
		result.Suggestions = append(result.Suggestions, types.WikiSuggestion{
			Type:    "consolidate",
			Target:  c[0],
			Message: fmt.Sprintf("Pages %q and %q have significant content overlap — consider merging.", c[0], c[1]),
		})
		result.PagesConsolidated++
	}

	// Phase 4: Boost — increase vitality of recently queried pages.
	for _, p := range pages {
		if p.LastQueried != nil && time.Since(*p.LastQueried) < 7*24*time.Hour {
			boost := 0.05 * float64(p.QueryCount)
			if boost > 0.2 {
				boost = 0.2
			}
			newVitality := p.Vitality + boost
			if newVitality > 1.0 {
				newVitality = 1.0
			}
			if newVitality != p.Vitality {
				e.store.db.Exec(`UPDATE wiki_pages SET vitality=? WHERE slug=?`, newVitality, p.Slug)
			}
		}
	}

	_ = e.store.AppendLog("metabolism",
		fmt.Sprintf("Metabolism: %d decayed, %d consolidation candidates, %d archived, %d pressure alerts",
			result.PagesDecayed, result.PagesConsolidated, result.PagesArchived, len(result.PressureAlerts)),
		nil, nil)

	return result, nil
}

// RecordQueryAccess increments the query count and updates last_queried for a page.
func (s *Store) RecordQueryAccess(slug string) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	s.db.Exec(`UPDATE wiki_pages SET query_count = query_count + 1, last_queried = ? WHERE slug = ?`, now, slug)
}

// decayVitality calculates the new vitality for a page based on time since
// last update and query activity.
func decayVitality(p *types.WikiPage, cfg MetabolismConfig) float64 {
	daysSinceUpdate := time.Since(p.UpdatedAt).Hours() / 24

	// Pages updated within the last week don't decay.
	if daysSinceUpdate < 7 {
		return p.Vitality
	}

	// Decay proportional to days of inactivity beyond the first week.
	inactiveDays := daysSinceUpdate - 7
	decay := inactiveDays * cfg.DecayRate

	// Recently queried pages decay slower.
	if p.LastQueried != nil && time.Since(*p.LastQueried) < 14*24*time.Hour {
		decay *= 0.5
	}

	// Source-backed pages decay slower.
	if len(p.SourceIDs) > 0 {
		decay *= 0.7
	}

	// Tool output pages decay very slowly (they're factual records).
	if p.Category == "tool" {
		decay *= 0.3
	}

	newVitality := p.Vitality - decay
	if newVitality < cfg.DecayFloor {
		newVitality = cfg.DecayFloor
	}
	if newVitality < 0 {
		newVitality = 0
	}

	return newVitality
}

// findConsolidationCandidates finds pairs of pages with high word overlap.
func findConsolidationCandidates(pages []*types.WikiPage, minSim float64) [][]string {
	var candidates [][]string

	for i := 0; i < len(pages); i++ {
		for j := i + 1; j < len(pages); j++ {
			a, b := pages[i], pages[j]
			// Skip tool/source pages — they're records, not knowledge.
			if a.Category == "tool" || b.Category == "tool" {
				continue
			}
			if a.Category == "source" || b.Category == "source" {
				continue
			}

			sim := wordOverlap(a.Content, b.Content)
			if sim >= minSim {
				candidates = append(candidates, []string{a.Slug, b.Slug})
			}
		}
		// Cap to avoid O(n²) explosion on large wikis.
		if len(candidates) >= 10 {
			break
		}
	}

	return candidates
}

// wordOverlap computes Jaccard similarity of word sets between two texts.
func wordOverlap(a, b string) float64 {
	wordsA := uniqueWords(a)
	wordsB := uniqueWords(b)
	if len(wordsA) == 0 || len(wordsB) == 0 {
		return 0
	}

	intersection := 0
	for w := range wordsA {
		if wordsB[w] {
			intersection++
		}
	}

	union := len(wordsA) + len(wordsB) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

func uniqueWords(text string) map[string]bool {
	words := make(map[string]bool)
	for _, w := range strings.Fields(strings.ToLower(text)) {
		if len(w) >= 3 { // skip short words
			words[w] = true
		}
	}
	return words
}

// --- Pressure tracking ---

// RecordPressure adds a contradiction pressure entry against a target page.
func (s *Store) RecordPressure(targetSlug, sourceSlug, evidence, pressureType string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.Exec(`
		INSERT OR IGNORE INTO wiki_pressure (target_slug, source_slug, evidence, pressure_type, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		targetSlug, sourceSlug, evidence, pressureType, now,
	)
	return err
}

// GetPressure returns all unresolved pressure entries for a target page.
func (s *Store) GetPressure(targetSlug string) ([]*types.WikiPressure, error) {
	rows, err := s.db.Query(`
		SELECT id, target_slug, source_slug, evidence, pressure_type, resolved, created_at
		FROM wiki_pressure WHERE target_slug = ? AND resolved = 0
		ORDER BY created_at DESC`, targetSlug,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*types.WikiPressure
	for rows.Next() {
		var p types.WikiPressure
		var resolved int
		var createdAt string
		if err := rows.Scan(&p.ID, &p.TargetSlug, &p.SourceSlug, &p.Evidence,
			&p.PressureType, &resolved, &createdAt); err != nil {
			continue
		}
		p.Resolved = resolved != 0
		if t, err := time.Parse(time.RFC3339Nano, createdAt); err == nil {
			p.CreatedAt = t
		}
		entries = append(entries, &p)
	}
	return entries, nil
}

// ResolvePressure marks all pressure entries for a target as resolved.
func (s *Store) ResolvePressure(targetSlug string) error {
	_, err := s.db.Exec(`UPDATE wiki_pressure SET resolved = 1 WHERE target_slug = ?`, targetSlug)
	return err
}

// checkPressure finds pages where accumulated contradictions exceed the threshold.
func (e *Engine) checkPressure(threshold int) []types.PressureAlert {
	rows, err := e.store.db.Query(`
		SELECT target_slug, COUNT(*) as cnt, GROUP_CONCAT(source_slug, ',')
		FROM wiki_pressure WHERE resolved = 0
		GROUP BY target_slug HAVING cnt >= ?`, threshold,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var alerts []types.PressureAlert
	for rows.Next() {
		var slug string
		var count int
		var sourcesStr sql.NullString
		if err := rows.Scan(&slug, &count, &sourcesStr); err != nil {
			continue
		}
		var sources []string
		if sourcesStr.Valid {
			sources = strings.Split(sourcesStr.String, ",")
		}
		alerts = append(alerts, types.PressureAlert{
			TargetSlug:    slug,
			PressureCount: count,
			Sources:       sources,
			Message: fmt.Sprintf("Page %q has %d unresolved contradictions from %s — accumulated evidence suggests revision is needed.",
				slug, count, strings.Join(sources, ", ")),
		})
	}
	return alerts
}
