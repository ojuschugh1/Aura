package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/ojuschugh1/aura/internal/db"
	"github.com/ojuschugh1/aura/internal/memory"
	"github.com/spf13/cobra"
)

// NewMemoryCmd returns the `aura memory` command with all subcommands.
func NewMemoryCmd(auraDir *string, jsonOut *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "memory",
		Short: "Manage persistent cross-tool memory",
	}
	cmd.AddCommand(newMemoryAddCmd(auraDir, jsonOut))
	cmd.AddCommand(newMemoryGetCmd(auraDir, jsonOut))
	cmd.AddCommand(newMemoryLsCmd(auraDir, jsonOut))
	cmd.AddCommand(newMemoryRmCmd(auraDir))
	cmd.AddCommand(newMemoryExportCmd(auraDir))
	cmd.AddCommand(newMemoryImportCmd(auraDir))
	return cmd
}

func openStore(auraDir string) (*memory.Store, func(), error) {
	dbPath := filepath.Join(auraDir, "aura.db")
	database, err := db.Open(dbPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open db: %w", err)
	}
	if err := db.RunMigrations(database); err != nil {
		database.Close()
		return nil, nil, fmt.Errorf("migrate db: %w", err)
	}
	return memory.New(database), func() { database.Close() }, nil
}

func newMemoryAddCmd(auraDir *string, jsonOut *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "add <key> <value>",
		Short: "Add or update a memory entry",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, close, err := openStore(*auraDir)
			if err != nil {
				return err
			}
			defer close()

			entry, err := store.Add(args[0], args[1], "cli", "")
			if err != nil {
				return err
			}
			if *jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(entry)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "stored: %s\n", entry.Key)
			return nil
		},
	}
}

func newMemoryGetCmd(auraDir *string, jsonOut *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Retrieve a memory entry by key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, close, err := openStore(*auraDir)
			if err != nil {
				return err
			}
			defer close()

			entry, err := store.Get(args[0])
			if err != nil {
				return err
			}
			if *jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(entry)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "key:     %s\nvalue:   %s\nsource:  %s\nupdated: %s\n",
				entry.Key, entry.Value, entry.SourceTool, entry.UpdatedAt.Format("2006-01-02 15:04:05"))
			return nil
		},
	}
}

func newMemoryLsCmd(auraDir *string, jsonOut *bool) *cobra.Command {
	var agent string
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List all memory entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, close, err := openStore(*auraDir)
			if err != nil {
				return err
			}
			defer close()

			entries, err := store.List(memory.ListFilter{Agent: agent})
			if err != nil {
				return err
			}
			if *jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(entries)
			}
			if len(entries) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no entries")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-12s %-20s %s\n", "KEY", "SOURCE", "UPDATED", "VALUE")
			for _, e := range entries {
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-12s %-20s %s\n",
					e.Key, e.SourceTool, e.UpdatedAt.Format("2006-01-02 15:04:05"), e.Value)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&agent, "agent", "", "Filter by source tool name")
	return cmd
}

func newMemoryRmCmd(auraDir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "rm <key>",
		Short: "Delete a memory entry by key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, close, err := openStore(*auraDir)
			if err != nil {
				return err
			}
			defer close()

			if err := store.Delete(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted: %s\n", args[0])
			return nil
		},
	}
}

func newMemoryExportCmd(auraDir *string) *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export memory entries to JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, close, err := openStore(*auraDir)
			if err != nil {
				return err
			}
			defer close()

			if file == "" {
				file = filepath.Join(*auraDir, "memory_export.json")
			}
			if err := store.Export(file); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "exported to %s\n", file)
			return nil
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "Output file path (default: .aura/memory_export.json)")
	return cmd
}

func newMemoryImportCmd(auraDir *string) *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import memory entries from JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, close, err := openStore(*auraDir)
			if err != nil {
				return err
			}
			defer close()

			if file == "" {
				file = filepath.Join(*auraDir, "memory_export.json")
			}
			n, err := store.Import(file)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "imported %d entries from %s\n", n, file)
			return nil
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "Input file path (default: .aura/memory_export.json)")
	return cmd
}
