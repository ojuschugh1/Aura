package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/ojuschugh1/aura/pkg/types"
)

// --- WriteDefaultConfigs creates all three files ----------------------------

func TestWriteDefaultConfigs_CreatesAllFiles(t *testing.T) {
	dir := t.TempDir()
	secret := "test-secret-abc123"

	if err := WriteDefaultConfigs(dir, secret); err != nil {
		t.Fatalf("WriteDefaultConfigs: %v", err)
	}

	for _, name := range []string{"config.toml", "policy.toml", "routing.toml"} {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected %s to exist, but it does not", name)
		}
	}
}

// --- Config directory is created if it doesn't exist ------------------------

func TestWriteDefaultConfigs_CreatesDirectory(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "nested", ".aura")

	if err := WriteDefaultConfigs(dir, "secret"); err != nil {
		t.Fatalf("WriteDefaultConfigs: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected a directory, got a file")
	}
}

// --- Does NOT overwrite existing config files -------------------------------

func TestWriteDefaultConfigs_DoesNotOverwriteExisting(t *testing.T) {
	dir := t.TempDir()

	// Pre-create config.toml with custom content.
	customContent := "# my custom config\n"
	configPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(configPath, []byte(customContent), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := WriteDefaultConfigs(dir, "new-secret"); err != nil {
		t.Fatalf("WriteDefaultConfigs: %v", err)
	}

	// config.toml should still have the custom content.
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != customContent {
		t.Errorf("config.toml was overwritten; got %q, want %q", string(data), customContent)
	}
}

// --- Generated config.toml contains expected default values -----------------

func TestConfigTOML_ContainsDefaultPort(t *testing.T) {
	content := configTOML("secret")
	if !strings.Contains(content, "port = 7437") {
		t.Error("config.toml should contain port = 7437")
	}
}

func TestConfigTOML_ContainsLogLevel(t *testing.T) {
	content := configTOML("secret")
	if !strings.Contains(content, `log_level = "info"`) {
		t.Error("config.toml should contain log_level = info")
	}
}

func TestConfigTOML_ContainsAuthSecret(t *testing.T) {
	secret := "my-unique-secret-value"
	content := configTOML(secret)
	if !strings.Contains(content, secret) {
		t.Error("config.toml should contain the provided auth secret")
	}
	if !strings.Contains(content, "[auth]") {
		t.Error("config.toml should contain [auth] section")
	}
}

func TestConfigTOML_ContainsMemoryLimits(t *testing.T) {
	content := configTOML("secret")
	if !strings.Contains(content, "max_entries = 10000") {
		t.Error("config.toml should contain max_entries = 10000")
	}
}

func TestConfigTOML_ContainsCompressionSettings(t *testing.T) {
	content := configTOML("secret")
	if !strings.Contains(content, "[compression]") {
		t.Error("config.toml should contain [compression] section")
	}
	if !strings.Contains(content, `sqz_binary = "sqz"`) {
		t.Error("config.toml should contain sqz_binary setting")
	}
	if !strings.Contains(content, "min_compression_ratio = 0.20") {
		t.Error("config.toml should contain min_compression_ratio setting")
	}
}

func TestConfigTOML_ContainsModelPricing(t *testing.T) {
	content := configTOML("secret")
	for _, model := range []string{"claude-sonnet", "claude-haiku", "gpt-4o", "gpt-4o-mini"} {
		if !strings.Contains(content, model) {
			t.Errorf("config.toml should contain pricing for model %q", model)
		}
	}
}

func TestConfigTOML_ContainsTraceTTL(t *testing.T) {
	content := configTOML("secret")
	if !strings.Contains(content, "ttl_days = 14") {
		t.Error("config.toml should contain ttl_days = 14")
	}
	if !strings.Contains(content, "max_size_mb = 500") {
		t.Error("config.toml should contain max_size_mb = 500")
	}
}

func TestConfigTOML_ContainsSubprocessTimeout(t *testing.T) {
	content := configTOML("secret")
	if !strings.Contains(content, "timeout_seconds = 30") {
		t.Error("config.toml should contain timeout_seconds = 30")
	}
}

// --- Generated config.toml is valid TOML ------------------------------------

func TestConfigTOML_IsValidTOML(t *testing.T) {
	content := configTOML("test-secret")
	var parsed map[string]interface{}
	if err := toml.Unmarshal([]byte(content), &parsed); err != nil {
		t.Fatalf("config.toml is not valid TOML: %v", err)
	}
}

// --- Generated policy.toml contains default policy rules --------------------

func TestPolicyTOML_ContainsDefaultRules(t *testing.T) {
	content := policyTOML()

	expectations := []struct {
		category    string
		disposition string
	}{
		{"read", "auto-approve"},
		{"write", "require-approval"},
		{"execute", "require-approval"},
		{"network", "deny"},
	}

	for _, exp := range expectations {
		if !strings.Contains(content, exp.category) {
			t.Errorf("policy.toml should contain category %q", exp.category)
		}
		if !strings.Contains(content, exp.disposition) {
			t.Errorf("policy.toml should contain disposition %q", exp.disposition)
		}
	}
}

// --- Generated policy.toml is valid TOML and parseable as PolicyConfig ------

func TestPolicyTOML_IsValidTOML(t *testing.T) {
	content := policyTOML()
	var cfg types.PolicyConfig
	if err := toml.Unmarshal([]byte(content), &cfg); err != nil {
		t.Fatalf("policy.toml is not valid TOML: %v", err)
	}

	if len(cfg.Rules) != 4 {
		t.Fatalf("expected 4 policy rules, got %d", len(cfg.Rules))
	}

	expected := map[string]string{
		"read":    "auto-approve",
		"write":   "require-approval",
		"execute": "require-approval",
		"network": "deny",
	}
	for _, rule := range cfg.Rules {
		want, ok := expected[rule.Category]
		if !ok {
			t.Errorf("unexpected category %q in policy.toml", rule.Category)
			continue
		}
		if rule.Disposition != want {
			t.Errorf("category %q: disposition = %q, want %q", rule.Category, rule.Disposition, want)
		}
	}
}

// --- Generated routing.toml is valid TOML -----------------------------------

func TestRoutingTOML_IsValidTOML(t *testing.T) {
	content := routingTOML()
	var parsed map[string]interface{}
	if err := toml.Unmarshal([]byte(content), &parsed); err != nil {
		t.Fatalf("routing.toml is not valid TOML: %v", err)
	}
}

// --- Integration: full WriteDefaultConfigs round-trip -----------------------

func TestWriteDefaultConfigs_FilesAreParseable(t *testing.T) {
	dir := t.TempDir()
	secret := "integration-test-secret"

	if err := WriteDefaultConfigs(dir, secret); err != nil {
		t.Fatalf("WriteDefaultConfigs: %v", err)
	}

	// Verify config.toml is parseable.
	configData, err := os.ReadFile(filepath.Join(dir, "config.toml"))
	if err != nil {
		t.Fatalf("ReadFile config.toml: %v", err)
	}
	var configParsed map[string]interface{}
	if err := toml.Unmarshal(configData, &configParsed); err != nil {
		t.Fatalf("config.toml not parseable: %v", err)
	}

	// Verify policy.toml is parseable as PolicyConfig.
	policyData, err := os.ReadFile(filepath.Join(dir, "policy.toml"))
	if err != nil {
		t.Fatalf("ReadFile policy.toml: %v", err)
	}
	var policyCfg types.PolicyConfig
	if err := toml.Unmarshal(policyData, &policyCfg); err != nil {
		t.Fatalf("policy.toml not parseable: %v", err)
	}
	if len(policyCfg.Rules) == 0 {
		t.Error("policy.toml should have at least one rule")
	}

	// Verify routing.toml is parseable.
	routingData, err := os.ReadFile(filepath.Join(dir, "routing.toml"))
	if err != nil {
		t.Fatalf("ReadFile routing.toml: %v", err)
	}
	var routingParsed map[string]interface{}
	if err := toml.Unmarshal(routingData, &routingParsed); err != nil {
		t.Fatalf("routing.toml not parseable: %v", err)
	}
}

func TestWriteDefaultConfigs_ConfigContainsSecret(t *testing.T) {
	dir := t.TempDir()
	secret := "round-trip-secret-xyz"

	if err := WriteDefaultConfigs(dir, secret); err != nil {
		t.Fatalf("WriteDefaultConfigs: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "config.toml"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), secret) {
		t.Error("written config.toml should contain the auth secret")
	}
}
