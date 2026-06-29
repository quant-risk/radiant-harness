//go:build with_full

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/quant-risk/radiant-harness/internal/scaffold"
	"github.com/spf13/cobra"
)

func registerSpecCmds(root *cobra.Command) {
	// ── spec (Sprint 10 third batch — interview + AC→test pré-check) ──
	// `radiant spec "<intent>" --tier=feature --ac=... --task=... --gate=...`
	//
	// Non-interactive mode (flag-driven). The interactive interview
	// lives in `nova-feature` SKILL.md — agents can run that. The CLI
	// version is for power users who already know what they want.
	//
	// Pré-check: every AC must be matched to at least one task, and
	// every task must have a gate command. This is the lesson from
	// video #1: TLC won the benchmark by forcing AC→test mapping.
	specCmd := &cobra.Command{
		Use:   "spec <intent>",
		Short: "Create spec.md + tasks.md for a new feature (tier-driven, AC→test mapping)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			intent := args[0]
			tier, _ := cmd.Flags().GetString("tier")
			slug, _ := cmd.Flags().GetString("slug")
			acsRaw, _ := cmd.Flags().GetStringArray("ac")
			tasksRaw, _ := cmd.Flags().GetStringArray("task")
			gatesRaw, _ := cmd.Flags().GetStringArray("gate")
			coversRaw, _ := cmd.Flags().GetStringArray("covers")

			// Validate tier
			switch tier {
			case "trivial", "feature", "architecture":
				// ok
			case "":
				tier = "feature" // sensible default
			default:
				return fmt.Errorf("invalid --tier=%q (use trivial, feature, or architecture)", tier)
			}

			if slug == "" {
				slug = slugify(intent)
				if slug == "" {
					return fmt.Errorf("could not derive slug from intent; pass --slug=explicit")
				}
			}

			// Pré-check: every AC mapped to at least one task; every
			// task has a gate. (Video research #1: TLC won because it
			// forced AC→test mapping.)
			if len(acsRaw) == 0 {
				return fmt.Errorf("no --ac provided; pass each AC as a separate --ac flag (Given/When/Then format recommended)")
			}
			if len(tasksRaw) == 0 {
				return fmt.Errorf("no --task provided; pass each task as a separate --task flag")
			}
			if len(gatesRaw) != len(tasksRaw) {
				return fmt.Errorf("--task count (%d) != --gate count (%d); every task needs a gate command", len(tasksRaw), len(gatesRaw))
			}
			if len(coversRaw) != len(tasksRaw) {
				return fmt.Errorf("--task count (%d) != --covers count (%d); every task must declare which ACs it covers (comma-separated AC numbers, e.g. '1,2')", len(tasksRaw), len(coversRaw))
			}

			// Compute next sequence number
			seq, err := nextSpecSeq("specs")
			if err != nil {
				return err
			}
			specDir := filepath.Join("specs", fmt.Sprintf("%04d-%s", seq, slug))

			if err := os.MkdirAll(specDir, 0o755); err != nil {
				return err
			}

			// Write spec.md
			specMD := renderSpecMD(seq, slug, intent, tier, acsRaw)
			if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(specMD), 0o644); err != nil {
				return err
			}
			// Write tasks.md
			tasksMD := renderTasksMD(seq, slug, tier, tasksRaw, gatesRaw, coversRaw, acsRaw)
			if err := os.WriteFile(filepath.Join(specDir, "tasks.md"), []byte(tasksMD), 0o644); err != nil {
				return err
			}

			// Update state.md with the new feature in flight
			statePath := filepath.Join(".radiant-harness", "state.md")
			if _, err := os.Stat(statePath); err == nil {
				body, _ := os.ReadFile(statePath)
				updated := upsertStateCurrentFeature(string(body), fmt.Sprintf("%04d-%s", seq, slug), tier, fmt.Sprintf("radiant run %s", specDir))
				atomicWrite(statePath, updated)
			}

			fmt.Printf("  ✓ created %s/spec.md (%d ACs)\n", specDir, len(acsRaw))
			fmt.Printf("  ✓ created %s/tasks.md (%d tasks)\n", specDir, len(tasksRaw))
			fmt.Printf("  ✓ state.md updated: current_feature=%04d-%s tier=%s\n", seq, slug, tier)
			fmt.Printf("\n  Next: radiant run %s --model <model>\n", specDir)
			return nil
		},
	}
	specCmd.Flags().String("tier", "", "tier: trivial | feature | architecture (default: feature)")
	specCmd.Flags().String("slug", "", "kebab-case slug (auto-derived from intent if empty)")
	specCmd.Flags().StringArray("ac", nil, "acceptance criterion (repeatable); \"Given ... When ... Then ...\" recommended")
	specCmd.Flags().StringArray("task", nil, "task name (repeatable, must match --ac coverage)")
	specCmd.Flags().StringArray("gate", nil, "gate command per task (must match --task count)")
	specCmd.Flags().StringArray("covers", nil, "comma-separated AC numbers per task (e.g. '1,2'); AC→test mapping enforced")
	root.AddCommand(specCmd)

	// ── adr (Sprint 11 — Architecture Decision Records, Nygard format) ──
	// `radiant adr "<decision>"` creates docs/architecture/adr/NNNN-<slug>.md
	// in Nygard format. The file's path is auto-numbered (next NNNN in
	// the directory) and the title is derived from the decision text.
	// Per the `adr` skill: context + alternatives + consequences are
	// required sections (the validator catches missing ones).
	adrCmd := &cobra.Command{
		Use:   "adr <decision>",
		Short: "Create an Architecture Decision Record in Nygard format",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			decision := args[0]
			statusFlag, _ := cmd.Flags().GetString("status")
			adrDir := filepath.Join("docs", "architecture", "adr")
			if err := os.MkdirAll(adrDir, 0o755); err != nil {
				return err
			}
			seq, err := nextADRSequence(adrDir)
			if err != nil {
				return err
			}
			slug := slugify(decision)
			if slug == "" {
				return fmt.Errorf("could not derive slug from decision; pass a more descriptive decision text")
			}
			fileName := fmt.Sprintf("%04d-%s.md", seq, slug)
			dest := filepath.Join(adrDir, fileName)
			body := renderADR(seq, decision, statusFlag)
			if err := os.WriteFile(dest, []byte(body), 0o644); err != nil {
				return err
			}
			fmt.Printf("  ✓ created %s\n", dest)
			fmt.Printf("\n  Next steps:\n")
			fmt.Printf("    1. Edit %s to fill in:\n", dest)
			fmt.Printf("       - Context: the forces at play\n")
			fmt.Printf("       - Alternatives considered (≥2)\n")
			fmt.Printf("       - Consequences (positive AND negative)\n")
			fmt.Printf("    2. Reference this ADR in code comments where the decision applies.\n")
			fmt.Printf("    3. Commit alongside the change it justifies.\n")
			return nil
		},
	}
	adrCmd.Flags().String("status", "proposed", "ADR status: proposed | accepted | deprecated | superseded")
	root.AddCommand(adrCmd)

	// ── diagramar (Sprint 11.3 — C4 Mermaid scaffold) ──
	// `radiant diagramar <level>` produces a starter Mermaid
	// diagram at the chosen C4 level (context, container, component,
	// code). The output is a template — the user (or an agent)
	// fills in the actual nodes/edges. This is intentionally
	// lighter than auto-extraction: most useful diagrams need
	// domain context the LLM should add via the diagramar skill.
	diagramarCmd := &cobra.Command{
		Use:   "diagramar <level>",
		Short: "Generate a C4 Mermaid diagram template (context|container|component|code)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out, _ := cmd.Flags().GetString("out")
			level := strings.ToLower(args[0])
			diagram, err := renderDiagram(level)
			if err != nil {
				return err
			}
			if out == "" {
				fmt.Print(diagram)
				return nil
			}
			if err := atomicWrite(out, diagram); err != nil {
				return fmt.Errorf("write %s: %w", out, err)
			}
			fmt.Printf("  ✓ wrote %s\n", out)
			return nil
		},
	}
	diagramarCmd.Flags().StringP("out", "o", "", "output file (default: stdout)")
	root.AddCommand(diagramarCmd)

	// ── product (Sprint 12 — Lean Inception scaffold) ──
	// `radiant product "<vision>"` scaffolds docs/product/ with the
	// 6-phase Lean Inception template (Why/What/Who/How/When/Where),
	// plus a personas.md file. The user (or an agent invoking the
	// nova-product skill) fills in each phase one at a time. Output
	// is template-only — no LLM call; that's the skill's job.
	productCmd := &cobra.Command{
		Use:   "product <vision>",
		Short: "Start a Lean Inception (Why/What/Who/How/When/Where) at docs/product/inception.md",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mvpWeeks, _ := cmd.Flags().GetInt("mvp-weeks")
			if mvpWeeks <= 0 {
				mvpWeeks = 8
			}
			productDir := "docs/product"
			if err := os.MkdirAll(productDir, 0o755); err != nil {
				return err
			}

			slug := slugify(args[0])
			inceptionPath := filepath.Join(productDir, "inception.md")
			body := renderInception(slug, args[0], mvpWeeks)
			if err := atomicWrite(inceptionPath, body); err != nil {
				return fmt.Errorf("write %s: %w", inceptionPath, err)
			}
			fmt.Printf("  ✓ created %s\n", inceptionPath)

			personasPath := filepath.Join(productDir, "personas.md")
			personasBody := renderPersonasTemplate()
			if err := atomicWrite(personasPath, personasBody); err != nil {
				return fmt.Errorf("write %s: %w", personasPath, err)
			}
			fmt.Printf("  ✓ created %s\n", personasPath)

			fmt.Println("\n  Next steps (Lean Inception phases — work them in order):")
			fmt.Println("    1. Why   — persona + job-to-be-done + alternative")
			fmt.Println("    2. What  — brainstorm features (untagged)")
			fmt.Println("    3. Who   — fill personas.md (2-4 personas)")
			fmt.Println("    4. How   — technical / business approach (1-2 paragraphs)")
			fmt.Println("    5. When  — Q1 MVP / Q2 Growth / Q3+ Vision")
			fmt.Println("    6. Where — bounded contexts (new vs existing)")
			fmt.Println("    7. Cut the MVP (3-7 features max) and run `radiant spec <feature>` per MVP item.")
			fmt.Printf("\n  MVP target: %d weeks. Adjust via --mvp-weeks=<n> on next regen.\n", mvpWeeks)
			return nil
		},
	}
	productCmd.Flags().Int("mvp-weeks", 8, "target weeks to MVP (drives the When phase)")
	root.AddCommand(productCmd)

	// ── integrations (Sprint 12.2 — MCP discovery, read-only) ──
	// `radiant integrations list` reads the project's `.mcp.json`
	// (per the integracoes skill — NEVER auto-writes; the user/agent
	// must approve each MCP via the skill first). The skill is
	// explicit: "Discovered is not ready." This command is the
	// READ-ONLY half: surface what's already declared.
	integrationsCmd := &cobra.Command{
		Use:   "integrations",
		Short: "Manage declared MCP integrations (read-only listing; never auto-configures)",
	}
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List MCP servers declared in the project's .mcp.json",
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonOut, _ := cmd.Flags().GetBool("json")
			docOut, _ := cmd.Flags().GetString("write-docs")
			return runIntegrationsList(jsonOut, docOut)
		},
	}
	listCmd.Flags().Bool("json", false, "machine-readable JSON output")
	listCmd.Flags().String("write-docs", "", "also write docs/engineering/integrations.md from current MCPs (pass empty for default path)")
	integrationsCmd.AddCommand(listCmd)
	root.AddCommand(integrationsCmd)

	// ── views (Sprint 13 — native agent views opt-in) ──
	// `radiant views --agent=<list>` regenerates native agent
	// views on demand (without re-running `radiant init`). Useful
	// when:
	//   - User adds a new skill and wants the agent to see it.
	//   - User switches between agents (Cursor today, Codex tomorrow).
	//   - User wants to drop a vendor (--force overwrites existing).
	//
	// By default, existing files are SKIPPED — user's local edits
	// to .cursor/rules/sdd.mdc etc. win. Pass --force to overwrite.
	viewsCmd := &cobra.Command{
		Use:   "views",
		Short: "Generate native agent views (.claude/, .cursor/, .codex/, etc.) on demand",
		RunE: func(cmd *cobra.Command, args []string) error {
			agentFlag, _ := cmd.Flags().GetString("agent")
			force, _ := cmd.Flags().GetBool("force")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			diffMode, _ := cmd.Flags().GetBool("diff")

			if agentFlag == "" {
				return fmt.Errorf("--agent=<list> required (e.g. --agent=claude,codex,cursor,copilot,gemini,windsurf)")
			}
			agents := resolveAgents(agentFlag, false)
			if len(agents) == 0 {
				return fmt.Errorf("no valid agents in --agent=%q (allowed: claude, codex, cursor, copilot, gemini, windsurf)", agentFlag)
			}

			cwd, _ := os.Getwd()
			var written, skipped int
			for _, agent := range agents {
				// --diff: show changes before writing
				if diffMode {
					diffs := scaffold.DiffViews(agent, cwd)
					fmt.Printf("  [%s]\n", agent)
					fmt.Print(scaffold.FormatDiff(diffs))
					continue
				}

				views := scaffold.GenerateViewsForAgent(agent)
				if len(views) == 0 {
					fmt.Printf("  [skip] %s: no adapter registered\n", agent)
					continue
				}
				fmt.Printf("  [%s]\n", agent)
				for _, v := range views {
					if dryRun {
						fmt.Printf("    [would-write] %s (%d bytes)\n", v.Path, len(v.Content))
						continue
					}
					if _, err := os.Stat(v.Path); err == nil && !force {
						fmt.Printf("    [skipped] %s (exists; pass --force to overwrite)\n", v.Path)
						skipped++
						continue
					}
					if err := os.MkdirAll(filepath.Dir(v.Path), 0o755); err != nil {
						return fmt.Errorf("mkdir %s: %w", filepath.Dir(v.Path), err)
					}
					if err := atomicWrite(v.Path, v.Content); err != nil {
						return fmt.Errorf("write %s: %w", v.Path, err)
					}
					fmt.Printf("    [wrote] %s\n", v.Path)
					written++
				}
			}
			if !diffMode {
				fmt.Printf("\n  Summary: %d written, %d skipped\n", written, skipped)
				if !force && skipped > 0 {
					fmt.Println("  Re-run with --force to overwrite existing views.")
				}
			}
			return nil
		},
	}
	viewsCmd.Flags().String("agent", "", "comma-separated agent list (claude,codex,cursor,copilot,gemini,windsurf) or --agent=all")
	viewsCmd.Flags().Bool("force", false, "overwrite existing views (DESTRUCTIVE — loses local edits)")
	viewsCmd.Flags().Bool("dry-run", false, "show what would change without writing")
	viewsCmd.Flags().Bool("diff", false, "show diff between generated and on-disk views before writing")
	root.AddCommand(viewsCmd)

	// ── review-pr (Sprint 13.2 — PR review against spec ACs) ──
	// `radiant review-pr <spec-path> [--diff=...] [--run-gates]`
	// generates `pr-review.md` next to spec.md. The MVP is
	// template-based — it parses spec.md for ACs, tasks.md for
	// gates, optionally runs each gate (--run-gates), and emits a
	// structured review report. The LLM (via the revisar-pr skill)
	// does the semantic AC↔code matching; this command is the
	// scaffold that makes that workflow reproducible.
	reviewPRCmd := &cobra.Command{
		Use:   "review-pr <spec-path>",
		Short: "Generate specs/<NNNN>/pr-review.md: AC coverage, gate results, SPEC_DEVIATIONs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			diffPath, _ := cmd.Flags().GetString("diff")
			runGates, _ := cmd.Flags().GetBool("run-gates")
			out, _ := cmd.Flags().GetString("output")
			if out == "" {
				out = filepath.Join(args[0], "pr-review.md")
			}
			return runReviewPR(args[0], diffPath, runGates, out)
		},
	}
	reviewPRCmd.Flags().String("diff", "", "path to PR diff file (optional; if absent, only ACs + gates are checked)")
	reviewPRCmd.Flags().Bool("run-gates", false, "execute each gate command from tasks.md and record pass/fail")
	reviewPRCmd.Flags().StringP("output", "o", "", "output path (default: <spec-path>/pr-review.md)")
	root.AddCommand(reviewPRCmd)

	// ── setup-ci (Sprint 13.3 — CI scaffold) ──
	// `radiant setup-ci [--provider=github|gitlab|circleci]
	// [--output=...] [--model=...]` generates the CI workflow
	// that enforces radiant gates on every PR: validate, audit,
	// tests, build. Default provider is GitHub Actions.
	setupCICmd := &cobra.Command{
		Use:   "setup-ci",
		Short: "Generate CI workflow file (GitHub Actions / GitLab CI / CircleCI)",
		RunE: func(cmd *cobra.Command, args []string) error {
			provider, _ := cmd.Flags().GetString("provider")
			outPath, _ := cmd.Flags().GetString("output")
			model, _ := cmd.Flags().GetString("model")
			return runSetupCI(provider, outPath, model)
		},
	}
	setupCICmd.Flags().String("provider", "github", "CI provider: github | gitlab | circleci")
	setupCICmd.Flags().StringP("output", "o", "", "output path (default: <provider's canonical path>)")
	setupCICmd.Flags().String("model", "", "LLM model for the validate step (optional)")
	root.AddCommand(setupCICmd)

}
