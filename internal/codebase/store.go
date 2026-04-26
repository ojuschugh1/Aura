package codebase

import (
	"fmt"
	"strings"

	"github.com/ojuschugh1/aura/internal/memory"
)

// StoreResult persists scan results as memory entries with the "codebase-scan" source_tool.
func StoreResult(store *memory.Store, result *ScanResult, sessionID string) (int, error) {
	entries := map[string]string{
		"codebase.languages":    strings.Join(result.Languages, ", "),
		"codebase.entry_points": strings.Join(result.EntryPoints, ", "),
		"codebase.packages":     strings.Join(result.Packages, ", "),
		"codebase.dependencies": strings.Join(result.Dependencies, ", "),
		"codebase.stats":        fmt.Sprintf("%d files, %d lines", result.FileCount, result.TotalLines),
	}

	stored := 0
	for key, value := range entries {
		if value == "" {
			value = "(none)"
		}
		_, err := store.AddWithMeta(key, value, "codebase-scan", sessionID, 1.0, nil)
		if err != nil {
			return stored, fmt.Errorf("store %s: %w", key, err)
		}
		stored++
	}
	return stored, nil
}

// ReconcileCodebase re-scans the project directory and updates codebase.* memory entries.
func ReconcileCodebase(store *memory.Store, projectDir, sessionID string) (int, error) {
	result, err := Scan(projectDir)
	if err != nil {
		return 0, fmt.Errorf("reconcile codebase: scan: %w", err)
	}
	return StoreResult(store, result, sessionID)
}
