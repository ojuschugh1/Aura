package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ojuschugh1/aura/internal/verify"
	"github.com/spf13/cobra"
)

// NewVerifyCmd returns the `aura verify` command.
func NewVerifyCmd(jsonOut *bool) *cobra.Command {
	var sessionID string
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify agent claims against filesystem and git history",
		RunE: func(cmd *cobra.Command, args []string) error {
			transcriptPath, err := findTranscript(sessionID)
			if err != nil {
				return err
			}

			entries, err := verify.ParseJSONL(transcriptPath)
			if err != nil {
				return fmt.Errorf("parse transcript: %w", err)
			}

			claims := verify.ExtractClaims(entries)
			if len(claims) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no verifiable claims found in transcript")
				return nil
			}

			root, _ := os.Getwd()
			result := verify.Verify(claims, root)

			if *jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "claims:  %d total\n", result.TotalClaims)
			fmt.Fprintf(cmd.OutOrStdout(), "pass:    %d\n", result.PassCount)
			fmt.Fprintf(cmd.OutOrStdout(), "fail:    %d\n", result.FailCount)
			fmt.Fprintf(cmd.OutOrStdout(), "truth:   %.1f%%\n", result.TruthPct)
			fmt.Fprintln(cmd.OutOrStdout())
			for _, c := range result.Claims {
				status := "PASS"
				if !c.Pass {
					status = "FAIL"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s %s — %s\n", status, c.Type, c.Target, c.Detail)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&sessionID, "session", "", "Session ID to verify (default: most recent)")
	return cmd
}

// findTranscript locates a JSONL transcript file for the given session.
// If sessionID is empty, it looks for the most recent transcript in common locations.
func findTranscript(sessionID string) (string, error) {
	home, _ := os.UserHomeDir()
	cwd, _ := os.Getwd()

	// Common transcript directories for Claude Code and Cursor.
	searchDirs := []string{
		filepath.Join(home, ".claude", "projects"),
		filepath.Join(home, ".cursor", "logs"),
		filepath.Join(cwd, ".aura", "traces"),
	}

	for _, dir := range searchDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if filepath.Ext(name) != ".jsonl" {
				continue
			}
			if sessionID == "" || name == sessionID+".jsonl" {
				return filepath.Join(dir, name), nil
			}
		}
	}

	if sessionID != "" {
		return "", fmt.Errorf("no transcript found for session %q; check ~/.claude/projects/ or ~/.cursor/logs/", sessionID)
	}
	return "", fmt.Errorf("no transcript found; check ~/.claude/projects/ or ~/.cursor/logs/")
}
