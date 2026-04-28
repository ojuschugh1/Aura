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
	veryStrongConfidence = 0.95
	strongConfidence     = 0.85
	mediumConfidence     = 0.7
	weakConfidence       = 0.55
)

// decisionPattern pairs a compiled regex with its confidence and label.
type decisionPattern struct {
	re         *regexp.Regexp
	confidence float64
	label      string
	// group determines which capture group holds the topic.
	// Default is 1. For patterns with subject-first structure (X uses Y),
	// group 1 is subject and group 2 is object.
	keyGroup int
	// valueGroup, if set, uses this group for the value instead of the full match.
	valueGroup int
}

var decisionPatterns = []decisionPattern{
	// === VERY STRONG: explicit decision phrases ===
	{
		re:         regexp.MustCompile(`(?i)\bwe decided\s+(?:to\s+)?(.+)`),
		confidence: veryStrongConfidence,
		label:      "we decided",
	},
	{
		re:         regexp.MustCompile(`(?i)\bI decided\s+(?:to\s+)?(.+)`),
		confidence: veryStrongConfidence,
		label:      "I decided",
	},
	{
		re:         regexp.MustCompile(`(?i)\bdecision[:\s]+(.+)`),
		confidence: veryStrongConfidence,
		label:      "decision:",
	},
	{
		re:         regexp.MustCompile(`(?i)\bI chose\s+(?:to\s+)?(.+)`),
		confidence: veryStrongConfidence,
		label:      "I chose",
	},
	{
		re:         regexp.MustCompile(`(?i)\bwe chose\s+(?:to\s+)?(.+)`),
		confidence: veryStrongConfidence,
		label:      "we chose",
	},
	{
		re:         regexp.MustCompile(`(?i)\bdecided to\s+(.+)`),
		confidence: veryStrongConfidence,
		label:      "decided to",
	},

	// === STRONG: technology/stack declarations ===
	{
		re:         regexp.MustCompile(`(?i)\busing\s+([A-Z][\w.-]+(?:\s+[\w.-]+){0,3})\s+(?:for|as|to)\s+(.+)`),
		confidence: strongConfidence,
		label:      "using X for Y",
	},
	{
		re:         regexp.MustCompile(`(?i)\bwe(?:'re|\s+are)\s+using\s+(.+)`),
		confidence: strongConfidence,
		label:      "we're using",
	},
	{
		re:         regexp.MustCompile(`(?i)\bI(?:'m|\s+am)\s+using\s+(.+)`),
		confidence: strongConfidence,
		label:      "I'm using",
	},
	{
		re:         regexp.MustCompile(`(?i)\bthe\s+(?:stack|backend|frontend|database|cache|queue|api|architecture)\s+is\s+(.+)`),
		confidence: strongConfidence,
		label:      "the X is",
	},
	{
		re:         regexp.MustCompile(`(?i)\bgoing with\s+(.+)`),
		confidence: strongConfidence,
		label:      "going with",
	},
	{
		re:         regexp.MustCompile(`(?i)\bswitched\s+(?:to|from)\s+(.+)`),
		confidence: strongConfidence,
		label:      "switched to",
	},
	{
		re:         regexp.MustCompile(`(?i)\bmigrated\s+(?:to|from)\s+(.+)`),
		confidence: strongConfidence,
		label:      "migrated",
	},
	{
		re:         regexp.MustCompile(`(?i)\badopted\s+(.+)`),
		confidence: strongConfidence,
		label:      "adopted",
	},

	// === MEDIUM: configuration and architecture statements ===
	{
		re:         regexp.MustCompile(`(?i)\blet'?s use\s+(.+)`),
		confidence: mediumConfidence,
		label:      "let's use",
	},
	{
		re:         regexp.MustCompile(`(?i)\bthe approach is\s+(.+)`),
		confidence: mediumConfidence,
		label:      "the approach is",
	},
	{
		re:         regexp.MustCompile(`(?i)\bpicked\s+(.+)`),
		confidence: mediumConfidence,
		label:      "picked",
	},
	{
		re:         regexp.MustCompile(`(?i)\bselected\s+(.+)`),
		confidence: mediumConfidence,
		label:      "selected",
	},
	{
		re:         regexp.MustCompile(`(?i)\bconfigured\s+(.+)\s+(?:to|for|with)\s+(.+)`),
		confidence: mediumConfidence,
		label:      "configured X",
	},
	{
		re:         regexp.MustCompile(`(?i)\bset\s+up\s+(.+)\s+(?:to|for|with)\s+(.+)`),
		confidence: mediumConfidence,
		label:      "set up X",
	},
	{
		re:         regexp.MustCompile(`(?i)\bimplemented\s+(.+)\s+(?:using|with|via)\s+(.+)`),
		confidence: mediumConfidence,
		label:      "implemented X using Y",
	},

	// === WEAK: softer signals ===
	{
		re:         regexp.MustCompile(`(?i)\bprefer\s+(.+)\s+over\s+(.+)`),
		confidence: weakConfidence,
		label:      "prefer X over Y",
	},
	{
		re:         regexp.MustCompile(`(?i)\bshould\s+(?:use|be)\s+(.+)`),
		confidence: weakConfidence,
		label:      "should use",
	},
	{
		re:         regexp.MustCompile(`(?i)\bwill\s+(?:use|be)\s+(.+)`),
		confidence: weakConfidence,
		label:      "will use",
	},
}

// keywordsForTopic are high-signal words that indicate the sentence contains
// a likely decision or architecture fact worth capturing.
var signalKeywords = []string{
	"database", "postgres", "postgresql", "mysql", "mongodb", "redis", "sqlite",
	"backend", "frontend", "framework", "library", "language", "runtime",
	"authentication", "auth", "jwt", "oauth", "session", "token",
	"deploy", "deployment", "infrastructure", "kubernetes", "docker", "aws", "gcp", "azure",
	"architecture", "pattern", "design", "microservice", "monolith",
	"api", "rest", "graphql", "grpc",
	"test", "testing", "framework",
	"cache", "caching", "queue", "messaging",
	"monitoring", "logging", "metrics",
	"version", "go", "python", "rust", "typescript", "javascript",
}

// MatchDecisions scans text for decision phrases and returns all matches.
// No LLM calls are made — extraction is purely regex-based.
func MatchDecisions(text string) []DecisionMatch {
	var matches []DecisionMatch
	lines := splitSentences(text)

	seenKeys := make(map[string]float64) // track best confidence per key

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) < 8 || len(line) > 400 {
			continue
		}

		for _, p := range decisionPatterns {
			m := p.re.FindStringSubmatch(line)
			if m == nil {
				continue
			}

			keyGroup := p.keyGroup
			if keyGroup == 0 {
				keyGroup = 1
			}
			if len(m) <= keyGroup {
				continue
			}

			topic := extractTopic(m[keyGroup])
			if topic == "" {
				continue
			}

			// Boost confidence if the sentence contains signal keywords.
			confidence := p.confidence
			if containsSignalKeyword(line) {
				confidence += 0.05
				if confidence > 1.0 {
					confidence = 1.0
				}
			}

			// Deduplicate: keep the highest-confidence match for each key.
			if existing, ok := seenKeys[topic]; ok && existing >= confidence {
				break
			}
			seenKeys[topic] = confidence

			matches = append(matches, DecisionMatch{
				Key:        topic,
				Value:      strings.TrimSpace(line),
				Confidence: confidence,
				Pattern:    p.label,
			})
			break // one pattern per sentence is enough
		}
	}

	// Filter out duplicate keys, keeping the best match.
	best := make(map[string]DecisionMatch)
	for _, m := range matches {
		if existing, ok := best[m.Key]; !ok || m.Confidence > existing.Confidence {
			best[m.Key] = m
		}
	}
	var result []DecisionMatch
	for _, m := range best {
		result = append(result, m)
	}
	return result
}

// containsSignalKeyword returns true if the text contains any signal keyword.
func containsSignalKeyword(text string) bool {
	lower := strings.ToLower(text)
	for _, kw := range signalKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// splitSentences breaks text on sentence-ending punctuation and newlines.
func splitSentences(text string) []string {
	var sentences []string
	for _, para := range strings.Split(text, "\n") {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		parts := regexp.MustCompile(`[.!?]\s+`).Split(para, -1)
		sentences = append(sentences, parts...)
	}
	return sentences
}

// extractTopic derives a short key from the captured decision content.
func extractTopic(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimRight(raw, ".!?,;:")
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	// Strip leading filler words.
	words := strings.Fields(raw)
	fillers := map[string]bool{
		"to": true, "a": true, "an": true, "the": true,
		"some": true, "any": true, "this": true, "that": true,
	}
	for len(words) > 0 && fillers[strings.ToLower(words[0])] {
		words = words[1:]
	}
	if len(words) == 0 {
		return ""
	}

	// Use up to the first 5 meaningful words as the topic key.
	maxWords := 5
	if len(words) < maxWords {
		maxWords = len(words)
	}
	topic := strings.Join(words[:maxWords], " ")
	return strings.ToLower(topic)
}
