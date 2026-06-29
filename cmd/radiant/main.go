
// radiant entrypoint. Inference comes exclusively from the host agent via
// MCP sampling/createMessage — no API key, no HTTP LLM client.
//
// Subcommands registered here:
//   - setup-mcp   (vendor-neutral agent config writes — 11 agents)
//   - mcp serve   (the MCP server: routes tool calls to the host agent)
//   - host-info   (detect which agent, if any, is invoking radiant)

package main

import (
	"os"

	"github.com/spf13/cobra"
)

var version = "3.0.0"

func main() {
	root := &cobra.Command{
		Use:     "radiant",
		Short:   "Self-driving loops for any LLM agent",
		Long:    "radiant turns any MCP-compatible agent into a verifiable, budgeted, persistent dev loop. Inference comes from the host agent via MCP sampling/createMessage — no API key needed. Wire it into your agent with `radiant setup-mcp`, then ask it to ship something.",
		Version: version,
	}

	// Command set: setup-mcp, mcp serve, host-info.
	registerSetupMCPCmd(root) // 11 agents, vendor-neutral config writes
	registerMCPServeCmd(root) // the MCP server itself
	registerHostInfoCmd(root) // show detected host agent

	root.SetVersionTemplate("{{.Version}}\n")

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
