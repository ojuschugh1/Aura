package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/ojuschugh1/aura/internal/cost"
	"github.com/ojuschugh1/aura/internal/db"
	"github.com/spf13/cobra"
)

// NewCostCmd returns the `aura cost` command.
func NewCostCmd(auraDir *string, jsonOut *bool) *cobra.Command {
	var daily, weekly bool
	cmd := &cobra.Command{
		Use:   "cost",
		Short: "Show token usage and cost summary",
		RunE: func(cmd *cobra.Command, args []string) error {
			database, err := db.Open(filepath.Join(*auraDir, "aura.db"))
			if err != nil {
				return fmt.Errorf("open db: %w", err)
			}
			defer database.Close()

			var summary *cost.Summary
			switch {
			case weekly:
				summary, err = cost.WeeklySummary(database)
			case daily:
				summary, err = cost.DailySummary(database)
			default:
				summary, err = cost.DailySummary(database) // default to today
			}
			if err != nil {
				return err
			}

			if *jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(summary)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "period:   %s\n", summary.Period)
			fmt.Fprintf(cmd.OutOrStdout(), "input:    %d tokens\n", summary.InputTokens)
			fmt.Fprintf(cmd.OutOrStdout(), "output:   %d tokens\n", summary.OutputTokens)
			fmt.Fprintf(cmd.OutOrStdout(), "total:    %d tokens\n", summary.TotalTokens)
			fmt.Fprintf(cmd.OutOrStdout(), "cost:     $%.4f\n", summary.CostUSD)
			fmt.Fprintf(cmd.OutOrStdout(), "saved:    $%.4f (%d tokens)\n", summary.SavedUSD, summary.SavedTokens)
			return nil
		},
	}
	cmd.Flags().BoolVar(&daily, "daily", false, "Show daily cost summary")
	cmd.Flags().BoolVar(&weekly, "weekly", false, "Show weekly cost summary")
	return cmd
}
