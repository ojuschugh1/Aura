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
	if m.Confidence != strongConfidence {
		t.Errorf("confidence = %v, want %v", m.Confidence, strongConfidence)
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
	if m.Confidence != strongConfidence {
		t.Errorf("confidence = %v, want %v", m.Confidence, strongConfidence)
	}
	if m.Pattern != "I chose" {
		t.Errorf("pattern = %q, want %q", m.Pattern, "I chose")
	}
}

func TestMatchDecisions_WeakPatterns(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		pattern string
	}{
		{"lets use", "Let's use Redis for caching", "let's use"},
		{"going with", "Going with gRPC for the API layer", "going with"},
		{"the approach is", "The approach is to use microservices", "the approach is"},
		{"switched to", "Switched to Vite from Webpack", "switched to"},
		{"picked", "Picked Tailwind CSS for styling", "picked"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := MatchDecisions(tt.text)
			if len(matches) == 0 {
				t.Fatal("expected at least one match")
			}
			m := matches[0]
			if m.Confidence != weakConfidence {
				t.Errorf("confidence = %v, want %v", m.Confidence, weakConfidence)
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
	if len(matches) != 3 {
		t.Fatalf("got %d matches, want 3", len(matches))
	}
	// First match should be strong.
	if matches[0].Confidence != strongConfidence {
		t.Errorf("first match confidence = %v, want %v", matches[0].Confidence, strongConfidence)
	}
	// Second match should be weak.
	if matches[1].Confidence != weakConfidence {
		t.Errorf("second match confidence = %v, want %v", matches[1].Confidence, weakConfidence)
	}
	// Third match should be strong.
	if matches[2].Confidence != strongConfidence {
		t.Errorf("third match confidence = %v, want %v", matches[2].Confidence, strongConfidence)
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
	// Topic should be lowercase.
	if matches[0].Key != "use postgresql" {
		t.Errorf("key = %q, want %q", matches[0].Key, "use postgresql")
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
