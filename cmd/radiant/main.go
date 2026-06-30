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

var version = "3.7.1"

// publicCommands is the closed set of CLI subcommands that the Light
// binary exposes without RADIANT_INTERNAL=1.
//
// Anything that would otherwise be used by a host agent to bypass the
// MCP path lives in the internal complement and is gated by requireInternal().
//
// The commands listed here are safe for any caller — including a host
// agent or a fresh shell with no MCP wired — to invoke directly:
//
//   - **setup-mcp / doctor / host-info / mcp / update / test-case**:
//     install + diagnose + serve + drive a case end-to-end with
//     simulated sampling latency.
//   - **spec** (and adr, diagramar, product, integrations, views,
//     review-pr, setup-ci under it): scaffold spec.md / tasks.md /
//     architecture docs. Pure templates — no LLM, no API calls.
//   - **audit** (and camada-agentica, evals, release, security under
//     it): read-only project auditing. Lets the host agent verify
//     its own work without going through the MCP loop.
//   - **skills** (and boot under it): list bundled skills + emit the
//     project manifest. Lets the host agent discover methodology.
//   - **context** (and ontology under it): assemble + compress
//     CONTEXT.md. Lets the host agent narrow the project context.
//   - **loop / run / fleet** (v3.7.x): the harness engine on the CLI.
//     In Light mode the loop auto-routes to the self-driven scaffold
//     pipeline when no host agent is wired (no API keys needed, no
//     HTTP egress — the offline 4-phase pipeline produces a
//     populated state.json + specs/0001-<slug>/ tree the host can
//     then fill in). Gating these commands behind RADIANT_INTERNAL=1
//     prevented the canonical "use from any shell" path the README
//     documents from working — exposing them here is the v3.7.x
//     fix that lands the drop-in.
//   - **worktree / state / handoff / improve** (v3.7.x): low-risk
//     project-state helpers, also safe for direct invocation.
//
// Everything else (`eval`, `models`, `train`, `predict`, `evaluate`,
// `drift`, `model`, `profile`, `stats`, `causal-estimate`, `autodata`,
// `integrate`, `semantic`, `incident`, `bench`, `budget`,
// `tools`, `pricing`, `telemetry`) stays gated. Either they touch
// project state in ways that benefit from explicit operator
// opt-in, or they're legacy stubs.
//
// List rationale: a host agent (Codex / Claude Code / Hermes / …) that
// gets a task in a project with radiant-harness installed should be
// able to drive the full harness loop, audit its own work, load
// skills, and resume from any tool's POV without going through the
// MCP `radiant_possess` call. Read AGENTS-FOR-TASKS.md § MCP tools
// first; this list covers the case where the MCP wire isn't
// available yet (new sandbox, hostile env, sync TUI deadlock).
var publicCommands = map[string]bool{
	// install + diagnose + serve
	"setup-mcp": true, // wire MCP into a host agent
	"mcp":       true, // serve + self-test + possess + possess_async + run_gate + phase_status + skill_list/load
	"host-info": true, // show detected host agent
	"doctor":    true, // diagnose wiring + agent config
	"update":    true, // self-update the binary
	"test-case": true, // drive a real case with simulated sampling latency

	// SDD scaffolding (templates, no LLM)
	"spec": true, // radiant spec / adr / diagramar / product / integrations / views / review-pr / setup-ci

	// Verification (read-only)
	"audit": true, // radiant audit / camada-agentica / evals / release / security

	// Methodology discoverability
	"skills": true, // radiant skills list / validate / boot
	"context": true, // radiant context detect / assemble / compress / ontology

	// Harness engine on the CLI (the "use from any shell" path)
	"loop":      true, // radiant loop {start|status|resume|cancel|history|export|diff|trace|list} — Light auto-routes to self-driven when no host wired
	"run":       true, // radiant run <spec-dir> — one-shot harness run against a given spec
	"fleet":     true, // radiant fleet {start|status|dispatch|summary|resume} — multi-agent coordinator

	// Low-risk project-state helpers
	"worktree": true, // isolated git worktree management
	"state":    true, // show current session state (resume point)
	"handoff":  true, // pause + write session state to a handoff file
	"improve":  true, // self-improvement engine — analyse traces, propose skill edits
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