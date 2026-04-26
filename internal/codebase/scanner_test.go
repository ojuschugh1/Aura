package codebase

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScan_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	result, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if result.FileCount != 0 {
		t.Errorf("FileCount = %d, want 0", result.FileCount)
	}
	if result.TotalLines != 0 {
		t.Errorf("TotalLines = %d, want 0", result.TotalLines)
	}
}

func TestScan_DetectsLanguages(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), "package main\nfunc main() {}\n")
	writeFile(t, filepath.Join(dir, "app.js"), "console.log('hello');\n")

	result, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(result.Languages) != 2 {
		t.Errorf("expected 2 languages, got %d: %v", len(result.Languages), result.Languages)
	}
}

func TestScan_DetectsEntryPoints(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n")
	writeFile(t, filepath.Join(dir, "lib.go"), "package lib\n")

	result, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(result.EntryPoints) != 1 {
		t.Errorf("expected 1 entry point, got %d: %v", len(result.EntryPoints), result.EntryPoints)
	}
	if result.EntryPoints[0] != "main.go" {
		t.Errorf("EntryPoints[0] = %q, want %q", result.EntryPoints[0], "main.go")
	}
}

func TestScan_CountsFilesAndLines(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.go"), "line1\nline2\nline3\n")
	writeFile(t, filepath.Join(dir, "b.go"), "line1\nline2\n")

	result, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if result.FileCount != 2 {
		t.Errorf("FileCount = %d, want 2", result.FileCount)
	}
	if result.TotalLines != 5 {
		t.Errorf("TotalLines = %d, want 5", result.TotalLines)
	}
}

func TestScan_SkipsHiddenDirs(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git"), 0755)
	writeFile(t, filepath.Join(dir, ".git", "config"), "git config\n")
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n")

	result, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if result.FileCount != 1 {
		t.Errorf("FileCount = %d, want 1 (should skip .git)", result.FileCount)
	}
}

func TestScan_SkipsNodeModules(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "node_modules", "pkg"), 0755)
	writeFile(t, filepath.Join(dir, "node_modules", "pkg", "index.js"), "module.exports = {}\n")
	writeFile(t, filepath.Join(dir, "app.js"), "require('pkg');\n")

	result, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if result.FileCount != 1 {
		t.Errorf("FileCount = %d, want 1 (should skip node_modules)", result.FileCount)
	}
}

func TestScan_DetectsTopLevelPackages(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "internal", "memory"), 0755)
	os.MkdirAll(filepath.Join(dir, "pkg", "types"), 0755)
	writeFile(t, filepath.Join(dir, "internal", "memory", "store.go"), "package memory\n")
	writeFile(t, filepath.Join(dir, "pkg", "types", "types.go"), "package types\n")

	result, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(result.Packages) < 2 {
		t.Errorf("expected at least 2 packages, got %d: %v", len(result.Packages), result.Packages)
	}
}

func TestScan_ReadsDependenciesFromGoMod(t *testing.T) {
	dir := t.TempDir()
	gomod := `module example.com/test

go 1.21

require (
	github.com/spf13/cobra v1.8.1
	github.com/spf13/viper v1.21.0
)
`
	writeFile(t, filepath.Join(dir, "go.mod"), gomod)
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n")

	result, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(result.Dependencies) != 2 {
		t.Errorf("expected 2 dependencies, got %d: %v", len(result.Dependencies), result.Dependencies)
	}
}

func TestScan_ReadsDependenciesFromPackageJSON(t *testing.T) {
	dir := t.TempDir()
	pkgjson := `{
  "dependencies": {
    "express": "^4.18.0",
    "lodash": "^4.17.0"
  },
  "devDependencies": {
    "jest": "^29.0.0"
  }
}`
	writeFile(t, filepath.Join(dir, "package.json"), pkgjson)
	writeFile(t, filepath.Join(dir, "index.js"), "const express = require('express');\n")

	result, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(result.Dependencies) != 3 {
		t.Errorf("expected 3 dependencies, got %d: %v", len(result.Dependencies), result.Dependencies)
	}
}

func TestScan_ReadsDependenciesFromRequirementsTxt(t *testing.T) {
	dir := t.TempDir()
	reqs := "flask==2.3.0\nrequests>=2.28.0\n# comment\n"
	writeFile(t, filepath.Join(dir, "requirements.txt"), reqs)
	writeFile(t, filepath.Join(dir, "app.py"), "from flask import Flask\n")

	result, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(result.Dependencies) != 2 {
		t.Errorf("expected 2 dependencies, got %d: %v", len(result.Dependencies), result.Dependencies)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("MkdirAll %s: %v", dir, err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}
