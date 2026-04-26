package compress

import (
	"database/sql"
	"fmt"
	"strings"
)

// Result holds the output of a compression operation.
type Result struct {
	OriginalTokens   int
	CompressedTokens int
	ReductionPct     float64
	Compressed       string
	Deduplicated     bool // true if content was already in the dedup cache
}

// Engine orchestrates compression and deduplication.
type Engine struct {
	dedup *DedupCache
}

// New creates an Engine backed by the given database.
func New(db *sql.DB) *Engine {
	return &Engine{dedup: NewDedupCache(db)}
}

// Compact compresses content via sqz, skipping if already seen in the dedup cache.
func (e *Engine) Compact(content string) (*Result, error) {
	seen, err := e.dedup.Seen(content)
	if err != nil {
		return nil, fmt.Errorf("dedup check: %w", err)
	}
	if seen {
		tokens := estimateTokens(content)
		return &Result{
			OriginalTokens:   tokens,
			CompressedTokens: tokens,
			ReductionPct:     0,
			Compressed:       content,
			Deduplicated:     true,
		}, nil
	}

	sqzRes, err := runSqz(content)
	if err != nil {
		// Graceful degradation: return content unchanged if sqz is unavailable.
		tokens := estimateTokens(content)
		return &Result{
			OriginalTokens:   tokens,
			CompressedTokens: tokens,
			ReductionPct:     0,
			Compressed:       content,
		}, fmt.Errorf("compression unavailable: %w", err)
	}

	_ = e.dedup.Record(content, sqzRes.OriginalTokens)

	return &Result{
		OriginalTokens:   sqzRes.OriginalTokens,
		CompressedTokens: sqzRes.CompressTokens,
		ReductionPct:     sqzRes.ReductionPct,
		Compressed:       sqzRes.Compressed,
	}, nil
}

// estimateTokens approximates token count as word count (rough heuristic).
func estimateTokens(content string) int {
	return len(strings.Fields(content))
}
