//go:build light_only

// This is the Light build entrypoint. Light means: NO HTTP LLM backend,
// NO API key infrastructure. The Light binary can only do work via the
// host agent (MCP sampling). It is meant for users who want to run
// the harness without bringing their own LLM credentials.
//
// Subcommands registered here (Light subset):
//   - setup-mcp   (vendor-neutral agent config writes — 11 agents)
//   - mcp serve   (the actual Light path — MCP sampling)
//
// To build: `go build -tags light_only -o /tmp/radiant-light ./cmd/radiant`
//
// For the Full build (every subcommand, including HTTP LLM providers
// like OpenRouter/OpenAI/Anthropic, requires API key): see main_full.go.

package main

import (
	"os"

	"github.com/spf13/cobra"
)

var version = "2.48.0-light"

func main() {
	root := &cobra.Command{
		Use:     "radiant",
		Short:   "Universal autonomous development harness for any LLM (light build)",
		Long:    "Light build of the harness. Inference comes exclusively from the host agent via MCP sampling/createMessage. No API key required, no HTTP LLM backend included. For the Full build (HTTP LLM providers + API key infrastructure), use the default build.",
		Version: version,
	}

	// Light command set: only MCP-related subcommands.
	registerSetupMCPCmd(root) // 11 agents, vendor-neutral config writes
	registerMCPServeCmd(root) // the MCP server itself (Light by definition)

	root.SetVersionTemplate("{{.Version}}\n")

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
