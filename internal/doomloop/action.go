package doomloop

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
)

// Action represents a single agent action for fingerprinting.
type Action struct {
	Type    string                 // e.g. "file_write", "shell", "http_post"
	Target  string                 // file path, URL, command
	Params  map[string]interface{} // additional parameters
	Outcome string                 // "success" or "failure"
}

// Fingerprint returns a stable hash of the action's type, target, and params.
// Minor param variations (e.g. whitespace) are normalised before hashing.
func Fingerprint(a Action) string {
	paramsJSON, _ := json.Marshal(a.Params)
	raw := fmt.Sprintf("%s|%s|%s", a.Type, a.Target, string(paramsJSON))
	sum := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", sum)
}
