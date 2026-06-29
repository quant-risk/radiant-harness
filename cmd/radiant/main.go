// radiant entrypoint. Inference comes exclusively from the host agent via
// MCP sampling/createMessage — no API key, no HTTP LLM client.
//
// Every subcommand registered here. LLM-driven subcommands (loop, run,
// fleet, spec, validate, evals, audit, improve, eval, autodata, semantic)
// delegate inference to the host agent via MCP sampling.
//
// The 16 register functions below add the 54 commands total — many of
// them are registered inside the functions themselves (e.g. registerSpecCmds
// also adds adr, diagramar, product, integrations, views, review-pr,
// setup-ci; registerAuditCmds also adds camada-agentica, evals, release,
// audit; registerRunCmds also adds init, validate, bench, config, models,
// eval; registerTelemetryCmds also adds stats, causal-estimate, model,
// predict, train, evaluate, drift, autodata, validate-file, profile,
// incident; registerSkillsCmds also adds boot; registerFleetCmds also
// adds worktree, improve, budget; registerSessionCmds also adds state,
// handoff; registerLoopCmds also adds trace).

package main

import (
	"os"

	"github.com/spf13/cobra"
)

var version = "3.2.8"

func main() {
	root := &cobra.Command{
		Use:     "radiant",
		Short:   "Self-driving dev harness for any MCP-compatible agent",
		Long:    "radiant is a verifiable, budgeted, persistent dev harness. Inference comes from the host agent via MCP sampling/createMessage — no API key, no HTTP LLM client. Wire it into your agent with `radiant setup-mcp`, then ask it to ship something. Use `radiant loop` / `radiant run` / `radiant fleet` from any shell.",
		Version: version,
	}

	// Core MCP commands
	registerSetupMCPCmd(root) // 11 agents, vendor-neutral config writes
	registerMCPServeCmd(root) // the MCP server itself
	registerHostInfoCmd(root) // show detected host agent

	// Loop engine
	registerLoopCmds(root)  // radiant loop (start, status, resume, …) + trace
	registerRunCmds(root)   // radiant run, init, validate, bench, config, models, eval
	registerFleetCmds(root) // radiant fleet + worktree + improve + budget

	// SDD scaffolding
	registerSpecCmds(root) // radiant spec, adr, diagramar, product, integrations, views, review-pr, setup-ci

	// Verification & quality
	registerAuditCmds(root)   // radiant audit, camada-agentica, evals, release, security
	registerSemanticCmd(root) // radiant semantic

	// Skills management
	registerSkillsCmds(root) // radiant skills, boot
	registerContextCmds(root) // radiant context + ontology

	// Diagnostics & session
	registerDoctorCmd(root)    // radiant doctor
	registerSessionCmds(root)  // radiant state, handoff
	registerOpsCmds(root)      // radiant update
	registerToolsCmd(root)     // radiant tools
	registerPricingCmd(root)   // radiant pricing (read-only, no HTTP fetch)

	// Vertical scaffolds (use host agent via MCP sampling for content gen)
	registerTelemetryCmds(root) // radiant telemetry + stats + causal-estimate + model + predict + train + evaluate + drift + autodata + validate-file + profile + incident

	root.SetVersionTemplate("{{.Version}}\n")

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}