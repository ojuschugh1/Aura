package memory

import (
	"testing"
)

// --- Search -----------------------------------------------------------------

func TestSearch_MatchesKey(t *testing.T) {
	s := newTestStore(t)

	_, _ = s.Add("db.host", "localhost", "cli", "")
	_, _ = s.Add("api.url", "https://example.com", "cli", "")

	entries, err := s.Search("db")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 match, got %d", len(entries))
	}
	if entries[0].Key != "db.host" {
		t.Errorf("Key = %q, want %q", entries[0].Key, "db.host")
	}
}

func TestSearch_MatchesValue(t *testing.T) {
	s := newTestStore(t)

	_, _ = s.Add("config", "use postgres for storage", "cli", "")
	_, _ = s.Add("other", "unrelated value", "cli", "")

	entries, err := s.Search("postgres")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 match, got %d", len(entries))
	}
	if entries[0].Key != "config" {
		t.Errorf("Key = %q, want %q", entries[0].Key, "config")
	}
}

func TestSearch_KeyMatchesRankedFirst(t *testing.T) {
	s := newTestStore(t)

	_, _ = s.Add("other", "database config here", "cli", "")
	_, _ = s.Add("database", "some value", "cli", "")

	entries, err := s.Search("database")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(entries))
	}
	// Key match should come first.
	if entries[0].Key != "database" {
		t.Errorf("expected key match first, got %q", entries[0].Key)
	}
}

func TestSearch_NoResults(t *testing.T) {
	s := newTestStore(t)

	_, _ = s.Add("key", "value", "cli", "")

	entries, err := s.Search("nonexistent")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 matches, got %d", len(entries))
	}
}

func TestSearch_MatchesTags(t *testing.T) {
	s := newTestStore(t)

	_, _ = s.AddWithMeta("config", "value", "cli", "", 1.0, []string{"important", "production"})
	_, _ = s.Add("other", "unrelated", "cli", "")

	entries, err := s.Search("important")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 match, got %d", len(entries))
	}
}

// --- SearchByTag ------------------------------------------------------------

func TestSearchByTag_FindsTaggedEntries(t *testing.T) {
	s := newTestStore(t)

	_, _ = s.AddWithMeta("k1", "v1", "cli", "", 1.0, []string{"backend", "config"})
	_, _ = s.AddWithMeta("k2", "v2", "cli", "", 1.0, []string{"frontend"})
	_, _ = s.Add("k3", "v3", "cli", "")

	entries, err := s.SearchByTag("backend")
	if err != nil {
		t.Fatalf("SearchByTag: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 match, got %d", len(entries))
	}
	if entries[0].Key != "k1" {
		t.Errorf("Key = %q, want %q", entries[0].Key, "k1")
	}
}

func TestSearchByTag_NoMatch(t *testing.T) {
	s := newTestStore(t)

	_, _ = s.AddWithMeta("k1", "v1", "cli", "", 1.0, []string{"backend"})

	entries, err := s.SearchByTag("nonexistent")
	if err != nil {
		t.Fatalf("SearchByTag: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 matches, got %d", len(entries))
	}
}

// --- AddTags ----------------------------------------------------------------

func TestAddTags_AppendsNewTags(t *testing.T) {
	s := newTestStore(t)

	_, _ = s.Add("k1", "v1", "cli", "")

	entry, err := s.AddTags("k1", []string{"tag1", "tag2"})
	if err != nil {
		t.Fatalf("AddTags: %v", err)
	}
	if len(entry.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(entry.Tags))
	}
}

func TestAddTags_DeduplicatesTags(t *testing.T) {
	s := newTestStore(t)

	_, _ = s.AddWithMeta("k1", "v1", "cli", "", 1.0, []string{"existing"})

	entry, err := s.AddTags("k1", []string{"existing", "new"})
	if err != nil {
		t.Fatalf("AddTags: %v", err)
	}
	if len(entry.Tags) != 2 {
		t.Errorf("expected 2 tags (deduplicated), got %d: %v", len(entry.Tags), entry.Tags)
	}
}

func TestAddTags_NonExistentKeyReturnsError(t *testing.T) {
	s := newTestStore(t)

	_, err := s.AddTags("nonexistent", []string{"tag"})
	if err == nil {
		t.Fatal("expected error for non-existent key, got nil")
	}
}

// --- AddWithMeta / Confidence -----------------------------------------------

func TestAddWithMeta_SetsConfidence(t *testing.T) {
	s := newTestStore(t)

	entry, err := s.AddWithMeta("k1", "v1", "auto-capture", "sess-1", 0.75, nil)
	if err != nil {
		t.Fatalf("AddWithMeta: %v", err)
	}
	if entry.Confidence != 0.75 {
		t.Errorf("Confidence = %f, want 0.75", entry.Confidence)
	}
}

func TestAdd_DefaultConfidenceIsOne(t *testing.T) {
	s := newTestStore(t)

	entry, err := s.Add("k1", "v1", "cli", "")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if entry.Confidence != 1.0 {
		t.Errorf("Confidence = %f, want 1.0", entry.Confidence)
	}
}

func TestAddWithMeta_SetsTags(t *testing.T) {
	s := newTestStore(t)

	entry, err := s.AddWithMeta("k1", "v1", "cli", "", 1.0, []string{"tag1", "tag2"})
	if err != nil {
		t.Fatalf("AddWithMeta: %v", err)
	}
	if len(entry.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(entry.Tags))
	}
}
