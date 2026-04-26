package router

import (
	"fmt"
	"sync"
)

// BudgetTracker enforces per-model spending limits.
type BudgetTracker struct {
	mu      sync.Mutex
	ceilings map[string]float64 // model → max USD
	spent    map[string]float64 // model → USD spent so far
}

// NewBudgetTracker creates a tracker with the given ceilings.
func NewBudgetTracker(ceilings map[string]float64) *BudgetTracker {
	return &BudgetTracker{
		ceilings: ceilings,
		spent:    make(map[string]float64),
	}
}

// Check returns an error if the model has reached its budget ceiling.
func (b *BudgetTracker) Check(model string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	ceiling, ok := b.ceilings[model]
	if !ok {
		return nil // no ceiling configured
	}
	if b.spent[model] >= ceiling {
		return fmt.Errorf("budget ceiling reached for %s ($%.2f)", model, ceiling)
	}
	return nil
}

// Record adds cost to the model's running total.
func (b *BudgetTracker) Record(model string, costUSD float64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.spent[model] += costUSD
}
