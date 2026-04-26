package cli

import (
	"encoding/json"
	"fmt"

	"github.com/ojuschugh1/aura/internal/scan"
	"github.com/spf13/cobra"
)

// NewScanCmd returns the `aura scan` command.
func NewScanCmd(jsonOut *bool) *cobra.Command {
	var sarif, fix bool
	cmd := &cobra.Command{
		Use:   "scan [path]",
		Short: "Scan for phantom dependencies via ghostdep",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "."
			if len(args) > 0 {
				root = args[0]
			}

			result, err := scan.Scan(root)
			if err != nil {
				return err
			}

			if sarif {
				sarifJSON, err := scan.ToSARIF(result.Phantoms)
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(sarifJSON))
				return nil
			}

			if *jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
			}

			if len(result.Phantoms) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no phantom dependencies found")
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-40s %5s %-30s %10s\n", "FILE", "LINE", "IMPORT", "CONFIDENCE")
			for _, d := range result.Phantoms {
				risk := ""
				if d.Confidence > 0.8 {
					risk = " [HIGH RISK]"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-40s %5d %-30s %10.2f%s\n",
					d.File, d.Line, d.Import, d.Confidence, risk)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\ntotal: %d (%d high-risk)\n", len(result.Phantoms), len(result.HighRisk))

			if fix {
				fmt.Fprintln(cmd.ErrOrStderr(), "note: run ghostdep fix directly for auto-fix suggestions")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&sarif, "sarif", false, "Output results in SARIF format")
	cmd.Flags().BoolVar(&fix, "fix", false, "Show auto-fix suggestions")
	return cmd
}
