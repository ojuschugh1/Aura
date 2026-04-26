package subprocess

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// --- ResolveBinary: resolution order ($PATH → .aura/bin/ → ~/.aura/bin/) ---

func TestResolveBinary_PrefersPathOverLocalBin(t *testing.T) {
	// Create a fake binary in a temp "PATH" directory.
	pathDir := t.TempDir()
	fakeBin := filepath.Join(pathDir, "testbin")
	if err := os.WriteFile(fakeBin, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	// Also create the binary in .aura/bin/ relative to cwd.
	cwd := t.TempDir()
	localBin := filepath.Join(cwd, ".aura", "bin", "testbin")
	if err := os.MkdirAll(filepath.Dir(localBin), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(localBin, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("write local binary: %v", err)
	}

	// Override PATH so exec.LookPath finds our temp dir first.
	t.Setenv("PATH", pathDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	// Change working directory so .aura/bin/ is relative to cwd.
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(cwd); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	resolved, err := ResolveBinary("testbin")
	if err != nil {
		t.Fatalf("ResolveBinary: %v", err)
	}

	// The resolved path should come from $PATH, not .aura/bin/.
	resolvedReal, _ := filepath.EvalSymlinks(resolved)
	fakeBinReal, _ := filepath.EvalSymlinks(fakeBin)
	if resolvedReal != fakeBinReal {
		t.Errorf("expected binary from $PATH (%s), got %s", fakeBinReal, resolvedReal)
	}
}

func TestResolveBinary_PrefersLocalBinOverHomeBin(t *testing.T) {
	// Ensure the binary is NOT on $PATH by using a name that won't exist.
	binaryName := "aura_test_nonexistent_binary_xyz"

	// Create .aura/bin/ in a temp cwd.
	cwd := t.TempDir()
	localBin := filepath.Join(cwd, ".aura", "bin", binaryName)
	if err := os.MkdirAll(filepath.Dir(localBin), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(localBin, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("write local binary: %v", err)
	}

	// Create ~/.aura/bin/ in a temp home.
	fakeHome := t.TempDir()
	homeBin := filepath.Join(fakeHome, ".aura", "bin", binaryName)
	if err := os.MkdirAll(filepath.Dir(homeBin), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(homeBin, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("write home binary: %v", err)
	}

	// Override HOME so os.UserHomeDir returns our fake home.
	t.Setenv("HOME", fakeHome)

	// Strip PATH so exec.LookPath won't find the binary.
	t.Setenv("PATH", "")

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(cwd); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	resolved, err := ResolveBinary(binaryName)
	if err != nil {
		t.Fatalf("ResolveBinary: %v", err)
	}

	// Use EvalSymlinks to normalize paths (macOS /var → /private/var).
	resolvedReal, _ := filepath.EvalSymlinks(resolved)
	localBinReal, _ := filepath.EvalSymlinks(localBin)
	if resolvedReal != localBinReal {
		t.Errorf("expected binary from .aura/bin/ (%s), got %s", localBinReal, resolvedReal)
	}
}

func TestResolveBinary_FallsBackToHomeBin(t *testing.T) {
	binaryName := "aura_test_nonexistent_binary_xyz"

	// Create ~/.aura/bin/ in a temp home.
	fakeHome := t.TempDir()
	homeBin := filepath.Join(fakeHome, ".aura", "bin", binaryName)
	if err := os.MkdirAll(filepath.Dir(homeBin), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(homeBin, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("write home binary: %v", err)
	}

	t.Setenv("HOME", fakeHome)
	t.Setenv("PATH", "")

	// Use a cwd with no .aura/bin/.
	cwd := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(cwd); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	resolved, err := ResolveBinary(binaryName)
	if err != nil {
		t.Fatalf("ResolveBinary: %v", err)
	}

	resolvedReal, _ := filepath.EvalSymlinks(resolved)
	homeBinReal, _ := filepath.EvalSymlinks(homeBin)
	if resolvedReal != homeBinReal {
		t.Errorf("expected binary from ~/.aura/bin/ (%s), got %s", homeBinReal, resolvedReal)
	}
}

func TestResolveBinary_NotFoundReturnsDescriptiveError(t *testing.T) {
	binaryName := "aura_test_totally_missing_binary"

	// Empty PATH, empty cwd, empty home — binary should not be found anywhere.
	t.Setenv("PATH", "")
	t.Setenv("HOME", t.TempDir())

	cwd := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(cwd); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	_, err = ResolveBinary(binaryName)
	if err == nil {
		t.Fatal("expected error when binary is not found, got nil")
	}
	if !strings.Contains(err.Error(), binaryName) {
		t.Errorf("error should mention binary name %q, got: %v", binaryName, err)
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should contain 'not found', got: %v", err)
	}
}

// --- isExecutable helper ---

func TestIsExecutable_ReturnsTrueForExecutableFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "exec")
	if err := os.WriteFile(f, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("write: %v", err)
	}
	if !isExecutable(f) {
		t.Error("expected isExecutable=true for file with 0755 permissions")
	}
}

func TestIsExecutable_ReturnsFalseForNonExecutableFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "noexec")
	if err := os.WriteFile(f, []byte("data"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if isExecutable(f) {
		t.Error("expected isExecutable=false for file with 0644 permissions")
	}
}

func TestIsExecutable_ReturnsFalseForDirectory(t *testing.T) {
	d := t.TempDir()
	if isExecutable(d) {
		t.Error("expected isExecutable=false for a directory")
	}
}

func TestIsExecutable_ReturnsFalseForMissingPath(t *testing.T) {
	if isExecutable("/nonexistent/path/to/binary") {
		t.Error("expected isExecutable=false for nonexistent path")
	}
}

// --- Checksum verification (Download with local fromDir) ---

func TestDownload_ValidChecksum_Succeeds(t *testing.T) {
	// Create a fake binary with known content.
	fromDir := t.TempDir()
	binaryContent := []byte("valid binary content for checksum test")
	binaryName := "sqz"

	if err := os.WriteFile(filepath.Join(fromDir, binaryName), binaryContent, 0644); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	// Compute the real SHA-256 of the content.
	h := sha256.Sum256(binaryContent)
	realChecksum := fmt.Sprintf("%x", h)

	// Temporarily patch the embedded deps to use our checksum.
	// We do this by overriding the depsJSON package variable.
	platform := runtime.GOOS + "-" + runtime.GOARCH
	origDeps := depsJSON
	depsJSON = []byte(fmt.Sprintf(`[{"name":"sqz","repo":"https://example.com","version":"v0.0.1","checksums":{%q:%q}}]`, platform, realChecksum))
	t.Cleanup(func() { depsJSON = origDeps })

	// Override HOME so Download installs to a temp directory.
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	path, err := Download(binaryName, fromDir)
	if err != nil {
		t.Fatalf("Download with valid checksum should succeed, got: %v", err)
	}

	if !strings.HasSuffix(path, binaryName) {
		t.Errorf("expected path to end with %q, got %s", binaryName, path)
	}

	// Verify the file was installed.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("installed binary not found: %v", err)
	}
	if info.Mode()&0111 == 0 {
		t.Error("installed binary should be executable")
	}
}

func TestDownload_TamperedBinary_Rejected(t *testing.T) {
	fromDir := t.TempDir()
	binaryContent := []byte("tampered binary content")
	binaryName := "sqz"

	if err := os.WriteFile(filepath.Join(fromDir, binaryName), binaryContent, 0644); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	// Use a checksum that does NOT match the content.
	platform := runtime.GOOS + "-" + runtime.GOARCH
	wrongChecksum := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	origDeps := depsJSON
	depsJSON = []byte(fmt.Sprintf(`[{"name":"sqz","repo":"https://example.com","version":"v0.0.1","checksums":{%q:%q}}]`, platform, wrongChecksum))
	t.Cleanup(func() { depsJSON = origDeps })

	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	_, err := Download(binaryName, fromDir)
	if err == nil {
		t.Fatal("Download with tampered binary should fail, got nil error")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("expected 'checksum mismatch' error, got: %v", err)
	}
}

func TestDownload_PlaceholderChecksum_SkipsVerification(t *testing.T) {
	// When the checksum is all zeros (placeholder), verification is skipped.
	fromDir := t.TempDir()
	binaryContent := []byte("any content at all")
	binaryName := "sqz"

	if err := os.WriteFile(filepath.Join(fromDir, binaryName), binaryContent, 0644); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	platform := runtime.GOOS + "-" + runtime.GOARCH
	placeholder := "0000000000000000000000000000000000000000000000000000000000000000"

	origDeps := depsJSON
	depsJSON = []byte(fmt.Sprintf(`[{"name":"sqz","repo":"https://example.com","version":"v0.0.1","checksums":{%q:%q}}]`, platform, placeholder))
	t.Cleanup(func() { depsJSON = origDeps })

	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	path, err := Download(binaryName, fromDir)
	if err != nil {
		t.Fatalf("Download with placeholder checksum should succeed, got: %v", err)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Errorf("installed binary not found: %v", statErr)
	}
}

// --- Graceful degradation: binary missing, feature unavailable ---

func TestEnsureBinary_NonInteractive_ReturnsUnavailableError(t *testing.T) {
	binaryName := "aura_test_missing_binary"

	// Ensure binary is not found anywhere.
	t.Setenv("PATH", "")
	t.Setenv("HOME", t.TempDir())

	cwd := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(cwd); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	_, err = EnsureBinary(binaryName, "test-feature", "", true /* nonInteractive */)
	if err == nil {
		t.Fatal("expected error when binary is missing in non-interactive mode")
	}
	if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "non-interactive") {
		t.Errorf("error should mention non-interactive mode, got: %v", err)
	}
	if !strings.Contains(err.Error(), "test-feature") {
		t.Errorf("error should mention the feature name, got: %v", err)
	}
}

func TestEnsureBinary_ReturnsPathWhenBinaryExists(t *testing.T) {
	binaryName := "aura_test_present_binary"

	// Place binary on PATH.
	pathDir := t.TempDir()
	fakeBin := filepath.Join(pathDir, binaryName)
	if err := os.WriteFile(fakeBin, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Setenv("PATH", pathDir)

	path, err := EnsureBinary(binaryName, "test-feature", "", true)
	if err != nil {
		t.Fatalf("EnsureBinary: %v", err)
	}

	// Normalize paths for comparison (macOS /var → /private/var).
	resolvedReal, _ := filepath.EvalSymlinks(path)
	fakeBinReal, _ := filepath.EvalSymlinks(fakeBin)
	if resolvedReal != fakeBinReal {
		t.Errorf("expected %s, got %s", fakeBinReal, resolvedReal)
	}
}

// --- Dependency manifest: all three binaries defined ---

func TestLoadDeps_ContainsAllExpectedBinaries(t *testing.T) {
	deps, err := LoadDeps()
	if err != nil {
		t.Fatalf("LoadDeps: %v", err)
	}

	expected := map[string]bool{
		"sqz":        false,
		"claimcheck": false,
		"ghostdep":   false,
	}

	for _, d := range deps {
		if _, ok := expected[d.Name]; ok {
			expected[d.Name] = true
		}
	}

	for name, found := range expected {
		if !found {
			t.Errorf("expected binary %q in deps manifest, but not found", name)
		}
	}
}

func TestFindDep_ReturnsCorrectBinary(t *testing.T) {
	dep, err := FindDep("sqz")
	if err != nil {
		t.Fatalf("FindDep(sqz): %v", err)
	}
	if dep.Name != "sqz" {
		t.Errorf("expected Name=sqz, got %s", dep.Name)
	}
	if dep.Repo == "" {
		t.Error("expected non-empty Repo")
	}
	if dep.Version == "" {
		t.Error("expected non-empty Version")
	}
	if len(dep.Checksums) == 0 {
		t.Error("expected non-empty Checksums map")
	}
}

func TestFindDep_UnknownBinaryReturnsError(t *testing.T) {
	_, err := FindDep("nonexistent_binary")
	if err == nil {
		t.Fatal("expected error for unknown binary, got nil")
	}
	if !strings.Contains(err.Error(), "unknown dependency") {
		t.Errorf("expected 'unknown dependency' error, got: %v", err)
	}
}

// --- Runner: basic construction ---

func TestRunner_RunWithMissingBinary(t *testing.T) {
	r := &Runner{BinaryPath: "/nonexistent/binary/path"}
	_, err := r.Run(t.Context(), []string{"--version"}, nil)
	if err == nil {
		t.Fatal("expected error when running nonexistent binary")
	}
}

func TestRunner_RunEchoCommand(t *testing.T) {
	// Use a real binary (echo) to verify Runner works end-to-end.
	echoPath, err := exec.LookPath("echo")
	if err != nil {
		t.Skip("echo not found on PATH")
	}

	r := &Runner{BinaryPath: echoPath}
	res, err := r.Run(t.Context(), []string{"hello", "world"}, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(string(res.Stdout), "hello world") {
		t.Errorf("expected stdout to contain 'hello world', got %q", string(res.Stdout))
	}
	if res.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", res.ExitCode)
	}
}
