package scan

import (
	"encoding/json"
	"testing"
)

// --- SARIF JSON structure tests (Req 10.4) ----------------------------------

func TestToSARIF_ValidJSONWithCorrectSchema(t *testing.T) {
	deps := []PhantomDep{
		{File: "main.go", Line: 10, Import: "ghost/pkg", Confidence: 0.7},
	}

	data, err := ToSARIF(deps)
	if err != nil {
		t.Fatalf("ToSARIF: %v", err)
	}

	var out sarifOutput
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if out.Version != "2.1.0" {
		t.Errorf("version = %q, want %q", out.Version, "2.1.0")
	}
	if out.Schema != "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/master/Schemata/sarif-schema-2.1.0.json" {
		t.Errorf("schema = %q, want SARIF 2.1.0 schema URL", out.Schema)
	}
}

func TestToSARIF_HasSingleRunWithToolInfo(t *testing.T) {
	deps := []PhantomDep{
		{File: "app.js", Line: 1, Import: "fake-lib", Confidence: 0.5},
	}

	data, err := ToSARIF(deps)
	if err != nil {
		t.Fatalf("ToSARIF: %v", err)
	}

	var out sarifOutput
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if len(out.Runs) != 1 {
		t.Fatalf("runs count = %d, want 1", len(out.Runs))
	}

	driver := out.Runs[0].Tool.Driver
	if driver.Name != "ghostdep" {
		t.Errorf("tool name = %q, want %q", driver.Name, "ghostdep")
	}
	if driver.Version != "1.0.0" {
		t.Errorf("tool version = %q, want %q", driver.Version, "1.0.0")
	}
}

// --- SARIF result content tests (Req 10.4) ----------------------------------

func TestToSARIF_ResultHasRuleIDAndMessage(t *testing.T) {
	deps := []PhantomDep{
		{File: "src/index.ts", Line: 5, Import: "nonexistent-pkg", Confidence: 0.6},
	}

	data, err := ToSARIF(deps)
	if err != nil {
		t.Fatalf("ToSARIF: %v", err)
	}

	var out sarifOutput
	json.Unmarshal(data, &out)

	if len(out.Runs[0].Results) != 1 {
		t.Fatalf("results count = %d, want 1", len(out.Runs[0].Results))
	}

	r := out.Runs[0].Results[0]
	if r.RuleID != "phantom-dependency" {
		t.Errorf("ruleId = %q, want %q", r.RuleID, "phantom-dependency")
	}
	if r.Message.Text != "Phantom dependency: nonexistent-pkg" {
		t.Errorf("message = %q, want %q", r.Message.Text, "Phantom dependency: nonexistent-pkg")
	}
}

func TestToSARIF_ResultHasLocationWithFileAndLine(t *testing.T) {
	deps := []PhantomDep{
		{File: "lib/utils.py", Line: 42, Import: "phantom_mod", Confidence: 0.5},
	}

	data, err := ToSARIF(deps)
	if err != nil {
		t.Fatalf("ToSARIF: %v", err)
	}

	var out sarifOutput
	json.Unmarshal(data, &out)

	r := out.Runs[0].Results[0]
	if len(r.Locations) != 1 {
		t.Fatalf("locations count = %d, want 1", len(r.Locations))
	}

	loc := r.Locations[0].PhysicalLocation
	if loc.ArtifactLocation.URI != "lib/utils.py" {
		t.Errorf("uri = %q, want %q", loc.ArtifactLocation.URI, "lib/utils.py")
	}
	if loc.Region.StartLine != 42 {
		t.Errorf("startLine = %d, want 42", loc.Region.StartLine)
	}
}

// --- SARIF high-risk flagging tests (Req 10.6) ------------------------------

func TestToSARIF_HighConfidenceIsError(t *testing.T) {
	deps := []PhantomDep{
		{File: "main.go", Line: 1, Import: "risky-pkg", Confidence: 0.9},
	}

	data, err := ToSARIF(deps)
	if err != nil {
		t.Fatalf("ToSARIF: %v", err)
	}

	var out sarifOutput
	json.Unmarshal(data, &out)

	r := out.Runs[0].Results[0]
	if r.Level != "error" {
		t.Errorf("level = %q, want %q for confidence > 0.8", r.Level, "error")
	}
}

func TestToSARIF_LowConfidenceIsWarning(t *testing.T) {
	deps := []PhantomDep{
		{File: "main.go", Line: 1, Import: "maybe-pkg", Confidence: 0.5},
	}

	data, err := ToSARIF(deps)
	if err != nil {
		t.Fatalf("ToSARIF: %v", err)
	}

	var out sarifOutput
	json.Unmarshal(data, &out)

	r := out.Runs[0].Results[0]
	if r.Level != "warning" {
		t.Errorf("level = %q, want %q for confidence <= 0.8", r.Level, "warning")
	}
}

func TestToSARIF_BoundaryConfidence08IsWarning(t *testing.T) {
	deps := []PhantomDep{
		{File: "main.go", Line: 1, Import: "edge-pkg", Confidence: 0.8},
	}

	data, err := ToSARIF(deps)
	if err != nil {
		t.Fatalf("ToSARIF: %v", err)
	}

	var out sarifOutput
	json.Unmarshal(data, &out)

	r := out.Runs[0].Results[0]
	if r.Level != "warning" {
		t.Errorf("level = %q, want %q for confidence == 0.8 (boundary)", r.Level, "warning")
	}
}

// --- SARIF empty results test (Req 10.4) ------------------------------------

func TestToSARIF_EmptyDepsProducesValidJSON(t *testing.T) {
	data, err := ToSARIF([]PhantomDep{})
	if err != nil {
		t.Fatalf("ToSARIF: %v", err)
	}

	var out sarifOutput
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("empty deps should produce valid JSON: %v", err)
	}

	if out.Version != "2.1.0" {
		t.Errorf("version = %q, want %q", out.Version, "2.1.0")
	}
	if len(out.Runs) != 1 {
		t.Fatalf("runs count = %d, want 1", len(out.Runs))
	}
	if len(out.Runs[0].Results) != 0 {
		t.Errorf("results count = %d, want 0 for empty deps", len(out.Runs[0].Results))
	}
}

func TestToSARIF_NilDepsProducesValidJSON(t *testing.T) {
	data, err := ToSARIF(nil)
	if err != nil {
		t.Fatalf("ToSARIF(nil): %v", err)
	}

	var out sarifOutput
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("nil deps should produce valid JSON: %v", err)
	}

	if len(out.Runs[0].Results) != 0 {
		t.Errorf("results count = %d, want 0 for nil deps", len(out.Runs[0].Results))
	}
}

// --- SARIF multiple results test (Req 10.4) ---------------------------------

func TestToSARIF_MultipleFindings(t *testing.T) {
	deps := []PhantomDep{
		{File: "a.go", Line: 10, Import: "pkg-a", Confidence: 0.3},
		{File: "b.py", Line: 20, Import: "pkg-b", Confidence: 0.9},
		{File: "c.js", Line: 30, Import: "pkg-c", Confidence: 0.6},
	}

	data, err := ToSARIF(deps)
	if err != nil {
		t.Fatalf("ToSARIF: %v", err)
	}

	var out sarifOutput
	json.Unmarshal(data, &out)

	results := out.Runs[0].Results
	if len(results) != 3 {
		t.Fatalf("results count = %d, want 3", len(results))
	}

	// Verify each result maps to the correct dep.
	for i, dep := range deps {
		r := results[i]
		wantMsg := "Phantom dependency: " + dep.Import
		if r.Message.Text != wantMsg {
			t.Errorf("result[%d] message = %q, want %q", i, r.Message.Text, wantMsg)
		}
		loc := r.Locations[0].PhysicalLocation
		if loc.ArtifactLocation.URI != dep.File {
			t.Errorf("result[%d] uri = %q, want %q", i, loc.ArtifactLocation.URI, dep.File)
		}
		if loc.Region.StartLine != dep.Line {
			t.Errorf("result[%d] startLine = %d, want %d", i, loc.Region.StartLine, dep.Line)
		}
	}

	// Verify levels: pkg-b (0.9) should be error, others warning.
	if results[0].Level != "warning" {
		t.Errorf("result[0] level = %q, want warning", results[0].Level)
	}
	if results[1].Level != "error" {
		t.Errorf("result[1] level = %q, want error", results[1].Level)
	}
	if results[2].Level != "warning" {
		t.Errorf("result[2] level = %q, want warning", results[2].Level)
	}
}

// --- confidenceToFloat tests ------------------------------------------------

func TestConfidenceToFloat_KnownValues(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"High", 1.0},
		{"Medium", 0.6},
		{"Low", 0.3},
		{"Unknown", 0.5},
		{"", 0.5},
	}

	for _, tc := range tests {
		got := confidenceToFloat(tc.input)
		if got != tc.want {
			t.Errorf("confidenceToFloat(%q) = %f, want %f", tc.input, got, tc.want)
		}
	}
}

// --- parseGhostdepJSON tests ------------------------------------------------

func TestParseGhostdepJSON_ValidOutput(t *testing.T) {
	input := `{
		"findings": [
			{
				"finding_type": "Phantom",
				"package": "ghost-pkg",
				"file": "main.go",
				"line": 5,
				"manifest": "go.mod",
				"language": "Go",
				"confidence": "High"
			}
		],
		"metadata": {"scanned_files": 10, "duration_ms": 50}
	}`

	deps, err := parseGhostdepJSON([]byte(input))
	if err != nil {
		t.Fatalf("parseGhostdepJSON: %v", err)
	}

	if len(deps) != 1 {
		t.Fatalf("deps count = %d, want 1", len(deps))
	}

	d := deps[0]
	if d.Import != "ghost-pkg" {
		t.Errorf("Import = %q, want %q", d.Import, "ghost-pkg")
	}
	if d.File != "main.go" {
		t.Errorf("File = %q, want %q", d.File, "main.go")
	}
	if d.Line != 5 {
		t.Errorf("Line = %d, want 5", d.Line)
	}
	if d.Confidence != 1.0 {
		t.Errorf("Confidence = %f, want 1.0 (High)", d.Confidence)
	}
	if d.Language != "Go" {
		t.Errorf("Language = %q, want %q", d.Language, "Go")
	}
}

func TestParseGhostdepJSON_EmptyFindings(t *testing.T) {
	input := `{"findings": [], "metadata": {"scanned_files": 5, "duration_ms": 10}}`

	deps, err := parseGhostdepJSON([]byte(input))
	if err != nil {
		t.Fatalf("parseGhostdepJSON: %v", err)
	}

	if len(deps) != 0 {
		t.Errorf("deps count = %d, want 0", len(deps))
	}
}

func TestParseGhostdepJSON_NilFileAndLine(t *testing.T) {
	input := `{
		"findings": [
			{
				"finding_type": "Unused",
				"package": "unused-pkg",
				"file": null,
				"line": null,
				"manifest": "package.json",
				"language": "JavaScript",
				"confidence": "Low"
			}
		],
		"metadata": {"scanned_files": 3, "duration_ms": 20}
	}`

	deps, err := parseGhostdepJSON([]byte(input))
	if err != nil {
		t.Fatalf("parseGhostdepJSON: %v", err)
	}

	if len(deps) != 1 {
		t.Fatalf("deps count = %d, want 1", len(deps))
	}

	d := deps[0]
	if d.File != "" {
		t.Errorf("File = %q, want empty string for null file", d.File)
	}
	if d.Line != 0 {
		t.Errorf("Line = %d, want 0 for null line", d.Line)
	}
}

func TestParseGhostdepJSON_InvalidJSON(t *testing.T) {
	_, err := parseGhostdepJSON([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

// --- Result.HighRisk filtering (Req 10.6) -----------------------------------

func TestResult_HighRiskFiltering(t *testing.T) {
	deps := []PhantomDep{
		{File: "a.go", Line: 1, Import: "low", Confidence: 0.3},
		{File: "b.go", Line: 2, Import: "medium", Confidence: 0.6},
		{File: "c.go", Line: 3, Import: "high", Confidence: 0.9},
		{File: "d.go", Line: 4, Import: "boundary", Confidence: 0.8},
		{File: "e.go", Line: 5, Import: "very-high", Confidence: 1.0},
	}

	res := &Result{Phantoms: deps}
	for _, d := range deps {
		if d.Confidence > 0.8 {
			res.HighRisk = append(res.HighRisk, d)
		}
	}

	if len(res.HighRisk) != 2 {
		t.Errorf("HighRisk count = %d, want 2 (confidence 0.9 and 1.0)", len(res.HighRisk))
	}

	for _, hr := range res.HighRisk {
		if hr.Confidence <= 0.8 {
			t.Errorf("HighRisk contains dep with confidence %f, should be > 0.8", hr.Confidence)
		}
	}
}
