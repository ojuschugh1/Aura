package autocapture

import (
	"testing"
)

func TestMatchDecisions_StrongPatterns(t *testing.T) {
	text := "We decided to use PostgreSQL for the database layer."
	matches := MatchDecisions(text)
	if len(matches) == 0 {
		t.Fatal("expected at least one match")
	}
	m := matches[0]
	// Should be very-strong tier (0.95) plus keyword boost.
	if m.Confidence < 0.9 {
		t.Errorf("confidence = %v, want >= 0.9", m.Confidence)
	}
	if m.Pattern != "we decided" {
		t.Errorf("pattern = %q, want %q", m.Pattern, "we decided")
	}
	if m.Key == "" {
		t.Error("key should not be empty")
	}
}

func TestMatchDecisions_IChose(t *testing.T) {
	text := "I chose React over Vue for the frontend."
	matches := MatchDecisions(text)
	if len(matches) == 0 {
		t.Fatal("expected at least one match")
	}
	m := matches[0]
	if m.Confidence < 0.9 {
		t.Errorf("confidence = %v, want >= 0.9", m.Confidence)
	}
	if m.Pattern != "I chose" {
		t.Errorf("pattern = %q, want %q", m.Pattern, "I chose")
	}
}

func TestMatchDecisions_MediumPatterns(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		pattern string
		minConf float64
	}{
		{"lets use", "Let's use Redis for caching", "let's use", 0.65},
		{"the approach is", "The approach is microservices", "the approach is", 0.65},
		{"picked", "Picked Tailwind for styling", "picked", 0.65},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := MatchDecisions(tt.text)
			if len(matches) == 0 {
				t.Fatalf("expected at least one match for %q", tt.text)
			}
			m := matches[0]
			if m.Confidence < tt.minConf {
				t.Errorf("confidence = %v, want >= %v", m.Confidence, tt.minConf)
			}
			if m.Pattern != tt.pattern {
				t.Errorf("pattern = %q, want %q", m.Pattern, tt.pattern)
			}
		})
	}
}

func TestMatchDecisions_MultipleDecisions(t *testing.T) {
	text := `We decided to use Go for the backend.
Let's use React for the frontend.
I chose PostgreSQL for persistence.`

	matches := MatchDecisions(text)
	if len(matches) < 3 {
		t.Fatalf("got %d matches, want at least 3", len(matches))
	}
}

func TestMatchDecisions_NoMatch(t *testing.T) {
	text := "This is a regular sentence with no decisions."
	matches := MatchDecisions(text)
	if len(matches) != 0 {
		t.Errorf("got %d matches, want 0", len(matches))
	}
}

func TestMatchDecisions_EmptyInput(t *testing.T) {
	matches := MatchDecisions("")
	if len(matches) != 0 {
		t.Errorf("got %d matches, want 0", len(matches))
	}
}

func TestMatchDecisions_CaseInsensitive(t *testing.T) {
	text := "WE DECIDED to use Kubernetes for orchestration"
	matches := MatchDecisions(text)
	if len(matches) == 0 {
		t.Fatal("expected case-insensitive match")
	}
	if matches[0].Pattern != "we decided" {
		t.Errorf("pattern = %q, want %q", matches[0].Pattern, "we decided")
	}
}

func TestMatchDecisions_ValueContainsFullSentence(t *testing.T) {
	text := "Let's use Docker for containerization"
	matches := MatchDecisions(text)
	if len(matches) == 0 {
		t.Fatal("expected a match")
	}
	if matches[0].Value != text {
		t.Errorf("value = %q, want %q", matches[0].Value, text)
	}
}

func TestMatchDecisions_TopicExtraction(t *testing.T) {
	text := "We decided to use PostgreSQL"
	matches := MatchDecisions(text)
	if len(matches) == 0 {
		t.Fatal("expected a match")
	}
	if matches[0].Key == "" {
		t.Error("key should not be empty")
	}
	// Topic should be lowercase. New extractor strips "to" as filler.
	if matches[0].Key != "use postgresql" {
		t.Errorf("key = %q, want %q", matches[0].Key, "use postgresql")
	}
}

// === New tests for expanded patterns ===

func TestMatchDecisions_UsingPattern(t *testing.T) {
	tests := []string{
		"We're using PostgreSQL for persistence",
		"I'm using Redis for caching",
		"Using Kubernetes for deployment",
	}
	for _, text := range tests {
		matches := MatchDecisions(text)
		if len(matches) == 0 {
			t.Errorf("expected match for %q", text)
		}
	}
}

func TestMatchDecisions_TheStackIs(t *testing.T) {
	text := "The backend is written in Go with PostgreSQL"
	matches := MatchDecisions(text)
	if len(matches) == 0 {
		t.Fatal("expected a match for 'the X is'")
	}
}

func TestMatchDecisions_Migrated(t *testing.T) {
	text := "We migrated from MySQL to PostgreSQL last month"
	matches := MatchDecisions(text)
	if len(matches) == 0 {
		t.Fatal("expected match for 'migrated'")
	}
}

func TestMatchDecisions_KeywordBoost(t *testing.T) {
	// Signal-keyword sentences should not be below their tier baseline.
	text := "Picked PostgreSQL for the database"
	matches := MatchDecisions(text)
	if len(matches) == 0 {
		t.Fatal("expected a match")
	}
	// Just verify it's in medium tier (with possible boost).
	if matches[0].Confidence < mediumConfidence {
		t.Errorf("confidence = %v, want >= %v", matches[0].Confidence, mediumConfidence)
	}
}

func TestExtractTopic_TruncatesLongContent(t *testing.T) {
	raw := "one two three four five six seven eight"
	topic := extractTopic(raw)
	// Should contain at most 5 words.
	want := "one two three four five"
	if topic != want {
		t.Errorf("topic = %q, want %q", topic, want)
	}
}

func TestExtractTopic_EmptyInput(t *testing.T) {
	if topic := extractTopic(""); topic != "" {
		t.Errorf("topic = %q, want empty", topic)
	}
}

func TestExtractTopic_StripsPunctuation(t *testing.T) {
	topic := extractTopic("Redis for caching.")
	if topic != "redis for caching" {
		t.Errorf("topic = %q, want %q", topic, "redis for caching")
	}
}

func TestExtractTopic_StripsLeadingFillers(t *testing.T) {
	tests := []struct{ in, want string }{
		{"to use Redis", "use redis"},
		{"the backend stack", "backend stack"},
		{"a new approach", "new approach"},
	}
	for _, tt := range tests {
		if got := extractTopic(tt.in); got != tt.want {
			t.Errorf("extractTopic(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
