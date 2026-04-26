package policy

import "github.com/ojuschugh1/aura/pkg/types"

// DefaultConfig returns the built-in policy configuration.
func DefaultConfig() types.PolicyConfig {
	return types.PolicyConfig{
		Rules: []types.PolicyRule{
			{Category: "read", Disposition: "auto-approve"},
			{Category: "write", Disposition: "require-approval"},
			{Category: "execute", Disposition: "require-approval"},
			{Category: "network", Disposition: "deny"},
		},
	}
}
