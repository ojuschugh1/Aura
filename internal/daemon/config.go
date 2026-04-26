package daemon

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteDefaultConfigs generates .aura/config.toml, policy.toml, and routing.toml
// if they do not already exist.
func WriteDefaultConfigs(dir string, secret string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create aura dir: %w", err)
	}

	configs := map[string]string{
		"config.toml":  configTOML(secret),
		"policy.toml":  policyTOML(),
		"routing.toml": routingTOML(),
	}

	for name, content := range configs {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				return fmt.Errorf("write %s: %w", name, err)
			}
		}
	}
	return nil
}

func configTOML(secret string) string {
	return fmt.Sprintf(`[daemon]
port = 7437
log_level = "info"

[auth]
shared_secret = %q

[memory]
max_entries = 10000

[compression]
sqz_binary = "sqz"
min_compression_ratio = 0.20

[cost.models.claude-sonnet]
input = 3.00
output = 15.00

[cost.models.claude-haiku]
input = 0.25
output = 1.25

[cost.models.gpt-4o]
input = 2.50
output = 10.00

[cost.models.gpt-4o-mini]
input = 0.15
output = 0.60

[traces]
ttl_days = 14
max_size_mb = 500

[subprocess]
timeout_seconds = 30
`, secret)
}

func policyTOML() string {
	return `[[rules]]
category = "read"
disposition = "auto-approve"

[[rules]]
category = "write"
disposition = "require-approval"

[[rules]]
category = "execute"
disposition = "require-approval"

[[rules]]
category = "network"
disposition = "deny"
`
}

func routingTOML() string {
	return `# Model Router Configuration (v0.6+)
[classification]
low_max_tokens = 500
medium_max_tokens = 2000

[models.low]
name = "gpt-4o-mini"
budget_usd = 5.00

[models.medium]
name = "claude-sonnet"
budget_usd = 20.00

[models.high]
name = "claude-opus"
budget_usd = 50.00

[supervisor]
enabled = false
generator = "claude-sonnet"
reviewer = "gpt-4o"
`
}

// generateLocalSecret creates a random 32-byte hex secret for the daemon.
func generateLocalSecret() string {
	b := make([]byte, 32)
	f, err := os.Open("/dev/urandom")
	if err != nil {
		// Fallback: use PID + timestamp as a weak secret.
		return fmt.Sprintf("%x%x", os.Getpid(), os.Getpid()*1000003)
	}
	defer f.Close()
	_, _ = f.Read(b)
	return fmt.Sprintf("%x", b)
}
