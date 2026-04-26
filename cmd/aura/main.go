// Command aura is the entry point for the Aura AI Continuity OS daemon and CLI.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ojuschugh1/aura/internal/cli"
	"github.com/spf13/cobra"
)

var (
	auraDir string
	jsonOut bool
)

var rootCmd = &cobra.Command{
	Use:   "aura",
	Short: "Aura — AI Continuity OS",
	Long:  "Aura is a local-first daemon and MCP server for persistent AI memory, verification, and governance.",
	// Print usage hint on error, but not on --help.
	SilenceUsage: true,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		fmt.Fprintf(os.Stderr, "run 'aura --help' for usage\n")
		os.Exit(1)
	}
}

func init() {
	home, _ := os.UserHomeDir()
	defaultDir := filepath.Join(home, ".aura")

	rootCmd.PersistentFlags().StringVar(&auraDir, "dir", defaultDir, "Aura data directory")
	rootCmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "Output as JSON")

	rootCmd.AddCommand(cli.NewInitCmd(&auraDir))
	rootCmd.AddCommand(cli.NewStatusCmd(&auraDir, &jsonOut))
	rootCmd.AddCommand(cli.NewStopCmd(&auraDir))

	rootCmd.AddCommand(cli.NewMemoryCmd(&auraDir, &jsonOut))

	// Placeholder commands — wired up in later tasks.
	rootCmd.AddCommand(cli.NewVerifyCmd(&jsonOut))
	rootCmd.AddCommand(cli.NewCompactCmd(&auraDir, &jsonOut))
	rootCmd.AddCommand(cli.NewCostCmd(&auraDir, &jsonOut))
	rootCmd.AddCommand(cli.NewScanCmd(&jsonOut))
	rootCmd.AddCommand(cli.NewTrustCmd())
	rootCmd.AddCommand(cli.NewSetupCmd(&auraDir))
	rootCmd.AddCommand(cli.NewReplayCmd(&auraDir, &jsonOut))
	rootCmd.AddCommand(cli.NewTraceCmd(&auraDir, &jsonOut))

	rootCmd.AddCommand(cli.NewHelpCmd(rootCmd))
	rootCmd.AddCommand(cli.NewVersionCmd(&jsonOut))
	rootCmd.AddCommand(cli.NewCompletionCmd(rootCmd))
}

func placeholderCmd(use, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "aura %s: not yet implemented\n", use)
			return nil
		},
	}
}


