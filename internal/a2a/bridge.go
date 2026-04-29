package a2a

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/ojuschugh1/aura/internal/memory"
)

// Bridge implements the A2A protocol endpoints, allowing external agents
// to discover Aura's capabilities and share verified memory.
type Bridge struct {
	port     int
	store    *memory.Store
	agentID  string
	httpSrv  *http.Server
}

// AgentCard is the A2A discovery document that describes Aura's capabilities.
type AgentCard struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	URL          string   `json:"url"`
	Version      string   `json:"version"`
	Capabilities []string `json:"capabilities"`
	Skills       []Skill  `json:"skills"`
}

// Skill describes a single capability exposed via A2A.
type Skill struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// MemoryShareRequest is an A2A request to read or write shared memory.
type MemoryShareRequest struct {
	Action    string `json:"action"`     // "read", "write", "list"
	Key       string `json:"key,omitempty"`
	Value     string `json:"value,omitempty"`
	AgentID   string `json:"agent_id"`   // requesting agent's identity
	SessionID string `json:"session_id,omitempty"`
}

// MemoryShareResponse is the A2A response.
type MemoryShareResponse struct {
	Status  string      `json:"status"` // "ok", "error", "not_found"
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
}

// New creates an A2A bridge.
func New(port int, store *memory.Store, agentID string) *Bridge {
	return &Bridge{
		port:    port,
		store:   store,
		agentID: agentID,
	}
}

// Start begins serving A2A endpoints.
func (b *Bridge) Start() error {
	mux := http.NewServeMux()

	// A2A discovery endpoint.
	mux.HandleFunc("/.well-known/agent.json", b.handleAgentCard)

	// A2A task endpoints.
	mux.HandleFunc("/a2a/memory", b.handleMemory)
	mux.HandleFunc("/a2a/health", b.handleHealth)

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", b.port))
	if err != nil {
		return fmt.Errorf("a2a listen: %w", err)
	}
	b.port = ln.Addr().(*net.TCPAddr).Port

	b.httpSrv = &http.Server{Handler: mux}
	go func() {
		if err := b.httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Error("a2a server error", "err", err)
		}
	}()
	slog.Info("a2a bridge started", "port", b.port)
	return nil
}

// Stop shuts down the A2A bridge.
func (b *Bridge) Stop() error {
	if b.httpSrv == nil {
		return nil
	}
	return b.httpSrv.Close()
}

// Port returns the bridge's listening port.
func (b *Bridge) Port() int { return b.port }

func (b *Bridge) handleAgentCard(w http.ResponseWriter, r *http.Request) {
	card := AgentCard{
		Name:        "Aura",
		Description: "Local-first AI memory, verification, and governance daemon",
		URL:         fmt.Sprintf("http://localhost:%d", b.port),
		Version:     "0.9.0",
		Capabilities: []string{
			"memory-read", "memory-write", "memory-list",
			"verification", "wiki-query",
		},
		Skills: []Skill{
			{ID: "memory-share", Name: "Shared Memory", Description: "Read and write verified cross-agent memory"},
			{ID: "wiki-query", Name: "Wiki Query", Description: "Search the compounding knowledge wiki"},
			{ID: "verify", Name: "Claim Verification", Description: "Verify whether agent claims are true"},
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(card)
}

func (b *Bridge) handleMemory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req MemoryShareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(MemoryShareResponse{Status: "error", Message: "invalid JSON"})
		return
	}

	if req.AgentID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(MemoryShareResponse{Status: "error", Message: "agent_id required"})
		return
	}

	switch req.Action {
	case "read":
		entry, err := b.store.Get(req.Key)
		if err != nil {
			json.NewEncoder(w).Encode(MemoryShareResponse{Status: "not_found"})
			return
		}
		json.NewEncoder(w).Encode(MemoryShareResponse{Status: "ok", Data: entry})

	case "write":
		_, err := b.store.Add(req.Key, req.Value, "a2a:"+req.AgentID, req.SessionID)
		if err != nil {
			json.NewEncoder(w).Encode(MemoryShareResponse{Status: "error", Message: err.Error()})
			return
		}
		slog.Info("a2a memory write", "key", req.Key, "agent", req.AgentID)
		json.NewEncoder(w).Encode(MemoryShareResponse{Status: "ok"})

	case "list":
		entries, err := b.store.List(memory.ListFilter{})
		if err != nil {
			json.NewEncoder(w).Encode(MemoryShareResponse{Status: "error", Message: err.Error()})
			return
		}
		json.NewEncoder(w).Encode(MemoryShareResponse{Status: "ok", Data: entries})

	default:
		json.NewEncoder(w).Encode(MemoryShareResponse{Status: "error", Message: "unknown action: " + req.Action})
	}
}

func (b *Bridge) handleHealth(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "healthy",
		"agent_id": b.agentID,
		"uptime":   time.Now().Format(time.RFC3339),
	})
}
