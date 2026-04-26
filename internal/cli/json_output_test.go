package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ojuschugh1/aura/pkg/types"
)

// TestMemoryAdd_JSONOutputValid verifies that `memory add` with --json
// produces valid, parseable JSON containing the stored entry fields.
func TestMemoryAdd_JSONOutputValid(t *testing.T) {
	dir := setupTestAuraDir(t)
	jsonOut := true

	cmd := newMemoryAddCmd(&dir, &jsonOut)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"test.key", "test-value"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	raw := buf.Bytes()

	// Must be valid JSON.
	if !json.Valid(raw) {
		t.Fatalf("output is not valid JSON: %s", raw)
	}

	var entry types.MemoryEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if entry.Key != "test.key" {
		t.Errorf("expected key=test.key, got %q", entry.Key)
	}
	if entry.Value != "test-value" {
		t.Errorf("expected value=test-value, got %q", entry.Value)
	}
	if entry.SourceTool != "cli" {
		t.Errorf("expected source_tool=cli, got %q", entry.SourceTool)
	}
	if entry.ContentHash == "" {
		t.Error("expected non-empty content_hash")
	}
	if entry.UpdatedAt.IsZero() {
		t.Error("expected non-zero updated_at")
	}
}

// TestMemoryAdd_PlainTextWithoutJSON verifies that `memory add` without --json
// produces human-readable text, not JSON.
func TestMemoryAdd_PlainTextWithoutJSON(t *testing.T) {
	dir := setupTestAuraDir(t)
	jsonOut := false

	cmd := newMemoryAddCmd(&dir, &jsonOut)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"plain.key", "plain-value"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	output := buf.String()

	// Plain text should contain "stored:" prefix.
	if !strings.Contains(output, "stored:") {
		t.Errorf("expected plain text with 'stored:', got %q", output)
	}

	// Should NOT be valid JSON object.
	var m map[string]any
	if json.Unmarshal(buf.Bytes(), &m) == nil {
		t.Error("plain text output should not be parseable as JSON object")
	}
}

// TestMemoryGet_JSONOutputValid verifies that `memory get` with --json
// produces valid JSON with all expected fields.
func TestMemoryGet_JSONOutputValid(t *testing.T) {
	dir := setupTestAuraDir(t)
	seedEntries(t, dir, []struct{ key, value, source, session string }{
		{"fetch.key", "fetch-value", "mcp", "sess-1"},
	})

	jsonOut := true
	cmd := newMemoryGetCmd(&dir, &jsonOut)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"fetch.key"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	raw := buf.Bytes()

	if !json.Valid(raw) {
		t.Fatalf("output is not valid JSON: %s", raw)
	}

	var entry types.MemoryEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if entry.Key != "fetch.key" {
		t.Errorf("expected key=fetch.key, got %q", entry.Key)
	}
	if entry.Value != "fetch-value" {
		t.Errorf("expected value=fetch-value, got %q", entry.Value)
	}
	if entry.SourceTool != "mcp" {
		t.Errorf("expected source_tool=mcp, got %q", entry.SourceTool)
	}
	if entry.ID == 0 {
		t.Error("expected non-zero id")
	}
	if entry.CreatedAt.IsZero() {
		t.Error("expected non-zero created_at")
	}
}

// TestMemoryGet_PlainTextWithoutJSON verifies that `memory get` without --json
// produces human-readable text, not JSON.
func TestMemoryGet_PlainTextWithoutJSON(t *testing.T) {
	dir := setupTestAuraDir(t)
	seedEntries(t, dir, []struct{ key, value, source, session string }{
		{"plain.get", "some-value", "cli", "s1"},
	})

	jsonOut := false
	cmd := newMemoryGetCmd(&dir, &jsonOut)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"plain.get"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	output := buf.String()

	// Plain text should contain labeled fields.
	if !strings.Contains(output, "key:") {
		t.Errorf("expected 'key:' label in plain text, got %q", output)
	}
	if !strings.Contains(output, "value:") {
		t.Errorf("expected 'value:' label in plain text, got %q", output)
	}

	// Should NOT be valid JSON object.
	var m map[string]any
	if json.Unmarshal(buf.Bytes(), &m) == nil {
		t.Error("plain text output should not be parseable as JSON object")
	}
}

// TestMemoryLs_JSONOutputValidArray verifies that `memory ls` with --json
// produces a valid JSON array of entries.
func TestMemoryLs_JSONOutputValidArray(t *testing.T) {
	dir := setupTestAuraDir(t)
	seedEntries(t, dir, []struct{ key, value, source, session string }{
		{"ls.one", "val-1", "cli", "s1"},
		{"ls.two", "val-2", "mcp", "s2"},
		{"ls.three", "val-3", "auto-capture", "s3"},
	})

	jsonOut := true
	cmd := newMemoryLsCmd(&dir, &jsonOut)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	raw := buf.Bytes()

	if !json.Valid(raw) {
		t.Fatalf("output is not valid JSON: %s", raw)
	}

	var entries []*types.MemoryEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// Verify each entry has required fields populated.
	for _, e := range entries {
		if e.Key == "" {
			t.Error("entry has empty key")
		}
		if e.Value == "" {
			t.Error("entry has empty value")
		}
		if e.SourceTool == "" {
			t.Error("entry has empty source_tool")
		}
		if e.ContentHash == "" {
			t.Error("entry has empty content_hash")
		}
	}
}

// TestMemoryLs_EmptyJSONArray verifies that `memory ls --json` with no entries
// produces a valid JSON null or empty array (not an error).
func TestMemoryLs_EmptyJSONArray(t *testing.T) {
	dir := setupTestAuraDir(t)

	jsonOut := true
	cmd := newMemoryLsCmd(&dir, &jsonOut)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	raw := buf.Bytes()

	if !json.Valid(raw) {
		t.Fatalf("output is not valid JSON: %s", raw)
	}

	// Should decode as a JSON array (possibly null).
	var entries []*types.MemoryEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
}

// TestMemoryLs_PlainTextWithoutJSON verifies that `memory ls` without --json
// produces tabular text, not JSON.
func TestMemoryLs_PlainTextWithoutJSON(t *testing.T) {
	dir := setupTestAuraDir(t)
	seedEntries(t, dir, []struct{ key, value, source, session string }{
		{"text.key", "text-val", "cli", "s1"},
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

	// Plain text listing should have a header row.
	if !strings.Contains(output, "KEY") {
		t.Errorf("expected 'KEY' header in plain text listing, got %q", output)
	}
	if !strings.Contains(output, "SOURCE") {
		t.Errorf("expected 'SOURCE' header in plain text listing, got %q", output)
	}

	// Should NOT be valid JSON array.
	var entries []any
	if json.Unmarshal(buf.Bytes(), &entries) == nil {
		t.Error("plain text output should not be parseable as JSON array")
	}
}

// TestMemoryAdd_JSONContainsAllExpectedFields verifies the JSON output from
// `memory add` includes every field defined on MemoryEntry.
func TestMemoryAdd_JSONContainsAllExpectedFields(t *testing.T) {
	dir := setupTestAuraDir(t)
	jsonOut := true

	cmd := newMemoryAddCmd(&dir, &jsonOut)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"fields.key", "fields-value"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// Decode into a generic map to check field presence.
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	requiredFields := []string{"id", "key", "value", "source_tool", "session_id", "created_at", "updated_at", "content_hash"}
	for _, field := range requiredFields {
		if _, ok := m[field]; !ok {
			t.Errorf("JSON output missing required field %q", field)
		}
	}
}

// TestMemoryGet_JSONContainsAllExpectedFields verifies the JSON output from
// `memory get` includes every field defined on MemoryEntry.
func TestMemoryGet_JSONContainsAllExpectedFields(t *testing.T) {
	dir := setupTestAuraDir(t)
	seedEntries(t, dir, []struct{ key, value, source, session string }{
		{"fields.get", "val", "cli", "s1"},
	})

	jsonOut := true
	cmd := newMemoryGetCmd(&dir, &jsonOut)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"fields.get"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	requiredFields := []string{"id", "key", "value", "source_tool", "session_id", "created_at", "updated_at", "content_hash"}
	for _, field := range requiredFields {
		if _, ok := m[field]; !ok {
			t.Errorf("JSON output missing required field %q", field)
		}
	}
}
