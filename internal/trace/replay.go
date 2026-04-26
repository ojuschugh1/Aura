package trace

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ojuschugh1/aura/pkg/types"
)

// DiffEntry describes a difference between the original trace and the replay.
type DiffEntry struct {
	ActionType string `json:"action_type"`
	Target     string `json:"target"`
	Original   string `json:"original"`
	Replay     string `json:"replay"`
}

// ReplayResult holds the outcome of replaying a trace.
type ReplayResult struct {
	SessionID string      `json:"session_id"`
	Total     int         `json:"total"`
	Matched   int         `json:"matched"`
	Diffs     []DiffEntry `json:"diffs"`
}

// Replay loads a trace file and checks each file-based action against the current filesystem.
// It returns a diff report showing which outcomes changed.
func Replay(tracesDir, sessionID, projectRoot string) (*ReplayResult, error) {
	path := filepath.Join(tracesDir, sessionID+".jsonl")
	entries, err := loadTrace(path)
	if err != nil {
		return nil, fmt.Errorf("load trace: %w", err)
	}

	result := &ReplayResult{SessionID: sessionID, Total: len(entries)}
	for _, e := range entries {
		replayOutcome := replayEntry(e, projectRoot)
		if replayOutcome == e.Outcome {
			result.Matched++
		} else {
			result.Diffs = append(result.Diffs, DiffEntry{
				ActionType: e.ActionType,
				Target:     e.Target,
				Original:   e.Outcome,
				Replay:     replayOutcome,
			})
		}
	}
	return result, nil
}

// loadTrace reads all TraceEntry lines from a JSONL file.
func loadTrace(path string) ([]types.TraceEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []types.TraceEntry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var e types.TraceEntry
		if err := json.Unmarshal(line, &e); err != nil {
			continue // skip malformed lines
		}
		entries = append(entries, e)
	}
	return entries, scanner.Err()
}

// replayEntry checks whether a file-based action still produces the same outcome.
func replayEntry(e types.TraceEntry, root string) string {
	switch e.ActionType {
	case "file_created", "file_write":
		path := e.Target
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
		if _, err := os.Stat(path); err == nil {
			return "success"
		}
		return "failure"
	default:
		// For non-file actions, assume the outcome is unchanged.
		return e.Outcome
	}
}
