package subprocess

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed deps.json
var depsJSON []byte

// BinaryInfo describes a required Rust binary dependency.
type BinaryInfo struct {
	Name      string            `json:"name"`
	Repo      string            `json:"repo"`
	Version   string            `json:"version"`
	Checksums map[string]string `json:"checksums"` // platform key → SHA-256 hex
}

// LoadDeps parses the embedded deps.json manifest.
func LoadDeps() ([]BinaryInfo, error) {
	var deps []BinaryInfo
	if err := json.Unmarshal(depsJSON, &deps); err != nil {
		return nil, fmt.Errorf("parse deps manifest: %w", err)
	}
	return deps, nil
}

// FindDep returns the BinaryInfo for the named binary, or an error if not found.
func FindDep(name string) (*BinaryInfo, error) {
	deps, err := LoadDeps()
	if err != nil {
		return nil, err
	}
	for _, d := range deps {
		if d.Name == name {
			return &d, nil
		}
	}
	return nil, fmt.Errorf("unknown dependency %q", name)
}
