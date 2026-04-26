package memory

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"strings"
)

// Reconcile checks memory entries whose keys look like file paths.
// If the file on disk has changed since the entry was stored (hash mismatch),
// the entry is deleted so stale context is not served to agents.
// Returns the number of invalidated entries.
func (s *Store) Reconcile() (int, error) {
	entries, err := s.List(ListFilter{})
	if err != nil {
		return 0, fmt.Errorf("reconcile: list: %w", err)
	}

	var invalidated int
	for _, e := range entries {
		if !looksLikePath(e.Key) {
			continue
		}
		diskHash, err := hashFile(e.Key)
		if err != nil {
			// File no longer exists — invalidate the entry.
			if os.IsNotExist(err) {
				_ = s.Delete(e.Key)
				invalidated++
			}
			continue
		}
		if diskHash != e.ContentHash {
			_ = s.Delete(e.Key)
			invalidated++
		}
	}
	return invalidated, nil
}

// looksLikePath returns true if key starts with / or ./ or contains a path separator.
func looksLikePath(key string) bool {
	return strings.HasPrefix(key, "/") ||
		strings.HasPrefix(key, "./") ||
		strings.HasPrefix(key, "../") ||
		strings.Contains(key, string(os.PathSeparator))
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
