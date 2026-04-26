package policy

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ojuschugh1/aura/pkg/types"
)

// --- DefaultConfig ----------------------------------------------------------

func TestDefaultConfig_ReadAutoApprove(t *testing.T) {
	cfg := DefaultConfig()
	e := New(&cfg)
	got := e.Evaluate("read", "/any/path")
	if got != "auto-approve" {
		t.Errorf("read disposition = %q, want auto-approve", got)
	}
}

func TestDefaultConfig_WriteRequireApproval(t *testing.T) {
	cfg := DefaultConfig()
	e := New(&cfg)
	got := e.Evaluate("write", "/any/path")
	if got != "require-approval" {
		t.Errorf("write disposition = %q, want require-approval", got)
	}
}

func TestDefaultConfig_ExecuteRequireApproval(t *testing.T) {
	cfg := DefaultConfig()
	e := New(&cfg)
	got := e.Evaluate("execute", "/any/path")
	if got != "require-approval" {
		t.Errorf("execute disposition = %q, want require-approval", got)
	}
}

func TestDefaultConfig_NetworkDeny(t *testing.T) {
	cfg := DefaultConfig()
	e := New(&cfg)
	got := e.Evaluate("network", "https://example.com")
	if got != "deny" {
		t.Errorf("network disposition = %q, want deny", got)
	}
}

func TestDefaultConfig_UnknownCategoryFallsBackToRequireApproval(t *testing.T) {
	cfg := DefaultConfig()
	e := New(&cfg)
	got := e.Evaluate("unknown-category", "/any/path")
	if got != "require-approval" {
		t.Errorf("unknown category disposition = %q, want require-approval", got)
	}
}

// --- Evaluate with path-based overrides -------------------------------------

func TestEvaluate_PathOverrideTakesPrecedence(t *testing.T) {
	cfg := types.PolicyConfig{
		Rules: []types.PolicyRule{
			{Category: "write", Disposition: "require-approval"},
		},
		Overrides: []types.PolicyRule{
			{Category: "write", PathPattern: "src/test/*", Disposition: "auto-approve"},
		},
	}
	e := New(&cfg)

	got := e.Evaluate("write", "src/test/foo.go")
	if got != "auto-approve" {
		t.Errorf("write to src/test/foo.go = %q, want auto-approve (override)", got)
	}
}

func TestEvaluate_PathOverrideDoesNotMatchOtherPaths(t *testing.T) {
	cfg := types.PolicyConfig{
		Rules: []types.PolicyRule{
			{Category: "write", Disposition: "require-approval"},
		},
		Overrides: []types.PolicyRule{
			{Category: "write", PathPattern: "src/test/*", Disposition: "auto-approve"},
		},
	}
	e := New(&cfg)

	got := e.Evaluate("write", "src/main/app.go")
	if got != "require-approval" {
		t.Errorf("write to src/main/app.go = %q, want require-approval (no override match)", got)
	}
}

func TestEvaluate_OverrideOnlyMatchesCorrectCategory(t *testing.T) {
	cfg := types.PolicyConfig{
		Rules: []types.PolicyRule{
			{Category: "read", Disposition: "auto-approve"},
			{Category: "execute", Disposition: "require-approval"},
		},
		Overrides: []types.PolicyRule{
			{Category: "write", PathPattern: "src/test/*", Disposition: "auto-approve"},
		},
	}
	e := New(&cfg)

	// "execute" should not match the write override even if path matches.
	got := e.Evaluate("execute", "src/test/foo.go")
	if got != "require-approval" {
		t.Errorf("execute to src/test/foo.go = %q, want require-approval", got)
	}
}

// --- Evaluate performance ---------------------------------------------------

func TestEvaluate_CompletesWithin10ms(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Overrides = []types.PolicyRule{
		{Category: "write", PathPattern: "src/test/*", Disposition: "auto-approve"},
		{Category: "write", PathPattern: "tmp/*", Disposition: "auto-approve"},
	}
	e := New(&cfg)

	start := time.Now()
	for i := 0; i < 1000; i++ {
		e.Evaluate("write", "src/main/app.go")
	}
	elapsed := time.Since(start)

	// 1000 evaluations should complete well within 10ms total (10µs each).
	if avg := elapsed / 1000; avg > 10*time.Millisecond {
		t.Errorf("average evaluation time = %v, want < 10ms", avg)
	}
}

// --- Reload -----------------------------------------------------------------

func TestReload_SwapsConfig(t *testing.T) {
	cfg := DefaultConfig()
	e := New(&cfg)

	// Initially network is deny.
	if got := e.Evaluate("network", ""); got != "deny" {
		t.Fatalf("before reload: network = %q, want deny", got)
	}

	// Reload with a permissive config.
	newCfg := types.PolicyConfig{
		Rules: []types.PolicyRule{
			{Category: "network", Disposition: "auto-approve"},
		},
	}
	e.Reload(&newCfg)

	if got := e.Evaluate("network", ""); got != "auto-approve" {
		t.Errorf("after reload: network = %q, want auto-approve", got)
	}
}

// --- Load from TOML ---------------------------------------------------------

func TestLoad_MissingFileReturnsDefault(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "nonexistent.toml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	e := New(cfg)
	if got := e.Evaluate("read", ""); got != "auto-approve" {
		t.Errorf("read = %q, want auto-approve (default)", got)
	}
	if got := e.Evaluate("network", ""); got != "deny" {
		t.Errorf("network = %q, want deny (default)", got)
	}
}

func TestLoad_ParsesTOMLFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.toml")

	tomlContent := `
[[rules]]
category = "read"
disposition = "auto-approve"

[[rules]]
category = "write"
disposition = "deny"

[[overrides]]
category = "write"
path = "tmp/*"
disposition = "auto-approve"
`
	if err := os.WriteFile(path, []byte(tomlContent), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	e := New(cfg)

	if got := e.Evaluate("read", "any"); got != "auto-approve" {
		t.Errorf("read = %q, want auto-approve", got)
	}
	if got := e.Evaluate("write", "src/main.go"); got != "deny" {
		t.Errorf("write to src/main.go = %q, want deny", got)
	}
	if got := e.Evaluate("write", "tmp/scratch.txt"); got != "auto-approve" {
		t.Errorf("write to tmp/scratch.txt = %q, want auto-approve (override)", got)
	}
}

func TestLoad_InvalidTOMLReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.toml")
	if err := os.WriteFile(path, []byte("this is not valid toml [[["), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid TOML, got nil")
	}
}

// --- Watch (hot-reload) -----------------------------------------------------

func TestWatch_DetectsFileChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.toml")

	// Write initial policy.
	initial := `
[[rules]]
category = "network"
disposition = "deny"
`
	if err := os.WriteFile(path, []byte(initial), 0644); err != nil {
		t.Fatalf("WriteFile initial: %v", err)
	}

	reloaded := make(chan *types.PolicyConfig, 1)
	stop, err := Watch(path, func(cfg *types.PolicyConfig) {
		select {
		case reloaded <- cfg:
		default:
		}
	})
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}
	defer stop()

	// Give the watcher time to start.
	time.Sleep(200 * time.Millisecond)

	// Write updated policy.
	updated := `
[[rules]]
category = "network"
disposition = "auto-approve"
`
	if err := os.WriteFile(path, []byte(updated), 0644); err != nil {
		t.Fatalf("WriteFile updated: %v", err)
	}

	select {
	case cfg := <-reloaded:
		if len(cfg.Rules) == 0 {
			t.Fatal("reloaded config has no rules")
		}
		if cfg.Rules[0].Disposition != "auto-approve" {
			t.Errorf("reloaded disposition = %q, want auto-approve", cfg.Rules[0].Disposition)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("policy change not detected within 5 seconds")
	}
}
