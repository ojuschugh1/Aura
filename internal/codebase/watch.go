package codebase

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FileState tracks the hash and mod time of a file.
type FileState struct {
	Path    string    `json:"path"`
	Hash    string    `json:"hash"`
	ModTime time.Time `json:"mod_time"`
	Lines   int       `json:"lines"`
}

// DetectChanges compares current file states against stored states and returns changed files.
func DetectChanges(projectDir string, stored []FileState) (added, modified, deleted []string, err error) {
	storedMap := make(map[string]FileState)
	for _, s := range stored {
		storedMap[s.Path] = s
	}

	seen := make(map[string]bool)

	err = filepath.Walk(projectDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if skipDirs[base] || strings.HasPrefix(base, ".") {
				return filepath.SkipDir
			}
			return nil
		}

		rel, _ := filepath.Rel(projectDir, path)
		seen[rel] = true

		prev, existed := storedMap[rel]
		if !existed {
			added = append(added, rel)
			return nil
		}

		// Check if modified by hash.
		hash, hashErr := hashFile(path)
		if hashErr != nil {
			return nil
		}
		if hash != prev.Hash {
			modified = append(modified, rel)
		}
		return nil
	})
	if err != nil {
		return nil, nil, nil, err
	}

	// Find deleted files.
	for path := range storedMap {
		if !seen[path] {
			deleted = append(deleted, path)
		}
	}

	return added, modified, deleted, nil
}

// SnapshotFiles creates FileState entries for all files in a project directory.
func SnapshotFiles(projectDir string) ([]FileState, error) {
	var states []FileState

	err := filepath.Walk(projectDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if skipDirs[base] || strings.HasPrefix(base, ".") {
				return filepath.SkipDir
			}
			return nil
		}

		rel, _ := filepath.Rel(projectDir, path)
		hash, _ := hashFile(path)
		lines, _ := countLines(path)

		states = append(states, FileState{
			Path:    rel,
			Hash:    hash,
			ModTime: info.ModTime(),
			Lines:   lines,
		})
		return nil
	})
	return states, err
}

// hashFile returns the SHA-256 hex digest of the file at path.
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
