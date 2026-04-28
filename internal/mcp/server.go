// Package mcp implements a minimal MCP (Model Context Protocol) server over HTTP.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
)

// Server is a minimal MCP server that exposes tools over HTTP/JSON-RPC.
type Server struct {
	port     int
	secret   string
	handlers map[string]ToolHandler
	mu       sync.RWMutex
	httpSrv  *http.Server
	// middlewares run after each tool call succeeds. They get the tool name,
	// input params, and result. Used for real-time auto-capture.
	middlewares []PostCallMiddleware
}

// PostCallMiddleware is called after every successful tool invocation.
// It runs asynchronously — errors are logged but don't affect the response.
type PostCallMiddleware func(tool string, params map[string]interface{}, result interface{})

// ToolHandler is a function that handles an MCP tool call.
type ToolHandler func(ctx context.Context, params map[string]interface{}) (interface{}, error)

// New creates a new MCP server on the given port, authenticated with secret.
func New(port int, secret string) *Server {
	s := &Server{
		port:     port,
		secret:   secret,
		handlers: make(map[string]ToolHandler),
	}
	return s
}

// Register adds a tool handler by name.
func (s *Server) Register(name string, h ToolHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[name] = h
}

// Use registers a post-call middleware that runs asynchronously after
// every successful tool invocation.
func (s *Server) Use(mw PostCallMiddleware) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.middlewares = append(s.middlewares, mw)
}

// Start begins listening for MCP requests. It returns once the listener is ready.
func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", s.handleMCP)

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", s.port))
	if err != nil {
		return fmt.Errorf("listen on port %d: %w", s.port, err)
	}
	// Use the actual port (in case 0 was passed).
	s.port = ln.Addr().(*net.TCPAddr).Port

	s.httpSrv = &http.Server{Handler: mux}
	go func() {
		if err := s.httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Error("mcp server error", "err", err)
		}
	}()
	return nil
}

// Stop shuts down the HTTP server gracefully.
func (s *Server) Stop(ctx context.Context) error {
	if s.httpSrv == nil {
		return nil
	}
	return s.httpSrv.Shutdown(ctx)
}

// Port returns the port the server is listening on.
func (s *Server) Port() int { return s.port }

// mcpRequest is the JSON body for an MCP tool call.
type mcpRequest struct {
	Tool   string                 `json:"tool"`
	Params map[string]interface{} `json:"params"`
}

// mcpResponse is the JSON body returned to the client.
type mcpResponse struct {
	Result interface{} `json:"result,omitempty"`
	Error  *mcpError   `json:"error,omitempty"`
}

type mcpError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (s *Server) handleMCP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Authenticate via shared secret in Authorization header.
	if s.secret != "" && r.Header.Get("Authorization") != "Bearer "+s.secret {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(mcpResponse{Error: &mcpError{Code: "UNAUTHORIZED", Message: "invalid secret"}})
		return
	}

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req mcpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(mcpResponse{Error: &mcpError{Code: "INVALID_REQUEST", Message: err.Error()}})
		return
	}

	s.mu.RLock()
	h, ok := s.handlers[req.Tool]
	s.mu.RUnlock()

	if !ok {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(mcpResponse{Error: &mcpError{Code: "UNKNOWN_TOOL", Message: "tool not found: " + req.Tool}})
		return
	}

	result, err := h(r.Context(), req.Params)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(mcpResponse{Error: &mcpError{Code: "TOOL_ERROR", Message: err.Error()}})
		return
	}

	// Run post-call middlewares asynchronously (real-time auto-capture).
	s.mu.RLock()
	mws := append([]PostCallMiddleware(nil), s.middlewares...)
	s.mu.RUnlock()
	for _, mw := range mws {
		go func(fn PostCallMiddleware) {
			defer func() {
				if r := recover(); r != nil {
					slog.Warn("middleware panicked", "tool", req.Tool, "err", r)
				}
			}()
			fn(req.Tool, req.Params, result)
		}(mw)
	}

	_ = json.NewEncoder(w).Encode(mcpResponse{Result: result})
}
