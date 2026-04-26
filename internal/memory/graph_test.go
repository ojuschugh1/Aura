package memory

import (
	"testing"
)

// --- AddEdge ----------------------------------------------------------------

func TestAddEdge_CreatesEdge(t *testing.T) {
	s := newTestStore(t)

	edge, err := s.AddEdge("db.host", "db.port", "depends-on", "cli", "sess-1", 0.9)
	if err != nil {
		t.Fatalf("AddEdge: %v", err)
	}
	if edge.FromKey != "db.host" {
		t.Errorf("FromKey = %q, want %q", edge.FromKey, "db.host")
	}
	if edge.ToKey != "db.port" {
		t.Errorf("ToKey = %q, want %q", edge.ToKey, "db.port")
	}
	if edge.Relation != "depends-on" {
		t.Errorf("Relation = %q, want %q", edge.Relation, "depends-on")
	}
	if edge.Confidence != 0.9 {
		t.Errorf("Confidence = %f, want 0.9", edge.Confidence)
	}
	if edge.ID == 0 {
		t.Error("expected non-zero ID")
	}
}

func TestAddEdge_DefaultRelation(t *testing.T) {
	s := newTestStore(t)

	edge, err := s.AddEdge("a", "b", "", "cli", "", 1.0)
	if err != nil {
		t.Fatalf("AddEdge: %v", err)
	}
	if edge.Relation != "related-to" {
		t.Errorf("Relation = %q, want %q", edge.Relation, "related-to")
	}
}

func TestAddEdge_UpsertUpdatesConfidence(t *testing.T) {
	s := newTestStore(t)

	_, err := s.AddEdge("a", "b", "depends-on", "cli", "", 0.5)
	if err != nil {
		t.Fatalf("AddEdge: %v", err)
	}

	edge, err := s.AddEdge("a", "b", "depends-on", "mcp", "", 0.9)
	if err != nil {
		t.Fatalf("AddEdge upsert: %v", err)
	}
	if edge.Confidence != 0.9 {
		t.Errorf("Confidence = %f, want 0.9 after upsert", edge.Confidence)
	}
	if edge.SourceTool != "mcp" {
		t.Errorf("SourceTool = %q, want %q after upsert", edge.SourceTool, "mcp")
	}
}

func TestAddEdge_DifferentRelationsAreDistinct(t *testing.T) {
	s := newTestStore(t)

	_, err := s.AddEdge("a", "b", "depends-on", "cli", "", 1.0)
	if err != nil {
		t.Fatalf("AddEdge depends-on: %v", err)
	}
	_, err = s.AddEdge("a", "b", "includes", "cli", "", 1.0)
	if err != nil {
		t.Fatalf("AddEdge includes: %v", err)
	}

	edges, err := s.GetEdges("a")
	if err != nil {
		t.Fatalf("GetEdges: %v", err)
	}
	if len(edges) != 2 {
		t.Errorf("expected 2 edges, got %d", len(edges))
	}
}

// --- GetEdges ---------------------------------------------------------------

func TestGetEdges_ReturnsFromAndToEdges(t *testing.T) {
	s := newTestStore(t)

	_, _ = s.AddEdge("a", "b", "depends-on", "cli", "", 1.0)
	_, _ = s.AddEdge("c", "a", "includes", "cli", "", 1.0)

	edges, err := s.GetEdges("a")
	if err != nil {
		t.Fatalf("GetEdges: %v", err)
	}
	if len(edges) != 2 {
		t.Errorf("expected 2 edges for 'a', got %d", len(edges))
	}
}

func TestGetEdges_EmptyForUnknownKey(t *testing.T) {
	s := newTestStore(t)

	edges, err := s.GetEdges("nonexistent")
	if err != nil {
		t.Fatalf("GetEdges: %v", err)
	}
	if len(edges) != 0 {
		t.Errorf("expected 0 edges, got %d", len(edges))
	}
}

// --- GetRelated -------------------------------------------------------------

func TestGetRelated_ReturnsConnectedEntries(t *testing.T) {
	s := newTestStore(t)

	_, _ = s.Add("a", "value-a", "cli", "")
	_, _ = s.Add("b", "value-b", "cli", "")
	_, _ = s.Add("c", "value-c", "cli", "")
	_, _ = s.AddEdge("a", "b", "depends-on", "cli", "", 1.0)
	_, _ = s.AddEdge("c", "a", "includes", "cli", "", 1.0)

	entries, err := s.GetRelated("a")
	if err != nil {
		t.Fatalf("GetRelated: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 related entries, got %d", len(entries))
	}

	keys := make(map[string]bool)
	for _, e := range entries {
		keys[e.Key] = true
	}
	if !keys["b"] {
		t.Error("expected 'b' in related entries")
	}
	if !keys["c"] {
		t.Error("expected 'c' in related entries")
	}
}

func TestGetRelated_EmptyWhenNoEdges(t *testing.T) {
	s := newTestStore(t)

	_, _ = s.Add("lonely", "value", "cli", "")

	entries, err := s.GetRelated("lonely")
	if err != nil {
		t.Fatalf("GetRelated: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 related entries, got %d", len(entries))
	}
}

// --- DeleteEdge -------------------------------------------------------------

func TestDeleteEdge_RemovesEdge(t *testing.T) {
	s := newTestStore(t)

	_, _ = s.AddEdge("a", "b", "depends-on", "cli", "", 1.0)

	if err := s.DeleteEdge("a", "b", "depends-on"); err != nil {
		t.Fatalf("DeleteEdge: %v", err)
	}

	edges, _ := s.GetEdges("a")
	if len(edges) != 0 {
		t.Errorf("expected 0 edges after delete, got %d", len(edges))
	}
}

func TestDeleteEdge_NonExistentReturnsError(t *testing.T) {
	s := newTestStore(t)

	err := s.DeleteEdge("x", "y", "related-to")
	if err == nil {
		t.Fatal("expected error for non-existent edge, got nil")
	}
}

func TestDeleteEdge_DefaultRelation(t *testing.T) {
	s := newTestStore(t)

	_, _ = s.AddEdge("a", "b", "related-to", "cli", "", 1.0)

	if err := s.DeleteEdge("a", "b", ""); err != nil {
		t.Fatalf("DeleteEdge with empty relation: %v", err)
	}
}

// --- AllEdges ---------------------------------------------------------------

func TestAllEdges_ReturnsAll(t *testing.T) {
	s := newTestStore(t)

	_, _ = s.AddEdge("a", "b", "depends-on", "cli", "", 1.0)
	_, _ = s.AddEdge("c", "d", "includes", "cli", "", 0.8)

	edges, err := s.AllEdges()
	if err != nil {
		t.Fatalf("AllEdges: %v", err)
	}
	if len(edges) != 2 {
		t.Errorf("expected 2 edges, got %d", len(edges))
	}
}
