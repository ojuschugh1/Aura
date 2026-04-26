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
	cmd.AddCommand(newMemoryLinkCmd(auraDir, jsonOut))
	cmd.AddCommand(newMemoryUnlinkCmd(auraDir))
	cmd.AddCommand(newMemoryRelatedCmd(auraDir, jsonOut))
	cmd.AddCommand(newMemorySearchCmd(auraDir, jsonOut))
	cmd.AddCommand(newMemoryTagCmd(auraDir))
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
	var autoOnly bool
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List all memory entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, close, err := openStore(*auraDir)
			if err != nil {
				return err
			}
			defer close()

			filter := memory.ListFilter{Agent: agent}
			// --auto takes precedence over --agent.
			if autoOnly {
				filter.Agent = "auto-capture"
			}

			entries, err := store.List(filter)
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
			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-18s %-20s %s\n", "KEY", "SOURCE", "UPDATED", "VALUE")
			for _, e := range entries {
				source := e.SourceTool
				if e.SourceTool == "auto-capture" {
					source = e.SourceTool + " [auto]"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-18s %-20s %s\n",
					e.Key, source, e.UpdatedAt.Format("2006-01-02 15:04:05"), e.Value)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&agent, "agent", "", "Filter by source tool name")
	cmd.Flags().BoolVar(&autoOnly, "auto", false, "Show only auto-captured entries")
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

func newMemoryLinkCmd(auraDir *string, jsonOut *bool) *cobra.Command {
	var relation string
	var confidence float64
	cmd := &cobra.Command{
		Use:   "link <from> <to>",
		Short: "Create a relationship between two memory entries",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, close, err := openStore(*auraDir)
			if err != nil {
				return err
			}
			defer close()

			edge, err := store.AddEdge(args[0], args[1], relation, "cli", "", confidence)
			if err != nil {
				return err
			}
			if *jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(edge)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "linked: %s -[%s]-> %s (confidence: %.2f)\n",
				edge.FromKey, edge.Relation, edge.ToKey, edge.Confidence)
			return nil
		},
	}
	cmd.Flags().StringVar(&relation, "relation", "related-to", "Relationship type (depends-on, includes, related-to, contradicts, supersedes)")
	cmd.Flags().Float64Var(&confidence, "confidence", 1.0, "Confidence score (0.0-1.0)")
	return cmd
}

func newMemoryUnlinkCmd(auraDir *string) *cobra.Command {
	var relation string
	cmd := &cobra.Command{
		Use:   "unlink <from> <to>",
		Short: "Remove a relationship between two memory entries",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, close, err := openStore(*auraDir)
			if err != nil {
				return err
			}
			defer close()

			if err := store.DeleteEdge(args[0], args[1], relation); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "unlinked: %s -[%s]-> %s\n", args[0], relation, args[1])
			return nil
		},
	}
	cmd.Flags().StringVar(&relation, "relation", "related-to", "Relationship type to remove")
	return cmd
}

func newMemoryRelatedCmd(auraDir *string, jsonOut *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "related <key>",
		Short: "Show all entries connected to a key with their relationships",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, close, err := openStore(*auraDir)
			if err != nil {
				return err
			}
			defer close()

			edges, err := store.GetEdges(args[0])
			if err != nil {
				return err
			}
			entries, err := store.GetRelated(args[0])
			if err != nil {
				return err
			}

			if *jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]interface{}{
					"edges":   edges,
					"entries": entries,
				})
			}

			if len(edges) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no relationships found")
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Relationships for %q:\n", args[0])
			for _, e := range edges {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s -[%s]-> %s (confidence: %.2f)\n",
					e.FromKey, e.Relation, e.ToKey, e.Confidence)
			}

			if len(entries) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "\nConnected entries:\n")
				for _, e := range entries {
					val := e.Value
					if len(val) > 60 {
						val = val[:60] + "…"
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  %-30s %s\n", e.Key, val)
				}
			}
			return nil
		},
	}
}

func newMemorySearchCmd(auraDir *string, jsonOut *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "search <query>",
		Short: "Full-text search across all memory entries",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, close, err := openStore(*auraDir)
			if err != nil {
				return err
			}
			defer close()

			entries, err := store.Search(args[0])
			if err != nil {
				return err
			}

			if *jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(entries)
			}

			if len(entries) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no matches")
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-18s %s\n", "KEY", "SOURCE", "VALUE")
			for _, e := range entries {
				val := e.Value
				if len(val) > 50 {
					val = val[:50] + "…"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-18s %s\n", e.Key, e.SourceTool, val)
			}
			return nil
		},
	}
}

func newMemoryTagCmd(auraDir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "tag <key> <tag1> [tag2...]",
		Short: "Add tags to a memory entry",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, close, err := openStore(*auraDir)
			if err != nil {
				return err
			}
			defer close()

			entry, err := store.AddTags(args[0], args[1:])
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "tagged %s: %v\n", entry.Key, entry.Tags)
			return nil
		},
	}
}
