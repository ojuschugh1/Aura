package a2a

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	auradb "github.com/ojuschugh1/aura/internal/db"
	"github.com/ojuschugh1/aura/internal/memory"
)

func setupTestStore(t *testing.T) *memory.Store {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	database, err := auradb.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := auradb.RunMigrations(database); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return memory.New(database)
}

func TestAgentCard(t *testing.T) {
	store := setupTestStore(t)
	bridge := New(0, store, "test-agent")

	req := httptest.NewRequest("GET", "/.well-known/agent.json", nil)
	w := httptest.NewRecorder()
	bridge.handleAgentCard(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var card AgentCard
	if err := json.Unmarshal(w.Body.Bytes(), &card); err != nil {
		t.Fatalf("parse card: %v", err)
	}
	if card.Name != "Aura" {
		t.Errorf("name = %q, want %q", card.Name, "Aura")
	}
	if len(card.Skills) == 0 {
		t.Error("expected skills in agent card")
	}
}

func TestMemoryShareReadWrite(t *testing.T) {
	store := setupTestStore(t)
	bridge := New(0, store, "test-agent")

	// Write via A2A.
	writeReq := MemoryShareRequest{
		Action:  "write",
		Key:     "test-key",
		Value:   "test-value",
		AgentID: "external-agent",
	}
	body, _ := json.Marshal(writeReq)
	req := httptest.NewRequest("POST", "/a2a/memory", bytes.NewReader(body))
	w := httptest.NewRecorder()
	bridge.handleMemory(w, req)

	var writeResp MemoryShareResponse
	json.Unmarshal(w.Body.Bytes(), &writeResp)
	if writeResp.Status != "ok" {
		t.Errorf("write status = %q, want %q", writeResp.Status, "ok")
	}

	// Read via A2A.
	readReq := MemoryShareRequest{
		Action:  "read",
		Key:     "test-key",
		AgentID: "external-agent",
	}
	body, _ = json.Marshal(readReq)
	req = httptest.NewRequest("POST", "/a2a/memory", bytes.NewReader(body))
	w = httptest.NewRecorder()
	bridge.handleMemory(w, req)

	var readResp MemoryShareResponse
	json.Unmarshal(w.Body.Bytes(), &readResp)
	if readResp.Status != "ok" {
		t.Errorf("read status = %q, want %q", readResp.Status, "ok")
	}
	if readResp.Data == nil {
		t.Error("expected data in read response")
	}

	// Verify the source_tool was tagged with the A2A agent.
	entry, _ := store.Get("test-key")
	if entry.SourceTool != "a2a:external-agent" {
		t.Errorf("source_tool = %q, want %q", entry.SourceTool, "a2a:external-agent")
	}
}

func TestMemoryShareMissingAgentID(t *testing.T) {
	store := setupTestStore(t)
	bridge := New(0, store, "test-agent")

	writeReq := MemoryShareRequest{
		Action: "write",
		Key:    "test",
		Value:  "test",
		// Missing AgentID.
	}
	body, _ := json.Marshal(writeReq)
	req := httptest.NewRequest("POST", "/a2a/memory", bytes.NewReader(body))
	w := httptest.NewRecorder()
	bridge.handleMemory(w, req)

	var resp MemoryShareResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Status != "error" {
		t.Errorf("status = %q, want %q (missing agent_id)", resp.Status, "error")
	}
}

func TestMemoryShareList(t *testing.T) {
	store := setupTestStore(t)
	bridge := New(0, store, "test-agent")

	// Add some entries.
	store.Add("key1", "val1", "cli", "")
	store.Add("key2", "val2", "cli", "")

	listReq := MemoryShareRequest{
		Action:  "list",
		AgentID: "external-agent",
	}
	body, _ := json.Marshal(listReq)
	req := httptest.NewRequest("POST", "/a2a/memory", bytes.NewReader(body))
	w := httptest.NewRecorder()
	bridge.handleMemory(w, req)

	var resp MemoryShareResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Status != "ok" {
		t.Errorf("status = %q, want %q", resp.Status, "ok")
	}
}

// Suppress unused import warning.
var _ *sql.DB
