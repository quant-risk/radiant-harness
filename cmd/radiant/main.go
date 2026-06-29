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
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "3.5.0"

// publicCommands is the closed set of CLI subcommands that the Light
// binary exposes without RADIANT_INTERNAL=1. Anything that the host agent
// would otherwise use to bypass the MCP path (scaffold, scaffold-run
// helpers, audit, telemetry, validate, etc.) lives in the internal
// complement and is gated by requireInternal().
//
// List rationale: these are the commands an AI agent (or a human) needs
// to install + diagnose the harness. The MCP-driven loop, possession,
// and skill loading are exposed as MCP tools and as such do not need a
// CLI counterpart under a public subcommand.
var publicCommands = map[string]bool{
	"setup-mcp": true, // wire MCP into a host agent
	"mcp":       true, // serve + self-test + possess
	"host-info": true, // show detected host agent
	"doctor":    true, // diagnose wiring + agent config
	"update":    true, // self-update the binary
	"test-case": true, // drive a real case with simulated sampling latency
}

func main() {
	root := &cobra.Command{
		Use:     "radiant",
		Short:   "Self-driving dev harness for any MCP-compatible agent",
		Long:    "radiant is a verifiable, budgeted, persistent dev harness. Inference comes from the host agent via MCP sampling/createMessage — no API key, no HTTP LLM client. Wire it into your agent with `radiant setup-mcp`, then ask it to ship something. Use `radiant loop` / `radiant run` / `radiant fleet` from any shell.",
		Version: version,
	}

	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		name := effectiveTopLevelName(cmd)
		if internalEnabled() || publicCommands[name] {
			return nil
		}
		fmt.Fprintf(os.Stderr,
			"radiant %q is an internal helper. "+
				"To run the harness, the host agent must invoke the MCP tool\n"+
				"  mcp__radiant__possess(task=\"<the user's prompt>\", workdir=\"<cwd>\")\n"+
				"after `radiant setup-mcp --agent=<host>` + restart.\n"+
				"\n"+
				"If you really need this command outside the harness, set\n"+
				"  RADIANT_INTERNAL=1 radiant %s …\n",
			name, name)
		return fmt.Errorf("command gated by possession contract (RADIANT_INTERNAL=1 to override)")
	}

	// Core MCP commands
	registerSetupMCPCmd(root) // 12 agents, vendor-neutral config writes
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

	// Diagnostics: test-case runs the harness against a real case with
	// simulated sampling latency.
	registerTestCaseCmd(root)

	root.SetVersionTemplate("{{.Version}}\n")

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

// internalEnabled reports whether the caller has explicitly opted into
// the internal command surface by setting RADIANT_INTERNAL=1. The public
// surface (`setup-mcp`, `mcp`, `host-info`, `doctor`, `update`, plus
// their subcommands) is always available.
func internalEnabled() bool {
	v := os.Getenv("RADIANT_INTERNAL")
	return v == "1" || v == "true" || v == "yes"
}

// effectiveTopLevelName returns the first non-root name in the command
// path. So `radiant loop start` and `radiant mcp serve` both report the
// top-level subcommand ("loop" or "mcp"), which is what the gate logic
// keys off. The root command itself (`radiant --version`, `radiant help`)
// is treated as "version" — always allowed.
func effectiveTopLevelName(cmd *cobra.Command) string {
	root := cmd.Root()
	if cmd == root {
		return "version"
	}
	c := cmd
	for c.Parent() != nil && c.Parent() != root {
		c = c.Parent()
	}
	// c is now either a top-level subcommand or `root` itself.
	if c == root || c.Parent() == root {
		if c == root {
			return "version"
		}
		return c.Name()
	}
	return ""
}