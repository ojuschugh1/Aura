package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ojuschugh1/aura/internal/trace"
	"github.com/spf13/cobra"
)

// NewReplayCmd returns the `aura replay` command.
func NewReplayCmd(auraDir *string, jsonOut *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "replay <session_id>",
		Short: "Replay a trace and diff against current state",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tracesDir := filepath.Join(*auraDir, "traces")
			root, _ := os.Getwd()

			result, err := trace.Replay(tracesDir, args[0], root)
			if err != nil {
				return err
			}

			if *jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "session: %s\n", result.SessionID)
			fmt.Fprintf(cmd.OutOrStdout(), "total:   %d actions\n", result.Total)
			fmt.Fprintf(cmd.OutOrStdout(), "matched: %d\n", result.Matched)
			fmt.Fprintf(cmd.OutOrStdout(), "diffs:   %d\n", len(result.Diffs))
			for _, d := range result.Diffs {
				fmt.Fprintf(cmd.OutOrStdout(), "  [DIFF] %s %s: was=%s now=%s\n",
					d.ActionType, d.Target, d.Original, d.Replay)
			}
			return nil
		},
	}
}
