package scan

import "fmt"

// Result holds the output of a dependency scan.
type Result struct {
	Phantoms []PhantomDep
	HighRisk []PhantomDep // confidence > 0.8
}

// Scan runs ghostdep on projectRoot and returns the results.
// If ghostdep is not available, returns a descriptive error.
func Scan(projectRoot string) (*Result, error) {
	raw, err := runGhostdep(projectRoot, "json")
	if err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}

	deps, err := parseGhostdepJSON(raw)
	if err != nil {
		return nil, err
	}

	res := &Result{Phantoms: deps}
	for _, d := range deps {
		if d.Confidence > 0.8 {
			res.HighRisk = append(res.HighRisk, d)
		}
	}
	return res, nil
}
