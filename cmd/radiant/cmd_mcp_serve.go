package main

// `radiant mcp serve` boots the MCP server that any host agent (Claude Code,
// Cursor, Hermes, Codex, OpenCode, etc.) can connect to and drive the harness
// via JSON-RPC + sampling/createMessage. Inference comes exclusively from
// the host agent — radiant never opens an HTTP connection to an LLM provider.

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// isCharDevice returns true if fd is a character device (TTY).
// Used to warn when `radiant mcp serve` is invoked from a terminal
// instead of from an MCP host.
func isCharDevice(fd *os.File) bool {
	info, err := fd.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func registerMCPServeCmd(root *cobra.Command) {
	mcpCmd := &cobra.Command{
		Use:   "mcp",
		Short: "MCP server commands",
	}

	mcpServeCmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the MCP server on stdio (sampling/createMessage to host agent)",
		Long: `Start the MCP server on stdio. Every LLM call is routed back
to the calling agent via MCP sampling/createMessage — Claude Code,
Hermes, Cursor, etc. The host agent pays for inference; radiant never
needs an API key.

Wire it into your agent with 'radiant setup-mcp', restart the agent,
and any prompt that calls 'radiant_run' will drive the loop.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Sanity-check: if stdin is a TTY, the operator probably
			// ran this from a terminal by accident. Warn but don't
			// refuse — the MCP server can be useful for debugging.
			if isCharDevice(os.Stdin) {
				fmt.Fprintln(os.Stderr,
					"warning: radiant mcp serve is intended to be invoked "+
						"by an MCP host (e.g. Claude Code). Running from a "+
						"terminal with a TTY stdin won't receive any JSON-RPC "+
						"requests and will exit immediately.")
			}
			return runMCPServe(os.Stdin, os.Stdout, true)
		},
	}
	mcpCmd.AddCommand(mcpServeCmd)
	root.AddCommand(mcpCmd)
}
