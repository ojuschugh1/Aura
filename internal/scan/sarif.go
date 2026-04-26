package scan

import "encoding/json"

// sarifOutput is the top-level SARIF 2.1.0 document.
type sarifOutput struct {
	Version string      `json:"version"`
	Schema  string      `json:"$schema"`
	Runs    []sarifRun  `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool    `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type sarifResult struct {
	RuleID  string          `json:"ruleId"`
	Message sarifMessage    `json:"message"`
	Level   string          `json:"level"`
	Locations []sarifLocation `json:"locations"`
}

type sarifMessage struct {
	Text string `json:"text"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
	Region           sarifRegion           `json:"region"`
}

type sarifArtifactLocation struct {
	URI string `json:"uri"`
}

type sarifRegion struct {
	StartLine int `json:"startLine"`
}

// ToSARIF converts phantom deps to a SARIF JSON string.
func ToSARIF(deps []PhantomDep) ([]byte, error) {
	results := make([]sarifResult, 0, len(deps))
	for _, d := range deps {
		level := "warning"
		if d.Confidence > 0.8 {
			level = "error"
		}
		results = append(results, sarifResult{
			RuleID:  "phantom-dependency",
			Message: sarifMessage{Text: "Phantom dependency: " + d.Import},
			Level:   level,
			Locations: []sarifLocation{{
				PhysicalLocation: sarifPhysicalLocation{
					ArtifactLocation: sarifArtifactLocation{URI: d.File},
					Region:           sarifRegion{StartLine: d.Line},
				},
			}},
		})
	}

	out := sarifOutput{
		Version: "2.1.0",
		Schema:  "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/master/Schemata/sarif-schema-2.1.0.json",
		Runs: []sarifRun{{
			Tool:    sarifTool{Driver: sarifDriver{Name: "ghostdep", Version: "1.0.0"}},
			Results: results,
		}},
	}
	return json.MarshalIndent(out, "", "  ")
}
