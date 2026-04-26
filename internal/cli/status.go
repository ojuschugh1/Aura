package cli

import (
	"encoding/json"
	"fmt"

	"github.com/ojuschugh1/aura/internal/daemon"
	"github.com/spf13/cobra"
)

// NewStatusCmd returns the `aura status` command.
func NewStatusCmd(auraDir *string, jsonOut *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show daemon status, port, memory count, and session ID",
		RunE: func(cmd *cobra.Command, args []string) error {
			si, err := daemon.Status(*auraDir)
			if err != nil {
				return err
			}

			if *jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(si)
			}

			if !si.Running {
				fmt.Fprintln(cmd.OutOrStdout(), "daemon: not running")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "daemon:  running (pid %d)\n", si.PID)
			fmt.Fprintf(cmd.OutOrStdout(), "port:    %d\n", si.Port)
			fmt.Fprintf(cmd.OutOrStdout(), "memory:  %d entries\n", si.MemoryCount)
			if si.SessionID != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "session: %s\n", si.SessionID)
			}
			return nil
		},
	}
}
