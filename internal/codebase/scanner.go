// Package codebase scans project directories and stores structure as memory entries.
package codebase

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ScanResult holds the extracted project structure.
type ScanResult struct {
	Languages    []string
	EntryPoints  []string
	Packages     []string
	Dependencies []string
	FileCount    int
	TotalLines   int
}

// skipDirs are directories to skip during scanning.
var skipDirs = map[string]bool{
	".git": true, ".aura": true, "node_modules": true,
	"vendor": true, "__pycache__": true, ".venv": true,
	"dist": true, "build": true, ".idea": true, ".vscode": true,
}

// entryPointNames are filenames that indicate project entry points.
var entryPointNames = map[string]bool{
	"main.go": true, "index.js": true, "index.ts": true,
	"app.py": true, "main.py": true, "main.rs": true,
	"Main.java": true, "index.tsx": true, "index.jsx": true,
}

// langExtensions maps file extensions to language names.
var langExtensions = map[string]string{
	".go":   "Go",
	".js":   "JavaScript",
	".ts":   "TypeScript",
	".py":   "Python",
	".rs":   "Rust",
	".java": "Java",
	".rb":   "Ruby",
	".c":    "C",
	".cpp":  "C++",
	".h":    "C",
	".cs":   "C#",
	".php":  "PHP",
	".sh":   "Shell",
	".tsx":  "TypeScript",
	".jsx":  "JavaScript",
}

// Scan walks the project directory and extracts structure deterministically.
func Scan(projectDir string) (*ScanResult, error) {
	result := &ScanResult{}
	langCount := make(map[string]int)
	topDirs := make(map[string]bool)

	err := filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}

		rel, _ := filepath.Rel(projectDir, path)
		if rel == "." {
			return nil
		}

		// Skip hidden and known non-source directories.
		if info.IsDir() {
			base := filepath.Base(path)
			if skipDirs[base] || strings.HasPrefix(base, ".") {
				return filepath.SkipDir
			}
			// Track top-level directories as packages.
			parts := strings.Split(rel, string(os.PathSeparator))
			if len(parts) >= 1 {
				topDirs[parts[0]] = true
			}
			return nil
		}

		result.FileCount++

		// Count lines.
		lines, _ := countLines(path)
		result.TotalLines += lines

		// Detect language by extension.
		ext := strings.ToLower(filepath.Ext(path))
		if lang, ok := langExtensions[ext]; ok {
			langCount[lang]++
		}

		// Detect entry points.
		base := filepath.Base(path)
		if entryPointNames[base] {
			result.EntryPoints = append(result.EntryPoints, rel)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Sort languages by file count (descending).
	type langEntry struct {
		name  string
		count int
	}
	var langs []langEntry
	for name, count := range langCount {
		langs = append(langs, langEntry{name, count})
	}
	sort.Slice(langs, func(i, j int) bool { return langs[i].count > langs[j].count })
	for _, l := range langs {
		result.Languages = append(result.Languages, l.name)
	}

	// Sort packages.
	for d := range topDirs {
		result.Packages = append(result.Packages, d)
	}
	sort.Strings(result.Packages)
	sort.Strings(result.EntryPoints)

	// Read dependency manifests.
	result.Dependencies = readDependencies(projectDir)

	return result, nil
}

// countLines counts the number of newlines in a file.
func countLines(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		count++
	}
	return count, scanner.Err()
}

// readDependencies reads dependency names from known manifest files.
func readDependencies(projectDir string) []string {
	var deps []string
	seen := make(map[string]bool)

	addDep := func(name string) {
		name = strings.TrimSpace(name)
		if name != "" && !seen[name] {
			seen[name] = true
			deps = append(deps, name)
		}
	}

	// go.mod
	if data, err := os.ReadFile(filepath.Join(projectDir, "go.mod")); err == nil {
		inRequire := false
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "require (" {
				inRequire = true
				continue
			}
			if line == ")" {
				inRequire = false
				continue
			}
			if inRequire && !strings.HasPrefix(line, "//") {
				parts := strings.Fields(line)
				if len(parts) >= 1 {
					addDep(parts[0])
				}
			}
		}
	}

	// package.json
	if data, err := os.ReadFile(filepath.Join(projectDir, "package.json")); err == nil {
		var pkg map[string]interface{}
		if err := parseJSON(data, &pkg); err == nil {
			for _, section := range []string{"dependencies", "devDependencies"} {
				if depsMap, ok := pkg[section].(map[string]interface{}); ok {
					for name := range depsMap {
						addDep(name)
					}
				}
			}
		}
	}

	// requirements.txt
	if data, err := os.ReadFile(filepath.Join(projectDir, "requirements.txt")); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			// Strip version specifiers.
			for _, sep := range []string{"==", ">=", "<=", "~=", "!="} {
				if idx := strings.Index(line, sep); idx > 0 {
					line = line[:idx]
				}
			}
			addDep(line)
		}
	}

	// Cargo.toml
	if data, err := os.ReadFile(filepath.Join(projectDir, "Cargo.toml")); err == nil {
		inDeps := false
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "[dependencies]" || line == "[dev-dependencies]" {
				inDeps = true
				continue
			}
			if strings.HasPrefix(line, "[") {
				inDeps = false
				continue
			}
			if inDeps {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) >= 1 {
					addDep(parts[0])
				}
			}
		}
	}

	sort.Strings(deps)
	return deps
}

// parseJSON is a minimal JSON parser for dependency reading.
func parseJSON(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
