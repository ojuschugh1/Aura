package subprocess

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
)

// Download downloads the named binary from GitHub Releases, verifies its checksum,
// and installs it to ~/.aura/bin/ with executable permissions.
// If fromDir is non-empty, the binary is copied from that local directory instead.
func Download(name, fromDir string) (string, error) {
	dep, err := FindDep(name)
	if err != nil {
		return "", err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	binDir := filepath.Join(home, ".aura", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return "", fmt.Errorf("create bin dir: %w", err)
	}

	destPath := filepath.Join(binDir, name)
	platform := runtime.GOOS + "-" + runtime.GOARCH

	expectedHash, ok := dep.Checksums[platform]
	if !ok {
		return "", fmt.Errorf("no checksum for platform %s", platform)
	}

	var src io.Reader
	if fromDir != "" {
		// Offline mode: copy from local directory.
		f, err := os.Open(filepath.Join(fromDir, name))
		if err != nil {
			return "", fmt.Errorf("open local binary: %w", err)
		}
		defer f.Close()
		src = f
	} else {
		// Download from GitHub Releases.
		url := fmt.Sprintf("%s/releases/download/%s/%s-%s", dep.Repo, dep.Version, name, platform)
		resp, err := http.Get(url) //nolint:gosec
		if err != nil {
			return "", fmt.Errorf("download %s: %w", name, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("download %s: HTTP %d", name, resp.StatusCode)
		}
		src = resp.Body
	}

	// Write to a temp file, verify checksum, then move to destination.
	tmp, err := os.CreateTemp(binDir, name+".tmp")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmp.Name())

	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, h), src); err != nil {
		tmp.Close()
		return "", fmt.Errorf("write binary: %w", err)
	}
	tmp.Close()

	// Verify checksum (skip if placeholder zeros).
	actualHash := fmt.Sprintf("%x", h.Sum(nil))
	if expectedHash != "0000000000000000000000000000000000000000000000000000000000000000" {
		if actualHash != expectedHash {
			return "", fmt.Errorf("checksum mismatch for %s: expected %s, got %s", name, expectedHash, actualHash)
		}
	}

	if err := os.Chmod(tmp.Name(), 0755); err != nil {
		return "", fmt.Errorf("chmod: %w", err)
	}
	if err := os.Rename(tmp.Name(), destPath); err != nil {
		return "", fmt.Errorf("install binary: %w", err)
	}
	return destPath, nil
}
