package trace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SearchResult is a single match from a trace search.
type SearchResult struct {
	SessionID string `json:"session_id"`
	File      string `json:"file"`
}

// Search scans all JSONL trace files in tracesDir for lines containing query.
func Search(tracesDir, query string) ([]SearchResult, error) {
	entries, err := os.ReadDir(tracesDir)
	if err != nil {
		return nil, fmt.Errorf("read traces dir: %w", err)
	}

	var results []SearchResult
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".jsonl" {
			continue
		}
		path := filepath.Join(tracesDir, e.Name())
		if matchesQuery(path, query) {
			sessionID := strings.TrimSuffix(e.Name(), ".jsonl")
			results = append(results, SearchResult{SessionID: sessionID, File: path})
		}
	}
	return results, nil
}

// matchesQuery returns true if any line in the file contains query (case-insensitive).
func matchesQuery(path, query string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(data)), strings.ToLower(query))
}
