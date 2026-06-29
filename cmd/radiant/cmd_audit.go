//go:build !light_only

package main

import (
	"github.com/spf13/cobra"
)

func registerAuditCmds(root *cobra.Command) {
	// ── camada-agentica (Sprint 13.4 — agentic layer audit) ──
	// `radiant camada-agentica` audits the project's agentic layer:
	// AGENTS.md presence + completeness, native views consistency,
	// version drift between AGENTS.md and bundled skills. Per the
	// camada-agentica skill, this is the "check" half; the
	// "generate" half is `radiant init --agent=<list>` and
	// `radiant update`.
	camadaCmd := &cobra.Command{
		Use:   "camada-agentica",
		Short: "Audit the agentic layer: AGENTS.md, native views, version drift",
		RunE: func(cmd *cobra.Command, args []string) error {
			agentFlag, _ := cmd.Flags().GetString("agents")
			fix, _ := cmd.Flags().GetBool("fix")
			return runCamadaAgentica(agentFlag, fix)
		},
	}
	camadaCmd.Flags().String("agents", "", "comma-separated agents in use (claude,codex,cursor,copilot,gemini,windsurf); default = empty (AGENTS.md only)")
	camadaCmd.Flags().Bool("fix", false, "regenerate AGENTS.md from current bundled skills (does NOT overwrite native views)")
	root.AddCommand(camadaCmd)

	// ── evals (Sprint 13.5 — AC→test coverage metrics) ──
	// `radiant evals [--scope=all|since-last-release|<spec-path>]
	// [--output=...]` walks specs/, parses ACs from each spec.md,
	// reads tasks.md coverage claims, and produces
	// `docs/evals-report.md` with per-feature fidelity scores.
	// The MVP computes "claimed coverage" (does tasks.md list this
	// AC?). The LLM (via the evals skill) does the real verification
	// (does the test actually pass + does it cover the AC's
	// Given/When/Then?).
	evalsCmd := &cobra.Command{
		Use:   "evals",
		Short: "Measure AC→test coverage (fidelity) across all specs",
		RunE: func(cmd *cobra.Command, args []string) error {
			scope, _ := cmd.Flags().GetString("scope")
			outPath, _ := cmd.Flags().GetString("output")
			if outPath == "" {
				outPath = "docs/evals-report.md"
			}
			return runEvals(scope, outPath)
		},
	}
	evalsCmd.Flags().String("scope", "all", "scope: all | since-last-release | <spec-path>")
	evalsCmd.Flags().StringP("output", "o", "", "output path (default: docs/evals-report.md)")
	root.AddCommand(evalsCmd)

	// ── release (Sprint 14 — first-class release command) ──
	// `radiant release v0.X.Y [--dry-run] [--skip-tests]
	// [--skip-cross-compile] [--skip-tag]` runs the full release
	// pipeline: pre-flight (clean tree) → version bump → tests →
	// cross-compile → commit → git tag. Composes everything we
	// built in the methodology merge into one operation.
	//
	// Safety: refuses to run on a dirty working tree. Refuses to
	// overwrite an existing tag of the same name. The user must
	// have a clean main + the version they want to cut.
	releaseCmd := &cobra.Command{
		Use:   "release <version>",
		Short: "Cut a release: version bump + tests + cross-compile + commit + git tag",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			version := args[0]
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			skipTests, _ := cmd.Flags().GetBool("skip-tests")
			skipCrossCompile, _ := cmd.Flags().GetBool("skip-cross-compile")
			skipTag, _ := cmd.Flags().GetBool("skip-tag")
			skipCommit, _ := cmd.Flags().GetBool("skip-commit")
			interactive, _ := cmd.Flags().GetBool("interactive")
			return runRelease(version, dryRun, skipTests, skipCrossCompile, skipTag, skipCommit, interactive)
		},
	}
	releaseCmd.Flags().Bool("dry-run", false, "show what would happen without writing/tagging anything")
	releaseCmd.Flags().Bool("skip-tests", false, "skip the test step (use only when you've already validated)")
	releaseCmd.Flags().Bool("skip-cross-compile", false, "skip the cross-compile step")
	releaseCmd.Flags().Bool("skip-tag", false, "skip the git tag step (only bump version + commit)")
	releaseCmd.Flags().Bool("skip-commit", false, "skip the git commit step (only bump version)")
	releaseCmd.Flags().Bool("interactive", false, "prompt for confirmation before commit/tag (skipped automatically in non-tty mode)")
	root.AddCommand(releaseCmd)

	// ── audit (Sprint 14.2 — project layout audit CLI) ──
	// `radiant audit [--scope=full|docs|specs|adrs] [--output=...]`
	// runs the project-wide conformity check from the `auditar`
	// skill as a CLI. MVP scope: AC traceability (every AC has
	// ≥1 task, every task has ≥1 AC), spec frontmatter validity,
	// ADR status validity. Returns non-zero if any errors found.
	auditCmd := &cobra.Command{
		Use:   "audit",
		Short: "Run the auditar skill: project layout, AC traceability, ADR validity",
		RunE: func(cmd *cobra.Command, args []string) error {
			scope, _ := cmd.Flags().GetString("scope")
			outPath, _ := cmd.Flags().GetString("output")
			failOnWarn, _ := cmd.Flags().GetBool("fail-on-warning")
			return runAudit(scope, outPath, failOnWarn)
		},
	}
	auditCmd.Flags().String("scope", "full", "audit scope: full | docs | specs | adrs")
	auditCmd.Flags().StringP("output", "o", "", "output path (default: docs/audit-report.md)")
	auditCmd.Flags().Bool("fail-on-warning", false, "exit non-zero on warnings (default: only errors)")
	root.AddCommand(auditCmd)

	// ── security (Sprint 16 — security posture audit) ──
	// Implementation moved to cmd_security.go in Sprint 74. This
	// registration just wires the cobra command into the root tree.
	registerSecurityCmd(root)

}
