package codebase

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestDetectChanges_NoChanges(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.go"), "package a\n")

	snapshot, err := SnapshotFiles(dir)
	if err != nil {
		t.Fatalf("SnapshotFiles: %v", err)
	}

	added, modified, deleted, err := DetectChanges(dir, snapshot)
	if err != nil {
		t.Fatalf("DetectChanges: %v", err)
	}
	if len(added) != 0 || len(modified) != 0 || len(deleted) != 0 {
		t.Errorf("expected no changes, got added=%v modified=%v deleted=%v", added, modified, deleted)
	}
}

func TestDetectChanges_DetectsAddedFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.go"), "package a\n")

	snapshot, err := SnapshotFiles(dir)
	if err != nil {
		t.Fatalf("SnapshotFiles: %v", err)
	}

	// Add a new file.
	writeFile(t, filepath.Join(dir, "b.go"), "package b\n")

	added, _, _, err := DetectChanges(dir, snapshot)
	if err != nil {
		t.Fatalf("DetectChanges: %v", err)
	}
	if len(added) != 1 || added[0] != "b.go" {
		t.Errorf("expected added=[b.go], got %v", added)
	}
}

func TestDetectChanges_DetectsModifiedFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.go"), "package a\n")

	snapshot, err := SnapshotFiles(dir)
	if err != nil {
		t.Fatalf("SnapshotFiles: %v", err)
	}

	// Modify the file.
	writeFile(t, filepath.Join(dir, "a.go"), "package a\n// modified\n")

	_, modified, _, err := DetectChanges(dir, snapshot)
	if err != nil {
		t.Fatalf("DetectChanges: %v", err)
	}
	if len(modified) != 1 || modified[0] != "a.go" {
		t.Errorf("expected modified=[a.go], got %v", modified)
	}
}

func TestDetectChanges_DetectsDeletedFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.go"), "package a\n")
	writeFile(t, filepath.Join(dir, "b.go"), "package b\n")

	snapshot, err := SnapshotFiles(dir)
	if err != nil {
		t.Fatalf("SnapshotFiles: %v", err)
	}

	// Delete a file.
	os.Remove(filepath.Join(dir, "b.go"))

	_, _, deleted, err := DetectChanges(dir, snapshot)
	if err != nil {
		t.Fatalf("DetectChanges: %v", err)
	}
	if len(deleted) != 1 || deleted[0] != "b.go" {
		t.Errorf("expected deleted=[b.go], got %v", deleted)
	}
}

func TestDetectChanges_MixedChanges(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "keep.go"), "package keep\n")
	writeFile(t, filepath.Join(dir, "modify.go"), "package modify\n")
	writeFile(t, filepath.Join(dir, "delete.go"), "package delete\n")

	snapshot, err := SnapshotFiles(dir)
	if err != nil {
		t.Fatalf("SnapshotFiles: %v", err)
	}

	writeFile(t, filepath.Join(dir, "new.go"), "package new\n")
	writeFile(t, filepath.Join(dir, "modify.go"), "package modify\n// changed\n")
	os.Remove(filepath.Join(dir, "delete.go"))

	added, modified, deleted, err := DetectChanges(dir, snapshot)
	if err != nil {
		t.Fatalf("DetectChanges: %v", err)
	}
	if len(added) != 1 {
		t.Errorf("expected 1 added, got %d: %v", len(added), added)
	}
	if len(modified) != 1 {
		t.Errorf("expected 1 modified, got %d: %v", len(modified), modified)
	}
	if len(deleted) != 1 {
		t.Errorf("expected 1 deleted, got %d: %v", len(deleted), deleted)
	}
}

func TestSnapshotFiles_CapturesAllFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.go"), "line1\nline2\n")
	writeFile(t, filepath.Join(dir, "b.go"), "line1\n")

	states, err := SnapshotFiles(dir)
	if err != nil {
		t.Fatalf("SnapshotFiles: %v", err)
	}
	if len(states) != 2 {
		t.Fatalf("expected 2 states, got %d", len(states))
	}

	sort.Slice(states, func(i, j int) bool { return states[i].Path < states[j].Path })

	if states[0].Path != "a.go" {
		t.Errorf("states[0].Path = %q, want %q", states[0].Path, "a.go")
	}
	if states[0].Lines != 2 {
		t.Errorf("states[0].Lines = %d, want 2", states[0].Lines)
	}
	if states[0].Hash == "" {
		t.Error("expected non-empty hash")
	}
}

func TestSnapshotFiles_SkipsHiddenDirs(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git"), 0755)
	writeFile(t, filepath.Join(dir, ".git", "config"), "content\n")
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n")

	states, err := SnapshotFiles(dir)
	if err != nil {
		t.Fatalf("SnapshotFiles: %v", err)
	}
	if len(states) != 1 {
		t.Errorf("expected 1 state (skip .git), got %d", len(states))
	}
}
