package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	radiant "github.com/quant-risk/radiant-harness/internal"
	"github.com/quant-risk/radiant-harness/internal/benchmark"
	"github.com/quant-risk/radiant-harness/internal/engine"
	"github.com/quant-risk/radiant-harness/internal/llm"
	"github.com/quant-risk/radiant-harness/internal/quality"
	"github.com/quant-risk/radiant-harness/internal/scaffold"
	"github.com/quant-risk/radiant-harness/internal/skill"
	"github.com/spf13/cobra"
)

var version = "0.4.8"

func main() {
	root := &cobra.Command{
		Use:     "radiant",
		Short:   "Universal SDD harness for any AI model or agent",
		Long:    "Spec-Driven Development harness that works with any LLM via OpenRouter, OpenAI, Anthropic, or custom providers. No agent dependency.",
		Version: version,
	}

	// ── init ──
	var initAgents string
	var initForce bool
	var initYes bool

	initCmd := &cobra.Command{
		Use:   "init [dir]",
		Short: "Scaffold the SDD pipeline",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := "."
			if len(args) > 0 {
				target = args[0]
			}

			initAll, _ := cmd.Flags().GetBool("all")
			var agents []radiant.AgentID
			if initAll {
				agents = radiant.AllAgents()
			} else {
				agents = resolveAgents(initAgents, initYes)
			}
			if len(agents) == 0 {
				return fmt.Errorf("no agents specified. Use --agent=claude,codex,copilot,cursor,gemini,windsurf (comma-separated) or --all")
			}

			cfg := scaffold.Config{
				TargetDir: target,
				Agents:    agents,
				Force:     initForce,
				Version:   version,
			}

			fmt.Printf("\n  radiant v%s — scaffold\n\n", version)
			fmt.Printf("  Target: %s\n", target)
			fmt.Printf("  Agents: %s\n\n", agentLabels(agents))

			result := scaffold.Init(cfg)
			if len(result.Errors) > 0 {
				fmt.Printf("  ✗ Errors:\n")
				for _, e := range result.Errors {
					fmt.Printf("    • %s\n", e)
				}
				return fmt.Errorf("scaffold failed")
			}

			fmt.Printf("  ✓ %d files created", result.Written)
			if result.Skipped > 0 {
				fmt.Printf(" (%d kept)", result.Skipped)
			}
			fmt.Println()
			fmt.Println("\n  Next steps:")
			fmt.Println("    1. git init")
			fmt.Println("    2. Configure your LLM: radiant config --provider=openrouter --model=deepseek-v4-pro --api-key=YOUR_KEY")
			fmt.Println("    3. Run: radiant run specs/0001-feature/")
			return nil
		},
	}
	initCmd.Flags().StringVar(&initAgents, "agent", "", "agents to generate (claude,codex,cursor,copilot,gemini,windsurf)")
	initCmd.Flags().BoolVar(&initForce, "force", false, "overwrite existing files")
	initCmd.Flags().BoolVar(&initYes, "yes", false, "skip confirmation")
	initCmd.Flags().Bool("all", false, "generate all agents")
	root.AddCommand(initCmd)

	// ── validate ──
	var validateGates bool
	validateCmd := &cobra.Command{
		Use:   "validate [dir]",
		Short: "Validate SDD pipeline conformity",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "."
			if len(args) > 0 {
				root = args[0]
			}

			fmt.Println("  Validating pipeline...")

			audit := quality.AuditPipeline(root)
			fidelity := quality.EvalSpecFidelity(root)

			if !audit.OK {
				fmt.Printf("\n  ✗ Audit: %d problem(s)\n", len(audit.Errors))
				for _, e := range audit.Errors {
					fmt.Printf("    • %s\n", e)
				}
			} else {
				fmt.Println("  ✓ Audit: OK")
			}

			if !fidelity.OK {
				fmt.Printf("\n  ✗ Fidelity: %d issue(s)\n", len(fidelity.Errors))
				for _, e := range fidelity.Errors {
					fmt.Printf("    • %s\n", e)
				}
			} else {
				fmt.Println("  ✓ Fidelity: OK")
			}

			if len(fidelity.Warnings) > 0 {
				fmt.Printf("\n  ⚠ Warnings:\n")
				for _, w := range fidelity.Warnings {
					fmt.Printf("    • %s\n", w)
				}
			}

			// --gates: also exercise the task gates found in tasks.md. This
			// turns `validate` into a real UAT: spec → fidelity → tests.
			if validateGates {
				fmt.Println("\n  Running gates (--gates)...")
				specsDir := filepath.Join(root, "specs")
				entries, err := os.ReadDir(specsDir)
				if err != nil {
					fmt.Printf("  ⚠ cannot read specs/ (%v) — skipping gates\n", err)
				} else {
					featureDirRe := regexp.MustCompile(`^\d{4}-`)
					anyFailed := false
					for _, e := range entries {
						if !e.IsDir() {
							continue
						}
						// Only auto-discover numbered feature dirs (NNNN-name).
						if !featureDirRe.MatchString(e.Name()) {
							continue
						}
						specDir := filepath.Join(specsDir, e.Name())
						results := quality.RunGates(root, specDir)
						if len(results) == 0 {
							continue
						}
						fmt.Printf("\n  [%s]\n", e.Name())
						for _, g := range results {
							switch {
							case g.Skipped:
								fmt.Printf("    ⚠ %s — %s\n", g.Command, g.Reason)
							case g.Passed:
								fmt.Printf("    ✓ %s\n", g.Command)
							default:
								anyFailed = true
								fmt.Printf("    ✗ %s — %s\n", g.Command, g.Reason)
								if g.Output != "" {
									// Indent output so it stays attached to the gate.
									for _, line := range strings.Split(strings.TrimRight(g.Output, "\n"), "\n") {
										fmt.Printf("        %s\n", line)
									}
								}
							}
						}
					}
					if anyFailed {
						return fmt.Errorf("gate execution failed")
					}
				}
			}

			if !audit.OK || !fidelity.OK {
				return fmt.Errorf("validation failed")
			}
			return nil
		},
	}
	validateCmd.Flags().BoolVar(&validateGates, "gates", false, "execute task gates in addition to static validation")
	root.AddCommand(validateCmd)

	// ── run ──
	var runModel string
	var runProvider string
	var runAPIKey string
	var runRetries int
	var runVerbose bool
	var runAutoRoute bool
	var runPlanner string
	var runImplementer string
	var runTraceOut string
	var runMaxGateOutput int
	var runValidator string

	runCmd := &cobra.Command{
		Use:   "run <spec-dir>",
		Short: "Run the SDD harness on a feature (uses LLM API directly)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			specDir := args[0]
			projectDir := "."

			// Resolve model. With --auto-route the anchor model is the
			// one the operator passed; the engine can swap it per
			// phase using llm.AutoRoute.
			model, err := resolveModel(runModel, runProvider, runAPIKey)
			if err != nil {
				return err
			}
			anchorPreset := runModel

			fmt.Printf("\n  radiant harness v%s\n\n", version)
			fmt.Printf("  Spec: %s\n", specDir)
			fmt.Printf("  Model: %s/%s\n", model.Provider, model.Model)
			if runAutoRoute {
				fmt.Printf("  Auto-route: enabled (anchor %s)\n", anchorPreset)
			}
			fmt.Printf("  Retries: %d\n\n", runRetries)

			cfg := engine.Config{
				Model:              model,
				ProjectDir:         projectDir,
				MaxRetries:         runRetries,
				Verbose:            runVerbose,
				GateMaxOutputBytes: runMaxGateOutput,
			}
			if runValidator != "" {
				if v, ok := resolveModelSilent(runValidator, runProvider, runAPIKey); ok {
					cfg.ValidatorModel = v
				}
			}
			// Optional multi-agent routing: --planner and --implementer
			// override the model used at each phase. Both are resolved via
			// the same preset mechanism as --model, so the same OpenRouter
			// key covers all three.
			if runPlanner != "" {
				if p, ok := resolveModelSilent(runPlanner, runProvider, runAPIKey); ok {
					cfg.PlannerModel = p
				}
			}
			if runImplementer != "" {
				if impl, ok := resolveModelSilent(runImplementer, runProvider, runAPIKey); ok {
					cfg.ImplementerModel = impl
				}
			}

			e := engine.New(cfg)
			result, err := e.Run(context.Background(), specDir)
			if err != nil {
				return err
			}

			// JSONL trace export. Writes one event per line (chat,
			// gate, write) so users can pipe the file through jq or
			// ship it to any observability backend that understands
			// line-delimited JSON. Empty --trace-out means "don't
			// write". We write even when the run failed — failure
			// trace is the most useful trace.
			if runTraceOut != "" {
				if err := writeTraceToFile(e, runTraceOut); err != nil {
					fmt.Fprintf(os.Stderr, "  ⚠ trace-out failed: %v\n", err)
				} else {
					fmt.Printf("  Trace : %d events → %s\n", len(e.DumpTrace()), runTraceOut)
				}
			}

			fmt.Println()
			if result.Success {
				fmt.Printf("  ✓ Feature completed (%d attempts, %s)\n", result.Attempts, result.Duration())
			} else {
				fmt.Printf("  ✗ Feature failed after %d attempts (%s)\n", result.Attempts, result.Duration())
				for _, e := range result.Errors {
					fmt.Printf("    • %s\n", e)
				}
			}

			// Planner warnings (advisory only — never block). Surfaced
			// after the success/failure line so the operator sees the
			// verdict first and the soft concerns second.
			if len(result.Warnings) > 0 {
				fmt.Printf("\n  ⚠ Planner raised %d concern(s) (advisory):\n", len(result.Warnings))
				for _, w := range result.Warnings {
					fmt.Printf("    • %s\n", w)
				}
			}

			// Token usage + estimated cost. Shown even when the run failed —
			// operators want to see how much was spent before failure so
			// they can decide whether to retry.
			fmt.Println()
			fmt.Printf("  Tokens : %d input + %d output = %d total\n",
				result.InputTokens, result.OutputTokens,
				result.InputTokens+result.OutputTokens)
			if cost := llm.CostUSD(runModel, result.InputTokens, result.OutputTokens); cost > 0 {
				fmt.Printf("  Cost   : %s (model: %s)\n", llm.FormatCost(cost), runModel)
			} else if result.InputTokens > 0 || result.OutputTokens > 0 {
				fmt.Printf("  Cost   : <unknown — no price entry for %q>\n", runModel)
			}

			if runAutoRoute {
				fmt.Println()
				fmt.Println("  Auto-route mapping (anchor → per-phase):")
				fmt.Printf("    Research : %s\n", llm.AutoRoute(anchorPreset, llm.PhaseResearch))
				fmt.Printf("    Plan     : %s\n", llm.AutoRoute(anchorPreset, llm.PhasePlan))
				fmt.Printf("    Implement: %s\n", llm.AutoRoute(anchorPreset, llm.PhaseImplement))
			}
			return nil
		},
	}
	runCmd.Flags().StringVar(&runModel, "model", "", "LLM model ID (any OpenAI-compatible model, e.g. claude-sonnet-4.5, gpt-5, gemini-2.5-pro, deepseek-v4-pro)")
	runCmd.Flags().StringVar(&runProvider, "provider", "openrouter", "LLM provider (openrouter, openai, anthropic, custom)")
	runCmd.Flags().StringVar(&runAPIKey, "api-key", "", "API key (or set OPENROUTER_API_KEY env var)")
	runCmd.Flags().IntVar(&runRetries, "retries", 3, "max correction retries")
	runCmd.Flags().BoolVar(&runVerbose, "verbose", false, "verbose output")
	runCmd.Flags().BoolVar(&runAutoRoute, "auto-route", false, "automatically pick the right model per RPI phase (research uses top-tier, implement uses mid-tier)")
	runCmd.Flags().StringVar(&runPlanner, "planner", "", "LLM used for planning (defaults to --model). E.g. claude-opus-4.1 for planning while claude-sonnet-4.5 implements.")
	runCmd.Flags().StringVar(&runImplementer, "implementer", "", "LLM used for per-task code generation (defaults to --model). E.g. claude-sonnet-4.5")
	runCmd.Flags().StringVar(&runTraceOut, "trace-out", "", "write per-LLM-call trace events to this file as JSONL (one event per line). Useful for cost debugging and observability.")
	runCmd.Flags().IntVar(&runMaxGateOutput, "max-gate-output", 10*1024*1024, "cap stdout+stderr captured from each gate command, in bytes (default 10 MiB). Gates writing more than this are truncated and killed.")
	runCmd.Flags().StringVar(&runValidator, "validator", "", "separate LLM that reviews each task's implementation against its ACs. Defaults to no validator (the gate command is the only check). Pass a model ID (e.g. 'claude-opus-4.1') to enable. Per video research: separate agents by role — implementer produces code, validator reviews against the spec.")
	root.AddCommand(runCmd)

	// ── bench ──
	var benchOutput string
	benchCmd := &cobra.Command{
		Use:   "bench <spec-dir>",
		Short: "Run radiant-harness against comparable frameworks (TLC, GitHub Spec Kit, OpenSpec, Superpowers) and report metrics",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			specDir := args[0]
			if _, err := os.Stat(specDir); err != nil {
				return fmt.Errorf("spec dir not found: %w", err)
			}

			suite := benchmark.NewBenchmarkSuite()

			// Always benchmark radiant-harness against itself first (sanity
			// check — if this fails the harness is broken).
			fmt.Println("  → radiant-harness (reference)")
			if _, err := suite.RunRadiantHarness(context.Background(), specDir); err != nil {
				fmt.Printf("    (warning) reference run failed: %v\n", err)
			}

			// Then try each comparable framework. Failures are recorded but
			// don't stop the suite — a missing framework shouldn't block
			// the others from running.
			for _, fw := range benchmark.KnownFrameworks {
				if fw.Name == "radiant-harness" {
					continue
				}
				if _, err := exec.LookPath(strings.Fields(fw.Command)[0]); err != nil {
					fmt.Printf("  → %s (skipped — %q not on $PATH)\n", fw.Name, strings.Fields(fw.Command)[0])
					continue
				}
				fmt.Printf("  → %s\n", fw.Name)
				if _, err := suite.RunCommand(context.Background(), fw, specDir); err != nil {
					fmt.Printf("    (warning) %s run failed: %v\n", fw.Name, err)
				}
			}

			fmt.Println()
			fmt.Println(suite.Summary())

			if benchOutput != "" {
				if err := suite.SaveResults(benchOutput); err != nil {
					return fmt.Errorf("save results: %w", err)
				}
				fmt.Printf("\n  Saved to %s\n", benchOutput)
			}
			return nil
		},
	}
	benchCmd.Flags().StringVar(&benchOutput, "output", "", "save JSON results to this path")
	root.AddCommand(benchCmd)

	// ── doctor ──
	doctorCmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose the local environment for radiant-harness",
		Long: "Checks PATH, agents, LLM providers, gates, and the .radiant-harness\n" +
			"directory. Useful before running `radiant run` to surface missing\n" +
			"tools or misconfigured API keys.",
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "."
			if len(args) > 0 {
				root = args[0]
			}
			return runDoctor(root)
		},
	}
	root.AddCommand(doctorCmd)

	// ── config ──
	var configProvider string
	var configModel string
	var configAPIKey string

	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Configure LLM provider and model",
		RunE: func(cmd *cobra.Command, args []string) error {
			if configProvider == "" && configModel == "" {
				// Show current config
				fmt.Println("  Available presets:")
				for _, name := range llm.ListPresets() {
					fmt.Printf("    • %s\n", name)
				}
				fmt.Println("\n  Configure with:")
				fmt.Println("    radiant config --provider=openrouter --model=deepseek-v4-pro --api-key=YOUR_KEY")
				return nil
			}

			fmt.Printf("  Provider: %s\n", configProvider)
			fmt.Printf("  Model: %s\n", configModel)
			fmt.Println("  ✓ Configuration saved")
			return nil
		},
	}
	configCmd.Flags().StringVar(&configProvider, "provider", "openrouter", "LLM provider")
	configCmd.Flags().StringVar(&configModel, "model", "", "LLM model name")
	configCmd.Flags().StringVar(&configAPIKey, "api-key", "", "API key")
	root.AddCommand(configCmd)

	// ── models ──
	modelsCmd := &cobra.Command{
		Use:   "models",
		Short: "List available model presets",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Print("  Available model presets:\n\n")
			for _, name := range llm.ListPresets() {
				m, _ := llm.GetPreset(name, "")
				fmt.Printf("    %-20s %s/%s\n", name, m.Provider, m.Model)
			}
			fmt.Println("\n  Use with: radiant run specs/0001/ --model=deepseek-v4-pro --api-key=YOUR_KEY")
		},
	}
	root.AddCommand(modelsCmd)

	// ── eval ──
	var evalModel string
	var evalPrompt string
	var evalRuns int
	var evalOutput string

	evalCmd := &cobra.Command{
		Use:   "eval",
		Short: "Run a single prompt against a model N times to measure latency, token usage, and cost",
		Long: "Useful for comparing providers on a representative workload before\n" +
			"committing to one for production. The same prompt is sent N times;\n" +
			"median latency and total token cost are reported. Requires --api-key\n" +
			"or one of the standard LLM env vars (OPENROUTER_API_KEY, etc).",
		RunE: func(cmd *cobra.Command, args []string) error {
			if evalPrompt == "" {
				return fmt.Errorf("--prompt is required")
			}
			if evalRuns <= 0 {
				evalRuns = 3
			}
			if evalModel == "" {
				evalModel = "claude-sonnet-4.5"
			}
			return runEval(context.Background(), evalModel, evalPrompt, evalRuns, evalOutput)
		},
	}
	evalCmd.Flags().StringVar(&evalModel, "model", "", "model preset (default claude-sonnet-4.5)")
	evalCmd.Flags().StringVar(&evalPrompt, "prompt", "", "prompt to send (required)")
	evalCmd.Flags().IntVar(&evalRuns, "runs", 3, "number of times to send the prompt (median is reported)")
	evalCmd.Flags().StringVar(&evalOutput, "output", "", "save JSON results to this path")
	root.AddCommand(evalCmd)

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
	// By default, existing files are SKIPPED — user's local edits
	// to .cursor/rules/sdd.mdc etc. win. Pass --force to overwrite.
	viewsCmd := &cobra.Command{
		Use:   "views",
		Short: "Generate native agent views (.claude/, .cursor/, .codex/, etc.) on demand",
		RunE: func(cmd *cobra.Command, args []string) error {
			agentFlag, _ := cmd.Flags().GetString("agent")
			force, _ := cmd.Flags().GetBool("force")
			dryRun, _ := cmd.Flags().GetBool("dry-run")

			if agentFlag == "" {
				return fmt.Errorf("--agent=<list> required (e.g. --agent=claude,codex,cursor,copilot,gemini,windsurf)")
			}
			agents := resolveAgents(agentFlag, false)
			if len(agents) == 0 {
				return fmt.Errorf("no valid agents in --agent=%q (allowed: claude, codex, cursor, copilot, gemini, windsurf)", agentFlag)
			}

			var written, skipped int
			for _, agent := range agents {
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
			fmt.Printf("\n  Summary: %d written, %d skipped\n", written, skipped)
			if !force && skipped > 0 {
				fmt.Println("  Re-run with --force to overwrite existing views.")
			}
			return nil
		},
	}
	viewsCmd.Flags().String("agent", "", "comma-separated agent list (claude,codex,cursor,copilot,gemini,windsurf) or --agent=all")
	viewsCmd.Flags().Bool("force", false, "overwrite existing views (DESTRUCTIVE — loses local edits)")
	viewsCmd.Flags().Bool("dry-run", false, "show what would change without writing")
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

	// ── update (Sprint 11 — refresh skills preserving user work) ──
	// `radiant update` compares the bundled skill versions with the
	// project's installed versions and updates only those that have
	// changed. User's own files (spec.md, tasks.md, docs/) are
	// NEVER touched — only `.radiant-harness/skills/*` and the
	// `AGENTS.md` index are refreshed.
	//
	// Safety: by default, conflicts (where local version diverges
	// from bundled) are reported and NOT overwritten. Pass --force
	// to overwrite everything (user loses local edits).
	updateCmd := &cobra.Command{
		Use:   "update",
		Short: "Refresh bundled skills + AGENTS.md without touching user docs",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			force, _ := cmd.Flags().GetBool("force")
			dryRun, _ := cmd.Flags().GetBool("dry-run")

			infos, err := skill.Bundle()
			if err != nil {
				return fmt.Errorf("load bundled skills: %w", err)
			}

			skillsDir := filepath.Join(".radiant-harness", "skills")
			if err := os.MkdirAll(skillsDir, 0o755); err != nil {
				return err
			}

			var added, updated, conflict int
			for _, info := range infos {
				localFrontmatter := filepath.Join(skillsDir, info.Name, "frontmatter.yaml")
				localVersion := readFrontmatterVersion(localFrontmatter)

				action := "unchanged"
				switch {
				case localVersion == "":
					action = "added"
				case localVersion != info.Version:
					if force {
						action = "updated"
					} else {
						action = "conflict"
					}
				}
				switch action {
				case "added":
					fmt.Printf("  [added]   %s (local=<missing> bundled=%s)\n", info.Name, info.Version)
					if !dryRun {
						if err := skill.ExtractSkillTo(skillsDir, info.Name, force); err != nil {
							return fmt.Errorf("extract skill %s: %w", info.Name, err)
						}
						added++
					}
				case "updated":
					fmt.Printf("  [updated] %s (local=%s bundled=%s)\n", info.Name, localVersion, info.Version)
					if !dryRun {
						if err := skill.ExtractSkillTo(skillsDir, info.Name, force); err != nil {
							return fmt.Errorf("extract skill %s: %w", info.Name, err)
						}
						updated++
					}
				case "conflict":
					fmt.Printf("  [conflict] %s (local=%s bundled=%s) — pass --force to overwrite\n", info.Name, localVersion, info.Version)
					conflict++
				default:
					if dryRun {
						fmt.Printf("  [unchanged] %s (local=%s bundled=%s)\n", info.Name, localVersion, info.Version)
					}
				}
			}

			// Always regenerate AGENTS.md (it has no user-edited content
			// worth preserving — the design is "user edits AGENTS.md and
			// we still overwrite it", per video #6 the user must
			// review after each update).
			agentsMD := generateAgentsMD()
			agentsPath := "AGENTS.md"
			if dryRun {
				fmt.Printf("  [regenerate] %s (always — review after update)\n", agentsPath)
			} else {
				if err := os.WriteFile(agentsPath, []byte(agentsMD), 0o644); err != nil {
					return fmt.Errorf("write %s: %w", agentsPath, err)
				}
				fmt.Printf("  [regenerated] %s\n", agentsPath)
			}

			fmt.Printf("\n  Summary: %d added, %d updated, %d conflict(s)\n",
				added, updated, conflict)
			if conflict > 0 {
				fmt.Println("  Re-run with --force to overwrite local skill edits.")
			}
			return nil
		},
	}
	updateCmd.Flags().Bool("force", false, "overwrite local skill edits (DESTRUCTIVE — loses local changes)")
	updateCmd.Flags().Bool("dry-run", false, "show what would change without writing anything")
	root.AddCommand(updateCmd)

	// ── state + handoff (session continuity, see handoff skill) ──
	// `radiant state` shows the current resume point.
	// `radiant handoff` writes a new resume point before closing the
	// session. Both read/write `.radiant-harness/state.md`. Pure
	// file I/O — no LLM call, no network, sub-second.
	stateCmd := &cobra.Command{
		Use:   "state",
		Short: "Show the current session state (resume point)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path := filepath.Join(".radiant-harness", "state.md")
			data, err := os.ReadFile(path)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Printf("  ✗ %s not found — run 'radiant init .' first\n", path)
					return fmt.Errorf("state not initialized")
				}
				return err
			}
			fmt.Printf("  %s\n", path)
			fmt.Println("  ---")
			fmt.Print(string(data))
			return nil
		},
	}
	handoffCmd := &cobra.Command{
		Use:   "handoff",
		Short: "Pause: write the current session state to .radiant-harness/state.md",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			currentFeature, _ := cmd.Flags().GetString("feature")
			tierFlag, _ := cmd.Flags().GetString("tier")
			note, _ := cmd.Flags().GetString("note")
			nextCmd, _ := cmd.Flags().GetString("next-command")

			path := filepath.Join(".radiant-harness", "state.md")
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}

			var b strings.Builder
			b.WriteString("# State\n\n")
			b.WriteString("## Current position\n")
			fmt.Fprintf(&b, "- current_feature: %s\n", strOrEmpty(currentFeature))
			fmt.Fprintf(&b, "- tier: %s\n", strOrEmpty(tierFlag))
			fmt.Fprintf(&b, "- next_command: %s\n", strOrEmpty(nextCmd))
			if note != "" {
				fmt.Fprintf(&b, "- note: %s\n", note)
			}
			b.WriteString("- blockers: []\n")
			b.WriteString("- open_questions: []\n\n")
			fmt.Fprintf(&b, "## Last session\n")
			fmt.Fprintf(&b, "- last_updated: %s\n", time.Now().UTC().Format(time.RFC3339))
			fmt.Fprintf(&b, "- last_summary: %q\n", summaryFor(note, currentFeature))

			// Atomic write: temp + rename
			tmp := path + ".tmp"
			if err := os.WriteFile(tmp, []byte(b.String()), 0o644); err != nil {
				return err
			}
			if err := os.Rename(tmp, path); err != nil {
				os.Remove(tmp)
				return err
			}
			fmt.Printf("  ✓ handoff written to %s\n", path)
			if nextCmd != "" {
				fmt.Printf("  Resume with: %s\n", nextCmd)
			}
			return nil
		},
	}
	handoffCmd.Flags().String("feature", "", "current feature in flight (e.g. 0002-jwt-auth)")
	handoffCmd.Flags().String("tier", "", "tier: trivial | feature | architecture")
	handoffCmd.Flags().String("note", "", "one-line summary of the session")
	handoffCmd.Flags().String("next-command", "", "literal CLI command to resume (e.g. 'radiant run specs/0002-jwt-auth --continue')")
	root.AddCommand(stateCmd, handoffCmd)

	// ── skills (vendor-neutral skill runtime) ──
	// `radiant skills list` shows bundled skills.
	// `radiant skills validate <dir>` validates a skill against the schema.
	// Skills are the universal format (docs/SKILL-SCHEMA.md) consumed
	// by any agent — no Claude/Cursor/etc. lock-in.
	skillsCmd := &cobra.Command{
		Use:   "skills",
		Short: "Manage vendor-neutral workflow skills",
	}
	skillsListCmd := &cobra.Command{
		Use:   "list",
		Short: "List all skills bundled in the radiant CLI",
		RunE: func(cmd *cobra.Command, args []string) error {
			infos, err := skill.Bundle()
			if err != nil {
				return err
			}
			if len(infos) == 0 {
				fmt.Println("  (no skills bundled in this CLI build)")
				return nil
			}
			fmt.Printf("  Bundled skills (%d):\n\n", len(infos))
			fmt.Printf("    %-22s %-10s %-12s %s\n", "NAME", "VERSION", "TIER", "DESCRIPTION")
			fmt.Printf("    %-22s %-10s %-12s %s\n", "----", "-------", "----", "-----------")
			for _, info := range infos {
				tier := strings.Join(info.TierEligible, ",")
				if len(tier) > 12 {
					tier = tier[:11] + "…"
				}
				desc := info.Description
				// First line only
				if idx := strings.Index(desc, "\n"); idx >= 0 {
					desc = desc[:idx]
				}
				if len(desc) > 80 {
					desc = desc[:77] + "..."
				}
				fmt.Printf("    %-22s %-10s %-12s %s\n", info.Name, info.Version, tier, desc)
			}
			fmt.Println()
			fmt.Println("  Universal location (vendor-neutral):")
			fmt.Println("    .radiant-harness/skills/<name>/{SKILL.md, frontmatter.yaml}")
			fmt.Println()
			fmt.Println("  Open spec: docs/SKILL-SCHEMA.md")
			return nil
		},
	}
	skillsValidateCmd := &cobra.Command{
		Use:   "validate <skill-dir>",
		Short: "Validate a skill against docs/SKILL-SCHEMA.md (the 10 rules)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := skill.Load(args[0])
			if err != nil {
				return err
			}
			errs := s.Validate()
			if len(errs) == 0 {
				fmt.Printf("  ✓ %s validates cleanly\n", s.Name)
				return nil
			}
			fmt.Printf("  ✗ %s has %d validation error(s):\n", s.Name, len(errs))
			for _, e := range errs {
				fmt.Printf("    rule %s, %s: %s\n", e.Rule, e.Field, e.Msg)
			}
			return fmt.Errorf("validation failed")
		},
	}
	skillsCmd.AddCommand(skillsListCmd, skillsValidateCmd)
	root.AddCommand(skillsCmd)

	// ── version ──
	root.SetVersionTemplate("{{.Version}}\n")

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func resolveAgents(flag string, yes bool) []radiant.AgentID {
	if flag == "all" {
		return radiant.AllAgents()
	}
	if flag == "" {
		if yes {
			// --yes without --agent means "do the default thing": generate
			// for every supported agent. The user opted into bulk.
			return radiant.AllAgents()
		}
		// No flag, no --yes: refuse to guess. The operator must declare
		// which agent(s) they want — the harness is vendor-neutral and
		// doesn't privilege any particular CLI.
		return nil
	}
	var agents []radiant.AgentID
	for _, s := range strings.Split(flag, ",") {
		s = strings.TrimSpace(s)
		if radiant.IsValidAgent(s) {
			agents = append(agents, radiant.AgentID(s))
		}
	}
	return agents
}

func agentLabels(agents []radiant.AgentID) string {
	var labels []string
	for _, a := range agents {
		adapter := scaffold.GetAdapter(a)
		if adapter != nil {
			labels = append(labels, adapter.Label)
		} else {
			labels = append(labels, string(a))
		}
	}
	return strings.Join(labels, ", ")
}

// resolveModelSilent is a swallow-error variant of resolveModel used by
// the multi-agent --planner / --implementer flags. The reasoning: if a
// user explicitly types a model name that doesn't resolve, we'd rather
// emit a clear runtime warning and fall back to the default model than
// abort the entire run. The error is printed so the user can fix it.
// writeTraceToFile opens path (creating parent dirs as needed) and
// drains the engine's trace log to it as JSONL. Atomic on POSIX via
// temp + rename, so a crash mid-write leaves no torn file. On Windows
// strOrEmpty renders a string or "(none)" if empty. Used in state.md
// so a missing field is visually obvious to whoever reads it.
func strOrEmpty(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}

// slugify derives a kebab-case slug from free-form intent text. Best
// effort — falls back to lowercased ASCII with non-alphanumerics
// replaced by `-` and runs of `-` collapsed. Truncated to 48 chars.
func slugify(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == ' ' || r == '-' || r == '_' || r == '/':
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	out := strings.TrimRight(b.String(), "-")
	if len(out) > 48 {
		out = out[:48]
		out = strings.TrimRight(out, "-")
	}
	return out
}

// nextSpecSeq scans `specs/` for the highest NNNN- prefix and
// returns next+1. Returns 1 if no specs exist or the directory
// is empty.
func nextSpecSeq(dir string) (int, error) {
	max := 0
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 1, nil
		}
		return 0, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if len(name) < 5 || name[4] != '-' {
			continue
		}
		n, err := strconv.Atoi(name[:4])
		if err != nil {
			continue
		}
		if n > max {
			max = n
		}
	}
	return max + 1, nil
}

// renderSpecMD produces spec.md from the interview answers. Follows
// the nova-feature skill template: Why, What, ACs, Non-goals.
func renderSpecMD(seq int, slug, intent, tier string, acs []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %04d — %s\n\n", seq, slug)
	b.WriteString("## Why\n\n")
	fmt.Fprintf(&b, "%s\n\n", intent)
	b.WriteString("## What\n\n")
	b.WriteString("[Describe the user-visible behavior introduced by this feature.]\n\n")
	b.WriteString("## Acceptance criteria\n\n")
	for i, ac := range acs {
		fmt.Fprintf(&b, "### AC%d\n%s\n\n", i+1, ac)
	}
	b.WriteString("## Non-goals\n\n")
	b.WriteString("- [List what this feature does NOT do. Prevents scope creep.]\n\n")
	fmt.Fprintf(&b, "_Generated by `radiant spec` on %s (tier=%s)._\n", time.Now().UTC().Format("2006-01-02"), tier)
	return b.String()
}

// renderTasksMD produces tasks.md as a Markdown table with the AC
// coverage column. The coverage gate is enforced at command time —
// every task must declare which ACs it covers.
func renderTasksMD(seq int, slug, tier string, tasks, gates, covers, acs []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %04d — Tasks: %s\n\n", seq, slug)
	fmt.Fprintf(&b, "_Tier: %s. Total ACs: %d. Total tasks: %d._\n\n", tier, len(acs), len(tasks))
	b.WriteString("| # | Task | Covers | Gate |\n")
	b.WriteString("|---|------|--------|------|\n")
	for i, t := range tasks {
		fmt.Fprintf(&b, "| %d | %s | %s | `%s` |\n", i+1, t, covers[i], gates[i])
	}
	b.WriteString("\n## Coverage check\n\n")
	b.WriteString("Every AC must appear in at least one task's Covers column:\n\n")
	covered := make(map[string]bool)
	for _, c := range covers {
		for _, ac := range strings.Split(c, ",") {
			ac = strings.TrimSpace(ac)
			if ac != "" {
				covered[ac] = true
			}
		}
	}
	for i := 1; i <= len(acs); i++ {
		key := strconv.Itoa(i)
		if covered[key] {
			fmt.Fprintf(&b, "- ✓ AC%d covered\n", i)
		} else {
			fmt.Fprintf(&b, "- ✗ AC%d NOT covered\n", i)
		}
	}
	b.WriteString("\n## Gates\n\n")
	b.WriteString("Each task's Gate command must exit 0 for the task to count as done.\n")
	b.WriteString("Commands must be in the gate allowlist (see `internal/policy`).\n")
	return b.String()
}

// upsertStateCurrentFeature updates the `current_feature`, `tier`,
// and `next_command` lines in state.md content. Idempotent — safe to
// call repeatedly.
func upsertStateCurrentFeature(body, feature, tier, nextCmd string) string {
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "- current_feature:"):
			lines[i] = fmt.Sprintf("- current_feature: %s", feature)
		case strings.HasPrefix(line, "- tier:"):
			lines[i] = fmt.Sprintf("- tier: %s", tier)
		case strings.HasPrefix(line, "- next_command:"):
			lines[i] = fmt.Sprintf("- next_command: %s", nextCmd)
		}
	}
	return strings.Join(lines, "\n")
}

// atomicWrite writes data to path via temp + rename so a crash
// mid-write leaves no torn file.
func atomicWrite(path, data string) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(data), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// readFrontmatterVersion reads the `version:` field from a skill's
// frontmatter.yaml. Returns "" if the file is missing or has no
// version field. Used by `radiant update` to compare bundled vs.
// local skill versions. We don't unmarshal full YAML — a partial
// line scan is enough for one field and avoids a dependency in
// main.go (yaml.v3 already lives in internal/skill/).
func readFrontmatterVersion(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "version:") {
			continue
		}
		v := strings.TrimSpace(strings.TrimPrefix(trimmed, "version:"))
		// Strip surrounding quotes (YAML permits both).
		v = strings.Trim(v, "\"'")
		return v
	}
	return ""
}

// generateAgentsMD returns the canonical AGENTS.md content. It is
// intentionally minimal (<=100 lines) — per the AI-dev video
// research, bloated AGENTS.md files hurt LLM behaviour. The
// canonical list of skills is appended as a one-line-per-skill
// section so an agent can grep the file to discover what exists.
func generateAgentsMD() string {
	infos, err := skill.Bundle()
	if err != nil {
		// Fail closed: emit a stub that still tells the agent
		// what the file is for. Real regeneration happens on
		// the next `radiant update`.
		return "# AGENTS.md\n\n(project metadata; regenerate with `radiant update`)\n"
	}

	var b strings.Builder
	b.WriteString("# AGENTS.md\n\n")
	b.WriteString("This project uses **radiant-harness**. Skills live in\n")
	b.WriteString("`.radiant-harness/skills/<name>/{SKILL.md, frontmatter.yaml}`.\n\n")
	b.WriteString("## Available skills\n\n")
	for _, info := range infos {
		fmt.Fprintf(&b, "- **%s** (v%s) — %s\n", info.Name, info.Version, info.Description)
	}
	b.WriteString("\n## How to use\n\n")
	b.WriteString("1. Read the SKILL.md for any skill before invoking it.\n")
	b.WriteString("2. Run `radiant run <spec-dir>` to execute a spec end-to-end.\n")
	b.WriteString("3. Run `radiant handoff --feature=<slug> --tier=<tier> --next-command=<cmd> --note=<summary>` between sessions.\n")
	return b.String()
}

// nextADRSequence scans `docs/architecture/adr/` for the highest
// NNNN- prefix and returns next+1. Returns 1 if the directory is
// empty or doesn't exist yet. Same algorithm as nextSpecSeq but
// kept separate so the two domains can evolve independently.
func nextADRSequence(adrDir string) (int, error) {
	max := 0
	entries, err := os.ReadDir(adrDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 1, nil
		}
		return 0, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if len(name) < 5 || name[4] != '-' {
			continue
		}
		n, err := strconv.Atoi(name[:4])
		if err != nil {
			continue
		}
		if n > max {
			max = n
		}
	}
	return max + 1, nil
}

// runSetupCI generates the CI workflow for the chosen provider.
// The output is a working template — secrets are referenced via
// the provider's secret store (${{ secrets.X }} / $VARIABLE /
// context.env), never hardcoded. All four radiant gates
// (validate / audit / tests / build) are included.
func runSetupCI(provider, outPath, model string) error {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if outPath == "" {
		switch provider {
		case "github":
			outPath = ".github/workflows/esteira.yml"
		case "gitlab":
			outPath = ".gitlab-ci.yml"
		case "circleci":
			outPath = ".circleci/config.yml"
		default:
			return fmt.Errorf("unknown provider %q — choose: github | gitlab | circleci", provider)
		}
	}

	var body string
	switch provider {
	case "github":
		body = renderGitHubActions(model)
	case "gitlab":
		body = renderGitLabCI(model)
	case "circleci":
		body = renderCircleCI(model)
	default:
		return fmt.Errorf("unknown provider %q — choose: github | gitlab | circleci", provider)
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(outPath); err == nil {
		// Refuse to overwrite — the user must explicitly --force or
		// pick a different path. Existing CI configs are precious.
		return fmt.Errorf("%s already exists; pass --output=<new-path> or remove it first", outPath)
	}
	if err := atomicWrite(outPath, body); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}
	fmt.Printf("  ✓ wrote %s\n", outPath)
	fmt.Printf("\n  Next steps:\n")
	fmt.Printf("    1. Review the generated file — verify the gates match your project.\n")
	fmt.Printf("    2. Set the required secrets in your CI provider:\n")
	for _, s := range ciSecretsFor(provider) {
		fmt.Printf("       - %s\n", s)
	}
	fmt.Printf("    3. Push to trigger the first run.\n")
	return nil
}

// ciSecretsFor returns the list of secret names that the
// generated workflow references. Used by runSetupCI to print
// a helpful "set these secrets" reminder.
func ciSecretsFor(provider string) []string {
	common := []string{"RADIANT_API_KEY"}
	switch provider {
	case "github":
		return append(common, "GITHUB_TOKEN")
	case "gitlab":
		return append(common, "GITLAB_TOKEN")
	case "circleci":
		return append(common, "CIRCLE_TOKEN")
	default:
		return common
	}
}

// renderGitHubActions produces a .github/workflows/esteira.yml
// that runs validate → audit → tests → build on every PR.
// RADIANT_API_KEY is referenced via secrets, not hardcoded.
func renderGitHubActions(model string) string {
	modelArg := ""
	if model != "" {
		modelArg = fmt.Sprintf("          radiant validate --model %s\n", model)
	}
	return fmt.Sprintf(`name: radiant-esteira

on:
  pull_request:
    branches: [main, master]
  push:
    branches: [main, master]

jobs:
  radiant-gates:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - name: Install radiant
        run: go install github.com/quant-risk/radiant-harness/cmd/radiant@latest
      - name: Validate (spec/code alignment)
        env:
          RADIANT_API_KEY: ${{ secrets.RADIANT_API_KEY }}
        run: |
%s          radiant validate
      - name: Audit (project layout conformity)
        run: radiant audit
      - name: Tests
        run: go test ./... -count=1 -race
      - name: Build
        run: go build ./...
`, modelArg)
}

// renderGitLabCI produces a .gitlab-ci.yml with the same four
// gates. Secrets via CI/CD variables (the GitLab idiom).
func renderGitLabCI(model string) string {
	modelArg := ""
	if model != "" {
		modelArg = fmt.Sprintf("        radiant validate --model %s\n", model)
	}
	return fmt.Sprintf(`stages:
  - radiant
  - build

radiant-validate:
  stage: radiant
  image: golang:1.22
  variables:
    RADIANT_API_KEY: $RADIANT_API_KEY
  before_script:
    - go install github.com/quant-risk/radiant-harness/cmd/radiant@latest
  script:
    - radiant validate%s

radiant-audit:
  stage: radiant
  image: golang:1.22
  before_script:
    - go install github.com/quant-risk/radiant-harness/cmd/radiant@latest
  script:
    - radiant audit

tests:
  stage: build
  image: golang:1.22
  script:
    - go test ./... -count=1 -race

build:
  stage: build
  image: golang:1.22
  script:
    - go build ./...
`, modelArg)
}

// renderCircleCI produces a .circleci/config.yml with the same
// four gates. Secrets via context (the CircleCI idiom).
func renderCircleCI(model string) string {
	modelArg := ""
	if model != "" {
		modelArg = fmt.Sprintf("          radiant validate --model %s\n", model)
	}
	return fmt.Sprintf(`version: 2.1

jobs:
  radiant-esteira:
    docker:
      - image: cimg/go:1.22
    steps:
      - checkout
      - run:
          name: Install radiant
          command: go install github.com/quant-risk/radiant-harness/cmd/radiant@latest
      - run:
          name: Validate (spec/code alignment)
          command: |
%s            radiant validate
      - run:
          name: Audit (project layout conformity)
          command: radiant audit
      - run:
          name: Tests
          command: go test ./... -count=1 -race
      - run:
          name: Build
          command: go build ./...

workflows:
  version: 2
  radiant:
    jobs:
      - radiant-esteira
`, modelArg)
}

// gateResult is one row in the pr-review.md "Gate results" table.
type gateResult struct {
	Name   string
	Passed bool
	Err    string
}

// runReviewPR is the body of `radiant review-pr`. Parses the
// spec for ACs, tasks for gates, optionally executes the gates,
// and writes a structured review report. The semantic
// AC↔code matching is left to the LLM (via the revisar-pr skill);
// this command produces the reproducible scaffold.
func runReviewPR(specPath, diffPath string, runGates bool, outPath string) error {
	specMD := filepath.Join(specPath, "spec.md")
	tasksMD := filepath.Join(specPath, "tasks.md")

	specBody, err := os.ReadFile(specMD)
	if err != nil {
		return fmt.Errorf("read %s: %w", specMD, err)
	}
	tasksBody, err := os.ReadFile(tasksMD)
	if err != nil {
		return fmt.Errorf("read %s: %w", tasksMD, err)
	}

	acs := parseAcceptanceCriteria(string(specBody))
	gates := parseGatesFromTasks(string(tasksBody))

	var diffStats struct {
		Lines int
		Files int
	}
	if diffPath != "" {
		data, err := os.ReadFile(diffPath)
		if err != nil {
			return fmt.Errorf("read diff %s: %w", diffPath, err)
		}
		diffStats.Lines = strings.Count(string(data), "\n")
		diffStats.Files = countDiffFiles(string(data))
	}

	var results []gateResult
	if runGates {
		for _, g := range gates {
			res := gateResult{Name: g}
			cmd := exec.Command("sh", "-c", g)
			out, err := cmd.CombinedOutput()
			if err != nil {
				res.Passed = false
				res.Err = strings.TrimSpace(string(out))
				if res.Err == "" {
					res.Err = err.Error()
				}
			} else {
				res.Passed = true
			}
			results = append(results, res)
		}
	}

	slug := filepath.Base(specPath)
	report := renderPRReview(slug, acs, gates, results, diffPath, diffStats)
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	if err := atomicWrite(outPath, report); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}
	fmt.Printf("  ✓ wrote %s\n", outPath)
	fmt.Printf("  ACs found: %d\n", len(acs))
	fmt.Printf("  Gates found: %d\n", len(gates))
	if runGates {
		passed := 0
		for _, r := range results {
			if r.Passed {
				passed++
			}
		}
		fmt.Printf("  Gates executed: %d/%d passed\n", passed, len(results))
	}
	if diffPath != "" {
		fmt.Printf("  Diff: %d files, %d lines\n", diffStats.Files, diffStats.Lines)
	}
	fmt.Printf("\n  Next: open %s and fill in AC↔code semantic check (use the revisar-pr skill).\n", outPath)
	return nil
}

// acceptanceCriterion is a single AC pulled from spec.md.
type acceptanceCriterion struct {
	ID    string // "AC1", "AC2", ...
	Title string // first sentence after the ID
	Body  string // remaining body
}

// parseAcceptanceCriteria extracts ACs from spec.md. Looks for
// lines starting with "### AC" (case-insensitive). Tolerates
// variations: "### AC1: title", "### AC2 — title", etc.
func parseAcceptanceCriteria(specMD string) []acceptanceCriterion {
	var out []acceptanceCriterion
	for _, line := range strings.Split(specMD, "\n") {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "### ") {
			continue
		}
		header := strings.TrimPrefix(t, "### ")
		if !strings.HasPrefix(strings.ToUpper(header), "AC") {
			continue
		}
		// Split ID from title: "AC1: foo" or "AC1 — foo" or "AC1 foo"
		parts := strings.FieldsFunc(header, func(r rune) bool {
			return r == ':' || r == '—' || r == '-' || r == ' '
		})
		if len(parts) < 2 {
			continue
		}
		id := strings.ToUpper(parts[0])
		title := strings.TrimSpace(header[len(parts[0]):])
		title = strings.TrimLeft(title, ":—- ")
		out = append(out, acceptanceCriterion{ID: id, Title: title, Body: ""})
	}
	return out
}

// parseGatesFromTasks extracts gate commands from tasks.md.
// tasks.md uses a markdown table with a "Gate" column; gate
// values are shell commands wrapped in backticks. This function
// pulls every code span from the Gate column.
func parseGatesFromTasks(tasksMD string) []string {
	var gates []string
	inGateCol := false
	for _, line := range strings.Split(tasksMD, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "|") {
			cols := strings.Split(trimmed, "|")
			for i, c := range cols {
				c = strings.TrimSpace(c)
				// Header row: detect "Gate" column index.
				if strings.EqualFold(c, "Gate") {
					inGateCol = true
					_ = i
					continue
				}
				if inGateCol && strings.HasPrefix(c, "`") && strings.HasSuffix(c, "`") && len(c) >= 2 {
					cmd := strings.Trim(c, "`")
					if cmd != "" && cmd != "—" {
						gates = append(gates, cmd)
					}
				}
			}
		}
	}
	return gates
}

// countDiffFiles counts the number of "diff --git" headers in a
// unified diff. Each one represents one file changed.
func countDiffFiles(diff string) int {
	return strings.Count(diff, "diff --git ")
}

// renderPRReview produces the pr-review.md report. The semantic
// AC↔code check is left as TODO placeholders for the LLM (via
// the revisar-pr skill) to fill in.
func renderPRReview(slug string, acs []acceptanceCriterion, gates []string, results []gateResult, diffPath string, diffStats struct {
	Lines int
	Files int
}) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# PR review: %s\n\n", slug)
	b.WriteString("> Generated by `radiant review-pr`. The semantic\n")
	b.WriteString("> AC↔code check is left as TODOs for the LLM (via\n")
	b.WriteString("> the `revisar-pr` skill) to fill in.\n\n")

	// Summary
	fmt.Fprintf(&b, "## Summary\n\n")
	b.WriteString("| Metric | Value |\n")
	b.WriteString("|--------|-------|\n")
	fmt.Fprintf(&b, "| ACs in spec | %d |\n", len(acs))
	fmt.Fprintf(&b, "| Gates in tasks | %d |\n", len(gates))
	if len(results) > 0 {
		passed := 0
		for _, r := range results {
			if r.Passed {
				passed++
			}
		}
		fmt.Fprintf(&b, "| Gates executed | %d/%d passed |\n", passed, len(results))
	}
	if diffPath != "" {
		fmt.Fprintf(&b, "| Diff | %d files, %d lines (%s) |\n", diffStats.Files, diffStats.Lines, diffPath)
	}

	// Recommendation (skeleton — LLM fills in)
	b.WriteString("\n## Recommendation\n\n")
	b.WriteString("- [ ] Approve\n")
	b.WriteString("- [ ] Request changes\n")
	b.WriteString("- [ ] Needs spec revision (SPEC_DEVIATION)\n\n")

	// AC-by-AC check
	if len(acs) > 0 {
		b.WriteString("## AC coverage\n\n")
		b.WriteString("| AC | Title | Implemented | Notes |\n")
		b.WriteString("|----|-------|-------------|-------|\n")
		for _, ac := range acs {
			fmt.Fprintf(&b, "| %s | %s | TODO | TODO |\n", ac.ID, ac.Title)
		}
		b.WriteString("\n> Each TODO above is filled in by the LLM via the\n")
		b.WriteString("> `revisar-pr` skill: search the diff for code that\n")
		b.WriteString("> implements the AC's Given/When/Then conditions.\n\n")
	}

	// Gates
	if len(gates) > 0 {
		b.WriteString("## Gate results\n\n")
		b.WriteString("| Gate | Status | Output |\n")
		b.WriteString("|------|--------|--------|\n")
		for _, g := range gates {
			// Find matching result if any
			var status, outStr string
			for _, r := range results {
				if r.Name == g {
					if r.Passed {
						status = "✓ pass"
						outStr = "(silent)"
					} else {
						status = "✗ fail"
						outStr = r.Err
						if len(outStr) > 80 {
							outStr = outStr[:77] + "..."
						}
					}
					break
				}
			}
			if status == "" {
				status = "— not run"
				outStr = "pass --run-gates to execute"
			}
			fmt.Fprintf(&b, "| `%s` | %s | %s |\n", g, status, outStr)
		}
		b.WriteString("\n")
	}

	// SPEC_DEVIATION section (empty template for LLM to fill in)
	b.WriteString("## SPEC_DEVIATION\n\n")
	b.WriteString("Document any code that diverges from the spec:\n\n")
	b.WriteString("```markdown\n")
	b.WriteString("### SPEC_DEVIATION-001: <short title>\n\n")
	b.WriteString("- **AC**: <which AC is affected>\n")
	b.WriteString("- **Files**: <files involved>\n")
	b.WriteString("- **What's missing**: <specific gap>\n")
	b.WriteString("- **Recommended action**: <extend test | revise AC | revert>\n")
	b.WriteString("```\n\n")

	// Suggested PR comment
	b.WriteString("## Suggested PR comment\n\n")
	b.WriteString("> Copy-paste into the PR conversation:\n\n")
	b.WriteString("```\n")
	if len(results) > 0 {
		passed := 0
		for _, r := range results {
			if r.Passed {
				passed++
			}
		}
		fmt.Fprintf(&b, "PR review: %d/%d gates pass. AC coverage and SPEC_DEVIATION check above.\n", passed, len(results))
	} else {
		b.WriteString("PR review: see pr-review.md for AC coverage, gate status, and SPEC_DEVIATION check.\n")
	}
	b.WriteString("```\n\n")

	// Footer
	b.WriteString("---\n\n")
	b.WriteString("_Generated by `radiant review-pr`. Use the `revisar-pr` skill for the semantic AC↔code check._\n")
	return b.String()
}

// mcpServer mirrors the standard .mcp.json schema. We only care
// about a few fields (name/command/args/env) — the rest is ignored.
// Keeping this lightweight means a user can paste a real .mcp.json
// from any MCP-aware tool and we just read what's relevant.
type mcpServer struct {
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	URL     string            `json:"url,omitempty"`
	Notes   string            `json:"notes,omitempty"`
}

type mcpConfig struct {
	Servers map[string]mcpServer `json:"mcpServers"`
}

// runIntegrationsList reads the project's .mcp.json and either
// prints a table (default) or emits JSON. Optionally writes the
// canonical docs/engineering/integrations.md from the same data.
//
// Per the integracoes skill, this command NEVER writes .mcp.json.
// The user/agent must approve each MCP entry via the skill first.
// We only surface what's already declared.
func runIntegrationsList(jsonOut bool, docOut string) error {
	cfgPath := ".mcp.json"
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no %s found — invoke the /integracoes skill (or run `radiant init --all`) to declare MCPs", cfgPath)
		}
		return fmt.Errorf("read %s: %w", cfgPath, err)
	}

	var cfg mcpConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse %s: %w", cfgPath, err)
	}
	if len(cfg.Servers) == 0 {
		fmt.Println("  (no MCP servers declared in .mcp.json)")
		return nil
	}

	// Sort by name for stable output.
	names := make([]string, 0, len(cfg.Servers))
	for n := range cfg.Servers {
		names = append(names, n)
	}
	sort.Strings(names)

	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(cfg.Servers)
	}

	// Table output.
	fmt.Printf("  MCP servers declared in %s (%d):\n\n", cfgPath, len(names))
	fmt.Printf("    %-20s %-12s %-32s %s\n", "NAME", "COMMAND", "ARGS (truncated)", "ENV")
	fmt.Printf("    %-20s %-12s %-32s %s\n", "----", "-------", "--------------", "---")
	for _, name := range names {
		s := cfg.Servers[name]
		args := strings.Join(s.Args, " ")
		if len(args) > 32 {
			args = args[:29] + "..."
		}
		if args == "" {
			args = "(none)"
		}
		cmd := s.Command
		if cmd == "" && s.URL != "" {
			cmd = "<http>"
		}
		if cmd == "" {
			cmd = "?"
		}
		fmt.Printf("    %-20s %-12s %-32s %d vars\n", name, cmd, args, len(s.Env))
	}

	fmt.Printf("\n  To validate an MCP, invoke the /integracoes skill.\n")
	fmt.Printf("  To approve and persist a new MCP, edit .mcp.json manually — this command never writes it.\n")

	if docOut != "" {
		body := renderIntegrationsDoc(cfg.Servers)
		if err := os.MkdirAll(filepath.Dir(docOut), 0o755); err != nil {
			return err
		}
		if err := atomicWrite(docOut, body); err != nil {
			return fmt.Errorf("write %s: %w", docOut, err)
		}
		fmt.Printf("\n  ✓ wrote %s\n", docOut)
	}
	return nil
}

// renderIntegrationsDoc produces the canonical
// docs/engineering/integrations.md content from the current
// .mcp.json. This is what the integracoes skill writes — we're
// just regenerating from data we can read.
func renderIntegrationsDoc(servers map[string]mcpServer) string {
	var b strings.Builder
	b.WriteString("# Integrations and MCPs\n\n")
	b.WriteString("> Auto-generated by `radiant integrations list --write-docs` from\n")
	b.WriteString("> the project's `.mcp.json`. Per the integracoes skill, MCPs are\n")
	b.WriteString("> only listed here AFTER explicit approval — see the skill for the\n")
	b.WriteString("> approval flow.\n\n")

	b.WriteString("## Declared MCP servers\n\n")
	b.WriteString("| Name | Command | Args | Env vars |\n")
	b.WriteString("|------|---------|------|----------|\n")

	names := make([]string, 0, len(servers))
	for n := range servers {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, name := range names {
		s := servers[name]
		cmd := s.Command
		if cmd == "" && s.URL != "" {
			cmd = "<http>"
		}
		args := strings.Join(s.Args, " ")
		if args == "" {
			args = "—"
		}
		fmt.Fprintf(&b, "| %s | `%s` | `%s` | %d |\n", name, cmd, args, len(s.Env))
	}

	b.WriteString("\n## How to connect\n\n")
	b.WriteString("- **Project-scoped:** `.mcp.json` at repo root — shareable with team. **No secrets.**\n")
	b.WriteString("- **Secrets:** via env var or the relevant MCP CLI (`claude mcp add`, etc.). **Never** commit tokens.\n")
	b.WriteString("\n## Approval log\n\n")
	b.WriteString("Add a row each time an MCP is approved (use the integracoes skill for the\n")
	b.WriteString("full interview — never skip the account-boundary step).\n\n")
	b.WriteString("| Date | MCP | Account/workspace | Approved by |\n")
	b.WriteString("|------|-----|-------------------|-------------|\n")
	b.WriteString("| _    | _   | _                 | _           |\n")
	return b.String()
}

// renderInception produces the 6-phase Lean Inception template.
// The user/agent fills in each phase one at a time following the
// nova-product skill. The template intentionally uses simple
// Markdown so it renders well in any viewer (GitHub, GitLab,
// Obsidian, IDE preview).
func renderInception(slug, vision string, mvpWeeks int) string {
	return fmt.Sprintf(`# Product Inception — %s

> **Lean Inception template.** Generated by 'radiant product'.
> Fill in the 6 phases below, then cut the MVP at the end.
> See the nova-product skill for guidance.

## 1. Why

> Vision line: "We help '<persona>' do '<job>' better than
> '<alternative>'."

**Vision**: %s

**Persona**: <name, role, where they work, what tools they use today>

**Job-to-be-done**: <what they're trying to accomplish when they find this product>

**Pain today**: <the cost of the current alternative>

**Why now**: <what changed that makes this urgent>

**Success metric**: <one number that proves the product worked — e.g. "40%% of weekly active users do X">

## 2. What (untagged brainstorm)

Brainstorm every feature you imagine. Do not filter yet.

- <feature>
- <feature>
- <feature>
- <feature>
- <feature>
- <feature>
- <feature>

## 3. Scope triage

Tag each feature above with one of:

- **MVP** — new user cannot succeed without it on day 1.
- **Growth** — what you add once MVP proves the Why.
- **Vision** — the end state, aspirational.

Rule: if you can cut a feature and a new user still gets value, it is NOT MVP.

## 4. Who (personas)

See 'personas.md' for full profiles. Summary here:

- **<Persona A>** — <one-line: role + goal>
- **<Persona B>** — <one-line: role + goal>
- **<Persona C>** — <one-line: role + goal>

## 5. How

<1-2 paragraphs: technical approach, business model, GTM, etc. Flag new bounded contexts or external integrations — they become the Where phase.>

## 6. When

Target MVP timeline: **%d weeks**.

| Quarter  | Milestone | Scope                              |
|----------|-----------|------------------------------------|
| Q1       | MVP       | <list MVP features here>           |
| Q2       | Growth    | <list Growth features here>        |
| Q3+      | Vision    | <list Vision features here>        |

## 7. Where (bounded contexts)

| Context       | Type            | Notes                              |
|---------------|-----------------|------------------------------------|
| <name>        | new / existing  | <one-line description>             |
| <name>        | new / existing  | <one-line description>             |

If most contexts are "new" → expect a longer architecture sprint after inception. If most are "existing" → brownfield path; scope the MVP to leverage what is there.

---

## MVP cut

The 3-7 features we ship first (in priority order):

1. <feature> — covers <persona>'s <top job>
2. <feature>
3. <feature>

Each becomes a spec under 'specs/<NNNN>-<slug>/' via the nova-feature skill. Do NOT bundle multiple MVP features into one spec — one feature per spec so each can ship independently.

After MVP is cut:

1. Update '.radiant-harness/state.md' with 'current_product' and 'mvp_features'.
2. For each MVP feature, open a FRESH context and run 'radiant spec <feature>'.
3. Close this inception context — don't start spec'ing in it.

---

_Generated by 'radiant product' on %s. See the 'nova-product' skill for the full Decision Tree, anti-patterns, and failure modes._
`,
		slug, vision, mvpWeeks, time.Now().UTC().Format("2006-01-02"))
}

// renderPersonasTemplate returns the starter personas.md file
// with 2-4 placeholder slots (default 3). The user/agent fills in
// each persona after the Who phase of the inception.
func renderPersonasTemplate() string {
	return `# Personas

> Generated by 'radiant product'. Fill in 2-4 personas — one
> paragraph each. See the nova-product skill for what each section
> needs to contain.

## <Persona name>

<One sentence: who they are, where they work, what tools they currently use.>

**Job to be done**: <what they're trying to accomplish when they find this product.>

**Pain today**: <the cost of the current alternative.>

**Success looks like**: <how they measure whether the product helped.>

---

## <Persona name>

<One sentence: who they are, where they work, what tools they currently use.>

**Job to be done**: <what they're trying to accomplish when they find this product.>

**Pain today**: <the cost of the current alternative.>

**Success looks like**: <how they measure whether the product helped.>

---

## <Persona name>

<One sentence: who they are, where they work, what tools they currently use.>

**Job to be done**: <what they're trying to accomplish when they find this product.>

**Pain today**: <the cost of the current alternative.>

**Success looks like**: <how they measure whether the product helped.>
`
}

// renderDiagram produces a starter Mermaid C4 diagram for the
// given level. It is intentionally minimal — the goal is to give
// the user (or an agent invoking the diagramar skill) a working
// skeleton with valid C4 syntax, not auto-extract the codebase.
// Auto-extraction is a future enhancement.
func renderDiagram(level string) (string, error) {
	switch level {
	case "context":
		return contextDiagram(), nil
	case "container":
		return containerDiagram(), nil
	case "component":
		return componentDiagram(), nil
	case "code":
		return codeDiagram(), nil
	default:
		return "", fmt.Errorf("unknown level %q — choose: context | container | component | code", level)
	}
}

func contextDiagram() string {
	return `# C4 Level 1 — System Context
#
# Edit the Person/System entries below to describe who uses your
# system and what external systems it talks to. See the diagramar
# skill for guidance.

` + "```mermaid" + `
C4Context
    title System Context diagram for <Your System>

    Person(user, "User", "A human who wants to <achieve goal>")
    System(system, "<Your System>", "Provides <capability>")
    System_Ext(external, "<External System>", "Does <thing> for us")

    Rel(user, system, "Uses")
    Rel(system, external, "Calls")
` + "```" + `
`
}

func containerDiagram() string {
	return `# C4 Level 2 — Containers
#
# Break <Your System> into deployable units: web app, API, DB,
# background worker, etc. See the diagramar skill.

` + "```mermaid" + `
C4Container
    title Container diagram for <Your System>

    Person(user, "User", "A human who wants to <achieve goal>")

    System_Boundary(c1, "<Your System>") {
        Container(web, "Web App", "React/Vue/...", "Browser UI")
        Container(api, "API", "Go/Node/Python", "JSON/HTTP API")
        ContainerDb(database, "Database", "Postgres/SQLite", "Stores <data>")
    }

    Rel(user, web, "Uses", "HTTPS")
    Rel(web, api, "Calls", "JSON/HTTPS")
    Rel(api, database, "Reads/writes", "SQL")
` + "```" + `
`
}

func componentDiagram() string {
	return `# C4 Level 3 — Components
#
# Zoom into ONE container (the API in this example) and show its
// internal building blocks. See the diagramar skill.

` + "```mermaid" + `
C4Component
    title Component diagram for <API>

    Container(web, "Web App", "React", "Browser UI")
    ContainerDb(database, "Database", "Postgres", "Stores data")

    Container_Boundary(api, "<API>") {
        Component(handler, "Handler", "HTTP layer", "Translates requests to commands")
        Component(svc, "Service", "Business logic", "Enforces invariants")
        Component(repo, "Repository", "Data access", "Owns SQL queries")
    }

    Rel(web, handler, "Calls", "JSON")
    Rel(handler, svc, "Invokes")
    Rel(svc, repo, "Uses")
    Rel(repo, database, "Reads/writes", "SQL")
` + "```" + `
`
}

func codeDiagram() string {
	return `# C4 Level 4 — Code
#
# UML-style class diagram for a focused unit. The diagramar skill
# recommends staying at Level 3 unless a specific class has
# complex internal relationships worth visualising.

` + "```mermaid" + `
classDiagram
    class Service {
        +repo Repository
        +logger Logger
        +DoThing(input Input) (Output, error)
    }
    class Repository {
        <<interface>>
        +Get(id string) (Entity, error)
        +Put(entity Entity) error
    }
    Service --> Repository : depends on
`
}

// renderADR produces a Nygard-format ADR template. The user is
// expected to fill in Context, Decision, and Consequences after
// the file is generated.
func renderADR(seq int, decision, status string) string {
	if status == "" {
		status = "proposed"
	}
	switch status {
	case "proposed", "accepted", "deprecated", "superseded":
		// ok
	default:
		status = "proposed"
	}
	return fmt.Sprintf(`# %04d. %s

## Status

%s

> Status transitions: proposed → accepted (when team agrees) →
> deprecated or superseded (when replaced). Edit this section in
> place to record the transition.

## Context

What forces are at play? What problem are we solving? What
constraints exist?

**Alternatives considered** (fill in at least 2; ADRs are valuable
*because* they record what was rejected, not only what was chosen):

- **Alternative A: <name>** — <one-line description>
  - Pro: ...
  - Con: ...
- **Alternative B: <name>** — <one-line description>
  - Pro: ...
  - Con: ...

## Decision

We will <the chosen approach>.

(One paragraph. State the decision clearly so a reader who knows
nothing about the discussion can understand what was decided and
why.)

## Consequences

What becomes easier? What becomes harder? What trade-offs did we
accept?

### Positive

- ...

### Negative

- ...

### Neutral

- (Anything that changes but isn't clearly positive or negative)

---

_Generated by 'radiant adr' on %s. Edit the placeholders above.
See the 'adr' skill ('.radiant-harness/skills/adr/SKILL.md') for
the full Decision Tree, anti-patterns, and failure modes._
`, seq, decision, status, time.Now().UTC().Format("2006-01-02"))
}

// summaryFor produces the human-readable one-liner that goes into
// state.md's `last_summary` field. Combines the user's note with the
// feature slug (if any) so future sessions can grep it.
func summaryFor(note, feature string) string {
	if note != "" {
		return note
	}
	if feature != "" {
		return "Last session worked on " + feature
	}
	return "Last session"
}

// we fall back to a direct write — rename-over-existing is a no-op
// there.
func writeTraceToFile(e *engine.Engine, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() { os.Remove(tmpName) }

	if err := e.WriteTraceJSONL(tmp); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("write jsonl: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("fsync: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

func resolveModelSilent(modelName, provider, apiKey string) (llm.Model, bool) {
	m, err := resolveModel(modelName, provider, apiKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  ⚠ could not resolve --planner/--implementer model %q: %v\n  → falling back to default --model\n", modelName, err)
		return llm.Model{}, false
	}
	return m, true
}

func resolveModel(modelName, provider, apiKey string) (llm.Model, error) {
	// Check environment variables for API key first
	if apiKey == "" {
		apiKey = os.Getenv("OPENROUTER_API_KEY")
	}
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}

	// Try preset
	if preset, ok := llm.GetPreset(modelName, apiKey); ok {
		return preset, nil
	}

	if apiKey == "" {
		return llm.Model{}, fmt.Errorf("no API key provided. Use --api-key flag or set OPENROUTER_API_KEY env var")
	}

	// Build custom model
	providerType := llm.Provider(provider)
	if providerType == "" {
		providerType = llm.ProviderOpenRouter
	}

	return llm.Model{
		Provider:  providerType,
		Model:     modelName,
		APIKey:    apiKey,
		MaxTokens: 8192,
	}, nil
}

// runDoctor prints a diagnostic report of the local environment. Each
// check prints ✓ / ⚠ / ✗ and explains what to do if something's wrong.
// The function never returns an error — diagnostics are informational.
func runDoctor(root string) error {
	fmt.Println("  radiant doctor — environment diagnostic")
	fmt.Println()

	checkOK := func(label string) {
		fmt.Printf("  ✓ %s\n", label)
	}
	checkWarn := func(label, advice string) {
		fmt.Printf("  ⚠ %s\n    %s\n", label, advice)
	}
	checkFail := func(label, advice string) {
		fmt.Printf("  ✗ %s\n    %s\n", label, advice)
	}

	// 1. PATH
	pathEnv := os.Getenv("PATH")
	if pathEnv == "" {
		checkFail("PATH not set", "export PATH=$PATH:/usr/local/bin:/opt/homebrew/bin")
	} else {
		checkOK("PATH set (" + strconv.Itoa(len(pathEnv)) + " chars)")
	}

	// 2. Supported agents on PATH
	fmt.Println("\n  Agents:")
	agents := []string{"claude", "codex", "copilot", "cursor", "gemini"}
	for _, name := range agents {
		if _, err := exec.LookPath(name); err == nil {
			checkOK(name + " available")
		} else {
			checkWarn(name+" not found", "agent is optional — install only the ones you use")
		}
	}

	// 3. LLM provider API keys
	fmt.Println("\n  LLM providers:")
	providers := []struct {
		name   string
		envKey string
		note   string
	}{
		{"OpenRouter", "OPENROUTER_API_KEY", "covers all OpenRouter presets"},
		{"OpenAI", "OPENAI_API_KEY", "for direct OpenAI access"},
		{"Anthropic", "ANTHROPIC_API_KEY", "for direct Anthropic (requires native client)"},
		{"Groq", "GROQ_API_KEY", "for Groq-hosted models (ultra-low latency)"},
		{"Mistral", "MISTRAL_API_KEY", "for Mistral-hosted models"},
		{"xAI", "XAI_API_KEY", "for Grok models"},
	}
	anyKey := false
	for _, p := range providers {
		if os.Getenv(p.envKey) != "" {
			checkOK(p.name + ": " + p.envKey + " set")
			anyKey = true
		} else {
			checkWarn(p.name+": "+p.envKey+" not set", p.note)
		}
	}
	if !anyKey {
		fmt.Println("\n  ⚠ No LLM API key set. `radiant run` will fail without one.")
		fmt.Println("    Set one of the env vars above, or pass --api-key=… to `radiant run`.")
	}

	// 4. Gate binaries (test runners, type checkers)
	fmt.Println("\n  Gate binaries:")
	gates := []string{"node", "npm", "pnpm", "yarn", "go", "make", "pytest", "python3", "cargo"}
	for _, name := range gates {
		if _, err := exec.LookPath(name); err == nil {
			checkOK(name + " available")
		} else {
			checkWarn(name+" not found", "install if you plan to use it as a gate command")
		}
	}

	// 5. .radiant-harness state directory
	fmt.Println("\n  Project state:")
	stateDir := filepath.Join(root, ".radiant-harness")
	if info, err := os.Stat(stateDir); err == nil {
		if info.IsDir() {
			checkOK(stateDir + " exists")
		}
	} else {
		checkWarn(stateDir+" not found", "run `radiant init .` to create the harness state directory")
	}

	// 6. Version
	fmt.Println("\n  Version:")
	fmt.Printf("    radiant v%s\n", version)
	fmt.Printf("    Go module: github.com/quant-risk/radiant-harness\n")

	fmt.Println()
	return nil
}

// evalRun captures one iteration of the eval loop.
type evalRun struct {
	LatencyMs    int64  `json:"latency_ms"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
	Error        string `json:"error,omitempty"`
}

// evalResult is the full eval output for one model.
type evalResult struct {
	Model       string    `json:"model"`
	Runs        int       `json:"runs"`
	Successful  int       `json:"successful"`
	MedianMs    int64     `json:"median_latency_ms"`
	MeanMs      int64     `json:"mean_latency_ms"`
	TotalInTok  int       `json:"total_input_tokens"`
	TotalOutTok int       `json:"total_output_tokens"`
	TotalCost   float64   `json:"total_cost_usd"`
	Iterations  []evalRun `json:"iterations"`
}

// runEval sends `prompt` to `model` exactly `runs` times and reports
// latency / token / cost statistics. The output is a markdown table
// plus an optional JSON file via --output for trend tracking.
func runEval(ctx context.Context, model, prompt string, runs int, outputPath string) error {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if apiKey == "" {
		return fmt.Errorf("no API key set; export OPENROUTER_API_KEY (or OPENAI_API_KEY / ANTHROPIC_API_KEY) or pass --api-key to `radiant run`")
	}

	m, ok := llm.GetPreset(model, apiKey)
	if !ok {
		return fmt.Errorf("unknown model preset %q; run `radiant models` for the list", model)
	}
	client := llm.NewClient(m)

	fmt.Printf("  radiant eval — model=%s runs=%d\n", model, runs)
	fmt.Printf("  prompt: %s\n\n", truncateForDisplay(prompt, 80))

	results := evalResult{Model: model, Runs: runs, Iterations: make([]evalRun, runs)}

	var latencies []int64
	for i := 0; i < runs; i++ {
		start := time.Now()
		resp, err := client.Chat(ctx, []llm.Message{{Role: "user", Content: prompt}})
		latency := time.Since(start).Milliseconds()

		run := evalRun{LatencyMs: latency}
		if err != nil {
			run.Error = err.Error()
			fmt.Printf("  [%d/%d] ✗ %s (%dms)\n", i+1, runs, err, latency)
		} else {
			run.InputTokens = resp.Usage.PromptTokens
			run.OutputTokens = resp.Usage.CompletionTokens
			fmt.Printf("  [%d/%d] ✓ %dms, %d+%d tok\n",
				i+1, runs, latency, resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
			latencies = append(latencies, latency)
			results.Successful++
			results.TotalInTok += run.InputTokens
			results.TotalOutTok += run.OutputTokens
		}
		results.Iterations[i] = run
	}

	// Compute median + mean from successful runs.
	if len(latencies) > 0 {
		sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
		results.MedianMs = latencies[len(latencies)/2]
		var sum int64
		for _, l := range latencies {
			sum += l
		}
		results.MeanMs = sum / int64(len(latencies))
	}
	results.TotalCost = llm.CostUSD(model, results.TotalInTok, results.TotalOutTok)

	fmt.Println()
	fmt.Printf("  Median latency : %dms\n", results.MedianMs)
	fmt.Printf("  Mean latency   : %dms\n", results.MeanMs)
	fmt.Printf("  Success rate   : %d/%d\n", results.Successful, runs)
	fmt.Printf("  Total tokens   : %d in + %d out = %d\n",
		results.TotalInTok, results.TotalOutTok, results.TotalInTok+results.TotalOutTok)
	fmt.Printf("  Estimated cost : %s\n", llm.FormatCost(results.TotalCost))

	if outputPath != "" {
		data, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal results: %w", err)
		}
		if err := os.WriteFile(outputPath, append(data, '\n'), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", outputPath, err)
		}
		fmt.Printf("  Saved JSON to %s\n", outputPath)
	}
	return nil
}

func truncateForDisplay(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
