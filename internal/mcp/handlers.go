package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/ojuschugh1/aura/internal/compress"
	"github.com/ojuschugh1/aura/internal/cost"
	"github.com/ojuschugh1/aura/internal/escrow"
	"github.com/ojuschugh1/aura/internal/memory"
	"github.com/ojuschugh1/aura/internal/policy"
	"github.com/ojuschugh1/aura/internal/router"
	"github.com/ojuschugh1/aura/internal/scan"
	"github.com/ojuschugh1/aura/internal/trace"
	"github.com/ojuschugh1/aura/internal/wiki"
)

// handlers holds service dependencies for MCP tool handlers.
type handlers struct {
	store        *memory.Store
	engine       *compress.Engine
	db           *sql.DB
	escrowStore  *escrow.Store
	policyEngine *policy.Engine
	tracesDir    string
	modelRouter  *router.Router
	wikiEngine   *wiki.Engine
}

func (h *handlers) memoryWrite(_ context.Context, params map[string]interface{}) (interface{}, error) {
	key, ok := stringParam(params, "key")
	if !ok {
		return nil, fmt.Errorf("INVALID_PARAMS: key is required")
	}
	value, ok := stringParam(params, "value")
	if !ok {
		return nil, fmt.Errorf("INVALID_PARAMS: value is required")
	}
	sourceTool, _ := stringParam(params, "source_tool")
	if sourceTool == "" {
		sourceTool = "mcp"
	}
	sessionID, _ := stringParam(params, "session_id")

	entry, err := h.store.Add(key, value, sourceTool, sessionID)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"key":        entry.Key,
		"timestamp":  entry.UpdatedAt,
		"session_id": entry.SessionID,
	}, nil
}

func (h *handlers) memoryRead(_ context.Context, params map[string]interface{}) (interface{}, error) {
	key, ok := stringParam(params, "key")
	if !ok {
		return nil, fmt.Errorf("INVALID_PARAMS: key is required")
	}
	entry, err := h.store.Get(key)
	if err != nil {
		return nil, fmt.Errorf("MEMORY_NOT_FOUND: %w", err)
	}
	return entry, nil
}

func (h *handlers) memoryList(_ context.Context, params map[string]interface{}) (interface{}, error) {
	agent, _ := stringParam(params, "agent")
	entries, err := h.store.List(memory.ListFilter{Agent: agent})
	if err != nil {
		return nil, err
	}
	return entries, nil
}

func (h *handlers) memoryDelete(_ context.Context, params map[string]interface{}) (interface{}, error) {
	key, ok := stringParam(params, "key")
	if !ok {
		return nil, fmt.Errorf("INVALID_PARAMS: key is required")
	}
	if err := h.store.Delete(key); err != nil {
		return nil, fmt.Errorf("MEMORY_NOT_FOUND: %w", err)
	}
	return map[string]bool{"deleted": true}, nil
}

func (h *handlers) verifySession(_ context.Context, params map[string]interface{}) (interface{}, error) {
	// Placeholder — full implementation in task 8.
	sessionID, _ := stringParam(params, "session_id")
	return map[string]interface{}{
		"session_id": sessionID,
		"message":    "verify_session not yet implemented",
	}, nil
}

func (h *handlers) costSummary(_ context.Context, params map[string]interface{}) (interface{}, error) {
	if h.db == nil {
		return nil, fmt.Errorf("database not initialised")
	}
	period, _ := stringParam(params, "period")
	if period == "" {
		period = "session"
	}
	sessionID, _ := stringParam(params, "session_id")

	var summary interface{}
	var err error
	switch period {
	case "daily":
		summary, err = cost.DailySummary(h.db)
	case "weekly":
		summary, err = cost.WeeklySummary(h.db)
	default:
		if sessionID == "" {
			return map[string]string{"message": "provide session_id for session summary"}, nil
		}
		summary, err = cost.SessionSummary(h.db, sessionID)
	}
	if err != nil {
		return nil, err
	}
	return summary, nil
}

// stringParam extracts a string value from params by key.
func stringParam(params map[string]interface{}, key string) (string, bool) {
	v, ok := params[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func (h *handlers) compactContext(_ context.Context, params map[string]interface{}) (interface{}, error) {
	content, ok := stringParam(params, "content")
	if !ok {
		return nil, fmt.Errorf("INVALID_PARAMS: content is required")
	}
	if h.engine == nil {
		return nil, fmt.Errorf("compression engine not initialised")
	}
	res, err := h.engine.Compact(content)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"original_tokens":   res.OriginalTokens,
		"compressed_tokens": res.CompressedTokens,
		"reduction_pct":     res.ReductionPct,
		"compressed":        res.Compressed,
	}, nil
}

func (h *handlers) checkAction(_ context.Context, params map[string]interface{}) (interface{}, error) {
	actionType, _ := stringParam(params, "action_type")
	target, _ := stringParam(params, "target")
	agent, _ := stringParam(params, "agent")
	sessionID, _ := stringParam(params, "session_id")

	if actionType == "" || target == "" {
		return nil, fmt.Errorf("INVALID_PARAMS: action_type and target are required")
	}

	disposition := "require-approval"
	if h.policyEngine != nil {
		disposition = h.policyEngine.Evaluate(actionType, target)
	}

	if disposition == "auto-approve" {
		return map[string]interface{}{"disposition": "approved"}, nil
	}
	if disposition == "deny" {
		return map[string]interface{}{"disposition": "denied"}, nil
	}

	// Create escrow for require-approval actions.
	if h.escrowStore == nil {
		return map[string]interface{}{"disposition": "pending"}, nil
	}
	ea, err := h.escrowStore.Create(sessionID, actionType, target, agent, "", nil)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"disposition": "pending", "escrow_id": ea.ID}, nil
}

func (h *handlers) escrowDecide(_ context.Context, params map[string]interface{}) (interface{}, error) {
	escrowID, ok := stringParam(params, "escrow_id")
	if !ok {
		return nil, fmt.Errorf("INVALID_PARAMS: escrow_id is required")
	}
	decision, ok := stringParam(params, "decision")
	if !ok {
		return nil, fmt.Errorf("INVALID_PARAMS: decision is required (approve or deny)")
	}
	if h.escrowStore == nil {
		return nil, fmt.Errorf("escrow store not initialised")
	}
	if err := h.escrowStore.Decide(escrowID, decision, "developer"); err != nil {
		return nil, err
	}
	return map[string]bool{"executed": decision == "approve"}, nil
}

func (h *handlers) scanDeps(_ context.Context, params map[string]interface{}) (interface{}, error) {
	path, _ := stringParam(params, "path")
	if path == "" {
		path = "."
	}
	format, _ := stringParam(params, "format")

	result, err := scan.Scan(path)
	if err != nil {
		return nil, err
	}

	if format == "sarif" {
		sarifJSON, err := scan.ToSARIF(result.Phantoms)
		if err != nil {
			return nil, err
		}
		return string(sarifJSON), nil
	}

	return map[string]interface{}{
		"phantoms":  result.Phantoms,
		"high_risk": result.HighRisk,
		"summary": map[string]int{
			"total":     len(result.Phantoms),
			"high_risk": len(result.HighRisk),
		},
	}, nil
}

func (h *handlers) traceSummary(_ context.Context, params map[string]interface{}) (interface{}, error) {
	sessionID, _ := stringParam(params, "session_id")
	if sessionID == "" {
		return nil, fmt.Errorf("INVALID_PARAMS: session_id is required")
	}
	results, err := trace.Search(h.tracesDir, sessionID)
	if err != nil || len(results) == 0 {
		return nil, fmt.Errorf("trace not found for session %s", sessionID)
	}
	return map[string]interface{}{
		"session_id": sessionID,
		"file":       results[0].File,
	}, nil
}

func (h *handlers) routeTask(_ context.Context, params map[string]interface{}) (interface{}, error) {
	content, ok := stringParam(params, "content")
	if !ok {
		return nil, fmt.Errorf("INVALID_PARAMS: content is required")
	}
	if h.modelRouter == nil {
		return nil, fmt.Errorf("model router not initialised")
	}
	sessionID, _ := stringParam(params, "session_id")
	decision, err := h.modelRouter.Route(sessionID, content)
	if err != nil {
		return nil, err
	}
	return decision, nil
}

// --- Wiki handlers (v0.7) ---

func (h *handlers) wikiIngest(_ context.Context, params map[string]interface{}) (interface{}, error) {
	if h.wikiEngine == nil {
		return nil, fmt.Errorf("wiki engine not initialised")
	}
	title, _ := stringParam(params, "title")
	content, ok := stringParam(params, "content")
	if !ok {
		return nil, fmt.Errorf("INVALID_PARAMS: content is required")
	}
	if title == "" {
		title = "Untitled Source"
	}
	format, _ := stringParam(params, "format")
	if format == "" {
		format = "text"
	}
	origin, _ := stringParam(params, "origin")

	result, err := h.wikiEngine.Ingest(title, content, format, origin)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (h *handlers) wikiQuery(_ context.Context, params map[string]interface{}) (interface{}, error) {
	if h.wikiEngine == nil {
		return nil, fmt.Errorf("wiki engine not initialised")
	}
	query, ok := stringParam(params, "query")
	if !ok {
		return nil, fmt.Errorf("INVALID_PARAMS: query is required")
	}
	result, err := h.wikiEngine.Query(query)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (h *handlers) wikiLint(_ context.Context, _ map[string]interface{}) (interface{}, error) {
	if h.wikiEngine == nil {
		return nil, fmt.Errorf("wiki engine not initialised")
	}
	result, err := h.wikiEngine.Lint()
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (h *handlers) wikiSearch(_ context.Context, params map[string]interface{}) (interface{}, error) {
	if h.wikiEngine == nil {
		return nil, fmt.Errorf("wiki engine not initialised")
	}
	query, ok := stringParam(params, "query")
	if !ok {
		return nil, fmt.Errorf("INVALID_PARAMS: query is required")
	}
	pages, err := h.wikiEngine.Store().SearchPages(query)
	if err != nil {
		return nil, err
	}
	return pages, nil
}

func (h *handlers) wikiRead(_ context.Context, params map[string]interface{}) (interface{}, error) {
	if h.wikiEngine == nil {
		return nil, fmt.Errorf("wiki engine not initialised")
	}
	slug, ok := stringParam(params, "slug")
	if !ok {
		return nil, fmt.Errorf("INVALID_PARAMS: slug is required")
	}
	page, err := h.wikiEngine.Store().GetPage(slug)
	if err != nil {
		return nil, fmt.Errorf("WIKI_PAGE_NOT_FOUND: %w", err)
	}
	return page, nil
}

func (h *handlers) wikiWrite(_ context.Context, params map[string]interface{}) (interface{}, error) {
	if h.wikiEngine == nil {
		return nil, fmt.Errorf("wiki engine not initialised")
	}
	slug, ok := stringParam(params, "slug")
	if !ok {
		return nil, fmt.Errorf("INVALID_PARAMS: slug is required")
	}
	title, _ := stringParam(params, "title")
	content, ok := stringParam(params, "content")
	if !ok {
		return nil, fmt.Errorf("INVALID_PARAMS: content is required")
	}
	category, _ := stringParam(params, "category")
	if category == "" {
		category = "entity"
	}

	// Try update first, create if not found.
	page, err := h.wikiEngine.Store().UpdatePage(slug, content, nil, nil, nil)
	if err != nil {
		if title == "" {
			title = slug
		}
		page, err = h.wikiEngine.Store().CreatePage(slug, title, content, category, nil, nil, nil)
		if err != nil {
			return nil, err
		}
	}
	return page, nil
}

func (h *handlers) wikiIndex(_ context.Context, _ map[string]interface{}) (interface{}, error) {
	if h.wikiEngine == nil {
		return nil, fmt.Errorf("wiki engine not initialised")
	}
	idx, err := h.wikiEngine.Store().BuildIndex()
	if err != nil {
		return nil, err
	}
	return idx, nil
}

func (h *handlers) wikiLog(_ context.Context, params map[string]interface{}) (interface{}, error) {
	if h.wikiEngine == nil {
		return nil, fmt.Errorf("wiki engine not initialised")
	}
	limit := 20
	if v, ok := params["limit"]; ok {
		if f, ok := v.(float64); ok {
			limit = int(f)
		}
	}
	entries, err := h.wikiEngine.Store().RecentLog(limit)
	if err != nil {
		return nil, err
	}
	return entries, nil
}

func (h *handlers) wikiExport(_ context.Context, params map[string]interface{}) (interface{}, error) {
	if h.wikiEngine == nil {
		return nil, fmt.Errorf("wiki engine not initialised")
	}
	outDir, _ := stringParam(params, "dir")
	if outDir == "" {
		outDir = "/tmp/aura-wiki-export"
	}
	result, err := h.wikiEngine.ExportMarkdown(outDir)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (h *handlers) wikiGraph(_ context.Context, _ map[string]interface{}) (interface{}, error) {
	if h.wikiEngine == nil {
		return nil, fmt.Errorf("wiki engine not initialised")
	}
	stats, err := h.wikiEngine.Graph()
	if err != nil {
		return nil, err
	}
	return stats, nil
}

func (h *handlers) wikiSaveQuery(_ context.Context, params map[string]interface{}) (interface{}, error) {
	if h.wikiEngine == nil {
		return nil, fmt.Errorf("wiki engine not initialised")
	}
	query, ok := stringParam(params, "query")
	if !ok {
		return nil, fmt.Errorf("INVALID_PARAMS: query is required")
	}
	// Run the query first, then save.
	result, err := h.wikiEngine.Query(query)
	if err != nil {
		return nil, err
	}
	if result.PageCount == 0 {
		return map[string]string{"message": "no results to save"}, nil
	}
	slug, err := h.wikiEngine.SaveQueryResult(result)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"saved_slug": slug,
		"query":      query,
		"page_count": result.PageCount,
	}, nil
}

// --- Wiki tool feed handlers ---

func (h *handlers) wikiFeedSQZ(_ context.Context, params map[string]interface{}) (interface{}, error) {
	if h.wikiEngine == nil {
		return nil, fmt.Errorf("wiki engine not initialised")
	}
	report := wiki.SQZReport{}
	if v, ok := params["session_id"]; ok {
		report.SessionID, _ = v.(string)
	}
	if v, ok := params["original_tokens"]; ok {
		if f, ok := v.(float64); ok {
			report.OriginalTokens = int(f)
		}
	}
	if v, ok := params["compressed_tokens"]; ok {
		if f, ok := v.(float64); ok {
			report.CompressedTokens = int(f)
		}
	}
	if v, ok := params["reduction_pct"]; ok {
		report.ReductionPct, _ = v.(float64)
	}
	if v, ok := params["deduplicated"]; ok {
		report.Deduplicated, _ = v.(bool)
	}
	result, err := h.wikiEngine.IngestSQZ(report)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (h *handlers) wikiFeedGhostDep(_ context.Context, params map[string]interface{}) (interface{}, error) {
	if h.wikiEngine == nil {
		return nil, fmt.Errorf("wiki engine not initialised")
	}
	// Accept the full report as a JSON blob in "report" param.
	reportData, ok := params["report"]
	if !ok {
		return nil, fmt.Errorf("INVALID_PARAMS: report is required")
	}
	b, err := json.Marshal(reportData)
	if err != nil {
		return nil, fmt.Errorf("marshal report: %w", err)
	}
	var report wiki.GhostDepReport
	if err := json.Unmarshal(b, &report); err != nil {
		return nil, fmt.Errorf("parse ghostdep report: %w", err)
	}
	result, err := h.wikiEngine.IngestGhostDep(report)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (h *handlers) wikiFeedClaimCheck(_ context.Context, params map[string]interface{}) (interface{}, error) {
	if h.wikiEngine == nil {
		return nil, fmt.Errorf("wiki engine not initialised")
	}
	reportData, ok := params["report"]
	if !ok {
		return nil, fmt.Errorf("INVALID_PARAMS: report is required")
	}
	b, err := json.Marshal(reportData)
	if err != nil {
		return nil, fmt.Errorf("marshal report: %w", err)
	}
	var report wiki.ClaimCheckReport
	if err := json.Unmarshal(b, &report); err != nil {
		return nil, fmt.Errorf("parse claimcheck report: %w", err)
	}
	result, err := h.wikiEngine.IngestClaimCheck(report)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (h *handlers) wikiFeedEtch(_ context.Context, params map[string]interface{}) (interface{}, error) {
	if h.wikiEngine == nil {
		return nil, fmt.Errorf("wiki engine not initialised")
	}
	reportData, ok := params["report"]
	if !ok {
		return nil, fmt.Errorf("INVALID_PARAMS: report is required")
	}
	b, err := json.Marshal(reportData)
	if err != nil {
		return nil, fmt.Errorf("marshal report: %w", err)
	}
	var report wiki.EtchReport
	if err := json.Unmarshal(b, &report); err != nil {
		return nil, fmt.Errorf("parse etch report: %w", err)
	}
	result, err := h.wikiEngine.IngestEtch(report)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (h *handlers) wikiFeedJSON(_ context.Context, params map[string]interface{}) (interface{}, error) {
	if h.wikiEngine == nil {
		return nil, fmt.Errorf("wiki engine not initialised")
	}
	toolName, _ := stringParam(params, "tool")
	if toolName == "" {
		toolName = "unknown"
	}
	data, ok := params["data"]
	if !ok {
		return nil, fmt.Errorf("INVALID_PARAMS: data is required")
	}
	b, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal data: %w", err)
	}
	result, err := h.wikiEngine.IngestToolJSON(toolName, b)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (h *handlers) wikiSchema(_ context.Context, params map[string]interface{}) (interface{}, error) {
	if h.wikiEngine == nil {
		return nil, fmt.Errorf("wiki engine not initialised")
	}
	format, _ := stringParam(params, "format")
	var schemaFormat wiki.SchemaFormat
	switch format {
	case "claude":
		schemaFormat = wiki.SchemaClaudeCode
	case "cursor":
		schemaFormat = wiki.SchemaCursor
	case "kiro":
		schemaFormat = wiki.SchemaKiro
	case "codex":
		schemaFormat = wiki.SchemaCodex
	default:
		schemaFormat = wiki.SchemaGeneric
	}
	schema := h.wikiEngine.GenerateSchema(schemaFormat)
	return map[string]string{"schema": schema}, nil
}

func (h *handlers) wikiFilter(_ context.Context, params map[string]interface{}) (interface{}, error) {
	if h.wikiEngine == nil {
		return nil, fmt.Errorf("wiki engine not initialised")
	}
	expr, ok := stringParam(params, "filter")
	if !ok {
		return nil, fmt.Errorf("INVALID_PARAMS: filter expression is required")
	}
	filters, err := wiki.ParseFilters(expr)
	if err != nil {
		return nil, err
	}
	pages, err := h.wikiEngine.Store().ListPages("")
	if err != nil {
		return nil, err
	}
	matched, err := wiki.FilterPages(pages, filters)
	if err != nil {
		return nil, err
	}
	return matched, nil
}

func (h *handlers) wikiIngestURL(_ context.Context, params map[string]interface{}) (interface{}, error) {
	if h.wikiEngine == nil {
		return nil, fmt.Errorf("wiki engine not initialised")
	}
	url, ok := stringParam(params, "url")
	if !ok {
		return nil, fmt.Errorf("INVALID_PARAMS: url is required")
	}
	title, _ := stringParam(params, "title")
	result, err := h.wikiEngine.FetchAndIngest(url, title)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// --- Context connection and search handlers ---

func (h *handlers) contextConnect(_ context.Context, params map[string]interface{}) (interface{}, error) {
	fromKey, ok := stringParam(params, "from_key")
	if !ok {
		return nil, fmt.Errorf("INVALID_PARAMS: from_key is required")
	}
	toKey, ok := stringParam(params, "to_key")
	if !ok {
		return nil, fmt.Errorf("INVALID_PARAMS: to_key is required")
	}
	relation, _ := stringParam(params, "relation")
	if relation == "" {
		relation = "related-to"
	}
	sourceTool, _ := stringParam(params, "source_tool")
	if sourceTool == "" {
		sourceTool = "mcp"
	}
	sessionID, _ := stringParam(params, "session_id")

	confidence := 1.0
	if v, ok := params["confidence"]; ok {
		if f, ok := v.(float64); ok {
			confidence = f
		}
	}

	edge, err := h.store.AddEdge(fromKey, toKey, relation, sourceTool, sessionID, confidence)
	if err != nil {
		return nil, err
	}
	return edge, nil
}

func (h *handlers) contextWeb(_ context.Context, params map[string]interface{}) (interface{}, error) {
	key, ok := stringParam(params, "key")
	if !ok {
		return nil, fmt.Errorf("INVALID_PARAMS: key is required")
	}

	edges, err := h.store.GetEdges(key)
	if err != nil {
		return nil, err
	}
	entries, err := h.store.GetRelated(key)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"key":     key,
		"edges":   edges,
		"entries": entries,
	}, nil
}

func (h *handlers) contextSearch(_ context.Context, params map[string]interface{}) (interface{}, error) {
	query, ok := stringParam(params, "query")
	if !ok {
		return nil, fmt.Errorf("INVALID_PARAMS: query is required")
	}

	entries, err := h.store.Search(query)
	if err != nil {
		return nil, err
	}
	return entries, nil
}

func (h *handlers) contextMap(_ context.Context, _ map[string]interface{}) (interface{}, error) {
	entries, err := h.store.List(memory.ListFilter{})
	if err != nil {
		return nil, err
	}
	edges, err := h.store.AllEdges()
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"entries": entries,
		"edges":   edges,
		"stats": map[string]int{
			"entry_count": len(entries),
			"edge_count":  len(edges),
		},
	}, nil
}
