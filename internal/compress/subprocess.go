package compress

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/ojuschugh1/aura/internal/subprocess"
)

// sqzResult holds parsed output from the sqz binary.
type sqzResult struct {
	Compressed     string
	OriginalTokens int
	CompressTokens int
	ReductionPct   float64
}

// headerRe matches: [sqz] 8/10 tokens (20% reduction) [stdin]
var headerRe = regexp.MustCompile(`\[sqz\]\s+(\d+)/(\d+)\s+tokens\s+\((\d+)%\s+reduction\)`)

// dedupRe matches: [sqz] dedup hit: §ref:HASH§ (LN)
var dedupRe = regexp.MustCompile(`\[sqz\]\s+dedup hit:`)

// runSqz invokes the sqz binary and parses its output.
// sqz writes the header to stderr and compressed content to stdout.
func runSqz(content string) (*sqzResult, error) {
	binPath, err := subprocess.ResolveBinary("sqz")
	if err != nil {
		return nil, fmt.Errorf("sqz not available: %w", err)
	}

	runner := &subprocess.Runner{BinaryPath: binPath}
	res, err := runner.Run(context.Background(), []string{"compress"}, bytes.NewBufferString(content))
	// sqz exits 0 but we still get stderr with the header — ignore the error if exit code is 0
	if err != nil && res != nil && res.ExitCode != 0 {
		return nil, fmt.Errorf("sqz execution: %w", err)
	}

	body := strings.TrimRight(string(res.Stdout), "\n\r ")
	header := strings.TrimSpace(string(res.Stderr))

	// Handle dedup hit from stderr
	if dedupRe.MatchString(header) {
		origTokens := estimateTokens(content)
		refTokens := estimateTokens(body)
		pct := 0.0
		if origTokens > 0 {
			pct = float64(origTokens-refTokens) / float64(origTokens) * 100
		}
		return &sqzResult{
			Compressed:     body,
			OriginalTokens: origTokens,
			CompressTokens: refTokens,
			ReductionPct:   pct,
		}, nil
	}

	// Parse normal header from stderr: [sqz] compressed/original tokens (N% reduction) [source]
	m := headerRe.FindStringSubmatch(header)
	if m == nil {
		// Fallback: sqz ran but we can't parse the header — return content unchanged
		tokens := estimateTokens(content)
		return &sqzResult{
			Compressed:     body,
			OriginalTokens: tokens,
			CompressTokens: tokens,
			ReductionPct:   0,
		}, nil
	}

	compressed, _ := strconv.Atoi(m[1])
	original, _ := strconv.Atoi(m[2])
	reduction, _ := strconv.ParseFloat(m[3], 64)

	return &sqzResult{
		Compressed:     body,
		OriginalTokens: original,
		CompressTokens: compressed,
		ReductionPct:   reduction,
	}, nil
}
