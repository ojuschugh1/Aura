package mcp

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const secretFile = "config.toml"

// GenerateSecret creates a 32-byte random hex secret.
func GenerateSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate secret: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// LoadSecret reads the shared_secret value from .aura/config.toml.
// Returns an empty string if the file or key is not found.
func LoadSecret(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, secretFile))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "shared_secret") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				return strings.Trim(strings.TrimSpace(parts[1]), `"`)
			}
		}
	}
	return ""
}
