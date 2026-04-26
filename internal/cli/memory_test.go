package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ojuschugh1/aura/internal/db"
	"github.com/ojuschugh1/aura/internal/memory"
	"github.com/ojuschugh1/aura/pkg/types"
)

// setupTestAuraDir creates a temp directory with an initialized SQLite DB and
// returns the path plus a cleanup function.
func setupTestAuraDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "aura.db")
	d, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	if err := db.RunMigrations(d); err != nil {
		d.Close()
		t.Fatalf("RunMigrations: %v", err)
	}
	d.Close()
	return dir
}

// seedEntries adds entries to the store for testing.
func seedEntries(t *testing.T, auraDir string, entries []struct {
	key, value, source, session string
}) {
	t.Helper()
	store, closer, err := openStore(auraDir)
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	defer closer()
	for _, e := range entries {
		if _, err := store.Add(e.key, e.value, e.source, e.session); err != nil {
			t.Fatalf("store.Add(%q): %v", e.key, err)
		}
	}
}

func TestMemoryLs_AutoFlagFiltersAutoCapturedEntries(t *testing.T) {
	dir := setupTestAuraDir(t)
	seedEntries(t, dir, []struct{ key, value, source, session string }{
		{"manual.key", "manual-val", "cli", "s1"},
		{"auto.decision", "use postgres", "auto-capture", "s1"},
		{"mcp.key", "mcp-val", "mcp", "s1"},
	})

	jsonOut := false
	cmd := newMemoryLsCmd(&dir, &jsonOut)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--auto"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "auto.decision") {
		t.Error("expected auto.decision in output")
	}
	if strings.Contains(output, "manual.key") {
		t.Error("manual.key should not appear with --auto flag")
	}
	if strings.Contains(output, "mcp.key") {
		t.Error("mcp.key should not appear with --auto flag")
	}
}

func TestMemoryLs_AutoFlagTakesPrecedenceOverAgent(t *testing.T) {
	dir := setupTestAuraDir(t)
	seedEntries(t, dir, []struct{ key, value, source, session string }{
		{"cli.key", "val", "cli", "s1"},
		{"auto.key", "decision", "auto-capture", "s1"},
	})

	jsonOut := false
	cmd := newMemoryLsCmd(&dir, &jsonOut)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	// Both flags set: --auto should win.
	cmd.SetArgs([]string{"--auto", "--agent", "cli"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "auto.key") {
		t.Error("expected auto.key in output when --auto takes precedence")
	}
	if strings.Contains(output, "cli.key") {
		t.Error("cli.key should not appear when --auto takes precedence over --agent")
	}
}

func TestMemoryLs_AutoIndicatorShownInNormalListing(t *testing.T) {
	dir := setupTestAuraDir(t)
	seedEntries(t, dir, []struct{ key, value, source, session string }{
		{"manual.key", "manual-val", "cli", "s1"},
		{"auto.decision", "use postgres", "auto-capture", "s1"},
	})

	jsonOut := false
	cmd := newMemoryLsCmd(&dir, &jsonOut)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	output := buf.String()
	// Auto-captured entry should have [auto] indicator.
	if !strings.Contains(output, "[auto]") {
		t.Error("expected [auto] indicator for auto-captured entry in normal listing")
	}
	// Manual entry should not have [auto].
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "manual.key") && strings.Contains(line, "[auto]") {
			t.Error("manual.key should not have [auto] indicator")
		}
	}
}

func TestMemoryLs_NoAutoEntriesShowsNoEntries(t *testing.T) {
	dir := setupTestAuraDir(t)
	seedEntries(t, dir, []struct{ key, value, source, session string }{
		{"manual.key", "val", "cli", "s1"},
	})

	jsonOut := false
	cmd := newMemoryLsCmd(&dir, &jsonOut)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--auto"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "no entries") {
		t.Error("expected 'no entries' when no auto-captured entries exist")
	}
}

func TestMemoryLs_AutoFlagWithJSONOutput(t *testing.T) {
	dir := setupTestAuraDir(t)
	seedEntries(t, dir, []struct{ key, value, source, session string }{
		{"manual.key", "val", "cli", "s1"},
		{"auto.key", "decision", "auto-capture", "s1"},
	})

	jsonOut := true
	cmd := newMemoryLsCmd(&dir, &jsonOut)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--auto"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var entries []*types.MemoryEntry
	if err := json.Unmarshal(buf.Bytes(), &entries); err != nil {
		t.Fatalf("JSON decode: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Key != "auto.key" {
		t.Errorf("expected auto.key, got %q", entries[0].Key)
	}
	if entries[0].SourceTool != "auto-capture" {
		t.Errorf("expected source_tool=auto-capture, got %q", entries[0].SourceTool)
	}
}

func TestMemoryLs_WithoutAutoFlagShowsAllEntries(t *testing.T) {
	dir := setupTestAuraDir(t)
	seedEntries(t, dir, []struct{ key, value, source, session string }{
		{"manual.key", "val", "cli", "s1"},
		{"auto.key", "decision", "auto-capture", "s1"},
	})

	jsonOut := false
	cmd := newMemoryLsCmd(&dir, &jsonOut)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "manual.key") {
		t.Error("expected manual.key in unfiltered listing")
	}
	if !strings.Contains(output, "auto.key") {
		t.Error("expected auto.key in unfiltered listing")
	}
}

// TestList_FilterByAutoCapture verifies the store-level filtering works for auto-capture.
func TestList_FilterByAutoCapture(t *testing.T) {
	dir := setupTestAuraDir(t)
	dbPath := filepath.Join(dir, "aura.db")
	d, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer d.Close()
	if err := db.RunMigrations(d); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	store := memory.New(d)
	if _, err := store.Add("k1", "v1", "cli", "s1"); err != nil {
		t.Fatalf("Add k1: %v", err)
	}
	if _, err := store.Add("k2", "v2", "auto-capture", "s1"); err != nil {
		t.Fatalf("Add k2: %v", err)
	}
	if _, err := store.Add("k3", "v3", "auto-capture", "s1"); err != nil {
		t.Fatalf("Add k3: %v", err)
	}

	entries, err := store.List(memory.ListFilter{Agent: "auto-capture"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 auto-capture entries, got %d", len(entries))
	}
	for _, e := range entries {
		if e.SourceTool != "auto-capture" {
			t.Errorf("expected SourceTool=auto-capture, got %q", e.SourceTool)
		}
	}
}
