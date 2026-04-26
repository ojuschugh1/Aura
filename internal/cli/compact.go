package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/ojuschugh1/aura/internal/compress"
	"github.com/ojuschugh1/aura/internal/db"
	"github.com/spf13/cobra"
)

// NewCompactCmd returns the `aura compact` command.
func NewCompactCmd(auraDir *string, jsonOut *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "compact",
		Short: "Compress current session context via sqz",
		RunE: func(cmd *cobra.Command, args []string) error {
			database, err := db.Open(*auraDir + "/aura.db")
			if err != nil {
				return fmt.Errorf("open db: %w", err)
			}
			defer database.Close()

			engine := compress.New(database)

			// Read content from stdin if available, otherwise report nothing to compact.
			stat, _ := os.Stdin.Stat()
			if stat.Mode()&os.ModeCharDevice != 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "pipe content to compact via stdin, e.g.: cat context.txt | aura compact")
				return nil
			}

			content, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("read stdin: %w", err)
			}

			result, err := engine.Compact(string(content))
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %v\n", err)
			}

			if *jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "original:   %d tokens\n", result.OriginalTokens)
			fmt.Fprintf(cmd.OutOrStdout(), "compressed: %d tokens\n", result.CompressedTokens)
			fmt.Fprintf(cmd.OutOrStdout(), "reduction:  %.1f%%\n", result.ReductionPct)
			if result.Deduplicated {
				fmt.Fprintln(cmd.OutOrStdout(), "note: content was already in dedup cache")
			}
			return nil
		},
	}
}
