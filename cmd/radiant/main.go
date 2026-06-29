package main

import (
	"os"

	"github.com/spf13/cobra"
)

var version = "2.40.0"

func main() {
	root := &cobra.Command{
		Use:     "radiant",
		Short:   "Universal autonomous development harness for any LLM",
		Long:    "Vendor-neutral harness for autonomous LLM-driven development. Single binary, no external dependencies. Works with Claude, GPT-4o, Gemini, Mistral, OpenRouter, and local models.",
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
	registerModeCmd(root)
	registerPricingCmd(root)
	registerSemanticCmd(root)
	registerToolsCmd(root)

	root.SetVersionTemplate("{{.Version}}\n")

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
