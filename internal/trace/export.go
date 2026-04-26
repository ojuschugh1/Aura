package trace

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
)

// Export writes a trace to outPath in the given format ("json" or "html").
func Export(tracesDir, sessionID, outPath, format string) error {
	srcPath := filepath.Join(tracesDir, sessionID+".jsonl")
	entries, err := loadTrace(srcPath)
	if err != nil {
		return fmt.Errorf("load trace: %w", err)
	}

	switch strings.ToLower(format) {
	case "html":
		return exportHTML(entries, sessionID, outPath)
	default:
		return exportJSON(entries, outPath)
	}
}

func exportJSON(entries interface{}, outPath string) error {
	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create export file: %w", err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(entries)
}

func exportHTML(entries interface{ }, sessionID, outPath string) error {
	b, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	htmlContent := fmt.Sprintf(`<!DOCTYPE html>
<html><head><title>Aura Trace — %s</title></head>
<body><h1>Session: %s</h1><pre>%s</pre></body></html>`,
		html.EscapeString(sessionID),
		html.EscapeString(sessionID),
		html.EscapeString(string(b)),
	)
	return os.WriteFile(outPath, []byte(htmlContent), 0644)
}
