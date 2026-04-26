package verify

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// TranscriptEntry is a single message from a transcript file.
type TranscriptEntry struct {
	Role      string
	Content   string
	Timestamp time.Time
}

// rawClaudeEntry matches the Claude Code JSONL format.
type rawClaudeEntry struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
}

// rawCursorEntry matches the Cursor JSONL format.
type rawCursorEntry struct {
	Type      string `json:"type"`
	Text      string `json:"text"`
	Timestamp string `json:"timestamp"`
}

// ParseJSONL reads a .jsonl file and returns transcript entries.
// It handles both Claude Code and Cursor formats.
func ParseJSONL(path string) ([]TranscriptEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open transcript: %w", err)
	}
	defer f.Close()

	var entries []TranscriptEntry
	scanner := bufio.NewScanner(f)
	// Allow lines up to 1 MiB (transcripts can be large).
	scanner.Buffer(make([]byte, 1<<20), 1<<20)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		entry, err := parseLine(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNum, err)
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan transcript: %w", err)
	}
	return entries, nil
}

// parseLine tries Claude Code format first, then Cursor format.
func parseLine(data []byte) (TranscriptEntry, error) {
	// Peek at the keys to decide format.
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return TranscriptEntry{}, fmt.Errorf("invalid JSON: %w", err)
	}

	if _, ok := probe["role"]; ok {
		// Claude Code format.
		var raw rawClaudeEntry
		if err := json.Unmarshal(data, &raw); err != nil {
			return TranscriptEntry{}, fmt.Errorf("claude format: %w", err)
		}
		return TranscriptEntry{
			Role:      raw.Role,
			Content:   raw.Content,
			Timestamp: parseTimestamp(raw.Timestamp),
		}, nil
	}

	if _, ok := probe["type"]; ok {
		// Cursor format.
		var raw rawCursorEntry
		if err := json.Unmarshal(data, &raw); err != nil {
			return TranscriptEntry{}, fmt.Errorf("cursor format: %w", err)
		}
		return TranscriptEntry{
			Role:      raw.Type,
			Content:   raw.Text,
			Timestamp: parseTimestamp(raw.Timestamp),
		}, nil
	}

	return TranscriptEntry{}, fmt.Errorf("unrecognised transcript format")
}

// parseTimestamp parses an RFC3339 string; returns zero time on failure.
func parseTimestamp(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339, s)
	return t
}
