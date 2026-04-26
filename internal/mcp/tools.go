package mcp

import (
	"database/sql"

	"github.com/ojuschugh1/aura/internal/compress"
	"github.com/ojuschugh1/aura/internal/escrow"
	"github.com/ojuschugh1/aura/internal/memory"
	"github.com/ojuschugh1/aura/internal/policy"
	"github.com/ojuschugh1/aura/internal/router"
	"github.com/ojuschugh1/aura/internal/wiki"
)

// RegisterCoreTools registers the v0.1 memory, verify, and cost tools on the server.
func RegisterCoreTools(s *Server, store *memory.Store, db *sql.DB) {
	h := &handlers{store: store, db: db}
	s.Register("memory_write", h.memoryWrite)
	s.Register("memory_read", h.memoryRead)
	s.Register("memory_list", h.memoryList)
	s.Register("memory_delete", h.memoryDelete)
	s.Register("verify_session", h.verifySession)
	s.Register("cost_summary", h.costSummary)
}

// RegisterCompressTools registers the v0.2 compact_context tool.
func RegisterCompressTools(s *Server, engine *compress.Engine) {
	h := &handlers{engine: engine}
	s.Register("compact_context", h.compactContext)
}

// RegisterEscrowTools registers the v0.3 check_action and escrow_decide tools.
func RegisterEscrowTools(s *Server, escrowStore *escrow.Store, policyEngine *policy.Engine) {
	h := &handlers{escrowStore: escrowStore, policyEngine: policyEngine}
	s.Register("check_action", h.checkAction)
	s.Register("escrow_decide", h.escrowDecide)
}

// RegisterScanTools registers the v0.3 scan_deps tool.
func RegisterScanTools(s *Server) {
	h := &handlers{}
	s.Register("scan_deps", h.scanDeps)
}

// RegisterTraceTools registers the v0.4 trace_summary tool.
func RegisterTraceTools(s *Server, tracesDir string) {
	h := &handlers{tracesDir: tracesDir}
	s.Register("trace_summary", h.traceSummary)
}

// RegisterRouterTools registers the v0.6 route_task tool.
func RegisterRouterTools(s *Server, r *router.Router) {
	h := &handlers{modelRouter: r}
	s.Register("route_task", h.routeTask)
}

// RegisterWikiTools registers the v0.7 wiki tools.
func RegisterWikiTools(s *Server, engine *wiki.Engine) {
	h := &handlers{wikiEngine: engine}
	s.Register("wiki_ingest", h.wikiIngest)
	s.Register("wiki_query", h.wikiQuery)
	s.Register("wiki_lint", h.wikiLint)
	s.Register("wiki_search", h.wikiSearch)
	s.Register("wiki_read", h.wikiRead)
	s.Register("wiki_write", h.wikiWrite)
	s.Register("wiki_index", h.wikiIndex)
	s.Register("wiki_log", h.wikiLog)
}
