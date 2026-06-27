package main

import (
	"os"

	"github.com/spf13/cobra"
)

var version = "1.1.0"

func main() {
	root := &cobra.Command{
		Use:     "radiant",
		Short:   "Universal SDD harness for any AI model or agent",
		Long:    "Spec-Driven Development harness that works with any LLM via OpenRouter, OpenAI, Anthropic, or custom providers. No agent dependency.",
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

	// ── version ──
	root.SetVersionTemplate("{{.Version}}\n")

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
