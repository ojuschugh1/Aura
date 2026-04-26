package verify

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// VerifyResult summarises the outcome of verifying a set of claims.
type VerifyResult struct {
	TotalClaims int
	PassCount   int
	FailCount   int
	TruthPct    float64
	Claims      []Claim
}

// Verify checks each claim against the filesystem / VCS and returns a result.
func Verify(claims []Claim, projectRoot string) VerifyResult {
	verified := make([]Claim, len(claims))
	copy(verified, claims)

	for i := range verified {
		c := &verified[i]
		c.Verified = true
		switch c.Type {
		case "file_created":
			c.Pass, c.Detail = verifyFileExists(c.Target, projectRoot)
		case "file_modified":
			c.Pass, c.Detail = verifyFileModified(c.Target, projectRoot)
		case "package_installed":
			c.Pass, c.Detail = verifyPackageInstalled(c.Target, projectRoot)
		case "test_passed", "command_executed":
			// Cannot re-run; accept the claim at face value.
			c.Pass = true
			c.Detail = "accepted without re-execution"
		default:
			c.Pass = false
			c.Detail = fmt.Sprintf("unknown claim type %q", c.Type)
		}
	}

	res := VerifyResult{TotalClaims: len(verified), Claims: verified}
	for _, c := range verified {
		if c.Pass {
			res.PassCount++
		} else {
			res.FailCount++
		}
	}
	if res.TotalClaims > 0 {
		res.TruthPct = float64(res.PassCount) / float64(res.TotalClaims) * 100
	}
	return res
}

// verifyFileExists checks that the target path exists on disk.
func verifyFileExists(target, root string) (bool, string) {
	path := resolvePath(target, root)
	if _, err := os.Stat(path); err == nil {
		return true, fmt.Sprintf("file exists: %s", path)
	}
	return false, fmt.Sprintf("file not found: %s", path)
}

// verifyFileModified checks git diff --name-only for the target file.
func verifyFileModified(target, root string) (bool, string) {
	cmd := exec.Command("git", "diff", "--name-only", "HEAD")
	cmd.Dir = root
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return false, fmt.Sprintf("git diff failed: %v", err)
	}
	base := filepath.Base(target)
	for _, line := range strings.Split(out.String(), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if filepath.Base(line) == base || line == target {
			return true, fmt.Sprintf("found in git diff: %s", line)
		}
	}
	return false, fmt.Sprintf("%s not in git diff output", target)
}

// verifyPackageInstalled searches common lock/manifest files for the package name.
func verifyPackageInstalled(pkg, root string) (bool, string) {
	candidates := []string{
		"go.sum",
		"package-lock.json",
		"Cargo.lock",
		"requirements.txt",
	}
	for _, name := range candidates {
		path := filepath.Join(root, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if strings.Contains(string(data), pkg) {
			return true, fmt.Sprintf("found %q in %s", pkg, name)
		}
	}
	return false, fmt.Sprintf("%q not found in any lock/manifest file", pkg)
}

// resolvePath makes target absolute relative to root when it is not already.
func resolvePath(target, root string) string {
	if filepath.IsAbs(target) {
		return target
	}
	return filepath.Join(root, target)
}
