package policy

import (
	"path/filepath"
	"sync"

	"github.com/ojuschugh1/aura/pkg/types"
)

// Engine evaluates actions against a PolicyConfig.
type Engine struct {
	mu  sync.RWMutex
	cfg *types.PolicyConfig
}

// New creates an Engine with the given config.
func New(cfg *types.PolicyConfig) *Engine {
	return &Engine{cfg: cfg}
}

// Reload hot-swaps the config under a write lock.
func (e *Engine) Reload(cfg *types.PolicyConfig) {
	e.mu.Lock()
	e.cfg = cfg
	e.mu.Unlock()
}

// Evaluate returns the disposition for an action category and target path.
// Path-based overrides are checked first; category rules are the fallback.
// Returns "require-approval" when no rule matches.
func (e *Engine) Evaluate(actionCategory, targetPath string) string {
	e.mu.RLock()
	cfg := e.cfg
	e.mu.RUnlock()

	// Check path-based overrides first.
	for _, r := range cfg.Overrides {
		if r.PathPattern != "" {
			matched, err := filepath.Match(r.PathPattern, targetPath)
			if err == nil && matched && r.Category == actionCategory {
				return r.Disposition
			}
		}
	}

	// Fall back to category-level rules.
	for _, r := range cfg.Rules {
		if r.Category == actionCategory {
			return r.Disposition
		}
	}

	return "require-approval"
}
