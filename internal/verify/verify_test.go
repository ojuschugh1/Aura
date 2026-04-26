package verify

import (
	"os"
	"path/filepath"
	"testing"
)

// --- parseLine (Claude Code format) -----------------------------------------

func TestParseLine_ClaudeCodeFormat(t *testing.T) {
	line := []byte(`{"role":"assistant","content":"I created file main.go","timestamp":"2025-01-15T10:30:00Z"}`)

	entry, err := parseLine(line)
	if err != nil {
		t.Fatalf("parseLine: %v", err)
	}
	if entry.Role != "assistant" {
		t.Errorf("Role = %q, want %q", entry.Role, "assistant")
	}
	if entry.Content != "I created file main.go" {
		t.Errorf("Content = %q, want %q", entry.Content, "I created file main.go")
	}
	if entry.Timestamp.IsZero() {
		t.Error("expected non-zero Timestamp")
	}
}

func TestParseLine_ClaudeCodeUserRole(t *testing.T) {
	line := []byte(`{"role":"user","content":"please fix the bug","timestamp":"2025-01-15T10:29:00Z"}`)

	entry, err := parseLine(line)
	if err != nil {
		t.Fatalf("parseLine: %v", err)
	}
	if entry.Role != "user" {
		t.Errorf("Role = %q, want %q", entry.Role, "user")
	}
	if entry.Content != "please fix the bug" {
		t.Errorf("Content = %q, want %q", entry.Content, "please fix the bug")
	}
}

// --- parseLine (Cursor format) ----------------------------------------------

func TestParseLine_CursorFormat(t *testing.T) {
	line := []byte(`{"type":"message","text":"I modified utils.go","timestamp":"2025-01-15T11:00:00Z"}`)

	entry, err := parseLine(line)
	if err != nil {
		t.Fatalf("parseLine: %v", err)
	}
	if entry.Role != "message" {
		t.Errorf("Role = %q, want %q", entry.Role, "message")
	}
	if entry.Content != "I modified utils.go" {
		t.Errorf("Content = %q, want %q", entry.Content, "I modified utils.go")
	}
	if entry.Timestamp.IsZero() {
		t.Error("expected non-zero Timestamp")
	}
}

// --- parseLine (invalid / unrecognised) -------------------------------------

func TestParseLine_InvalidJSON(t *testing.T) {
	line := []byte(`not json at all`)

	_, err := parseLine(line)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestParseLine_UnrecognisedFormat(t *testing.T) {
	line := []byte(`{"foo":"bar","baz":42}`)

	_, err := parseLine(line)
	if err == nil {
		t.Fatal("expected error for unrecognised format, got nil")
	}
}

// --- parseLine (missing timestamp) ------------------------------------------

func TestParseLine_MissingTimestamp(t *testing.T) {
	line := []byte(`{"role":"assistant","content":"hello"}`)

	entry, err := parseLine(line)
	if err != nil {
		t.Fatalf("parseLine: %v", err)
	}
	if !entry.Timestamp.IsZero() {
		t.Errorf("expected zero Timestamp for missing field, got %v", entry.Timestamp)
	}
}

// --- ParseJSONL (file-based) ------------------------------------------------

func writeJSONL(t *testing.T, lines string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "transcript.jsonl")
	if err := os.WriteFile(path, []byte(lines), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

func TestParseJSONL_ValidClaudeTranscript(t *testing.T) {
	jsonl := `{"role":"user","content":"create a server","timestamp":"2025-01-15T10:00:00Z"}
{"role":"assistant","content":"I created file server.go","timestamp":"2025-01-15T10:01:00Z"}
{"role":"assistant","content":"tests passed","timestamp":"2025-01-15T10:02:00Z"}
`
	path := writeJSONL(t, jsonl)

	entries, err := ParseJSONL(path)
	if err != nil {
		t.Fatalf("ParseJSONL: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Role != "user" {
		t.Errorf("entries[0].Role = %q, want user", entries[0].Role)
	}
	if entries[1].Role != "assistant" {
		t.Errorf("entries[1].Role = %q, want assistant", entries[1].Role)
	}
}

func TestParseJSONL_ValidCursorTranscript(t *testing.T) {
	jsonl := `{"type":"message","text":"I wrote file handler.go","timestamp":"2025-01-15T10:00:00Z"}
{"type":"message","text":"installed express","timestamp":"2025-01-15T10:01:00Z"}
`
	path := writeJSONL(t, jsonl)

	entries, err := ParseJSONL(path)
	if err != nil {
		t.Fatalf("ParseJSONL: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Role != "message" {
		t.Errorf("entries[0].Role = %q, want message", entries[0].Role)
	}
}

func TestParseJSONL_SkipsEmptyLines(t *testing.T) {
	jsonl := `{"role":"user","content":"hello"}

{"role":"assistant","content":"hi"}

`
	path := writeJSONL(t, jsonl)

	entries, err := ParseJSONL(path)
	if err != nil {
		t.Fatalf("ParseJSONL: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (empty lines skipped), got %d", len(entries))
	}
}

func TestParseJSONL_EmptyFile(t *testing.T) {
	path := writeJSONL(t, "")

	entries, err := ParseJSONL(path)
	if err != nil {
		t.Fatalf("ParseJSONL: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for empty file, got %d", len(entries))
	}
}

func TestParseJSONL_FileNotFound(t *testing.T) {
	_, err := ParseJSONL("/nonexistent/path/transcript.jsonl")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

// --- ExtractClaims ----------------------------------------------------------

func TestExtractClaims_FileCreated(t *testing.T) {
	entries := []TranscriptEntry{
		{Role: "assistant", Content: "I created file main.go for you"},
	}

	claims := ExtractClaims(entries)
	if len(claims) != 1 {
		t.Fatalf("expected 1 claim, got %d", len(claims))
	}
	if claims[0].Type != "file_created" {
		t.Errorf("Type = %q, want file_created", claims[0].Type)
	}
	if claims[0].Target != "main.go" {
		t.Errorf("Target = %q, want main.go", claims[0].Target)
	}
}

func TestExtractClaims_WroteFile(t *testing.T) {
	entries := []TranscriptEntry{
		{Role: "assistant", Content: "I wrote src/handler.go with the new logic"},
	}

	claims := ExtractClaims(entries)
	if len(claims) != 1 {
		t.Fatalf("expected 1 claim, got %d", len(claims))
	}
	if claims[0].Type != "file_created" {
		t.Errorf("Type = %q, want file_created", claims[0].Type)
	}
	if claims[0].Target != "src/handler.go" {
		t.Errorf("Target = %q, want src/handler.go", claims[0].Target)
	}
}

func TestExtractClaims_FileModified(t *testing.T) {
	entries := []TranscriptEntry{
		{Role: "assistant", Content: "I modified config.yaml to add the new setting"},
	}

	claims := ExtractClaims(entries)
	if len(claims) != 1 {
		t.Fatalf("expected 1 claim, got %d", len(claims))
	}
	if claims[0].Type != "file_modified" {
		t.Errorf("Type = %q, want file_modified", claims[0].Type)
	}
	if claims[0].Target != "config.yaml" {
		t.Errorf("Target = %q, want config.yaml", claims[0].Target)
	}
}

func TestExtractClaims_PackageInstalled(t *testing.T) {
	entries := []TranscriptEntry{
		{Role: "assistant", Content: "I installed express for the web server"},
	}

	claims := ExtractClaims(entries)
	if len(claims) != 1 {
		t.Fatalf("expected 1 claim, got %d", len(claims))
	}
	if claims[0].Type != "package_installed" {
		t.Errorf("Type = %q, want package_installed", claims[0].Type)
	}
	if claims[0].Target != "express" {
		t.Errorf("Target = %q, want express", claims[0].Target)
	}
}

func TestExtractClaims_TestPassed(t *testing.T) {
	entries := []TranscriptEntry{
		{Role: "assistant", Content: "All tests passed successfully"},
	}

	claims := ExtractClaims(entries)
	if len(claims) != 1 {
		t.Fatalf("expected 1 claim, got %d", len(claims))
	}
	if claims[0].Type != "test_passed" {
		t.Errorf("Type = %q, want test_passed", claims[0].Type)
	}
}

func TestExtractClaims_CommandExecuted(t *testing.T) {
	entries := []TranscriptEntry{
		{Role: "assistant", Content: "I ran command go-build to compile the project"},
	}

	claims := ExtractClaims(entries)
	if len(claims) != 1 {
		t.Fatalf("expected 1 claim, got %d", len(claims))
	}
	if claims[0].Type != "command_executed" {
		t.Errorf("Type = %q, want command_executed", claims[0].Type)
	}
	if claims[0].Target != "go-build" {
		t.Errorf("Target = %q, want go-build", claims[0].Target)
	}
}

// --- ExtractClaims (multiple claims in one message) -------------------------

func TestExtractClaims_MultipleClaims(t *testing.T) {
	entries := []TranscriptEntry{
		{Role: "assistant", Content: "I created file main.go and modified config.yaml. Then I installed lodash and tests passed."},
	}

	claims := ExtractClaims(entries)
	if len(claims) < 4 {
		t.Fatalf("expected at least 4 claims, got %d", len(claims))
	}

	types := map[string]bool{}
	for _, c := range claims {
		types[c.Type] = true
	}
	for _, want := range []string{"file_created", "file_modified", "package_installed", "test_passed"} {
		if !types[want] {
			t.Errorf("missing claim type %q", want)
		}
	}
}

// --- ExtractClaims (role filtering) -----------------------------------------

func TestExtractClaims_IgnoresUserMessages(t *testing.T) {
	entries := []TranscriptEntry{
		{Role: "user", Content: "I created file main.go"},
		{Role: "assistant", Content: "I created file server.go"},
	}

	claims := ExtractClaims(entries)
	if len(claims) != 1 {
		t.Fatalf("expected 1 claim (user messages ignored), got %d", len(claims))
	}
	if claims[0].Target != "server.go" {
		t.Errorf("Target = %q, want server.go", claims[0].Target)
	}
}

func TestExtractClaims_AcceptsMessageRole(t *testing.T) {
	entries := []TranscriptEntry{
		{Role: "message", Content: "I created file handler.go"},
	}

	claims := ExtractClaims(entries)
	if len(claims) != 1 {
		t.Fatalf("expected 1 claim from message role, got %d", len(claims))
	}
	if claims[0].Target != "handler.go" {
		t.Errorf("Target = %q, want handler.go", claims[0].Target)
	}
}

// --- ExtractClaims (edge cases) ---------------------------------------------

func TestExtractClaims_EmptyEntries(t *testing.T) {
	claims := ExtractClaims(nil)
	if len(claims) != 0 {
		t.Errorf("expected 0 claims for nil entries, got %d", len(claims))
	}
}

func TestExtractClaims_NoClaims(t *testing.T) {
	entries := []TranscriptEntry{
		{Role: "assistant", Content: "Let me think about the architecture."},
		{Role: "assistant", Content: "Here is my analysis of the problem."},
	}

	claims := ExtractClaims(entries)
	if len(claims) != 0 {
		t.Errorf("expected 0 claims for content with no claim patterns, got %d", len(claims))
	}
}

// --- Claim classification ---------------------------------------------------

func TestClaimClassification_AllTypes(t *testing.T) {
	tests := []struct {
		content  string
		wantType string
	}{
		{"created file app.go", "file_created"},
		{"wrote index.html", "file_created"},
		{"modified README.md", "file_modified"},
		{"updated package.json", "file_modified"},
		{"edited main.py", "file_modified"},
		{"installed react", "package_installed"},
		{"added dependency axios", "package_installed"},
		{"tests passed", "test_passed"},
		{"test pass", "test_passed"},
		{"ran command npm-test", "command_executed"},
		{"executed make-build", "command_executed"},
	}

	for _, tt := range tests {
		t.Run(tt.content, func(t *testing.T) {
			entries := []TranscriptEntry{
				{Role: "assistant", Content: tt.content},
			}
			claims := ExtractClaims(entries)
			if len(claims) == 0 {
				t.Fatalf("expected at least 1 claim for %q", tt.content)
			}
			found := false
			for _, c := range claims {
				if c.Type == tt.wantType {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected claim type %q for %q, got types: %v", tt.wantType, tt.content, claimTypes(claims))
			}
		})
	}
}

func claimTypes(claims []Claim) []string {
	var types []string
	for _, c := range claims {
		types = append(types, c.Type)
	}
	return types
}

// --- Verify (truth score calculation) ---------------------------------------

func TestVerify_AllClaimsPass(t *testing.T) {
	dir := t.TempDir()
	// Create a file so the file_created claim passes.
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	claims := []Claim{
		{Type: "file_created", Target: "main.go"},
		{Type: "test_passed"},
		{Type: "command_executed", Target: "go-build"},
	}

	result := Verify(claims, dir)

	if result.TotalClaims != 3 {
		t.Errorf("TotalClaims = %d, want 3", result.TotalClaims)
	}
	if result.PassCount != 3 {
		t.Errorf("PassCount = %d, want 3", result.PassCount)
	}
	if result.FailCount != 0 {
		t.Errorf("FailCount = %d, want 0", result.FailCount)
	}
	if result.TruthPct != 100.0 {
		t.Errorf("TruthPct = %f, want 100.0", result.TruthPct)
	}
}

func TestVerify_AllClaimsFail(t *testing.T) {
	dir := t.TempDir() // empty directory — no files exist

	claims := []Claim{
		{Type: "file_created", Target: "nonexistent.go"},
		{Type: "file_created", Target: "also-missing.go"},
	}

	result := Verify(claims, dir)

	if result.TotalClaims != 2 {
		t.Errorf("TotalClaims = %d, want 2", result.TotalClaims)
	}
	if result.PassCount != 0 {
		t.Errorf("PassCount = %d, want 0", result.PassCount)
	}
	if result.FailCount != 2 {
		t.Errorf("FailCount = %d, want 2", result.FailCount)
	}
	if result.TruthPct != 0.0 {
		t.Errorf("TruthPct = %f, want 0.0", result.TruthPct)
	}
}

func TestVerify_MixedPassFail(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "exists.go"), []byte("package main"), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	claims := []Claim{
		{Type: "file_created", Target: "exists.go"},
		{Type: "file_created", Target: "missing.go"},
	}

	result := Verify(claims, dir)

	if result.TotalClaims != 2 {
		t.Errorf("TotalClaims = %d, want 2", result.TotalClaims)
	}
	if result.PassCount != 1 {
		t.Errorf("PassCount = %d, want 1", result.PassCount)
	}
	if result.FailCount != 1 {
		t.Errorf("FailCount = %d, want 1", result.FailCount)
	}
	if result.TruthPct != 50.0 {
		t.Errorf("TruthPct = %f, want 50.0", result.TruthPct)
	}
}

func TestVerify_NoClaims(t *testing.T) {
	result := Verify(nil, t.TempDir())

	if result.TotalClaims != 0 {
		t.Errorf("TotalClaims = %d, want 0", result.TotalClaims)
	}
	if result.PassCount != 0 {
		t.Errorf("PassCount = %d, want 0", result.PassCount)
	}
	if result.FailCount != 0 {
		t.Errorf("FailCount = %d, want 0", result.FailCount)
	}
	if result.TruthPct != 0.0 {
		t.Errorf("TruthPct = %f, want 0.0", result.TruthPct)
	}
}

func TestVerify_TestPassedAndCommandExecutedAccepted(t *testing.T) {
	claims := []Claim{
		{Type: "test_passed"},
		{Type: "command_executed", Target: "npm-test"},
	}

	result := Verify(claims, t.TempDir())

	if result.PassCount != 2 {
		t.Errorf("PassCount = %d, want 2 (test_passed and command_executed accepted at face value)", result.PassCount)
	}
	// Verify detail messages.
	for _, c := range result.Claims {
		if c.Detail != "accepted without re-execution" {
			t.Errorf("claim %q detail = %q, want 'accepted without re-execution'", c.Type, c.Detail)
		}
	}
}

func TestVerify_UnknownClaimTypeFails(t *testing.T) {
	claims := []Claim{
		{Type: "unknown_type", Target: "something"},
	}

	result := Verify(claims, t.TempDir())

	if result.FailCount != 1 {
		t.Errorf("FailCount = %d, want 1 for unknown claim type", result.FailCount)
	}
}

func TestVerify_SetsVerifiedFlag(t *testing.T) {
	claims := []Claim{
		{Type: "test_passed"},
	}

	result := Verify(claims, t.TempDir())

	for _, c := range result.Claims {
		if !c.Verified {
			t.Errorf("expected Verified=true for claim %q", c.Type)
		}
	}
}

// --- End-to-end: parse transcript → extract claims → verify -----------------

func TestEndToEnd_ParseExtractVerify(t *testing.T) {
	dir := t.TempDir()
	// Create the file the agent claims to have created.
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte("package main"), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	jsonl := `{"role":"user","content":"build me an app"}
{"role":"assistant","content":"I created file app.go and tests passed"}
`
	path := writeJSONL(t, jsonl)

	entries, err := ParseJSONL(path)
	if err != nil {
		t.Fatalf("ParseJSONL: %v", err)
	}

	claims := ExtractClaims(entries)
	if len(claims) < 2 {
		t.Fatalf("expected at least 2 claims, got %d", len(claims))
	}

	result := Verify(claims, dir)

	if result.TotalClaims < 2 {
		t.Errorf("TotalClaims = %d, want >= 2", result.TotalClaims)
	}
	// file_created should pass (file exists), test_passed should pass (accepted).
	if result.PassCount < 2 {
		t.Errorf("PassCount = %d, want >= 2", result.PassCount)
	}
	if result.TruthPct != 100.0 {
		t.Errorf("TruthPct = %f, want 100.0", result.TruthPct)
	}
}
