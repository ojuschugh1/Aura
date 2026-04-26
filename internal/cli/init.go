package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/ojuschugh1/aura/internal/daemon"
	"github.com/ojuschugh1/aura/internal/subprocess"
	"github.com/spf13/cobra"
)

var requiredBinaries = []string{"sqz", "claimcheck", "ghostdep"}

// NewInitCmd returns the `aura init` command.
func NewInitCmd(auraDir *string) *cobra.Command {
	var port int
	var installDeps, skipDeps bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Start the Aura daemon and initialize .aura/",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := *auraDir

			// If we are the daemon process, run the daemon loop.
			if daemon.IsDaemonProcess() {
				sessID := os.Getenv("AURA_SESSION")
				portEnv, _ := strconv.Atoi(os.Getenv("AURA_PORT"))
				if portEnv == 0 {
					portEnv = daemon.DefaultPort
				}
				return daemon.RecoverAndLog(dir, func() error {
					return daemon.RunDaemon(dir, portEnv, sessID)
				})
			}

			// Check if already running.
			si, err := daemon.Status(dir)
			if err == nil && si.Running {
				fmt.Fprintf(cmd.OutOrStdout(), "daemon already running (pid %d, port %d)\n", si.PID, si.Port)
				return nil
			}

			// Dependency check.
			if !skipDeps {
				checkDeps(installDeps)
			}

			if err := daemon.Start(dir, port); err != nil {
				return fmt.Errorf("start daemon: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "aura daemon started (port %d, dir %s)\n", port, filepath.Clean(dir))
			return nil
		},
	}
	cmd.Flags().IntVar(&port, "port", daemon.DefaultPort, "MCP server port")
	cmd.Flags().BoolVar(&installDeps, "install-deps", false, "Download missing Rust binaries without prompting")
	cmd.Flags().BoolVar(&skipDeps, "skip-deps", false, "Skip dependency check")
	return cmd
}

// checkDeps reports the status of each required binary and optionally downloads missing ones.
func checkDeps(install bool) {
	fmt.Println("Dependencies:")
	for _, name := range requiredBinaries {
		path, err := subprocess.ResolveBinary(name)
		if err == nil {
			fmt.Printf("  %s ✓ (%s)\n", name, path)
			continue
		}
		if install {
			fmt.Printf("  %s ✗ (not found) — downloading...\n", name)
			if p, err := subprocess.Download(name, ""); err == nil {
				fmt.Printf("  %s ✓ (%s)\n", name, p)
			} else {
				fmt.Printf("  %s ✗ (download failed: %v)\n", name, err)
			}
		} else {
			fmt.Printf("  %s ✗ (not found)\n", name)
		}
	}
}
