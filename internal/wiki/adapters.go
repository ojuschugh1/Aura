package wiki

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ToolIngestResult summarises what happened when a tool's output was ingested.
type ToolIngestResult struct {
	Tool         string   `json:"tool"`
	PagesCreated []string `json:"pages_created"`
	PagesUpdated []string `json:"pages_updated"`
	SourceID     int64    `json:"source_id"`
	Summary      string   `json:"summary"`
}

// --- SQZ Adapter ---

// SQZReport is the structured output from a compression run.
type SQZReport struct {
	SessionID      string  `json:"session_id"`
	OriginalTokens int     `json:"original_tokens"`
	CompressedTokens int   `json:"compressed_tokens"`
	ReductionPct   float64 `json:"reduction_pct"`
	Deduplicated   bool    `json:"deduplicated"`
	Timestamp      time.Time `json:"timestamp"`
}

// IngestSQZ processes a compression report and updates the wiki with
// cumulative compression statistics and session-level detail.
func (e *Engine) IngestSQZ(report SQZReport) (*ToolIngestResult, error) {
	result := &ToolIngestResult{Tool: "sqz"}

	if report.Timestamp.IsZero() {
		report.Timestamp = time.Now()
	}

	// Build the source content.
	content := fmt.Sprintf(`## Compression Report

- **Session:** %s
- **Original tokens:** %d
- **Compressed tokens:** %d
- **Reduction:** %.1f%%
- **Deduplicated:** %v
- **Timestamp:** %s`,
		report.SessionID,
		report.OriginalTokens,
		report.CompressedTokens,
		report.ReductionPct,
		report.Deduplicated,
		report.Timestamp.Format(time.RFC3339),
	)

	// Ingest as a source.
	title := fmt.Sprintf("SQZ compression — %s", report.Timestamp.Format("2006-01-02 15:04"))
	src, isDup, err := e.store.IngestSource(title, content, "tool-output", "sqz")
	if err != nil {
		return nil, fmt.Errorf("ingest sqz source: %w", err)
	}
	result.SourceID = src.ID
	if isDup {
		result.Summary = "duplicate report, skipped"
		return result, nil
	}

	// Update or create the cumulative SQZ stats page.
	slug := "tool-sqz-compression"
	existing, _ := e.store.GetPage(slug)
	if existing != nil {
		entry := fmt.Sprintf("\n\n### %s\n\n%s",
			report.Timestamp.Format("2006-01-02 15:04"), content)
		newContent := existing.Content + entry
		newSourceIDs := appendUnique(existing.SourceIDs, src.ID)
		_, err := e.store.UpdatePage(slug, newContent, existing.Tags, newSourceIDs, existing.LinksSlugs)
		if err != nil {
			return nil, err
		}
		result.PagesUpdated = append(result.PagesUpdated, slug)
	} else {
		pageContent := fmt.Sprintf("# SQZ Compression History\n\n*Cumulative token compression statistics from sqz.*\n\n### %s\n\n%s",
			report.Timestamp.Format("2006-01-02 15:04"), content)
		_, err := e.store.CreatePage(slug, "SQZ Compression History", pageContent, "tool",
			[]string{"sqz", "compression", "auto-tool"}, []int64{src.ID}, nil)
		if err != nil {
			return nil, err
		}
		result.PagesCreated = append(result.PagesCreated, slug)
	}

	saved := report.OriginalTokens - report.CompressedTokens
	result.Summary = fmt.Sprintf("%.1f%% reduction, %d tokens saved", report.ReductionPct, saved)

	allSlugs := append(result.PagesCreated, result.PagesUpdated...)
	_ = e.store.AppendLog("tool-ingest",
		fmt.Sprintf("SQZ: %s", result.Summary), allSlugs, &src.ID)

	return result, nil
}

// --- GhostDep Adapter ---

// GhostDepReport is the structured output from a dependency scan.
type GhostDepReport struct {
	ProjectRoot  string            `json:"project_root"`
	Findings     []GhostDepFinding `json:"findings"`
	ScannedFiles int               `json:"scanned_files"`
	DurationMs   int               `json:"duration_ms"`
	Timestamp    time.Time         `json:"timestamp"`
}

// GhostDepFinding is a single phantom or unused dependency.
type GhostDepFinding struct {
	Type       string  `json:"type"`       // "phantom" or "unused"
	Package    string  `json:"package"`
	File       string  `json:"file"`
	Line       int     `json:"line"`
	Language   string  `json:"language"`
	Confidence float64 `json:"confidence"`
}

// IngestGhostDep processes a dependency scan report and creates wiki pages
// for each finding, plus a cumulative scan history page.
func (e *Engine) IngestGhostDep(report GhostDepReport) (*ToolIngestResult, error) {
	result := &ToolIngestResult{Tool: "ghostdep"}

	if report.Timestamp.IsZero() {
		report.Timestamp = time.Now()
	}

	// Build source content.
	var b strings.Builder
	b.WriteString(fmt.Sprintf("## Dependency Scan — %s\n\n", report.Timestamp.Format("2006-01-02 15:04")))
	b.WriteString(fmt.Sprintf("- **Project:** %s\n", report.ProjectRoot))
	b.WriteString(fmt.Sprintf("- **Files scanned:** %d\n", report.ScannedFiles))
	b.WriteString(fmt.Sprintf("- **Duration:** %dms\n", report.DurationMs))
	b.WriteString(fmt.Sprintf("- **Findings:** %d\n\n", len(report.Findings)))

	highRisk := 0
	for _, f := range report.Findings {
		risk := ""
		if f.Confidence > 0.8 {
			risk = " **[HIGH RISK]**"
			highRisk++
		}
		b.WriteString(fmt.Sprintf("- [%s] `%s` at %s:%d (%.0f%% confidence)%s\n",
			f.Type, f.Package, f.File, f.Line, f.Confidence*100, risk))
	}

	content := b.String()
	title := fmt.Sprintf("GhostDep scan — %s", report.Timestamp.Format("2006-01-02 15:04"))
	src, isDup, err := e.store.IngestSource(title, content, "tool-output", "ghostdep")
	if err != nil {
		return nil, fmt.Errorf("ingest ghostdep source: %w", err)
	}
	result.SourceID = src.ID
	if isDup {
		result.Summary = "duplicate scan, skipped"
		return result, nil
	}

	// Update or create the cumulative scan history page.
	slug := "tool-ghostdep-scans"
	existing, _ := e.store.GetPage(slug)
	if existing != nil {
		entry := fmt.Sprintf("\n\n### %s\n\n%s", report.Timestamp.Format("2006-01-02 15:04"), content)
		newContent := existing.Content + entry
		newSourceIDs := appendUnique(existing.SourceIDs, src.ID)
		_, err := e.store.UpdatePage(slug, newContent, existing.Tags, newSourceIDs, existing.LinksSlugs)
		if err != nil {
			return nil, err
		}
		result.PagesUpdated = append(result.PagesUpdated, slug)
	} else {
		pageContent := fmt.Sprintf("# GhostDep Scan History\n\n*Cumulative dependency scan results from ghostdep.*\n\n### %s\n\n%s",
			report.Timestamp.Format("2006-01-02 15:04"), content)
		_, err := e.store.CreatePage(slug, "GhostDep Scan History", pageContent, "tool",
			[]string{"ghostdep", "dependencies", "auto-tool"}, []int64{src.ID}, nil)
		if err != nil {
			return nil, err
		}
		result.PagesCreated = append(result.PagesCreated, slug)
	}

	// Create/update per-package entity pages for high-risk findings.
	for _, f := range report.Findings {
		if f.Confidence <= 0.8 {
			continue
		}
		pkgSlug := slugify("dep-" + f.Package)
		existingPkg, _ := e.store.GetPage(pkgSlug)
		if existingPkg != nil {
			ref := fmt.Sprintf("\n\n### %s scan\n\n- [%s] at %s:%d (%.0f%%)",
				report.Timestamp.Format("2006-01-02"), f.Type, f.File, f.Line, f.Confidence*100)
			newContent := existingPkg.Content + ref
			newSourceIDs := appendUnique(existingPkg.SourceIDs, src.ID)
			newLinks := appendUniqueStr(existingPkg.LinksSlugs, slug)
			_, err := e.store.UpdatePage(pkgSlug, newContent, existingPkg.Tags, newSourceIDs, newLinks)
			if err == nil {
				result.PagesUpdated = append(result.PagesUpdated, pkgSlug)
			}
		} else {
			pkgContent := fmt.Sprintf("# Dependency: %s\n\n**Type:** %s\n**Language:** %s\n**First seen:** %s\n\n- [%s] at %s:%d (%.0f%%)\n\n## Scan History\n\n- [[%s]]",
				f.Package, f.Type, f.Language, report.Timestamp.Format("2006-01-02"),
				f.Type, f.File, f.Line, f.Confidence*100, slug)
			_, err := e.store.CreatePage(pkgSlug, "Dep: "+f.Package, pkgContent, "entity",
				[]string{"dependency", f.Type, "auto-tool"}, []int64{src.ID}, []string{slug})
			if err == nil {
				result.PagesCreated = append(result.PagesCreated, pkgSlug)
			}
		}
	}

	result.Summary = fmt.Sprintf("%d findings (%d high-risk), %d files scanned",
		len(report.Findings), highRisk, report.ScannedFiles)

	allSlugs := append(result.PagesCreated, result.PagesUpdated...)
	_ = e.store.AppendLog("tool-ingest",
		fmt.Sprintf("GhostDep: %s", result.Summary), allSlugs, &src.ID)

	return result, nil
}

// --- ClaimCheck Adapter ---

// ClaimCheckReport is the structured output from a verification run.
type ClaimCheckReport struct {
	SessionID   string             `json:"session_id"`
	TotalClaims int                `json:"total_claims"`
	PassCount   int                `json:"pass_count"`
	FailCount   int                `json:"fail_count"`
	TruthPct    float64            `json:"truth_pct"`
	Claims      []ClaimCheckClaim  `json:"claims"`
	Timestamp   time.Time          `json:"timestamp"`
}

// ClaimCheckClaim is a single verified claim.
type ClaimCheckClaim struct {
	Type   string `json:"type"`
	Target string `json:"target"`
	Pass   bool   `json:"pass"`
	Detail string `json:"detail"`
}

// IngestClaimCheck processes a verification report and creates wiki pages
// tracking agent truthfulness over time.
func (e *Engine) IngestClaimCheck(report ClaimCheckReport) (*ToolIngestResult, error) {
	result := &ToolIngestResult{Tool: "claimcheck"}

	if report.Timestamp.IsZero() {
		report.Timestamp = time.Now()
	}

	// Build source content.
	var b strings.Builder
	b.WriteString(fmt.Sprintf("## Verification Report — %s\n\n", report.Timestamp.Format("2006-01-02 15:04")))
	b.WriteString(fmt.Sprintf("- **Session:** %s\n", report.SessionID))
	b.WriteString(fmt.Sprintf("- **Claims:** %d total, %d pass, %d fail\n",
		report.TotalClaims, report.PassCount, report.FailCount))
	b.WriteString(fmt.Sprintf("- **Truth score:** %.1f%%\n\n", report.TruthPct))

	for _, c := range report.Claims {
		status := "PASS"
		if !c.Pass {
			status = "FAIL"
		}
		b.WriteString(fmt.Sprintf("- [%s] %s `%s` — %s\n", status, c.Type, c.Target, c.Detail))
	}

	content := b.String()
	title := fmt.Sprintf("ClaimCheck — %s", report.Timestamp.Format("2006-01-02 15:04"))
	src, isDup, err := e.store.IngestSource(title, content, "tool-output", "claimcheck")
	if err != nil {
		return nil, fmt.Errorf("ingest claimcheck source: %w", err)
	}
	result.SourceID = src.ID
	if isDup {
		result.Summary = "duplicate report, skipped"
		return result, nil
	}

	// Update or create the cumulative verification history page.
	slug := "tool-claimcheck-history"
	existing, _ := e.store.GetPage(slug)
	if existing != nil {
		entry := fmt.Sprintf("\n\n### %s — %.1f%% truth\n\n%s",
			report.Timestamp.Format("2006-01-02 15:04"), report.TruthPct, content)
		newContent := existing.Content + entry
		newSourceIDs := appendUnique(existing.SourceIDs, src.ID)
		_, err := e.store.UpdatePage(slug, newContent, existing.Tags, newSourceIDs, existing.LinksSlugs)
		if err != nil {
			return nil, err
		}
		result.PagesUpdated = append(result.PagesUpdated, slug)
	} else {
		pageContent := fmt.Sprintf("# ClaimCheck Verification History\n\n*Cumulative agent claim verification results.*\n\n### %s — %.1f%% truth\n\n%s",
			report.Timestamp.Format("2006-01-02 15:04"), report.TruthPct, content)
		_, err := e.store.CreatePage(slug, "ClaimCheck Verification History", pageContent, "tool",
			[]string{"claimcheck", "verification", "auto-tool"}, []int64{src.ID}, nil)
		if err != nil {
			return nil, err
		}
		result.PagesCreated = append(result.PagesCreated, slug)
	}

	result.Summary = fmt.Sprintf("%.1f%% truth (%d/%d claims passed)",
		report.TruthPct, report.PassCount, report.TotalClaims)

	allSlugs := append(result.PagesCreated, result.PagesUpdated...)
	_ = e.store.AppendLog("tool-ingest",
		fmt.Sprintf("ClaimCheck: %s", result.Summary), allSlugs, &src.ID)

	return result, nil
}

// --- Etch Adapter ---

// EtchReport is the structured output from an API change detection run.
type EtchReport struct {
	ServiceName string        `json:"service_name"`
	Changes     []EtchChange  `json:"changes"`
	TrafficSpan string        `json:"traffic_span"` // e.g. "2026-04-20 to 2026-04-26"
	Timestamp   time.Time     `json:"timestamp"`
}

// EtchChange is a single detected API change.
type EtchChange struct {
	Endpoint    string `json:"endpoint"`    // e.g. "POST /api/users"
	ChangeType  string `json:"change_type"` // "added", "removed", "modified", "breaking"
	Description string `json:"description"`
	Breaking    bool   `json:"breaking"`
}

// IngestEtch processes an API change detection report and creates wiki pages
// tracking API evolution over time.
func (e *Engine) IngestEtch(report EtchReport) (*ToolIngestResult, error) {
	result := &ToolIngestResult{Tool: "etch"}

	if report.Timestamp.IsZero() {
		report.Timestamp = time.Now()
	}

	// Build source content.
	var b strings.Builder
	b.WriteString(fmt.Sprintf("## API Changes — %s\n\n", report.Timestamp.Format("2006-01-02 15:04")))
	b.WriteString(fmt.Sprintf("- **Service:** %s\n", report.ServiceName))
	b.WriteString(fmt.Sprintf("- **Traffic span:** %s\n", report.TrafficSpan))
	b.WriteString(fmt.Sprintf("- **Changes detected:** %d\n\n", len(report.Changes)))

	breaking := 0
	for _, c := range report.Changes {
		flag := ""
		if c.Breaking {
			flag = " **⚠ BREAKING**"
			breaking++
		}
		b.WriteString(fmt.Sprintf("- [%s] `%s` — %s%s\n", c.ChangeType, c.Endpoint, c.Description, flag))
	}

	content := b.String()
	title := fmt.Sprintf("Etch — %s — %s", report.ServiceName, report.Timestamp.Format("2006-01-02"))
	src, isDup, err := e.store.IngestSource(title, content, "tool-output", "etch")
	if err != nil {
		return nil, fmt.Errorf("ingest etch source: %w", err)
	}
	result.SourceID = src.ID
	if isDup {
		result.Summary = "duplicate report, skipped"
		return result, nil
	}

	// Update or create the service-level API history page.
	serviceSlug := slugify("api-" + report.ServiceName)
	existing, _ := e.store.GetPage(serviceSlug)
	if existing != nil {
		entry := fmt.Sprintf("\n\n### %s\n\n%s", report.Timestamp.Format("2006-01-02 15:04"), content)
		newContent := existing.Content + entry
		newSourceIDs := appendUnique(existing.SourceIDs, src.ID)
		_, err := e.store.UpdatePage(serviceSlug, newContent, existing.Tags, newSourceIDs, existing.LinksSlugs)
		if err != nil {
			return nil, err
		}
		result.PagesUpdated = append(result.PagesUpdated, serviceSlug)
	} else {
		pageContent := fmt.Sprintf("# API History: %s\n\n*Cumulative API change detection from Etch.*\n\n### %s\n\n%s",
			report.ServiceName, report.Timestamp.Format("2006-01-02 15:04"), content)
		_, err := e.store.CreatePage(serviceSlug, "API: "+report.ServiceName, pageContent, "tool",
			[]string{"etch", "api", report.ServiceName, "auto-tool"}, []int64{src.ID}, nil)
		if err != nil {
			return nil, err
		}
		result.PagesCreated = append(result.PagesCreated, serviceSlug)
	}

	// Create/update per-endpoint pages for breaking changes.
	for _, c := range report.Changes {
		if !c.Breaking {
			continue
		}
		epSlug := slugify("endpoint-" + c.Endpoint)
		existingEp, _ := e.store.GetPage(epSlug)
		if existingEp != nil {
			ref := fmt.Sprintf("\n\n### %s — %s\n\n%s",
				report.Timestamp.Format("2006-01-02"), c.ChangeType, c.Description)
			newContent := existingEp.Content + ref
			newSourceIDs := appendUnique(existingEp.SourceIDs, src.ID)
			newLinks := appendUniqueStr(existingEp.LinksSlugs, serviceSlug)
			_, err := e.store.UpdatePage(epSlug, newContent, existingEp.Tags, newSourceIDs, newLinks)
			if err == nil {
				result.PagesUpdated = append(result.PagesUpdated, epSlug)
			}
		} else {
			epContent := fmt.Sprintf("# Endpoint: %s\n\n**Service:** [[%s]]\n**First breaking change:** %s\n\n## Change History\n\n### %s — %s\n\n%s",
				c.Endpoint, serviceSlug, report.Timestamp.Format("2006-01-02"),
				report.Timestamp.Format("2006-01-02"), c.ChangeType, c.Description)
			_, err := e.store.CreatePage(epSlug, "Endpoint: "+c.Endpoint, epContent, "entity",
				[]string{"endpoint", "breaking", "auto-tool"}, []int64{src.ID}, []string{serviceSlug})
			if err == nil {
				result.PagesCreated = append(result.PagesCreated, epSlug)
			}
		}
	}

	result.Summary = fmt.Sprintf("%d changes (%d breaking) for %s",
		len(report.Changes), breaking, report.ServiceName)

	allSlugs := append(result.PagesCreated, result.PagesUpdated...)
	_ = e.store.AppendLog("tool-ingest",
		fmt.Sprintf("Etch: %s", result.Summary), allSlugs, &src.ID)

	return result, nil
}

// --- Generic JSON Adapter ---

// IngestToolJSON parses a generic JSON report from any tool and ingests it.
// This is the fallback for tools that don't have a dedicated adapter.
func (e *Engine) IngestToolJSON(toolName string, jsonData []byte) (*ToolIngestResult, error) {
	result := &ToolIngestResult{Tool: toolName}

	// Try to pretty-print the JSON.
	var parsed interface{}
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		// Not valid JSON — ingest as plain text.
		ir, err := e.Ingest(
			fmt.Sprintf("%s output — %s", toolName, time.Now().Format("2006-01-02 15:04")),
			string(jsonData), "text", toolName)
		if err != nil {
			return nil, err
		}
		result.SourceID = ir.SourceID
		result.PagesCreated = ir.PagesCreated
		result.PagesUpdated = ir.PagesUpdated
		result.Summary = "ingested as plain text"
		return result, nil
	}

	pretty, _ := json.MarshalIndent(parsed, "", "  ")
	content := fmt.Sprintf("## %s Output — %s\n\n```json\n%s\n```",
		toolName, time.Now().Format("2006-01-02 15:04"), string(pretty))

	title := fmt.Sprintf("%s — %s", toolName, time.Now().Format("2006-01-02 15:04"))
	src, isDup, err := e.store.IngestSource(title, content, "tool-output", toolName)
	if err != nil {
		return nil, err
	}
	result.SourceID = src.ID
	if isDup {
		result.Summary = "duplicate output, skipped"
		return result, nil
	}

	// Create/update the tool's history page.
	slug := slugify("tool-" + toolName)
	existing, _ := e.store.GetPage(slug)
	if existing != nil {
		entry := fmt.Sprintf("\n\n### %s\n\n%s", time.Now().Format("2006-01-02 15:04"), content)
		newContent := existing.Content + entry
		newSourceIDs := appendUnique(existing.SourceIDs, src.ID)
		_, err := e.store.UpdatePage(slug, newContent, existing.Tags, newSourceIDs, existing.LinksSlugs)
		if err != nil {
			return nil, err
		}
		result.PagesUpdated = append(result.PagesUpdated, slug)
	} else {
		pageContent := fmt.Sprintf("# %s History\n\n*Cumulative output from %s.*\n\n### %s\n\n%s",
			toolName, toolName, time.Now().Format("2006-01-02 15:04"), content)
		_, err := e.store.CreatePage(slug, toolName+" History", pageContent, "tool",
			[]string{toolName, "auto-tool"}, []int64{src.ID}, nil)
		if err != nil {
			return nil, err
		}
		result.PagesCreated = append(result.PagesCreated, slug)
	}

	result.Summary = fmt.Sprintf("ingested %d bytes of JSON", len(jsonData))

	allSlugs := append(result.PagesCreated, result.PagesUpdated...)
	_ = e.store.AppendLog("tool-ingest",
		fmt.Sprintf("%s: %s", toolName, result.Summary), allSlugs, &src.ID)

	return result, nil
}
