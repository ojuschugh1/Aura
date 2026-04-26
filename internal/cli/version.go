package cli

import (
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// Build-time variables injected via ldflags.
var (
	Version   = "dev"
	BuildDate = "unknown"
	Commit    = "unknown"
)

// NewVersionCmd returns the `aura version` command.
func NewVersionCmd(jsonOut *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version, build date, commit, and Go version",
		RunE: func(cmd *cobra.Command, args []string) error {
			info := map[string]string{
				"version":    Version,
				"build_date": BuildDate,
				"commit":     Commit,
				"go_version": runtime.Version(),
			}
			if *jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(info)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "aura %s (built %s, commit %s, %s)\n",
				Version, BuildDate, Commit, runtime.Version())
			return nil
		},
	}
}
