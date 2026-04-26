package autocapture

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ojuschugh1/aura/internal/memory"
	"github.com/ojuschugh1/aura/internal/verify"
)

// CaptureConfig controls auto-capture behaviour.
type CaptureConfig struct {
	Enabled       bool    // whether auto-capture is active (default: true)
	MinConfidence float64 // minimum confidence to capture a match (default: 0.0 = capture all)
}

// DefaultCaptureConfig returns the default configuration with auto-capture enabled
// and no confidence threshold (capture all matches).
func DefaultCaptureConfig() CaptureConfig {
	return CaptureConfig{
		Enabled:       true,
		MinConfidence: 0.0,
	}
}

// CaptureEngine extracts decisions from session transcripts and stores them
// in the memory store. No LLM calls are made (Req 4b.7).
type CaptureEngine struct {
	store *memory.Store
	cfg   CaptureConfig
}

// NewCaptureEngine creates a CaptureEngine backed by the given memory store.
func NewCaptureEngine(store *memory.Store, cfg CaptureConfig) *CaptureEngine {
	return &CaptureEngine{
		store: store,
		cfg:   cfg,
	}
}

// ProcessTranscript parses a session transcript file and captures decisions
// into the memory store. It supports JSONL files (parsed via verify.ParseJSONL).
// Returns the number of entries captured (inserted or updated).
func (e *CaptureEngine) ProcessTranscript(sessionID string, transcriptPath string) (int, error) {
	if !e.cfg.Enabled {
		return 0, nil
	}

	ext := strings.ToLower(filepath.Ext(transcriptPath))

	var text string
	switch ext {
	case ".jsonl":
		entries, err := verify.ParseJSONL(transcriptPath)
		if err != nil {
			return 0, fmt.Errorf("parse transcript: %w", err)
		}
		text = concatenateEntries(entries)
	case ".md", ".markdown", ".txt":
		// For markdown/text files, delegate to ProcessText after reading.
		// However, ProcessTranscript should handle the file reading itself.
		return 0, fmt.Errorf("unsupported transcript format %q; use ProcessText for raw text", ext)
	default:
		return 0, fmt.Errorf("unsupported transcript format: %q", ext)
	}

	return e.captureFromText(sessionID, text)
}

// ProcessText processes raw text (e.g. markdown transcript content) and captures
// decisions into the memory store. Returns the number of entries captured.
func (e *CaptureEngine) ProcessText(sessionID string, text string) (int, error) {
	if !e.cfg.Enabled {
		return 0, nil
	}
	return e.captureFromText(sessionID, text)
}

// captureFromText runs decision matching on text and stores results.
func (e *CaptureEngine) captureFromText(sessionID string, text string) (int, error) {
	matches := MatchDecisions(text)

	captured := 0
	seen := make(map[string]bool) // deduplicate within the same batch
	for _, m := range matches {
		if m.Confidence < e.cfg.MinConfidence {
			continue
		}

		// Deduplicate within the current batch — keep the first match per key.
		if seen[m.Key] {
			continue
		}
		seen[m.Key] = true

		// Upsert into the memory store with confidence from the match.
		_, err := e.store.AddWithMeta(m.Key, m.Value, "auto-capture", sessionID, m.Confidence, nil)
		if err != nil {
			return captured, fmt.Errorf("store decision %q: %w", m.Key, err)
		}
		captured++
	}
	return captured, nil
}

// concatenateEntries joins all transcript entry content into a single string,
// separated by newlines. Only assistant/message content is included.
func concatenateEntries(entries []verify.TranscriptEntry) string {
	var b strings.Builder
	for _, e := range entries {
		role := strings.ToLower(e.Role)
		if role == "assistant" || role == "message" {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(e.Content)
		}
	}
	return b.String()
}
