package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// Proxy is a transparent MCP proxy that intercepts all tool calls between
// an AI client and upstream MCP servers. It logs, governs, and enriches
// every interaction without the client or server knowing it's there.
type Proxy struct {
	port       int
	upstreams  map[string]*Upstream // name → upstream config
	mu         sync.RWMutex
	httpSrv    *http.Server
	hooks      []ProxyHook
	stats      ProxyStats
	callLog    []CallRecord
	callLogMu  sync.Mutex
	maxLogSize int
}

// Upstream represents a backend MCP server that the proxy forwards to.
type Upstream struct {
	Name    string            `json:"name"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

// ProxyHook is called on every proxied request. Return an error to block the call.
type ProxyHook func(ctx context.Context, call *CallRecord) error

// CallRecord captures a single proxied MCP tool call.
type CallRecord struct {
	ID           string                 `json:"id"`
	Timestamp    time.Time              `json:"timestamp"`
	Upstream     string                 `json:"upstream"`
	Tool         string                 `json:"tool"`
	Params       map[string]interface{} `json:"params"`
	Response     interface{}            `json:"response,omitempty"`
	Error        string                 `json:"error,omitempty"`
	Blocked      bool                   `json:"blocked"`
	BlockReason  string                 `json:"block_reason,omitempty"`
	LatencyMs    int64                  `json:"latency_ms"`
	TokensIn     int                    `json:"tokens_in,omitempty"`
	TokensOut    int                    `json:"tokens_out,omitempty"`
}

// ProxyStats tracks aggregate proxy metrics.
type ProxyStats struct {
	TotalCalls   atomic.Int64 `json:"total_calls"`
	BlockedCalls atomic.Int64 `json:"blocked_calls"`
	ErrorCalls   atomic.Int64 `json:"error_calls"`
	TotalLatency atomic.Int64 `json:"total_latency_ms"`
}

// New creates a proxy listening on the given port.
func New(port int) *Proxy {
	return &Proxy{
		port:       port,
		upstreams:  make(map[string]*Upstream),
		maxLogSize: 10000,
	}
}

// AddUpstream registers a backend MCP server.
func (p *Proxy) AddUpstream(name, url string, headers map[string]string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.upstreams[name] = &Upstream{Name: name, URL: url, Headers: headers}
}

// OnCall registers a hook that fires on every proxied call.
func (p *Proxy) OnCall(hook ProxyHook) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.hooks = append(p.hooks, hook)
}

// Start begins listening for proxied MCP requests.
func (p *Proxy) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/proxy/", p.handleProxy)
	mux.HandleFunc("/proxy/stats", p.handleStats)
	mux.HandleFunc("/proxy/log", p.handleLog)
	mux.HandleFunc("/proxy/upstreams", p.handleUpstreams)

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p.port))
	if err != nil {
		return fmt.Errorf("proxy listen: %w", err)
	}
	p.port = ln.Addr().(*net.TCPAddr).Port

	p.httpSrv = &http.Server{Handler: mux}
	go func() {
		if err := p.httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Error("proxy server error", "err", err)
		}
	}()
	slog.Info("mcp proxy started", "port", p.port)
	return nil
}

// Stop shuts down the proxy.
func (p *Proxy) Stop(ctx context.Context) error {
	if p.httpSrv == nil {
		return nil
	}
	return p.httpSrv.Shutdown(ctx)
}

// Port returns the proxy's listening port.
func (p *Proxy) Port() int { return p.port }

// GetStats returns current aggregate stats.
func (p *Proxy) GetStats() map[string]int64 {
	return map[string]int64{
		"total_calls":      p.stats.TotalCalls.Load(),
		"blocked_calls":    p.stats.BlockedCalls.Load(),
		"error_calls":      p.stats.ErrorCalls.Load(),
		"avg_latency_ms":   p.avgLatency(),
	}
}

// GetLog returns the most recent N call records.
func (p *Proxy) GetLog(limit int) []CallRecord {
	p.callLogMu.Lock()
	defer p.callLogMu.Unlock()
	if limit <= 0 || limit > len(p.callLog) {
		limit = len(p.callLog)
	}
	// Return most recent first.
	start := len(p.callLog) - limit
	result := make([]CallRecord, limit)
	copy(result, p.callLog[start:])
	// Reverse.
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result
}

func (p *Proxy) avgLatency() int64 {
	total := p.stats.TotalCalls.Load()
	if total == 0 {
		return 0
	}
	return p.stats.TotalLatency.Load() / total
}

// handleProxy routes /proxy/{upstream}/mcp to the named upstream.
func (p *Proxy) handleProxy(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Parse the upstream name from the URL: /proxy/{name}/mcp
	parts := splitPath(r.URL.Path)
	if len(parts) < 3 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "use /proxy/{upstream}/mcp"})
		return
	}
	upstreamName := parts[1]

	p.mu.RLock()
	upstream, ok := p.upstreams[upstreamName]
	p.mu.RUnlock()
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "unknown upstream: " + upstreamName})
		return
	}

	// Read and parse the request body.
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var req struct {
		Tool   string                 `json:"tool"`
		Params map[string]interface{} `json:"params"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON"})
		return
	}

	// Create call record.
	record := &CallRecord{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		Timestamp: time.Now(),
		Upstream:  upstreamName,
		Tool:      req.Tool,
		Params:    req.Params,
	}

	// Run pre-call hooks (policy enforcement).
	p.mu.RLock()
	hooks := append([]ProxyHook(nil), p.hooks...)
	p.mu.RUnlock()

	for _, hook := range hooks {
		if err := hook(r.Context(), record); err != nil {
			record.Blocked = true
			record.BlockReason = err.Error()
			p.recordCall(record)
			p.stats.BlockedCalls.Add(1)
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{
				"error":  "blocked by policy",
				"reason": err.Error(),
			})
			return
		}
	}

	// Forward to upstream.
	start := time.Now()
	upReq, _ := http.NewRequestWithContext(r.Context(), http.MethodPost, upstream.URL, bytes.NewReader(body))
	upReq.Header.Set("Content-Type", "application/json")
	for k, v := range upstream.Headers {
		upReq.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(upReq)
	record.LatencyMs = time.Since(start).Milliseconds()

	if err != nil {
		record.Error = err.Error()
		p.recordCall(record)
		p.stats.ErrorCalls.Add(1)
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(map[string]string{"error": "upstream error: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var respData interface{}
	json.Unmarshal(respBody, &respData)
	record.Response = respData

	// Estimate tokens from body sizes.
	record.TokensIn = len(body) / 4
	record.TokensOut = len(respBody) / 4

	p.recordCall(record)

	// Forward the response back to the client.
	w.WriteHeader(resp.StatusCode)
	w.Write(respBody)
}

func (p *Proxy) recordCall(record *CallRecord) {
	p.stats.TotalCalls.Add(1)
	p.stats.TotalLatency.Add(record.LatencyMs)

	p.callLogMu.Lock()
	defer p.callLogMu.Unlock()
	p.callLog = append(p.callLog, *record)
	// Trim if too large.
	if len(p.callLog) > p.maxLogSize {
		p.callLog = p.callLog[len(p.callLog)-p.maxLogSize:]
	}
}

func (p *Proxy) handleStats(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(p.GetStats())
}

func (p *Proxy) handleLog(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(p.GetLog(100))
}

func (p *Proxy) handleUpstreams(w http.ResponseWriter, r *http.Request) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	json.NewEncoder(w).Encode(p.upstreams)
}

func splitPath(path string) []string {
	var parts []string
	for _, p := range bytes.Split([]byte(path), []byte("/")) {
		if len(p) > 0 {
			parts = append(parts, string(p))
		}
	}
	return parts
}
