package verify

import (
	"regexp"
	"strings"
)

// Claim represents a single verifiable assertion made by an agent.
type Claim struct {
	Type     string // file_created, file_modified, package_installed, test_passed, command_executed
	Target   string // the subject of the claim (file path, package name, command, etc.)
	Verified bool   // whether verification was attempted
	Pass     bool   // whether the claim was confirmed true
	Detail   string // human-readable explanation of the verification result
}

// Patterns for each claim type (compiled once at package init).
var (
	reFileCreated = regexp.MustCompile(
		`(?i)(?:created\s+file|wrote)\s+(\S+)`)
	reFileModified = regexp.MustCompile(
		`(?i)(?:modified|updated|edited)\s+(\S+)`)
	rePackageInstalled = regexp.MustCompile(
		`(?i)(?:installed|added\s+dependency)\s+(\S+)`)
	reTestPassed = regexp.MustCompile(
		`(?i)tests?\s+pass(?:ed)?`)
	reCommandExecuted = regexp.MustCompile(
		`(?i)(?:ran\s+command|executed|ran)\s+(\S+)`)
)

// ExtractClaims scans assistant transcript entries and returns all detected claims.
func ExtractClaims(entries []TranscriptEntry) []Claim {
	var claims []Claim
	for _, e := range entries {
		// Only inspect assistant/agent messages.
		role := strings.ToLower(e.Role)
		if role != "assistant" && role != "message" {
			continue
		}
		claims = append(claims, extractFromContent(e.Content)...)
	}
	return claims
}

// extractFromContent applies all patterns to a single content string.
func extractFromContent(content string) []Claim {
	var claims []Claim

	// file_created
	for _, m := range reFileCreated.FindAllStringSubmatch(content, -1) {
		claims = append(claims, Claim{Type: "file_created", Target: m[1]})
	}

	// file_modified
	for _, m := range reFileModified.FindAllStringSubmatch(content, -1) {
		claims = append(claims, Claim{Type: "file_modified", Target: m[1]})
	}

	// package_installed
	for _, m := range rePackageInstalled.FindAllStringSubmatch(content, -1) {
		claims = append(claims, Claim{Type: "package_installed", Target: m[1]})
	}

	// test_passed (no target)
	if reTestPassed.MatchString(content) {
		claims = append(claims, Claim{Type: "test_passed"})
	}

	// command_executed
	for _, m := range reCommandExecuted.FindAllStringSubmatch(content, -1) {
		claims = append(claims, Claim{Type: "command_executed", Target: m[1]})
	}

	return claims
}
