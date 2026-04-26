package subprocess

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// ResolveBinary finds the binary named `name` by checking in order:
// 1. $PATH (via exec.LookPath)
// 2. .aura/bin/ relative to the current working directory
// 3. ~/.aura/bin/
// Returns the full path or an error with install instructions.
func ResolveBinary(name string) (string, error) {
	// 1. Check $PATH.
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}

	// 2. Check .aura/bin/ in cwd.
	if cwd, err := os.Getwd(); err == nil {
		local := filepath.Join(cwd, ".aura", "bin", name)
		if isExecutable(local) {
			return local, nil
		}
	}

	// 3. Check ~/.aura/bin/.
	if home, err := os.UserHomeDir(); err == nil {
		global := filepath.Join(home, ".aura", "bin", name)
		if isExecutable(global) {
			return global, nil
		}
	}

	return "", fmt.Errorf(
		"%s not found; install it or run `aura init --install-deps` to download automatically",
		name,
	)
}

// isExecutable returns true if path exists and is executable.
func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir() && info.Mode()&0111 != 0
}
