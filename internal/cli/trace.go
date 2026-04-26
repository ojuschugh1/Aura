package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ojuschugh1/aura/internal/db"
	"github.com/ojuschugh1/aura/internal/trace"
	"github.com/spf13/cobra"
)

// NewTraceCmd returns the `aura trace` command with subcommands.
func NewTraceCmd(auraDir *string, jsonOut *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trace",
		Short: "Manage session traces",
	}
	tracesDir := func() string { return filepath.Join(*auraDir, "traces") }

	cmd.AddCommand(newTraceLastCmd(auraDir, jsonOut, tracesDir))
	cmd.AddCommand(newTraceShowCmd(auraDir, jsonOut, tracesDir))
	cmd.AddCommand(newTraceSearchCmd(jsonOut, tracesDir))
	cmd.AddCommand(newTraceExportCmd(tracesDir))
	cmd.AddCommand(newTracePinCmd(auraDir, tracesDir))
	return cmd
}

func newTraceLastCmd(auraDir *string, jsonOut *bool, tracesDir func() string) *cobra.Command {
	return &cobra.Command{
		Use:   "last",
		Short: "Show the most recent trace summary",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := tracesDir()
			entries, err := os.ReadDir(dir)
			if err != nil {
				return fmt.Errorf("no traces found in %s", dir)
			}
			// Find the most recent .jsonl file.
			var latest os.DirEntry
			for _, e := range entries {
				if !e.IsDir() && filepath.Ext(e.Name()) == ".jsonl" {
					if latest == nil {
						latest = e
					} else {
						li, _ := latest.Info()
						ei, _ := e.Info()
						if ei != nil && li != nil && ei.ModTime().After(li.ModTime()) {
							latest = e
						}
					}
				}
			}
			if latest == nil {
				fmt.Fprintln(cmd.OutOrStdout(), "no traces found")
				return nil
			}
			sessionID := latest.Name()[:len(latest.Name())-6]
			info, _ := latest.Info()
			if *jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]interface{}{
					"session_id": sessionID,
					"size_bytes": info.Size(),
					"modified":   info.ModTime(),
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "session: %s\nsize:    %d bytes\n", sessionID, info.Size())
			return nil
		},
	}
}

func newTraceShowCmd(auraDir *string, jsonOut *bool, tracesDir func() string) *cobra.Command {
	return &cobra.Command{
		Use:   "show <session_id>",
		Short: "Show the full trace for a session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := filepath.Join(tracesDir(), args[0]+".jsonl")
			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("trace not found for session %s", args[0])
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}
}

func newTraceSearchCmd(jsonOut *bool, tracesDir func() string) *cobra.Command {
	return &cobra.Command{
		Use:   "search <query>",
		Short: "Search across stored traces",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			results, err := trace.Search(tracesDir(), args[0])
			if err != nil {
				return err
			}
			if *jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(results)
			}
			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no matches")
				return nil
			}
			for _, r := range results {
				fmt.Fprintln(cmd.OutOrStdout(), r.SessionID)
			}
			return nil
		},
	}
}

func newTraceExportCmd(tracesDir func() string) *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "export <session_id>",
		Short: "Export a trace in JSON or HTML format",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			outPath := args[0] + "." + format
			if err := trace.Export(tracesDir(), args[0], outPath, format); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "exported to %s\n", outPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "json", "Output format: json or html")
	return cmd
}

func newTracePinCmd(auraDir *string, tracesDir func() string) *cobra.Command {
	return &cobra.Command{
		Use:   "pin <session_id>",
		Short: "Pin a trace to prevent pruning",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			database, err := db.Open(filepath.Join(*auraDir, "aura.db"))
			if err != nil {
				return err
			}
			defer database.Close()
			if err := trace.Pin(database, tracesDir(), args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "pinned: %s\n", args[0])
			return nil
		},
	}
}
