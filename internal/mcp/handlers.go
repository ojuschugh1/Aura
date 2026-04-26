package mcp

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ojuschugh1/aura/internal/compress"
	"github.com/ojuschugh1/aura/internal/cost"
	"github.com/ojuschugh1/aura/internal/escrow"
	"github.com/ojuschugh1/aura/internal/memory"
	"github.com/ojuschugh1/aura/internal/policy"
	"github.com/ojuschugh1/aura/internal/router"
	"github.com/ojuschugh1/aura/internal/scan"
	"github.com/ojuschugh1/aura/internal/trace"
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
