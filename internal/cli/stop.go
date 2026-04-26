package cli

import (
	"fmt"

	"github.com/ojuschugh1/aura/internal/daemon"
	"github.com/spf13/cobra"
)

// NewStopCmd returns the `aura stop` command.
func NewStopCmd(auraDir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Gracefully stop the Aura daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := daemon.Stop(*auraDir); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "daemon stopped")
			return nil
		},
	}
}
