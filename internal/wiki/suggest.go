package wiki

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/ojuschugh1/aura/pkg/types"
)

// generateSuggestions produces actionable recommendations based on lint findings
// and wiki structure analysis. This implements the gist's idea that "the LLM is
// good at suggesting new questions to investigate and new sources to look for."
func generateSuggestions(pages []*types.WikiPage, lint *types.WikiLintResult, inbound map[string]int) []types.WikiSuggestion {
	var suggestions []types.WikiSuggestion

	// 1. Suggest creating pages for missing references.
	for _, slug := range lint.MissingPages {
		suggestions = append(suggestions, types.WikiSuggestion{
			Type:    "create_page",
			Target:  slug,
			Message: fmt.Sprintf("Create a page for %q — it's referenced by other pages but doesn't exist yet.", slug),
		})
	}

	// 2. Suggest adding links to orphan pages.
	for _, slug := range lint.Orphans {
		suggestions = append(suggestions, types.WikiSuggestion{
			Type:    "add_links",
			Target:  slug,
			Message: fmt.Sprintf("Page %q has no inbound links. Find related pages and add cross-references.", slug),
		})
	}

	// 3. Suggest investigating stale pages.
	for _, slug := range lint.Stale {
		suggestions = append(suggestions, types.WikiSuggestion{
			Type:    "investigate",
			Target:  slug,
			Message: fmt.Sprintf("Page %q hasn't been updated in 30+ days. Check if the information is still current.", slug),
		})
	}

	// 4. Suggest resolving contradictions.
	for _, c := range lint.Contradictions {
		suggestions = append(suggestions, types.WikiSuggestion{
			Type:    "investigate",
			Target:  c.PageA,
			Message: fmt.Sprintf("Contradiction between %q and %q — determine which is current and update the outdated page.", c.PageA, c.PageB),
		})
	}

	// 5. Detect mentioned-but-not-linked concepts across pages.
	suggestions = append(suggestions, suggestMissingLinks(pages)...)

	// 6. Suggest splitting large pages.
	for _, p := range pages {
		wordCount := len(strings.Fields(p.Content))
		if wordCount > 500 {
			suggestions = append(suggestions, types.WikiSuggestion{
				Type:    "split_page",
				Target:  p.Slug,
				Message: fmt.Sprintf("Page %q has %d words. Consider splitting into focused sub-pages.", p.Slug, wordCount),
			})
		}
	}

	// 7. Suggest adding sources for pages with no source backing.
	for _, p := range pages {
		if len(p.SourceIDs) == 0 && p.Category != "tool" && p.Category != "synthesis" {
			suggestions = append(suggestions, types.WikiSuggestion{
				Type:    "add_source",
				Target:  p.Slug,
				Message: fmt.Sprintf("Page %q has no backing sources. Consider finding a source document to support it.", p.Slug),
			})
		}
	}

	// Cap suggestions to avoid overwhelming output.
	if len(suggestions) > 20 {
		suggestions = suggestions[:20]
	}

	return suggestions
}

// suggestMissingLinks finds page titles mentioned in other pages' content
// but not linked via [[slug]].
func suggestMissingLinks(pages []*types.WikiPage) []types.WikiSuggestion {
	var suggestions []types.WikiSuggestion

	// Build a map of title → slug for all pages.
	titleToSlug := make(map[string]string)
	for _, p := range pages {
		lower := strings.ToLower(p.Title)
		if len(lower) >= 4 { // skip very short titles
			titleToSlug[lower] = p.Slug
		}
	}

	// For each page, check if any other page's title appears in the content
	// but isn't in the links list.
	for _, p := range pages {
		contentLower := strings.ToLower(p.Content)
		linkedSlugs := make(map[string]bool)
		for _, l := range p.LinksSlugs {
			linkedSlugs[l] = true
		}

		for title, slug := range titleToSlug {
			if slug == p.Slug {
				continue // skip self
			}
			if linkedSlugs[slug] {
				continue // already linked
			}
			// Check if the title appears as a whole word in the content.
			pattern := `(?i)\b` + regexp.QuoteMeta(title) + `\b`
			if matched, _ := regexp.MatchString(pattern, contentLower); matched {
				suggestions = append(suggestions, types.WikiSuggestion{
					Type:    "add_links",
					Target:  p.Slug,
					Message: fmt.Sprintf("Page %q mentions %q but doesn't link to [[%s]].", p.Slug, title, slug),
				})
				if len(suggestions) > 10 {
					return suggestions // cap early
				}
			}
		}
	}

	return suggestions
}
