package scan

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ojuschugh1/aura/internal/subprocess"
)

// ghostdepOutput matches ghostdep's actual JSON structure.
type ghostdepOutput struct {
	Findings []ghostdepFinding `json:"findings"`
	Metadata struct {
		ScannedFiles int `json:"scanned_files"`
		DurationMs   int `json:"duration_ms"`
	} `json:"metadata"`
}

type ghostdepFinding struct {
	FindingType string  `json:"finding_type"` // "Phantom" or "Unused"
	Package     string  `json:"package"`
	File        *string `json:"file"`
	Line        *int    `json:"line"`
	Manifest    string  `json:"manifest"`
	Language    string  `json:"language"`
	Confidence  string  `json:"confidence"` // "High", "Medium", "Low"
}

// PhantomDep is a single dependency finding for Aura's output.
type PhantomDep struct {
	File       string  `json:"file"`
	Line       int     `json:"line"`
	Import     string  `json:"import"`
	Type       string  `json:"type"` // "phantom" or "unused"
	Confidence float64 `json:"confidence"`
	Language   string  `json:"language"`
}

// runGhostdep invokes the ghostdep binary and returns parsed results.
func runGhostdep(projectRoot, format string) ([]byte, error) {
	binPath, err := subprocess.ResolveBinary("ghostdep")
	if err != nil {
		return nil, fmt.Errorf("ghostdep not available: %w", err)
	}

	args := []string{"-p", projectRoot, "-f", format}
	runner := &subprocess.Runner{BinaryPath: binPath}
	res, err := runner.Run(context.Background(), args, nil)

	// ghostdep exits 1 when findings exist — that's not an error
	if err != nil && res != nil && res.ExitCode == 1 {
		// Output is in stderr for ghostdep
		if len(res.Stderr) > 0 {
			return res.Stderr, nil
		}
		return res.Stdout, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ghostdep execution: %w", err)
	}

	if len(res.Stdout) > 0 {
		return res.Stdout, nil
	}
	return res.Stderr, nil
}

// confidenceToFloat converts ghostdep's string confidence to a float.
func confidenceToFloat(s string) float64 {
	switch s {
	case "High":
		return 1.0
	case "Medium":
		return 0.6
	case "Low":
		return 0.3
	default:
		return 0.5
	}
}

// parseGhostdepJSON parses ghostdep's JSON output into PhantomDep slice.
func parseGhostdepJSON(data []byte) ([]PhantomDep, error) {
	var output ghostdepOutput
	if err := json.Unmarshal(data, &output); err != nil {
		return nil, fmt.Errorf("parse ghostdep output: %w", err)
	}

	var deps []PhantomDep
	for _, f := range output.Findings {
		d := PhantomDep{
			Import:     f.Package,
			Type:       f.FindingType,
			Confidence: confidenceToFloat(f.Confidence),
			Language:   f.Language,
		}
		if f.File != nil {
			d.File = *f.File
		}
		if f.Line != nil {
			d.Line = *f.Line
		}
		deps = append(deps, d)
	}
	return deps, nil
}
