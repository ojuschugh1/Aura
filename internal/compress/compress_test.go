package compress

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ojuschugh1/aura/internal/db"
)

// openTestDB creates an isolated SQLite database with all migrations applied.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	d, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	if err := db.RunMigrations(d); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

// --- DedupCache: Seen (cache miss / cache hit) --- Req 5.2, 5.6 -----------

func TestDedupCache_SeenReturnsFalseForNewContent(t *testing.T) {
	cache := NewDedupCache(openTestDB(t))

	seen, err := cache.Seen("brand new content")
	if err != nil {
		t.Fatalf("Seen: %v", err)
	}
	if seen {
		t.Error("expected Seen=false for content never recorded, got true")
	}
}

func TestDedupCache_SeenReturnsTrueAfterRecord(t *testing.T) {
	cache := NewDedupCache(openTestDB(t))

	content := "hello world"
	if err := cache.Record(content, 2); err != nil {
		t.Fatalf("Record: %v", err)
	}

	seen, err := cache.Seen(content)
	if err != nil {
		t.Fatalf("Seen: %v", err)
	}
	if !seen {
		t.Error("expected Seen=true after Record, got false")
	}
}

func TestDedupCache_SeenReturnsFalseForDifferentContent(t *testing.T) {
	cache := NewDedupCache(openTestDB(t))

	if err := cache.Record("content A", 2); err != nil {
		t.Fatalf("Record: %v", err)
	}

	seen, err := cache.Seen("content B")
	if err != nil {
		t.Fatalf("Seen: %v", err)
	}
	if seen {
		t.Error("expected Seen=false for different content, got true")
	}
}

// --- DedupCache: SHA-256 hash correctness --- Req 5.2 ----------------------

func TestHashContent_MatchesSHA256(t *testing.T) {
	input := "test content for hashing"
	want := fmt.Sprintf("%x", sha256.Sum256([]byte(input)))
	got := hashContent(input)
	if got != want {
		t.Errorf("hashContent(%q) = %q, want %q", input, got, want)
	}
}

func TestHashContent_EmptyString(t *testing.T) {
	want := fmt.Sprintf("%x", sha256.Sum256([]byte("")))
	got := hashContent("")
	if got != want {
		t.Errorf("hashContent(\"\") = %q, want %q", got, want)
	}
}

func TestHashContent_DifferentContentProducesDifferentHashes(t *testing.T) {
	h1 := hashContent("alpha")
	h2 := hashContent("beta")
	if h1 == h2 {
		t.Errorf("expected different hashes for different content, both got %q", h1)
	}
}

func TestHashContent_SameContentProducesSameHash(t *testing.T) {
	h1 := hashContent("identical")
	h2 := hashContent("identical")
	if h1 != h2 {
		t.Errorf("expected same hash for same content, got %q and %q", h1, h2)
	}
}

// --- DedupCache: persistence across instances --- Req 5.6 ------------------

func TestDedupCache_PersistsAcrossInstances(t *testing.T) {
	d := openTestDB(t)

	cache1 := NewDedupCache(d)
	content := "persistent content"
	if err := cache1.Record(content, 2); err != nil {
		t.Fatalf("Record: %v", err)
	}

	// Create a second cache instance backed by the same DB.
	cache2 := NewDedupCache(d)
	seen, err := cache2.Seen(content)
	if err != nil {
		t.Fatalf("Seen: %v", err)
	}
	if !seen {
		t.Error("expected content to be visible from second DedupCache instance sharing the same DB")
	}
}

// --- DedupCache: Record is idempotent (INSERT OR IGNORE) -------------------

func TestDedupCache_RecordIdempotent(t *testing.T) {
	cache := NewDedupCache(openTestDB(t))

	content := "duplicate insert"
	if err := cache.Record(content, 5); err != nil {
		t.Fatalf("Record first: %v", err)
	}
	// Second insert of same content should not error (INSERT OR IGNORE).
	if err := cache.Record(content, 5); err != nil {
		t.Fatalf("Record second: %v", err)
	}
}

// --- estimateTokens --------------------------------------------------------

func TestEstimateTokens_CountsWords(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"hello world", 2},
		{"one", 1},
		{"", 0},
		{"  spaced   out  ", 2},
		{"a b c d e", 5},
	}
	for _, tc := range tests {
		got := estimateTokens(tc.input)
		if got != tc.want {
			t.Errorf("estimateTokens(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

// --- Engine: compression stats --- Req 5.2 ---------------------------------

func TestEngine_CompactDeduplicated_ReturnsCorrectStats(t *testing.T) {
	d := openTestDB(t)
	eng := New(d)

	content := "this is some repeated content for testing"

	// First call: sqz is not available, so graceful degradation.
	res1, _ := eng.Compact(content)
	if res1 == nil {
		t.Fatal("expected non-nil result from first Compact")
	}
	if res1.Deduplicated {
		t.Error("first Compact should not be deduplicated")
	}

	// Record the content in the dedup cache manually so second call hits cache.
	tokens := estimateTokens(content)
	if err := eng.dedup.Record(content, tokens); err != nil {
		t.Fatalf("Record: %v", err)
	}

	// Second call: should hit dedup cache.
	res2, err := eng.Compact(content)
	if err != nil {
		t.Fatalf("Compact (dedup hit): %v", err)
	}
	if !res2.Deduplicated {
		t.Error("expected Deduplicated=true on second Compact")
	}
	if res2.OriginalTokens != tokens {
		t.Errorf("OriginalTokens = %d, want %d", res2.OriginalTokens, tokens)
	}
	if res2.CompressedTokens != tokens {
		t.Errorf("CompressedTokens = %d, want %d (dedup returns same count)", res2.CompressedTokens, tokens)
	}
	if res2.ReductionPct != 0 {
		t.Errorf("ReductionPct = %f, want 0 (dedup returns 0%% reduction)", res2.ReductionPct)
	}
	if res2.Compressed != content {
		t.Errorf("Compressed content should equal original for dedup hit")
	}
}

// --- Engine: graceful degradation when sqz is unavailable --- Req 5.4 ------

func TestEngine_CompactWithoutSqz_ReturnsOriginalContent(t *testing.T) {
	eng := New(openTestDB(t))

	content := "some content that cannot be compressed because sqz is missing"
	res, err := eng.Compact(content)

	// err should be non-nil (compression unavailable) but result should still be returned.
	if res == nil {
		t.Fatal("expected non-nil result even when sqz is unavailable")
	}
	if err == nil {
		t.Log("sqz appears to be available on this system; skipping degradation check")
		return
	}

	if !strings.Contains(err.Error(), "compression unavailable") {
		t.Errorf("expected 'compression unavailable' error, got: %v", err)
	}
	if res.Compressed != content {
		t.Error("expected Compressed to equal original content on degradation")
	}

	expectedTokens := estimateTokens(content)
	if res.OriginalTokens != expectedTokens {
		t.Errorf("OriginalTokens = %d, want %d", res.OriginalTokens, expectedTokens)
	}
	if res.CompressedTokens != expectedTokens {
		t.Errorf("CompressedTokens = %d, want %d", res.CompressedTokens, expectedTokens)
	}
	if res.ReductionPct != 0 {
		t.Errorf("ReductionPct = %f, want 0", res.ReductionPct)
	}
}

// --- Engine: new content is not deduplicated on first encounter ------------

func TestEngine_CompactNewContent_NotDeduplicated(t *testing.T) {
	eng := New(openTestDB(t))

	res, _ := eng.Compact("completely new content never seen before")
	if res == nil {
		t.Fatal("expected non-nil result")
	}
	if res.Deduplicated {
		t.Error("expected Deduplicated=false for first-time content")
	}
}
