// Command aura is the entry point for the Aura AI Continuity OS daemon and CLI.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/ojuschugh1/aura/internal/cli"
	"github.com/ojuschugh1/aura/internal/daemon"
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
	// When the binary is forked as the daemon process (AURA_DAEMON=1),
	// bypass cobra entirely and run the daemon loop directly.
	// The forked process has no subcommand, so cobra would just print help.
	if daemon.IsDaemonProcess() {
		dir := os.Getenv("AURA_DIR")
		if dir == "" {
			home, _ := os.UserHomeDir()
			dir = filepath.Join(home, ".aura")
		}
		portEnv, _ := strconv.Atoi(os.Getenv("AURA_PORT"))
		if portEnv == 0 {
			portEnv = daemon.DefaultPort
		}
		sessID := os.Getenv("AURA_SESSION")
		if err := daemon.RecoverAndLog(dir, func() error {
			return daemon.RunDaemon(dir, portEnv, sessID)
		}); err != nil {
			fmt.Fprintf(os.Stderr, "daemon error: %v\n", err)
			os.Exit(1)
		}
		return
	}

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

	rootCmd.AddCommand(cli.NewWikiCmd(&auraDir, &jsonOut))

	rootCmd.AddCommand(cli.NewProxyCmd(&auraDir, &jsonOut))

	rootCmd.AddCommand(cli.NewProjectCmd(&auraDir, &jsonOut))

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


