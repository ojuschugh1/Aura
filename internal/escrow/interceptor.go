package escrow

import "strings"

// IsDestructive returns true if the action type and target warrant escrow interception.
func IsDestructive(actionType, target string) bool {
	switch strings.ToLower(actionType) {
	case "file_delete", "git_push", "git_force_push":
		return true
	case "http":
		method := strings.ToUpper(target)
		return method == "POST" || method == "PUT" || method == "DELETE"
	case "shell":
		return isDestructiveShell(target)
	}
	return false
}

// isDestructiveShell returns true for shell commands that modify system state.
func isDestructiveShell(cmd string) bool {
	dangerous := []string{"rm ", "rm\t", "rmdir", "mv ", "chmod", "chown", "dd ", "mkfs", "git push", "git force"}
	lower := strings.ToLower(cmd)
	for _, d := range dangerous {
		if strings.Contains(lower, d) {
			return true
		}
	}
	return false
}
