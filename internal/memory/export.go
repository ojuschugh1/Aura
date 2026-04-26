package memory

import (
	"encoding/json"
	"fmt"
	"os"
)

// Export writes all memory entries to a JSON file at path.
func (s *Store) Export(path string) error {
	entries, err := s.List(ListFilter{})
	if err != nil {
		return fmt.Errorf("export: list entries: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("export: create file: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(entries); err != nil {
		return fmt.Errorf("export: encode: %w", err)
	}
	return nil
}

// Import reads entries from a JSON file and inserts them into the store.
// Existing entries with the same key are overwritten.
func (s *Store) Import(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("import: read file: %w", err)
	}

	var entries []struct {
		Key        string `json:"key"`
		Value      string `json:"value"`
		SourceTool string `json:"source_tool"`
		SessionID  string `json:"session_id"`
	}
	if err := json.Unmarshal(data, &entries); err != nil {
		return 0, fmt.Errorf("import: decode: %w", err)
	}

	for _, e := range entries {
		if _, err := s.Add(e.Key, e.Value, e.SourceTool, e.SessionID); err != nil {
			return 0, fmt.Errorf("import: add %q: %w", e.Key, err)
		}
	}
	return len(entries), nil
}
