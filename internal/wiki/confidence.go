package wiki

import (
	"time"

	"github.com/ojuschugh1/aura/pkg/types"
)

// Evidence levels for wiki content. Unlike binary "found vs guessed",
// Aura uses a continuous confidence score based on multiple signals:
// source backing, freshness, cross-reference density, and origin.
const (
	ConfidenceVerified  = 1.0  // backed by multiple sources, recently updated
	ConfidenceStrong    = 0.85 // backed by at least one source
	ConfidenceInferred  = 0.7  // auto-extracted by heuristics, no direct source
	ConfidenceWeak      = 0.5  // stale or contradicted content
	ConfidenceUnknown   = 0.3  // no sources, no links, possibly orphaned
)

// computeConfidence calculates a confidence score for a page based on
// multiple signals. This is richer than a simple EXTRACTED/INFERRED tag —
// it accounts for how well-supported and current the content is.
func computeConfidence(p *types.WikiPage) float64 {
	score := 0.5 // baseline

	// Source backing: more sources = higher confidence.
	switch {
	case len(p.SourceIDs) >= 3:
		score += 0.3
	case len(p.SourceIDs) >= 1:
		score += 0.2
	default:
		score -= 0.1
	}

	// Freshness: recently updated content is more trustworthy.
	age := time.Since(p.UpdatedAt)
	switch {
	case age < 7*24*time.Hour: // updated within a week
		score += 0.1
	case age < 30*24*time.Hour: // updated within a month
		// no change
	default: // stale
		score -= 0.15
	}

	// Cross-reference density: well-linked pages are better validated.
	if len(p.LinksSlugs) >= 3 {
		score += 0.1
	}

	// Category bonus: tool output and source summaries are factual.
	switch p.Category {
	case "tool":
		score += 0.1 // tool output is deterministic
	case "source":
		score += 0.05 // direct from a source document
	}

	// Tag penalty: auto-extracted content is less certain.
	for _, tag := range p.Tags {
		if tag == "auto-extracted" {
			score -= 0.1
			break
		}
	}

	// Clamp to [0, 1].
	if score > 1.0 {
		score = 1.0
	}
	if score < 0.0 {
		score = 0.0
	}

	return score
}

// ConfidenceLabel returns a human-readable label for a confidence score.
func ConfidenceLabel(score float64) string {
	switch {
	case score >= 0.9:
		return "verified"
	case score >= 0.75:
		return "strong"
	case score >= 0.6:
		return "inferred"
	case score >= 0.4:
		return "weak"
	default:
		return "uncertain"
	}
}
