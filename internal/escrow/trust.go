package escrow

import (
	"path/filepath"
	"sync"
	"time"
)

// TrustWindow holds an active trust grant (time-boxed or path-scoped).
type TrustWindow struct {
	mu        sync.RWMutex
	expiresAt time.Time
	path      string // empty means all paths
}

// Grant activates a trust window for the given duration and optional path scope.
func (tw *TrustWindow) Grant(duration time.Duration, path string) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	tw.expiresAt = time.Now().Add(duration)
	tw.path = path
}

// IsActive returns true if the trust window covers the given target path.
func (tw *TrustWindow) IsActive(targetPath string) bool {
	tw.mu.RLock()
	defer tw.mu.RUnlock()

	if time.Now().After(tw.expiresAt) {
		return false
	}
	if tw.path == "" {
		return true
	}
	// Check if targetPath is within the trusted directory.
	rel, err := filepath.Rel(tw.path, targetPath)
	if err != nil {
		return false
	}
	return rel != ".." && len(rel) > 0 && rel[0] != '.'
}

// Revoke clears the trust window immediately.
func (tw *TrustWindow) Revoke() {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	tw.expiresAt = time.Time{}
}
