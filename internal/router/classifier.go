package router

import "strings"

// Complexity levels for task classification.
const (
	Low    = "low"
	Medium = "medium"
	High   = "high"
)

// ClassifyConfig holds token thresholds for complexity classification.
type ClassifyConfig struct {
	LowMaxTokens    int // tasks with ≤ this many tokens are "low"
	MediumMaxTokens int // tasks with ≤ this many tokens are "medium"; above is "high"
}

// DefaultClassifyConfig returns sensible defaults.
func DefaultClassifyConfig() ClassifyConfig {
	return ClassifyConfig{LowMaxTokens: 500, MediumMaxTokens: 2000}
}

// Classify returns the complexity level for the given content.
func Classify(content string, cfg ClassifyConfig) string {
	tokens := estimateTokens(content)
	switch {
	case tokens <= cfg.LowMaxTokens:
		return Low
	case tokens <= cfg.MediumMaxTokens:
		return Medium
	default:
		return High
	}
}

// estimateTokens approximates token count as word count.
func estimateTokens(content string) int {
	return len(strings.Fields(content))
}
