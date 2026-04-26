package wiki

import (
	"regexp"
	"strings"

	"github.com/ojuschugh1/aura/pkg/types"
)

// detectContradictions scans pages for potentially conflicting claims.
// It uses heuristic pattern matching — no LLM calls.
//
// Strategy: extract factual assertions (X is/uses/has Y) from each page,
// then find cases where two pages make different claims about the same subject.
type assertion struct {
	Subject   string
	Predicate string
	Object    string
	PageSlug  string
	Sentence  string
}

// assertionPatterns match simple factual claims.
var assertionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(\w[\w\s]{1,30})\s+(?:is|are)\s+(?:a\s+|an\s+|the\s+)?(\w[\w\s]{1,40})`),
	regexp.MustCompile(`(?i)(\w[\w\s]{1,30})\s+uses?\s+(\w[\w\s]{1,40})`),
	regexp.MustCompile(`(?i)(\w[\w\s]{1,30})\s+(?:has|have)\s+(\w[\w\s]{1,40})`),
	regexp.MustCompile(`(?i)(?:we|they|the team)\s+(?:decided|chose|picked|selected)\s+(\w[\w\s]{1,40})`),
}

// negationPatterns indicate a claim is being negated.
var negationWords = []string{
	"not", "no longer", "don't", "doesn't", "won't", "shouldn't",
	"never", "instead of", "rather than", "replaced", "deprecated",
	"removed", "dropped", "abandoned", "switched from",
}

// findContradictions compares assertions across pages to find conflicts.
func findContradictions(pages []*types.WikiPage) []types.WikiContradiction {
	// Extract assertions from all pages.
	var allAssertions []assertion
	for _, p := range pages {
		assertions := extractAssertions(p.Content, p.Slug)
		allAssertions = append(allAssertions, assertions...)
	}

	// Group assertions by normalized subject.
	bySubject := make(map[string][]assertion)
	for _, a := range allAssertions {
		key := normalizeSubject(a.Subject)
		if key != "" {
			bySubject[key] = append(bySubject[key], a)
		}
	}

	// Find contradictions: same subject, different pages, conflicting predicates.
	var contradictions []types.WikiContradiction
	seen := make(map[string]bool)

	for _, assertions := range bySubject {
		for i := 0; i < len(assertions); i++ {
			for j := i + 1; j < len(assertions); j++ {
				a, b := assertions[i], assertions[j]
				if a.PageSlug == b.PageSlug {
					continue
				}

				// Check if one assertion negates the other.
				if isContradiction(a, b) {
					key := a.PageSlug + ":" + b.PageSlug + ":" + normalizeSubject(a.Subject)
					if seen[key] {
						continue
					}
					seen[key] = true

					snippet := a.Sentence + " vs. " + b.Sentence
					if len(snippet) > 200 {
						snippet = snippet[:200] + "…"
					}
					contradictions = append(contradictions, types.WikiContradiction{
						PageA:   a.PageSlug,
						PageB:   b.PageSlug,
						Snippet: snippet,
					})
				}
			}
		}
	}

	return contradictions
}

// extractAssertions pulls factual claims from content.
func extractAssertions(content, pageSlug string) []assertion {
	var assertions []assertion
	sentences := splitIntoSentences(content)

	for _, sent := range sentences {
		sent = strings.TrimSpace(sent)
		if len(sent) < 10 || len(sent) > 300 {
			continue
		}

		for _, re := range assertionPatterns {
			matches := re.FindStringSubmatch(sent)
			if matches == nil {
				continue
			}

			var subject, object string
			if len(matches) >= 3 {
				subject = strings.TrimSpace(matches[1])
				object = strings.TrimSpace(matches[2])
			} else if len(matches) >= 2 {
				subject = "decision"
				object = strings.TrimSpace(matches[1])
			}

			if subject == "" || object == "" {
				continue
			}

			assertions = append(assertions, assertion{
				Subject:   subject,
				Predicate: extractPredicate(sent, subject, object),
				Object:    object,
				PageSlug:  pageSlug,
				Sentence:  sent,
			})
			break // one assertion per sentence
		}
	}

	return assertions
}

// isContradiction checks if two assertions about the same subject conflict.
func isContradiction(a, b assertion) bool {
	// Direct negation: one sentence contains negation words about the same object.
	aHasNeg := containsNegation(a.Sentence)
	bHasNeg := containsNegation(b.Sentence)

	// If one is negated and the other isn't, and they share similar objects, it's a contradiction.
	if aHasNeg != bHasNeg {
		objSimilarity := stringSimilarity(normalizeSubject(a.Object), normalizeSubject(b.Object))
		if objSimilarity > 0.6 {
			return true
		}
	}

	// Different objects for the same predicate type (e.g., "uses PostgreSQL" vs "uses MySQL").
	if a.Predicate == b.Predicate && a.Predicate != "" {
		normA := normalizeSubject(a.Object)
		normB := normalizeSubject(b.Object)
		if normA != normB && normA != "" && normB != "" {
			// Only flag if the objects are clearly different (not subsets).
			if !strings.Contains(normA, normB) && !strings.Contains(normB, normA) {
				return true
			}
		}
	}

	return false
}

// containsNegation checks if a sentence contains negation words.
func containsNegation(sentence string) bool {
	lower := strings.ToLower(sentence)
	for _, neg := range negationWords {
		if strings.Contains(lower, neg) {
			return true
		}
	}
	return false
}

// extractPredicate returns the verb/relationship between subject and object.
func extractPredicate(sentence, subject, object string) string {
	lower := strings.ToLower(sentence)
	subIdx := strings.Index(lower, strings.ToLower(subject))
	objIdx := strings.Index(lower, strings.ToLower(object))
	if subIdx < 0 || objIdx < 0 || subIdx >= objIdx {
		return ""
	}
	between := strings.TrimSpace(lower[subIdx+len(subject) : objIdx])
	// Normalize common predicates.
	switch {
	case strings.Contains(between, "use"):
		return "uses"
	case strings.Contains(between, "is") || strings.Contains(between, "are"):
		return "is"
	case strings.Contains(between, "has") || strings.Contains(between, "have"):
		return "has"
	default:
		return between
	}
}

// normalizeSubject lowercases and trims a subject for comparison.
func normalizeSubject(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	// Remove common articles.
	for _, prefix := range []string{"the ", "a ", "an "} {
		s = strings.TrimPrefix(s, prefix)
	}
	return s
}

// stringSimilarity returns a rough Jaccard similarity between two strings.
func stringSimilarity(a, b string) float64 {
	if a == b {
		return 1.0
	}
	wordsA := strings.Fields(a)
	wordsB := strings.Fields(b)
	if len(wordsA) == 0 || len(wordsB) == 0 {
		return 0
	}

	setA := make(map[string]bool)
	for _, w := range wordsA {
		setA[w] = true
	}
	setB := make(map[string]bool)
	for _, w := range wordsB {
		setB[w] = true
	}

	intersection := 0
	for w := range setA {
		if setB[w] {
			intersection++
		}
	}

	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

// splitIntoSentences breaks text on sentence boundaries.
func splitIntoSentences(text string) []string {
	var sentences []string
	for _, para := range strings.Split(text, "\n") {
		para = strings.TrimSpace(para)
		if para == "" || strings.HasPrefix(para, "#") || strings.HasPrefix(para, "-") ||
			strings.HasPrefix(para, "|") || strings.HasPrefix(para, "```") {
			continue
		}
		parts := regexp.MustCompile(`[.!?]\s+`).Split(para, -1)
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if len(p) > 10 {
				sentences = append(sentences, p)
			}
		}
	}
	return sentences
}
