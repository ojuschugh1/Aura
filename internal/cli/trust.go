package cli

import (
	"fmt"
	"time"

	"github.com/ojuschugh1/aura/internal/escrow"
	"github.com/spf13/cobra"
)

// NewTrustCmd returns the `aura trust` command.
func NewTrustCmd() *cobra.Command {
	var duration int
	var path string

	cmd := &cobra.Command{
		Use:   "trust",
		Short: "Grant a temporary trust window for auto-approved actions",
		RunE: func(cmd *cobra.Command, args []string) error {
			if duration == 0 && path == "" {
				return fmt.Errorf("specify --duration <minutes> or --path <directory>")
			}

			tw := &escrow.TrustWindow{}
			d := time.Duration(duration) * time.Minute
			if d == 0 {
				d = 60 * time.Minute // default 1h when only --path is given
			}
			tw.Grant(d, path)

			if path != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "trust window active: %d min, path: %s\n", duration, path)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "trust window active: %d min (all paths)\n", duration)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "note: trust window is in-process only; restart daemon to revoke early")
			return nil
		},
	}
	cmd.Flags().IntVar(&duration, "duration", 0, "Trust window duration in minutes")
	cmd.Flags().StringVar(&path, "path", "", "Restrict trust to this directory path")
	return cmd
}
