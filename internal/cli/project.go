package cli

import (
	"encoding/json"
	"fmt"

	"github.com/ojuschugh1/aura/internal/codebase"
	"github.com/ojuschugh1/aura/internal/memory"
	"github.com/spf13/cobra"
)

// NewProjectCmd returns the `aura project` command with subcommands.
func NewProjectCmd(auraDir *string, jsonOut *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Project awareness and structure mapping",
	}
	cmd.AddCommand(newProjectMapCmd(auraDir, jsonOut))
	cmd.AddCommand(newProjectOverviewCmd(auraDir, jsonOut))
	return cmd
}

func newProjectMapCmd(auraDir *string, jsonOut *bool) *cobra.Command {
	var projectDir string
	cmd := &cobra.Command{
		Use:   "map",
		Short: "Map your project structure into Aura's memory",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := projectDir
			if dir == "" {
				dir = "."
			}

			result, err := codebase.Scan(dir)
			if err != nil {
				return fmt.Errorf("scan: %w", err)
			}

			store, close, err := openStore(*auraDir)
			if err != nil {
				return err
			}
			defer close()

			n, err := codebase.StoreResult(store, result, "")
			if err != nil {
				return err
			}

			if *jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]interface{}{
					"result":  result,
					"stored":  n,
				})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Aura mapped %s:\n", dir)
			fmt.Fprintf(cmd.OutOrStdout(), "  Languages:    %v\n", result.Languages)
			fmt.Fprintf(cmd.OutOrStdout(), "  Entry points: %v\n", result.EntryPoints)
			fmt.Fprintf(cmd.OutOrStdout(), "  Packages:     %d top-level dirs\n", len(result.Packages))
			fmt.Fprintf(cmd.OutOrStdout(), "  Dependencies: %d\n", len(result.Dependencies))
			fmt.Fprintf(cmd.OutOrStdout(), "  Files:        %d (%d lines)\n", result.FileCount, result.TotalLines)
			fmt.Fprintf(cmd.OutOrStdout(), "Stored %d context entries in Aura's memory.\n", n)
			return nil
		},
	}
	cmd.Flags().StringVar(&projectDir, "project-dir", "", "Project directory to scan (default: current directory)")
	return cmd
}

func newProjectOverviewCmd(auraDir *string, jsonOut *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "overview",
		Short: "Show Aura's understanding of your project",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, close, err := openStore(*auraDir)
			if err != nil {
				return err
			}
			defer close()

			keys := []string{
				"aura.project.languages",
				"aura.project.entry_points",
				"aura.project.packages",
				"aura.project.dependencies",
				"aura.project.stats",
			}

			info := make(map[string]string)
			for _, key := range keys {
				entry, err := store.Get(key)
				if err != nil {
					continue
				}
				info[key] = entry.Value
			}

			if *jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(info)
			}

			if len(info) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no project map found — run 'aura project map' first")
				return nil
			}

			labels := map[string]string{
				"aura.project.languages":    "Languages",
				"aura.project.entry_points": "Entry Points",
				"aura.project.packages":     "Packages",
				"aura.project.dependencies": "Dependencies",
				"aura.project.stats":        "Stats",
			}
			for _, key := range keys {
				if val, ok := info[key]; ok {
					fmt.Fprintf(cmd.OutOrStdout(), "%-15s %s\n", labels[key]+":", val)
				}
			}
			return nil
		},
	}
}

// openStoreForProject is a helper that returns a memory store (reuses openStore from memory.go).
func openStoreForProject(auraDir string) (*memory.Store, func(), error) {
	return openStore(auraDir)
}
