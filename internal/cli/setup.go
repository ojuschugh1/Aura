package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/ojuschugh1/aura/internal/mcp"
	"github.com/spf13/cobra"
)

// NewSetupCmd returns the `aura setup <tool>` command.
func NewSetupCmd(auraDir *string) *cobra.Command {
	return &cobra.Command{
		Use:       "setup <tool>",
		Short:     "Generate MCP config for Claude Code, Cursor, or Kiro",
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"claude", "cursor", "kiro"},
		RunE: func(cmd *cobra.Command, args []string) error {
			secret := mcp.LoadSecret(*auraDir)
			port := 7437 // default MCP port

			mcpConfig := map[string]interface{}{
				"mcpServers": map[string]interface{}{
					"aura": map[string]interface{}{
						"url":    fmt.Sprintf("http://localhost:%d/mcp", port),
						"headers": map[string]string{
							"Authorization": "Bearer " + secret,
						},
					},
				},
			}

			b, err := json.MarshalIndent(mcpConfig, "", "  ")
			if err != nil {
				return err
			}

			tool := args[0]
			var configPath string
			switch tool {
			case "claude":
				configPath = filepath.Join("~", ".claude", "settings.json")
			case "cursor":
				configPath = filepath.Join("~", ".cursor", "mcp.json")
			case "kiro":
				configPath = filepath.Join(".kiro", "settings", "mcp.json")
			default:
				return fmt.Errorf("unknown tool %q: use claude, cursor, or kiro", tool)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "# Add to %s:\n%s\n", configPath, string(b))
			return nil
		},
	}
}
