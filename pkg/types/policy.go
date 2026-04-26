package types

// PolicyRule maps an action category to a disposition.
type PolicyRule struct {
	Category    string `toml:"category" json:"category"`
	Disposition string `toml:"disposition" json:"disposition"`
	PathPattern string `toml:"path,omitempty" json:"path,omitempty"`
}

// PolicyConfig holds the full set of policy rules and overrides.
type PolicyConfig struct {
	Rules     []PolicyRule `toml:"rules" json:"rules"`
	Overrides []PolicyRule `toml:"overrides,omitempty" json:"overrides,omitempty"`
}
