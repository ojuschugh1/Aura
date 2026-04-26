package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewHelpCmd returns the `aura help` command that lists all top-level commands.
func NewHelpCmd(root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:   "help [command]",
		Short: "Show help for a command",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return root.Help()
			}
			// Find the subcommand and print its help.
			for _, sub := range root.Commands() {
				if sub.Name() == args[0] {
					return sub.Help()
				}
			}
			return fmt.Errorf("unknown command %q", args[0])
		},
	}
}
