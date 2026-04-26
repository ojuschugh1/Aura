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
		Short: "Codebase structure awareness",
	}
	cmd.AddCommand(newProjectScanCmd(auraDir, jsonOut))
	cmd.AddCommand(newProjectInfoCmd(auraDir, jsonOut))
	return cmd
}

func newProjectScanCmd(auraDir *string, jsonOut *bool) *cobra.Command {
	var projectDir string
	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Scan and store project structure",
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

			fmt.Fprintf(cmd.OutOrStdout(), "Scanned %s:\n", dir)
			fmt.Fprintf(cmd.OutOrStdout(), "  Languages:    %v\n", result.Languages)
			fmt.Fprintf(cmd.OutOrStdout(), "  Entry points: %v\n", result.EntryPoints)
			fmt.Fprintf(cmd.OutOrStdout(), "  Packages:     %d top-level dirs\n", len(result.Packages))
			fmt.Fprintf(cmd.OutOrStdout(), "  Dependencies: %d\n", len(result.Dependencies))
			fmt.Fprintf(cmd.OutOrStdout(), "  Files:        %d (%d lines)\n", result.FileCount, result.TotalLines)
			fmt.Fprintf(cmd.OutOrStdout(), "Stored %d memory entries.\n", n)
			return nil
		},
	}
	cmd.Flags().StringVar(&projectDir, "project-dir", "", "Project directory to scan (default: current directory)")
	return cmd
}

func newProjectInfoCmd(auraDir *string, jsonOut *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Show stored project structure",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, close, err := openStore(*auraDir)
			if err != nil {
				return err
			}
			defer close()

			keys := []string{
				"codebase.languages",
				"codebase.entry_points",
				"codebase.packages",
				"codebase.dependencies",
				"codebase.stats",
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
				fmt.Fprintln(cmd.OutOrStdout(), "no project info stored; run 'aura project scan' first")
				return nil
			}

			labels := map[string]string{
				"codebase.languages":    "Languages",
				"codebase.entry_points": "Entry Points",
				"codebase.packages":     "Packages",
				"codebase.dependencies": "Dependencies",
				"codebase.stats":        "Stats",
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
