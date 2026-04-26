package autocapture

import (
	"regexp"
	"strings"
)

// DecisionMatch represents a decision extracted from transcript text.
type DecisionMatch struct {
	Key        string  // topic of the decision
	Value      string  // full decision content (matched sentence)
	Confidence float64 // 0.0–1.0 based on pattern strength
	Pattern    string  // the phrase pattern that matched
}

// Pattern strength tiers.
const (
	strongConfidence = 0.9
	weakConfidence   = 0.7
)

// decisionPattern pairs a compiled regex with its confidence and label.
type decisionPattern struct {
	re         *regexp.Regexp
	confidence float64
	label      string
}

// Patterns are compiled once at package init.
var decisionPatterns = []decisionPattern{
	// Strong patterns — explicit decision language.
	{
		re:         regexp.MustCompile(`(?i)\bwe decided\s+(?:to\s+)?(.+)`),
		confidence: strongConfidence,
		label:      "we decided",
	},
	{
		re:         regexp.MustCompile(`(?i)\bI chose\s+(?:to\s+)?(.+)`),
		confidence: strongConfidence,
		label:      "I chose",
	},
	// Weak patterns — informal decision language.
	{
		re:         regexp.MustCompile(`(?i)\blet'?s use\s+(.+)`),
		confidence: weakConfidence,
		label:      "let's use",
	},
	{
		re:         regexp.MustCompile(`(?i)\bgoing with\s+(.+)`),
		confidence: weakConfidence,
		label:      "going with",
	},
	{
		re:         regexp.MustCompile(`(?i)\bthe approach is\s+(.+)`),
		confidence: weakConfidence,
		label:      "the approach is",
	},
	{
		re:         regexp.MustCompile(`(?i)\bswitched to\s+(.+)`),
		confidence: weakConfidence,
		label:      "switched to",
	},
	{
		re:         regexp.MustCompile(`(?i)\bpicked\s+(.+)`),
		confidence: weakConfidence,
		label:      "picked",
	},
}

// MatchDecisions scans text for decision phrases and returns all matches.
// No LLM calls are made — extraction is purely regex-based (Req 4b.7).
func MatchDecisions(text string) []DecisionMatch {
	var matches []DecisionMatch
	lines := splitSentences(text)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		for _, p := range decisionPatterns {
			m := p.re.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			topic := extractTopic(m[1])
			if topic == "" {
				continue
			}
			matches = append(matches, DecisionMatch{
				Key:        topic,
				Value:      strings.TrimSpace(line),
				Confidence: p.confidence,
				Pattern:    p.label,
			})
			break // one pattern per sentence is enough
		}
	}
	return matches
}

// splitSentences breaks text on sentence-ending punctuation and newlines.
func splitSentences(text string) []string {
	// Split on newlines first, then on sentence-ending punctuation.
	var sentences []string
	for _, para := range strings.Split(text, "\n") {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		// Split on sentence boundaries (. ! ?) followed by space or end.
		parts := regexp.MustCompile(`[.!?]\s+`).Split(para, -1)
		sentences = append(sentences, parts...)
	}
	return sentences
}

// extractTopic derives a short key from the captured decision content.
// It takes the first few meaningful words as the topic identifier.
func extractTopic(raw string) string {
	raw = strings.TrimSpace(raw)
	// Strip trailing punctuation.
	raw = strings.TrimRight(raw, ".!?,;:")
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	words := strings.Fields(raw)
	// Use up to the first 5 words as the topic key.
	maxWords := 5
	if len(words) < maxWords {
		maxWords = len(words)
	}
	topic := strings.Join(words[:maxWords], " ")
	return strings.ToLower(topic)
}
