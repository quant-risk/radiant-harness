package main

// mcp-serve (Light-only registration). The `radiant mcp serve`
// command is the single most important entry point in the Light
// binary — it boots the MCP server that any host agent (Claude Code,
// Cursor, Hermes, Codex, OpenCode, etc.) can connect to and drive
// the harness via JSON-RPC + sampling/createMessage.
//
// Why this is its own file:
//   - cmd_audit.go is //go:build !light_only (registers camada/evals/
//     release/audit which need HTTP LLM).
//   - We need mcp serve available in BOTH Light and Full binaries.
//   - So we extract just the mcpCmd / mcpServeCmd registration here.
//   - This file has no LLM HTTP dependency: it just calls runMCPServe
//     (which is in helpers.go, gated //go:build !light_only too — see
//     note below).
//
// Why runMCPServe still works in Light:
//   - Even though helpers.go is tagged !light_only, runMCPServe uses
//     the mcpDispatcher which uses SamplingBackend (untagged, in
//     internal/llm/sampling.go) — not HTTPBackend. Light uses
//     SamplingBackend only, which is exactly what mcp serve wants.
//   - For Light to compile, we copy the runMCPServe + handleMCPRequest
//     + callMCPTool bodies into mcp_serve_light.go (the actual MCP
//     server runtime that works with sampling).

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
		Short: "Start the MCP server (Light mode — MCP sampling, no API key)",
		Long: `Start the MCP server on stdio. The harness operates in
Light mode: it uses MCP sampling/createMessage to request LLM inference
from the calling agent (Claude Code, Hermes, Cursor, etc.). No API key
is required — the host agent pays for the inference.

This is one half of the Light/Full split. The other half (Full mode,
autonomous HTTP calls) lives in the regular subcommands:
  - radiant loop start
  - radiant run
  - radiant fleet start
  - radiant init / validate / etc.

Behaviour emerges from the subcommand. No --mode flag, no
RADIANT_MODE env, no mode: field in .radiant.yaml.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// mcp serve is always Light — sampling always on.
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
