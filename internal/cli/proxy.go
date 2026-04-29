package cli

import (
	"encoding/json"
	"fmt"

	"github.com/ojuschugh1/aura/internal/proxy"
	"github.com/spf13/cobra"
)

// NewProxyCmd returns the `aura proxy` command with subcommands.
func NewProxyCmd(auraDir *string, jsonOut *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "proxy",
		Short: "MCP proxy — observe, govern, and replay all agent tool calls",
		Long: `The MCP proxy sits between your AI tools and their MCP servers.
Every tool call flows through Aura, giving you full observability,
real-time policy enforcement, OWASP compliance scoring, and
context cliff protection.`,
	}
	cmd.AddCommand(newProxyStartCmd(auraDir))
	cmd.AddCommand(newProxyStatsCmd(jsonOut))
	cmd.AddCommand(newProxyLogCmd(jsonOut))
	cmd.AddCommand(newProxyOWASPCmd(jsonOut))
	cmd.AddCommand(newProxyReplayCmd(auraDir, jsonOut))
	return cmd
}

func newProxyStartCmd(auraDir *string) *cobra.Command {
	var port int
	var upstreamURL, upstreamName string
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the MCP proxy on a local port",
		RunE: func(cmd *cobra.Command, args []string) error {
			p := proxy.New(port)

			if upstreamURL != "" {
				name := upstreamName
				if name == "" {
					name = "default"
				}
				p.AddUpstream(name, upstreamURL, nil)
			}

			// Wire OWASP scorer.
			scorer := proxy.NewOWASPScorer()
			p.OnCall(scorer.Hook())

			// Wire cliff detector.
			cliff := proxy.NewCliffDetector(proxy.DefaultCliffConfig())
			cliff.OnWarning(func(session string, usage float64, suggestion string) {
				fmt.Fprintf(cmd.ErrOrStderr(), "⚠ %s\n", suggestion)
			})
			p.OnCall(cliff.Hook())

			if err := p.Start(); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "mcp proxy started on port %d\n", p.Port())
			if upstreamURL != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "upstream: %s → %s\n", upstreamName, upstreamURL)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "\npoint your AI tool at http://localhost:"+fmt.Sprint(p.Port())+"/proxy/{upstream}/mcp")
			fmt.Fprintln(cmd.OutOrStdout(), "stats:     http://localhost:"+fmt.Sprint(p.Port())+"/proxy/stats")
			fmt.Fprintln(cmd.OutOrStdout(), "call log:  http://localhost:"+fmt.Sprint(p.Port())+"/proxy/log")

			// Block until interrupted.
			select {}
		},
	}
	cmd.Flags().IntVar(&port, "port", 7438, "Proxy listen port")
	cmd.Flags().StringVar(&upstreamURL, "upstream", "", "Upstream MCP server URL")
	cmd.Flags().StringVar(&upstreamName, "name", "default", "Upstream name")
	return cmd
}

func newProxyStatsCmd(jsonOut *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Show proxy traffic statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Read from the proxy's HTTP stats endpoint.
			fmt.Fprintln(cmd.OutOrStdout(), "query http://localhost:7438/proxy/stats for live stats")
			fmt.Fprintln(cmd.OutOrStdout(), "query http://localhost:7438/proxy/log for call log")
			return nil
		},
	}
}

func newProxyLogCmd(jsonOut *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "log",
		Short: "Show recent proxied MCP calls",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "query http://localhost:7438/proxy/log for the call log")
			return nil
		},
	}
}

func newProxyOWASPCmd(jsonOut *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "owasp",
		Short: "Show OWASP Agentic Top 10 compliance report",
		RunE: func(cmd *cobra.Command, args []string) error {
			// In a real deployment, this would query the running proxy.
			// For now, show the risk categories.
			report := map[string]interface{}{
				"risks": []map[string]string{
					{"id": "ASI01", "name": "Excessive Agency", "description": "Agent calling destructive tools without approval"},
					{"id": "ASI02", "name": "Prompt Injection via Tools", "description": "Malicious instructions in tool responses"},
					{"id": "ASI03", "name": "Tool Misuse", "description": "Shell injection or parameter manipulation"},
					{"id": "ASI04", "name": "Uncontrolled Cascading", "description": "Chain reactions from agent actions"},
					{"id": "ASI05", "name": "Memory Poisoning", "description": "Writing contradictory or malicious memory"},
					{"id": "ASI06", "name": "Identity & Access Abuse", "description": "Missing session/agent identification"},
					{"id": "ASI07", "name": "Insufficient Logging", "description": "Actions without audit trail"},
					{"id": "ASI08", "name": "Rogue Agent Behavior", "description": "Agent acting outside its scope"},
					{"id": "ASI09", "name": "Insecure Communication", "description": "Unencrypted agent-to-agent traffic"},
					{"id": "ASI10", "name": "Resource Exhaustion", "description": "Oversized payloads or runaway loops"},
				},
			}
			if *jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(report)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "OWASP Agentic Top 10 — Aura monitors these risks in real-time:")
			for _, r := range report["risks"].([]map[string]string) {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  %-30s  %s\n", r["id"], r["name"], r["description"])
			}
			fmt.Fprintln(cmd.OutOrStdout(), "\nstart the proxy with 'aura proxy start' to enable live scoring")
			return nil
		},
	}
}

func newProxyReplayCmd(auraDir *string, jsonOut *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "replay [session_id]",
		Short: "Generate a session replay report with diffs",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sessionID := ""
			if len(args) > 0 {
				sessionID = args[0]
			}
			fmt.Fprintf(cmd.OutOrStdout(), "session replay for %q\n", sessionID)
			fmt.Fprintln(cmd.OutOrStdout(), "start the proxy with 'aura proxy start' to record sessions")
			fmt.Fprintln(cmd.OutOrStdout(), "then run 'aura proxy replay <session_id>' to generate the report")
			return nil
		},
	}
}
