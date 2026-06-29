//go:build !light_only

// Full build entrypoint. Has every command, including the HTTP LLM
// layer (Anthropic native, OpenAI-compatible via OpenRouter, etc.).
// Requires an API key (env var or .radiant.yaml) for any LLM-driven
// subcommand.
//
// Subcommands registered here: everything the Light build registers,
// PLUS run, loop, fleet, scaffolds, audit sub-commands, full doctor,
// spec, eval, etc.
//
// To build: `go build -o /tmp/radiant-full ./cmd/radiant`  (default)

package main

import (
	"os"

	"github.com/spf13/cobra"
)

var version = "2.49.0"

func main() {
	root := &cobra.Command{
		Use:     "radiant",
		Short:   "Universal autonomous development harness for any LLM",
		Long:    "Vendor-neutral harness for autonomous LLM-driven development. Single binary, no external dependencies. Works with Claude, GPT-4o, Gemini, Mistral, OpenRouter, and local models. Inference via HTTP LLM provider (set API key) or via MCP sampling from a host agent.",
		Version: version,
	}

	registerRunCmds(root)
	registerSpecCmds(root)
	registerAuditCmds(root)
	registerTelemetryCmds(root)
	registerOpsCmds(root)
	registerSessionCmds(root)
	registerSkillsCmds(root)
	registerContextCmds(root)
	registerFleetCmds(root)
	registerLoopCmds(root)
	registerDoctorCmd(root)
	registerSetupMCPCmd(root)
	registerPricingCmd(root)
	registerSemanticCmd(root)
	registerToolsCmd(root)
	registerHostInfoCmd(root) // show detected host agent (Sprint 79)

	root.SetVersionTemplate("{{.Version}}\n")

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
