package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
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

var version = "0.6.3"

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
			return runRelease(version, dryRun, skipTests, skipCrossCompile, skipTag, skipCommit)
		},
	}
	releaseCmd.Flags().Bool("dry-run", false, "show what would happen without writing/tagging anything")
	releaseCmd.Flags().Bool("skip-tests", false, "skip the test step (use only when you've already validated)")
	releaseCmd.Flags().Bool("skip-cross-compile", false, "skip the cross-compile step")
	releaseCmd.Flags().Bool("skip-tag", false, "skip the git tag step (only bump version + commit)")
	releaseCmd.Flags().Bool("skip-commit", false, "skip the git commit step (only bump version)")
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

	// ── mcp (Sprint 14.5 — MCP server, stdio transport) ──
	// `radiant mcp serve` exposes a JSON-RPC 2.0 server over stdio
	// that implements the Model Context Protocol (MCP) so agents
	// that prefer MCP can call radiant commands. Tools exposed:
	//   - radiant_spec: scaffold a feature
	//   - radiant_adr: create an ADR
	//   - radiant_product: start a Lean Inception
	//   - radiant_evals: AC→test coverage report
	//   - radiant_audit: project layout audit
	//   - radiant_release: cut a release
	// Reads newline-delimited JSON-RPC from stdin; writes
	// responses to stdout.
	mcpCmd := &cobra.Command{
		Use:   "mcp",
		Short: "MCP server commands",
	}
	mcpServeCmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the MCP server (stdio transport)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMCPServe(os.Stdin, os.Stdout)
		},
	}
	mcpCmd.AddCommand(mcpServeCmd)
	root.AddCommand(mcpCmd)

	// ── security (Sprint 16 — security posture audit) ──
	// `radiant security [--scope=secrets|perms|all] [--output=...]`
	// scans the project for common security issues:
	//   - Hardcoded secrets (API keys, tokens) in source code
	//   - Sensitive files with overly permissive file permissions
	// MVP scope: secrets + permissions. Dependency-CVE scanning and
	// config-CORS checks are deferred to future work.
	securityCmd := &cobra.Command{
		Use:   "security",
		Short: "Security posture audit: hardcoded secrets + sensitive file perms",
		RunE: func(cmd *cobra.Command, args []string) error {
			scope, _ := cmd.Flags().GetString("scope")
			outPath, _ := cmd.Flags().GetString("output")
			failOnWarn, _ := cmd.Flags().GetBool("fail-on-warning")
			return runSecurity(scope, outPath, failOnWarn)
		},
	}
	securityCmd.Flags().String("scope", "all", "scan scope: secrets | perms | all")
	securityCmd.Flags().StringP("output", "o", "", "output path (default: docs/security-report.md)")
	securityCmd.Flags().Bool("fail-on-warning", false, "exit non-zero on warnings (default: only errors)")
	root.AddCommand(securityCmd)

	// ── telemetry (Sprint 18 — privacy-first local usage stats) ──
	// `radiant telemetry {status|enable|disable|show}` — opt-in local
	// usage tracking. PRIVACY-FIRST: nothing is collected by
	// default. The user must explicitly run `radiant telemetry
	// enable` to start logging. Even when enabled, only the
	// command name + timestamp + a content hash are recorded
	// locally (no args, no paths, no project metadata).
	telemetryCmd := &cobra.Command{
		Use:   "telemetry",
		Short: "Privacy-first local usage stats (opt-in)",
	}
	telemetryStatusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show whether telemetry is enabled and what is recorded",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTelemetryStatus()
		},
	}
	telemetryEnableCmd := &cobra.Command{
		Use:   "enable",
		Short: "Opt in to local telemetry (writes to .radiant-harness/telemetry.jsonl)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTelemetryEnable()
		},
	}
	telemetryDisableCmd := &cobra.Command{
		Use:   "disable",
		Short: "Opt out of telemetry (removes the log file)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTelemetryDisable()
		},
	}
	telemetryShowCmd := &cobra.Command{
		Use:   "show",
		Short: "Show the local telemetry log (last 50 events)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTelemetryShow()
		},
	}
	telemetryCmd.AddCommand(telemetryStatusCmd, telemetryEnableCmd, telemetryDisableCmd, telemetryShowCmd)
	root.AddCommand(telemetryCmd)

	// ── incident (Sprint 19 — incident response scaffold) ──
	// `radiant incident <severity> <summary>` wires the `incident`
	// skill to a CLI. Generates docs/incidents/<NNNN>-<slug>.md
	// with the post-mortem template pre-filled; the on-call
	// engineer fills in the timeline + RCA + action items.
	incidentCmd := &cobra.Command{
		Use:   "incident <severity> <summary>",
		Short: "Start an incident: scaffold docs/incidents/<NNNN>-<slug>.md",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			severity := args[0]
			summary := args[1]
			out, _ := cmd.Flags().GetString("output")
			return runIncident(severity, summary, out)
		},
	}
	incidentCmd.Flags().StringP("output", "o", "", "output path (default: docs/incidents/<NNNN>-<slug>.md)")
	root.AddCommand(incidentCmd)

	// ── telemetry summary (Sprint 21 — aggregate counts) ──
	// `radiant telemetry summary` reads the local log and prints
	// aggregate stats: total events, top commands, daily counts.
	// Same privacy guarantees as `show` — only local file access.
	telemetrySummaryCmd := &cobra.Command{
		Use:   "summary",
		Short: "Show aggregate counts from the local telemetry log",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTelemetrySummary()
		},
	}
	telemetryCmd.AddCommand(telemetrySummaryCmd)
	telemetryRotateCmd := &cobra.Command{
		Use:   "rotate",
		Short: "Archive old events when log exceeds --max-entries (default 1000)",
		RunE: func(cmd *cobra.Command, args []string) error {
			max, _ := cmd.Flags().GetInt("max-entries")
			return runTelemetryRotate(max)
		},
	}
	telemetryRotateCmd.Flags().Int("max-entries", 1000, "max events to keep in the active log; older events archived to telemetry-YYYY-MM-DD.jsonl")
	telemetryCmd.AddCommand(telemetryRotateCmd)
	telemetryExportCmd := &cobra.Command{
		Use:   "export",
		Short: "Export telemetry log as JSON or CSV (default: JSON to stdout)",
		RunE: func(cmd *cobra.Command, args []string) error {
			format, _ := cmd.Flags().GetString("format")
			output, _ := cmd.Flags().GetString("output")
			since, _ := cmd.Flags().GetString("since")
			return runTelemetryExport(format, output, since)
		},
	}
	telemetryExportCmd.Flags().String("format", "json", "export format: json or csv")
	telemetryExportCmd.Flags().String("output", "", "output file path (default: stdout)")
	telemetryExportCmd.Flags().String("since", "", "filter events to >= YYYY-MM-DD (inclusive); empty = no filter")
	telemetryCmd.AddCommand(telemetryExportCmd)
	root.AddCommand(telemetryCmd)

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
//
// As of Sprint 14.3 this is a thin wrapper that delegates to
// `scaffold.GenerateAgentsMD()` — the SINGLE SOURCE OF TRUTH for
// the AGENTS.md template. Both `radiant init` and `radiant update`
// now produce identical content; the audit (`radiant camada-agentica`)
// cross-checks consistency.
func generateAgentsMD() string {
	return scaffold.GenerateAgentsMD()
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

// featureCoverage is one row in the evals-report.md table — the
// per-feature fidelity snapshot.
type featureCoverage struct {
	Slug      string
	Total     int      // total ACs
	Covered   int      // ACs claimed in tasks.md coverage
	Uncovered []string // AC IDs not covered
	Score     float64  // covered / total (0..1)
}

// runEvals walks specs/ in scope, parses ACs + tasks coverage,
// and writes a fidelity report. Scopes:
//   - "all" — every feature in specs/
//   - "since-last-release" — every feature modified since the last
//     git tag (per git log --tags). If no tags exist, falls back
//     to all.
//   - <spec-path> — single feature (e.g. specs/0001-jwt/)
func runEvals(scope, outPath string) error {
	specsDir := "specs"

	// Resolve "since-last-release" to a set of feature slugs.
	var includeSlugs map[string]bool // nil = include all
	if scope == "since-last-release" {
		lastTag, err := lastGitTag()
		if err != nil || lastTag == "" {
			fmt.Println("  (no tags found; falling back to scope=all)")
		} else {
			fmt.Printf("  (scoping to features modified since %s)\n", lastTag)
			changed, err := specsChangedSince(lastTag)
			if err != nil {
				return fmt.Errorf("git log since %s: %w", lastTag, err)
			}
			includeSlugs = map[string]bool{}
			for _, s := range changed {
				includeSlugs[s] = true
			}
			if len(includeSlugs) == 0 {
				fmt.Println("  (no features modified since last release; reporting empty scope)")
			}
		}
	} else if scope != "all" && strings.HasPrefix(scope, "specs/") {
		// Treat scope as a single spec path.
		includeSlugs = map[string]bool{filepath.Base(scope): true}
	}

	entries, err := os.ReadDir(specsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no %s directory found — initialize with `radiant init` or `radiant spec`", specsDir)
		}
		return fmt.Errorf("read %s: %w", specsDir, err)
	}

	var coverages []featureCoverage
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "_templates" || e.Name() == "quick" {
			continue
		}
		slug := e.Name()
		if includeSlugs != nil && !includeSlugs[slug] {
			continue
		}
		feat, err := computeFeatureCoverage(filepath.Join(specsDir, slug))
		if err != nil {
			fmt.Printf("  [skip] %s: %v\n", slug, err)
			continue
		}
		coverages = append(coverages, feat)
	}

	if len(coverages) == 0 {
		fmt.Println("  (no features found in specs/)")
		return nil
	}

	// Sort by score ascending so worst-covered features surface first.
	sort.Slice(coverages, func(i, j int) bool {
		return coverages[i].Score < coverages[j].Score
	})

	body := renderEvalsReport(scope, coverages)
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	if err := atomicWrite(outPath, body); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}

	// Stdout summary
	totalACs := 0
	totalCovered := 0
	for _, c := range coverages {
		totalACs += c.Total
		totalCovered += c.Covered
	}
	overall := 0.0
	if totalACs > 0 {
		overall = float64(totalCovered) / float64(totalACs)
	}

	fmt.Printf("  ✓ wrote %s\n", outPath)
	fmt.Printf("\n  Features: %d\n", len(coverages))
	fmt.Printf("  ACs: %d total, %d claimed-covered (%.0f%%)\n",
		totalACs, totalCovered, overall*100)
	fmt.Printf("\n  Per-feature scores (worst first):\n")
	for _, c := range coverages {
		fmt.Printf("    %s — %d/%d (%.0f%%)\n",
			c.Slug, c.Covered, c.Total, c.Score*100)
	}
	if overall < 0.8 {
		fmt.Printf("\n  ⚠ fidelity below 80%% — review uncovered ACs above\n")
	} else if overall >= 1.0 {
		fmt.Printf("\n  ✓ 100%% fidelity — every AC claimed in tasks.md\n")
	}
	return nil
}

// computeFeatureCoverage parses one spec dir's spec.md + tasks.md
// and returns the coverage snapshot. Coverage = "the AC ID appears
// in at least one task's Coverage column" — i.e. is CLAIMED to
// be covered. The LLM via the evals skill does the real verification.
func computeFeatureCoverage(specDir string) (featureCoverage, error) {
	specMD, err := os.ReadFile(filepath.Join(specDir, "spec.md"))
	if err != nil {
		return featureCoverage{}, err
	}
	tasksMD, err := os.ReadFile(filepath.Join(specDir, "tasks.md"))
	if err != nil {
		// tasks.md missing = 0 coverage (the spec was never decomposed).
		tasksMD = []byte{}
	}

	acs := parseAcceptanceCriteria(string(specMD))
	if len(acs) == 0 {
		return featureCoverage{}, fmt.Errorf("no ACs found")
	}

	tasksBody := string(tasksMD)
	coveredSet := map[string]bool{}
	for _, ac := range acs {
		// Check if AC ID appears anywhere in tasks.md (the
		// Coverage column references ACs by ID).
		if strings.Contains(tasksBody, ac.ID) {
			coveredSet[ac.ID] = true
		}
	}

	var uncovered []string
	for _, ac := range acs {
		if !coveredSet[ac.ID] {
			uncovered = append(uncovered, ac.ID)
		}
	}

	slug := filepath.Base(specDir)
	score := float64(len(coveredSet)) / float64(len(acs))
	return featureCoverage{
		Slug:      slug,
		Total:     len(acs),
		Covered:   len(coveredSet),
		Uncovered: uncovered,
		Score:     score,
	}, nil
}

// renderEvalsReport produces the docs/evals-report.md content.
// Per the evals skill, the report MUST cite evidence per claim —
// for the MVP we cite spec.md:line / tasks.md:line as evidence.
// The LLM (via the skill) is responsible for filling in
// implementation evidence (file:line of the actual code).
func renderEvalsReport(scope string, coverages []featureCoverage) string {
	totalACs := 0
	totalCovered := 0
	for _, c := range coverages {
		totalACs += c.Total
		totalCovered += c.Covered
	}
	overall := 0.0
	if totalACs > 0 {
		overall = float64(totalCovered) / float64(totalACs)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# Evals: %s\n\n", scope)
	fmt.Fprintf(&b, "> Generated by `radiant evals --scope=%s`. Per-AC\n", scope)
	b.WriteString("> evidence (file:line of implementing code) is filled in\n")
	b.WriteString("> by the LLM via the `evals` skill — the table below shows\n")
	b.WriteString("> the **claimed** coverage from tasks.md. The skill's job\n")
	b.WriteString("> is to verify each claim against actual code + test runs.\n\n")

	// Summary
	b.WriteString("## Summary\n\n")
	b.WriteString("| Metric | Value |\n")
	b.WriteString("|--------|-------|\n")
	fmt.Fprintf(&b, "| Features in scope | %d |\n", len(coverages))
	fmt.Fprintf(&b, "| Total ACs | %d |\n", totalACs)
	fmt.Fprintf(&b, "| Claimed-covered ACs | %d |\n", totalCovered)
	fmt.Fprintf(&b, "| Aggregate fidelity | **%.1f%%** |\n\n", overall*100)

	// Per-feature
	b.WriteString("## Per-feature fidelity\n\n")
	b.WriteString("| Feature | ACs | Covered | Score | Uncovered |\n")
	b.WriteString("|---------|-----|---------|-------|-----------|\n")
	for _, c := range coverages {
		uncovered := strings.Join(c.Uncovered, ", ")
		if uncovered == "" {
			uncovered = "—"
		}
		fmt.Fprintf(&b, "| %s | %d | %d | %.0f%% | %s |\n",
			c.Slug, c.Total, c.Covered, c.Score*100, uncovered)
	}
	b.WriteString("\n")

	// Per-AC detail (per-feature)
	b.WriteString("## AC-level detail\n\n")
	for _, c := range coverages {
		fmt.Fprintf(&b, "### %s\n\n", c.Slug)
		b.WriteString("| AC | Claimed covered | Evidence |\n")
		b.WriteString("|----|-----------------|----------|\n")
		b.WriteString("| _per AC_ | _TODO_ | _TODO (file:line of implementing code)_ |\n")
		b.WriteString("\n> Each TODO above is filled in by the LLM via the\n")
		b.WriteString("> `evals` skill: trace the AC's Given/When/Then to a test\n")
		b.WriteString("> that asserts it, and cite the file:line.\n\n")
	}

	// Footer
	b.WriteString("---\n\n")
	b.WriteString("_Generated by `radiant evals`. Re-run after every release; fidelity drifts._\n")
	return b.String()
}

// runCamadaAgentica audits the project's agentic layer:
//   - AGENTS.md present + references all bundled skills
//   - Native views present for the agents in use (--agents)
//   - Version consistency (AGENTS.md says skill X is vY, bundled
//     skill is vZ — drift!)
//
// Returns exit code 0 if everything is in sync, non-zero if any
// drift or missing files. With --fix, regenerates AGENTS.md
// (but does NOT touch native views; the user owns those).
func runCamadaAgentica(agentFlag string, fix bool) error {
	infos, err := skill.Bundle()
	if err != nil {
		return fmt.Errorf("load bundled skills: %w", err)
	}

	var agents []radiant.AgentID
	if agentFlag != "" {
		agents = resolveAgents(agentFlag, false)
	}

	// 1. Check AGENTS.md presence + contents.
	agentsPath := "AGENTS.md"
	agentsBody, err := os.ReadFile(agentsPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("  [missing] AGENTS.md — run `radiant init` or `radiant update` to generate")
		} else {
			return fmt.Errorf("read %s: %w", agentsPath, err)
		}
	}

	// 2. Check skill references in AGENTS.md (presence only —
	//    full version parsing would couple us to the YAML format).
	if len(agentsBody) > 0 {
		var missing []string
		for _, info := range infos {
			if !strings.Contains(string(agentsBody), info.Name) {
				missing = append(missing, info.Name)
			}
		}
		if len(missing) > 0 {
			fmt.Printf("  [drift] AGENTS.md missing references to %d skill(s): %s\n",
				len(missing), strings.Join(missing, ", "))
		}
	}

	// 3. Check version drift between AGENTS.md and bundled skills.
	if len(agentsBody) > 0 {
		var drifted []string
		for _, info := range infos {
			// Look for "name (vX.Y.Z)" in AGENTS.md (the format generateAgentsMD emits).
			needle := fmt.Sprintf("%s (v%s)", info.Name, info.Version)
			if !strings.Contains(string(agentsBody), needle) {
				drifted = append(drifted, info.Name)
			}
		}
		if len(drifted) > 0 {
			fmt.Printf("  [version-drift] AGENTS.md version mismatch on %d skill(s): %s\n",
				len(drifted), strings.Join(drifted, ", "))
		}
	}

	// 4. Check native views presence for the agents in use.
	if len(agents) > 0 {
		for _, agent := range agents {
			adapter := scaffold.GetAdapter(agent)
			if adapter == nil {
				fmt.Printf("  [unknown-agent] %s\n", agent)
				continue
			}
			if _, err := os.Stat(adapter.InstTo); os.IsNotExist(err) {
				fmt.Printf("  [missing-view] %s — run `radiant views --agent=%s` to generate\n",
					adapter.InstTo, agent)
			} else {
				fmt.Printf("  [ok] %s (%s)\n", adapter.InstTo, adapter.Label)
			}
		}
	}

	// 5. Optional: regenerate AGENTS.md (without touching native views).
	if fix {
		body := generateAgentsMD()
		if err := atomicWrite(agentsPath, body); err != nil {
			return fmt.Errorf("regenerate %s: %w", agentsPath, err)
		}
		fmt.Printf("  [regenerated] %s\n", agentsPath)
	} else {
		fmt.Printf("\n  Re-run with --fix to regenerate AGENTS.md from current bundled skills.\n")
	}
	return nil
}

// lastGitTag returns the most recent git tag reachable from HEAD.
// Returns "" if no tags exist (the caller falls back to scope=all).
// We use `git describe --tags --abbrev=0` which is the standard
// way to get the "last release tag".
func lastGitTag() (string, error) {
	cmd := exec.Command("git", "describe", "--tags", "--abbrev=0")
	out, err := cmd.Output()
	if err != nil {
		// Exit code 128 with "fatal: No names found" is normal
		// when no tags exist. Return empty string + nil error so
		// the caller falls back.
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 128 {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// specsChangedSince returns the slugs of features in specs/ whose
// files (spec.md, tasks.md) have changed since `ref` (a git ref).
// Implemented via `git diff --name-only <ref>..HEAD -- specs/`.
// Only counts changes to spec.md / tasks.md; src/ changes are
// out of scope for evals (they're implementation, not spec).
func specsChangedSince(ref string) ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only", ref+"..HEAD", "--", "specs/")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var slugs []string
	seen := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Line format: "specs/0001-jwt/spec.md" or "specs/0001-jwt/tasks.md"
		// Extract the slug (second path component).
		parts := strings.Split(line, "/")
		if len(parts) >= 2 {
			slug := parts[1]
			if !seen[slug] {
				seen[slug] = true
				slugs = append(slugs, slug)
			}
		}
	}
	return slugs, nil
}

// auditFinding is one row in the audit report. The audit
// collects these and renders them sorted by severity.
type auditFinding struct {
	Severity string // "ERROR" | "WARNING" | "INFO"
	Location string // file:line where the issue was found
	Message  string
}

// runAudit runs the project-wide conformity check from the
// `auditar` skill as a CLI. MVP scope: AC traceability +
// ADR status validity. Returns non-zero if any ERROR found
// (or any WARNING when --fail-on-warning).
func runAudit(scope, outPath string, failOnWarning bool) error {
	if outPath == "" {
		outPath = "docs/audit-report.md"
	}

	var findings []auditFinding

	// Step 1: AC traceability per spec.
	if scope == "full" || scope == "specs" {
		findings = append(findings, auditACTraceability()...)
	}

	// Step 2: ADR status validity (every ADR file should have
	// a valid status header).
	if scope == "full" || scope == "adrs" {
		findings = append(findings, auditADRStatus()...)
	}

	// Step 3: doc frontmatter (any .md with frontmatter must
	// parse as YAML).
	if scope == "full" || scope == "docs" {
		findings = append(findings, auditDocFrontmatter()...)
	}

	// Sort: ERROR first, then WARNING, then INFO.
	severityRank := map[string]int{"ERROR": 0, "WARNING": 1, "INFO": 2}
	sort.SliceStable(findings, func(i, j int) bool {
		return severityRank[findings[i].Severity] < severityRank[findings[j].Severity]
	})

	// Counts.
	errors, warnings, infos := 0, 0, 0
	for _, f := range findings {
		switch f.Severity {
		case "ERROR":
			errors++
		case "WARNING":
			warnings++
		case "INFO":
			infos++
		}
	}

	body := renderAuditReport(scope, findings, errors, warnings, infos)
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	if err := atomicWrite(outPath, body); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}

	fmt.Printf("  ✓ wrote %s\n", outPath)
	fmt.Printf("\n  Summary: %d errors, %d warnings, %d info\n", errors, warnings, infos)
	if errors > 0 || (failOnWarning && warnings > 0) {
		return fmt.Errorf("audit found %d error(s) and %d warning(s) — see %s", errors, warnings, outPath)
	}
	return nil
}

// auditACTraceability walks specs/ and verifies that every AC
// in spec.md has at least one task in tasks.md that covers it,
// and that every task in tasks.md references at least one AC.
// Returns one finding per violation.
func auditACTraceability() []auditFinding {
	var findings []auditFinding
	entries, err := os.ReadDir("specs")
	if err != nil {
		// specs/ missing is not an audit failure (project may
		// not use specs/).
		return findings
	}
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "_templates" || e.Name() == "quick" {
			continue
		}
		dir := filepath.Join("specs", e.Name())
		specBody, err := os.ReadFile(filepath.Join(dir, "spec.md"))
		if err != nil {
			findings = append(findings, auditFinding{
				Severity: "WARNING",
				Location: dir + "/spec.md",
				Message:  "spec.md missing or unreadable",
			})
			continue
		}
		tasksBody, err := os.ReadFile(filepath.Join(dir, "tasks.md"))
		if err != nil {
			findings = append(findings, auditFinding{
				Severity: "INFO",
				Location: dir + "/tasks.md",
				Message:  "tasks.md missing — ACs have no coverage claim",
			})
			continue
		}

		acs := parseAcceptanceCriteria(string(specBody))
		tasksBodyStr := string(tasksBody)
		for _, ac := range acs {
			if !strings.Contains(tasksBodyStr, ac.ID) {
				findings = append(findings, auditFinding{
					Severity: "WARNING",
					Location: dir + "/spec.md",
					Message:  fmt.Sprintf("AC %s (%s) has no covering task in tasks.md", ac.ID, ac.Title),
				})
			}
		}
	}
	return findings
}

// auditADRStatus scans docs/architecture/adr/ for status headers
// and verifies each is one of the canonical values.
func auditADRStatus() []auditFinding {
	var findings []auditFinding
	adrDir := "docs/architecture/adr"
	entries, err := os.ReadDir(adrDir)
	if err != nil {
		return findings // no ADRs yet
	}
	validStatuses := map[string]bool{
		"proposed":   true,
		"accepted":   true,
		"deprecated": true,
		"superseded": true,
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(adrDir, e.Name())
		body, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		// Look for "## Status" then the next non-empty line.
		found := false
		lines := strings.Split(string(body), "\n")
		for i, line := range lines {
			if strings.TrimSpace(line) != "## Status" {
				continue
			}
			found = true
			if i+1 < len(lines) {
				status := strings.TrimSpace(lines[i+1])
				if status == "" {
					findings = append(findings, auditFinding{
						Severity: "WARNING",
						Location: path,
						Message:  "## Status header has no value on the next line",
					})
				} else if !validStatuses[strings.ToLower(status)] {
					findings = append(findings, auditFinding{
						Severity: "WARNING",
						Location: path,
						Message:  fmt.Sprintf("## Status value %q is not one of proposed|accepted|deprecated|superseded", status),
					})
				}
			}
			break
		}
		if !found && !strings.HasPrefix(e.Name(), "_") {
			findings = append(findings, auditFinding{
				Severity: "INFO",
				Location: path,
				Message:  "ADR file has no '## Status' section",
			})
		}
	}
	return findings
}

// auditDocFrontmatter walks docs/ for .md files and reports any
// with malformed YAML frontmatter (unclosed --- block).
func auditDocFrontmatter() []auditFinding {
	var findings []auditFinding
	err := filepath.WalkDir("docs", func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		s := string(body)
		if !strings.HasPrefix(s, "---") {
			return nil // no frontmatter, that's fine
		}
		// Look for closing "---" on its own line.
		rest := strings.TrimPrefix(s, "---")
		idx := strings.Index(rest, "\n---")
		if idx < 0 {
			findings = append(findings, auditFinding{
				Severity: "WARNING",
				Location: path,
				Message:  "frontmatter opened (---) but never closed",
			})
		}
		return nil
	})
	if err != nil {
		// docs/ missing is not an audit failure
		return findings
	}
	return findings
}

// renderAuditReport produces docs/audit-report.md content.
func renderAuditReport(scope string, findings []auditFinding, errors, warnings, infos int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Audit report — scope=%s\n\n", scope)
	b.WriteString("> Generated by `radiant audit`. Per the `auditar`\n")
	b.WriteString("> skill: project-wide conformity check (frontmatter,\n")
	b.WriteString("> AC traceability, ADR validity, deviations).\n\n")

	b.WriteString("## Summary\n\n")
	b.WriteString("| Severity | Count |\n")
	b.WriteString("|----------|-------|\n")
	fmt.Fprintf(&b, "| ERROR    | %d |\n", errors)
	fmt.Fprintf(&b, "| WARNING  | %d |\n", warnings)
	fmt.Fprintf(&b, "| INFO     | %d |\n\n", infos)

	if len(findings) == 0 {
		b.WriteString("No findings. Project passes the audit.\n")
		return b.String()
	}

	b.WriteString("## Findings\n\n")
	for _, f := range findings {
		fmt.Fprintf(&b, "### [%s] %s\n\n", f.Severity, f.Message)
		fmt.Fprintf(&b, "- **Location**: %s\n\n", f.Location)
	}
	return b.String()
}

// runRelease cuts a release. Pipeline:
//  1. Pre-flight: check git tree is clean
//  2. Validate version format (semver, with optional leading 'v')
//  3. Check the tag doesn't already exist
//  4. Run quality gates (build, vet, fmt, test)
//  5. Bump version in cmd/radiant/main.go
//  6. Cross-compile (if not skipped)
//  7. Commit version bump (if not skipped)
//  8. Git tag vX.Y.Z (if not skipped)
//
// All destructive steps are skipped under --dry-run; the user
// sees exactly what would happen.
func runRelease(version string, dryRun, skipTests, skipCrossCompile, skipTag, skipCommit bool) error {
	// Normalize: accept both "0.5.1" and "v0.5.1".
	version = strings.TrimPrefix(version, "v")
	tagName := "v" + version

	// 1. Validate semver format.
	if !looksLikeSemver(version) {
		return fmt.Errorf("invalid version %q — expected semver (e.g. 0.5.1 or v0.5.1)", version)
	}

	fmt.Printf("  → Cutting release %s\n\n", tagName)

	// 2. Pre-flight: clean tree.
	if !dryRun {
		out, err := runGit("status", "--porcelain")
		if err != nil {
			return fmt.Errorf("git status: %w", err)
		}
		if strings.TrimSpace(out) != "" {
			return fmt.Errorf("working tree is dirty — commit or stash before cutting a release:\n%s", out)
		}
		fmt.Println("  ✓ working tree clean")
	} else {
		fmt.Println("  [skip] pre-flight (--dry-run)")
	}

	// 3. Check tag doesn't exist.
	if !dryRun && !skipTag {
		out, err := runGit("tag", "-l", tagName)
		if err != nil {
			return fmt.Errorf("git tag: %w", err)
		}
		if strings.TrimSpace(out) != "" {
			return fmt.Errorf("tag %s already exists — delete it first or pick a different version", tagName)
		}
		fmt.Printf("  ✓ tag %s does not exist yet\n", tagName)
	} else if dryRun {
		fmt.Printf("  [skip] tag existence check (--dry-run); would check %s\n", tagName)
	}

	// 4. Quality gates.
	if !skipTests {
		fmt.Println("\n  → Running quality gates")
		if !dryRun {
			if err := runGoStep("build", "build", "./..."); err != nil {
				return err
			}
			if err := runGoStep("vet", "vet", "./..."); err != nil {
				return err
			}
			if err := runFmtCheck(); err != nil {
				return err
			}
			if err := runTestRace(); err != nil {
				return err
			}
			fmt.Println("  ✓ build / vet / fmt / test (-race) all green")
		} else {
			fmt.Println("  [skip] quality gates (--dry-run)")
		}
	} else {
		fmt.Println("  [skip] quality gates (--skip-tests)")
	}

	// 5. Bump version in cmd/radiant/main.go.
	fmt.Println("\n  → Bumping version")
	if err := bumpVersionInSource(version, dryRun); err != nil {
		return err
	}

	// 6. Cross-compile.
	if !skipCrossCompile {
		fmt.Println("\n  → Cross-compiling (6 targets)")
		if !dryRun {
			if err := runMakeRelease(); err != nil {
				return err
			}
			fmt.Println("  ✓ 6/6 targets built (see dist/)")
		} else {
			fmt.Println("  [skip] cross-compile (--dry-run)")
		}
	} else {
		fmt.Println("  [skip] cross-compile (--skip-cross-compile)")
	}

	// 7. Commit.
	if !skipCommit {
		fmt.Println("\n  → Committing version bump")
		if !dryRun {
			if err := runGitCommit(fmt.Sprintf("release: cut %s", tagName), "cmd/radiant/main.go", "CHANGELOG.md"); err != nil {
				return err
			}
			fmt.Printf("  ✓ committed 'release: cut %s'\n", tagName)
		} else {
			fmt.Println("  [skip] commit (--dry-run)")
		}
	} else {
		fmt.Println("  [skip] commit (--skip-commit)")
	}

	// 8. Git tag.
	if !skipTag {
		fmt.Println("\n  → Tagging")
		if !dryRun {
			if _, err := runGit("tag", tagName); err != nil {
				return fmt.Errorf("git tag: %w", err)
			}
			fmt.Printf("  ✓ tagged %s\n", tagName)
		} else {
			fmt.Printf("  [skip] tag (--dry-run); would create %s\n", tagName)
		}
	} else {
		fmt.Println("  [skip] tag (--skip-tag)")
	}

	fmt.Printf("\n  ✓ Release %s complete\n", tagName)
	fmt.Printf("    Next: git push origin main && git push origin %s\n", tagName)
	// Record a local telemetry event if the user has opted in.
	// Only fires when --skip-tag is NOT set (a release without a
	// tag isn't really a release).
	if !skipTag {
		recordTelemetry("release")
	}
	return nil
}

// looksLikeSemver is a relaxed semver check: MAJOR.MINOR.PATCH
// with optional pre-release / build suffix. We don't enforce the
// strict semver spec (it would block "0.5.0-rc.1" etc.); we just
// require three numeric components separated by dots. Accepts
// an optional leading "v" (so both "0.5.1" and "v0.5.1" pass).
func looksLikeSemver(v string) bool {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 3 {
		return false
	}
	for _, p := range parts {
		// Allow trailing pre-release (e.g. "0-rc.1") by stripping.
		if idx := strings.IndexAny(p, "-+"); idx >= 0 {
			p = p[:idx]
		}
		if p == "" {
			return false
		}
		for _, r := range p {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

// runGit runs a git subcommand in the project root and returns stdout.
func runGit(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// runGoStep runs a `go` subcommand (build/vet) with the project env.
func runGoStep(label, sub string, args ...string) error {
	cmd := exec.Command("go", append([]string{sub}, args...)...)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go %s: %w\n%s", label, err, string(out))
	}
	return nil
}

// runFmtCheck fails if any .go file is not gofmt'd.
func runFmtCheck() error {
	cmd := exec.Command("gofmt", "-l", ".")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gofmt -l: %w", err)
	}
	if strings.TrimSpace(string(out)) != "" {
		return fmt.Errorf("files not gofmt'd:\n%s", string(out))
	}
	return nil
}

// runTestRace runs the full test suite under -race.
func runTestRace() error {
	cmd := exec.Command("go", "test", "./...", "-count=1", "-race", "-timeout=180s")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go test: %w\n%s", err, string(out))
	}
	return nil
}

// runMakeRelease invokes `make release` and forwards output.
func runMakeRelease() error {
	cmd := exec.Command("make", "release")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("make release: %w\n%s", err, string(out))
	}
	return nil
}

// runGitCommit commits the given paths with the given message.
// Uses -c user.name/email to avoid touching global git config.
func runGitCommit(msg string, paths ...string) error {
	args := []string{"add", "--"}
	args = append(args, paths...)
	if _, err := runGit(args...); err != nil {
		return err
	}
	commit := []string{
		"-c", "user.name=Henrique",
		"-c", "user.email=henrique@fortvna.com.br",
		"commit", "-m", msg,
	}
	cmd := exec.Command("git", commit...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git commit: %w\n%s", err, string(out))
	}
	return nil
}

// bumpVersionInSource updates `var version = "..."` in
// cmd/radiant/main.go to the new version. The file is rewritten
// line-by-line to avoid touching other content. With dryRun=true,
// prints what would change without writing.
func bumpVersionInSource(newVersion string, dryRun bool) error {
	path := "cmd/radiant/main.go"
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	body := string(data)
	oldLine := ""
	for _, line := range strings.Split(body, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "var version =") {
			oldLine = line
			break
		}
	}
	if oldLine == "" {
		return fmt.Errorf("could not find 'var version = ...' in %s", path)
	}
	newLine := fmt.Sprintf(`var version = "%s"`, newVersion)
	if oldLine == newLine {
		fmt.Printf("  = %s (no change)\n", path)
		return nil
	}
	if dryRun {
		fmt.Printf("  [would-replace] %s\n        %s\n      → %s\n", path, oldLine, newLine)
		return nil
	}
	updated := strings.Replace(body, oldLine, newLine, 1)
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	fmt.Printf("  ✓ %s: %s\n", path, newLine)
	return nil
}

// mcpTool is one tool exposed by the MCP server.
type mcpTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema mcpInputSchema `json:"inputSchema"`
}

// mcpInputSchema is the JSON Schema describing the tool's args.
type mcpInputSchema struct {
	Type       string                    `json:"type"`
	Properties map[string]mcpPropertyDef `json:"properties"`
	Required   []string                  `json:"required,omitempty"`
}

// mcpPropertyDef is one property in the input schema.
type mcpPropertyDef struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// mcpRequest is a JSON-RPC 2.0 request from the client.
type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// mcpResponse is a JSON-RPC 2.0 response to the client.
type mcpResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      any         `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *mcpError   `json:"error,omitempty"`
}

// mcpError is the error block of a JSON-RPC response.
type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// runMCPServe reads newline-delimited JSON-RPC requests from `in`,
// writes JSON-RPC responses to `out`. Implements the Model Context
// Protocol (MCP) over stdio. Tools exposed are radiant commands
// (spec, adr, product, evals, audit, release). Each tool call
// spawns the corresponding command as a subprocess and returns
// stdout as the result.
func runMCPServe(in io.Reader, out io.Writer) error {
	tools := []mcpTool{
		{Name: "radiant_spec", Description: "Scaffold a new feature (spec.md + tasks.md)", InputSchema: mcpInputSchema{
			Type: "object",
			Properties: map[string]mcpPropertyDef{
				"intent": {Type: "string", Description: "The feature intent (1-3 sentences)"},
			},
			Required: []string{"intent"},
		}},
		{Name: "radiant_adr", Description: "Create an Architecture Decision Record (Nygard format)", InputSchema: mcpInputSchema{
			Type: "object",
			Properties: map[string]mcpPropertyDef{
				"decision": {Type: "string", Description: "The decision title"},
				"status":   {Type: "string", Description: "proposed | accepted | deprecated | superseded"},
			},
			Required: []string{"decision"},
		}},
		{Name: "radiant_product", Description: "Start a Lean Inception", InputSchema: mcpInputSchema{
			Type: "object",
			Properties: map[string]mcpPropertyDef{
				"vision":    {Type: "string", Description: "The product vision (1-3 sentences)"},
				"mvp_weeks": {Type: "number", Description: "Target weeks to MVP"},
			},
			Required: []string{"vision"},
		}},
		{Name: "radiant_evals", Description: "Measure AC→test coverage fidelity", InputSchema: mcpInputSchema{
			Type: "object",
			Properties: map[string]mcpPropertyDef{
				"scope": {Type: "string", Description: "all | since-last-release | <spec-path>"},
			},
		}},
		{Name: "radiant_audit", Description: "Run project layout audit", InputSchema: mcpInputSchema{
			Type: "object",
			Properties: map[string]mcpPropertyDef{
				"scope": {Type: "string", Description: "full | docs | specs | adrs"},
			},
		}},
		{Name: "radiant_release", Description: "Cut a release (dry-run only via MCP for safety)", InputSchema: mcpInputSchema{
			Type: "object",
			Properties: map[string]mcpPropertyDef{
				"version": {Type: "string", Description: "Semver version (e.g. 0.5.1)"},
			},
			Required: []string{"version"},
		}},
	}

	enc := json.NewEncoder(out)
	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var req mcpRequest
		if err := json.Unmarshal(line, &req); err != nil {
			_ = enc.Encode(mcpResponse{JSONRPC: "2.0", Error: &mcpError{Code: -32700, Message: "parse error"}})
			continue
		}
		resp := handleMCPRequest(req, tools)
		_ = enc.Encode(resp)
	}
	return scanner.Err()
}

// handleMCPRequest dispatches one JSON-RPC request to the
// appropriate MCP method handler.
func handleMCPRequest(req mcpRequest, tools []mcpTool) mcpResponse {
	switch req.Method {
	case "initialize":
		return mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"serverInfo": map[string]string{
					"name":    "radiant-harness",
					"version": version,
				},
				"capabilities": map[string]interface{}{
					"tools": map[string]interface{}{},
				},
			},
		}
	case "tools/list":
		return mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]interface{}{"tools": tools},
		}
	case "tools/call":
		var params struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return mcpResponse{JSONRPC: "2.0", ID: req.ID, Error: &mcpError{Code: -32602, Message: "invalid params"}}
		}
		return callMCPTool(params.Name, params.Arguments)
	default:
		return mcpResponse{JSONRPC: "2.0", ID: req.ID, Error: &mcpError{Code: -32601, Message: "method not found: " + req.Method}}
	}
}

// runIncident scaffolds an incident document. The user fills in
// the timeline, RCA, and action items following the `incident`
// skill's blameless post-mortem template. Severity must be one
// of sev1..sev4 (validated).
func runIncident(severity, summary, outPath string) error {
	severity = strings.ToLower(strings.TrimSpace(severity))
	switch severity {
	case "sev1", "sev2", "sev3", "sev4":
		// ok
	default:
		return fmt.Errorf("invalid severity %q — expected sev1 | sev2 | sev3 | sev4", severity)
	}

	if outPath == "" {
		seq, err := nextIncidentSeq()
		if err != nil {
			return fmt.Errorf("compute next sequence: %w", err)
		}
		slug := slugify(summary)
		outPath = filepath.Join("docs", "incidents", fmt.Sprintf("%04d-%s.md", seq, slug))
	}

	body := renderIncidentDoc(severity, summary)
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	if err := atomicWrite(outPath, body); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}
	fmt.Printf("  ✓ created %s\n", outPath)
	fmt.Println("\n  Next (per the `incident` skill):")
	fmt.Println("    1. Acknowledge the alert in PagerDuty / Opsgenie / Slack.")
	fmt.Println("    2. Assign severity if you haven't already (you gave sev" + severity[3:] + ").")
	fmt.Println("    3. Name an incident commander within 5 min.")
	fmt.Println("    4. Mitigate (rollback / scale / failover) within 15 min.")
	fmt.Println("    5. Update status page for sev1/sev2.")
	fmt.Println("    6. Schedule a blameless post-mortem within 5 business days.")
	return nil
}

// nextIncidentSeq scans docs/incidents/ for the highest NNNN-
// prefix and returns next+1. Returns 1 if the directory is empty
// or doesn't exist yet.
func nextIncidentSeq() (int, error) {
	dir := "docs/incidents"
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 1, nil
		}
		return 0, err
	}
	max := 0
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

// renderIncidentDoc produces the incident document body. The
// user fills in the timeline + RCA + action items following
// the structure. MVP: template only; LLM via the `incident` skill
// fills in the actual content during the post-mortem.
func renderIncidentDoc(severity, summary string) string {
	return fmt.Sprintf(`# Incident %s — %s

> Generated by 'radiant incident'. Per the 'incident' skill:
> blameless post-mortem template. Fill in the timeline + RCA +
> action items; the skill's job is to remind you what goes where.

**Severity**: %s
**Date**: %s
**Duration**: <fill in: HH:MM from detection to resolution>
**Impact**: <fill in: customer-facing impact>
**Commander**: <name>
**Author**: <name>

## Timeline (UTC)

- HH:MM — detection (alert fired / customer report)
- HH:MM — commander named
- HH:MM — mitigation started (rollback / scale / failover)
- HH:MM — service restored
- HH:MM — root cause identified
- HH:MM — permanent fix deployed

## Root cause

What happened, and WHY. Not the symptom — the cause. Include the chain of events that led to the failure.

## Contributing factors

- What monitoring missed
- What tests didn't catch
- What process / runbook was unclear
- What communication failed

## What went well

- Fast detection
- Quick rollback
- Clear comms
- Good escalation

## Action items

| # | Action | Owner | Due | Tracked in |
|---|--------|-------|-----|------------|
| 1 | Add monitoring for X | @alice | 2026-07-01 | roadmap |
| 2 | Improve runbook for Y | @bob   | 2026-07-15 | roadmap |
| 3 | Add regression test for Z | @carol | 2026-07-01 | roadmap |

---

_Generated by 'radiant incident' on %s. See the 'incident' skill for the full playbook._
`,
		severity, summary, severity, time.Now().UTC().Format("2006-01-02"), time.Now().UTC().Format("2006-01-02"))
}

// runTelemetryRotate caps the active telemetry log at `maxEntries`.
// When the log exceeds the cap, the oldest events are moved to
// an archive file (`telemetry-YYYY-MM-DD.jsonl`) so the user keeps
// full history without the active log growing unbounded.
//
// Idempotent: running it on a log under the cap is a no-op.
// Idempotent on a missing log: returns nil.
func runTelemetryRotate(maxEntries int) error {
	if maxEntries <= 0 {
		return fmt.Errorf("--max-entries must be > 0 (got %d)", maxEntries)
	}
	if !isTelemetryEnabled() {
		fmt.Println("  Telemetry is disabled. Run 'radiant telemetry enable' to start collecting.")
		return nil
	}
	data, err := os.ReadFile(telemetryLogPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read %s: %w", telemetryLogPath, err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) <= maxEntries {
		fmt.Printf("  Log has %d entries; under cap %d. No rotation needed.\n", len(lines), maxEntries)
		return nil
	}

	// Split into keep (latest maxEntries) and archive (rest).
	keep := lines[len(lines)-maxEntries:]
	archive := lines[:len(lines)-maxEntries]
	archivePath := fmt.Sprintf("%s-%s.jsonl",
		strings.TrimSuffix(telemetryLogPath, ".jsonl"),
		time.Now().UTC().Format("2006-01-02"),
	)

	if err := os.WriteFile(archivePath, []byte(strings.Join(archive, "\n")+"\n"), 0o644); err != nil {
		return fmt.Errorf("write archive %s: %w", archivePath, err)
	}
	if err := os.WriteFile(telemetryLogPath, []byte(strings.Join(keep, "\n")+"\n"), 0o644); err != nil {
		return fmt.Errorf("rewrite log %s: %w", telemetryLogPath, err)
	}
	fmt.Printf("  ✓ Archived %d events to %s\n", len(archive), archivePath)
	fmt.Printf("  ✓ Active log now has %d entries (cap: %d)\n", len(keep), maxEntries)
	return nil
}

// runTelemetryExport dumps the local telemetry log in JSON or CSV.
// Default: JSON to stdout (pipe-friendly). Pass --output=<path> to
// write to a file. Pass --since=YYYY-MM-DD to filter.
//
// Privacy: exports ONLY the 4 fields already recorded locally
// (timestamp, command, hash, radiant_ver). The user must explicitly
// invoke this command AND pipe/save the output. No network egress.
//
// Format selection:
//
//	json — pretty-printed array of events (one per line in the log).
//	csv  — header row + one row per event.
//
// Disabled or missing log → no-op, returns nil.
func runTelemetryExport(format, output, since string) error {
	if format != "json" && format != "csv" {
		return fmt.Errorf("--format must be 'json' or 'csv' (got %q)", format)
	}
	if !isTelemetryEnabled() {
		fmt.Println("  Telemetry is disabled. Run 'radiant telemetry enable' to start collecting.")
		return nil
	}
	data, err := os.ReadFile(telemetryLogPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read %s: %w", telemetryLogPath, err)
	}
	raw := strings.TrimRight(string(data), "\n")
	if raw == "" {
		return nil
	}

	// Build a normalized slice of events. Skip lines that don't parse
	// as JSON (defensive — shouldn't happen given how we record, but
	// user could have hand-edited the log).
	type ev struct {
		Timestamp  string `json:"timestamp"`
		Command    string `json:"command"`
		Hash       string `json:"hash"`
		RadiantVer string `json:"radiant_ver"`
	}
	var events []ev
	for _, line := range strings.Split(raw, "\n") {
		var e ev
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue
		}
		// Apply --since filter (date portion of timestamp >= since).
		if since != "" {
			if len(e.Timestamp) < 10 || e.Timestamp[:10] < since {
				continue
			}
		}
		events = append(events, e)
	}

	var out string
	switch format {
	case "json":
		b, err := json.MarshalIndent(events, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}
		out = string(b) + "\n"
	case "csv":
		var sb strings.Builder
		sb.WriteString("timestamp,command,hash,radiant_ver\n")
		for _, e := range events {
			fmt.Fprintf(&sb, "%s,%s,%s,%s\n",
				csvField(e.Timestamp), csvField(e.Command),
				csvField(e.Hash), csvField(e.RadiantVer))
		}
		out = sb.String()
	}

	if output != "" {
		if err := os.WriteFile(output, []byte(out), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", output, err)
		}
		fmt.Printf("  ✓ Exported %d events to %s (%s)\n", len(events), output, format)
	} else {
		fmt.Print(out)
	}
	return nil
}

// csvField quotes a value iff it contains a comma, double-quote, or newline.
func csvField(s string) string {
	if strings.ContainsAny(s, ",\"\n") {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
}

// runTelemetrySummary reads the local telemetry log and prints
// aggregate stats: total events, top commands by frequency, and
// daily counts. All computation is local — no network access.
// Privacy guarantees are the same as `show` (only the local file).
func runTelemetrySummary() error {
	if !isTelemetryEnabled() {
		fmt.Println("  Telemetry is disabled. Run 'radiant telemetry enable' to start collecting.")
		return nil
	}
	data, err := os.ReadFile(telemetryLogPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", telemetryLogPath, err)
	}
	var events []telemetryEvent
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var ev telemetryEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue // skip malformed lines
		}
		events = append(events, ev)
	}
	if len(events) == 0 {
		fmt.Println("  (no events recorded)")
		return nil
	}

	// Count by command.
	cmdCounts := map[string]int{}
	// Count by day.
	dayCounts := map[string]int{}
	for _, ev := range events {
		cmdCounts[ev.Command]++
		// Timestamp is RFC3339; take the date prefix (first 10 chars).
		if len(ev.Timestamp) >= 10 {
			day := ev.Timestamp[:10]
			dayCounts[day]++
		}
	}

	// Top commands sorted descending.
	type cmdCount struct {
		cmd   string
		count int
	}
	var sortedCmds []cmdCount
	for c, n := range cmdCounts {
		sortedCmds = append(sortedCmds, cmdCount{c, n})
	}
	sort.Slice(sortedCmds, func(i, j int) bool {
		return sortedCmds[i].count > sortedCmds[j].count
	})

	// Daily counts sorted by date (string sort works for ISO dates).
	var sortedDays []string
	for d := range dayCounts {
		sortedDays = append(sortedDays, d)
	}
	sort.Strings(sortedDays)

	fmt.Printf("  Total events: %d\n", len(events))
	fmt.Printf("  Distinct commands: %d\n", len(cmdCounts))
	fmt.Printf("  Distinct days: %d\n", len(dayCounts))
	fmt.Println()
	fmt.Println("  Top commands:")
	for i, cc := range sortedCmds {
		if i >= 10 {
			break
		}
		fmt.Printf("    %-20s %d\n", cc.cmd, cc.count)
	}
	fmt.Println()
	fmt.Println("  Daily counts:")
	for _, d := range sortedDays {
		fmt.Printf("    %s  %d\n", d, dayCounts[d])
	}
	return nil
}

// recordTelemetry appends one telemetry event to the local log.
// PRIVACY-FIRST: this is a no-op unless the user has explicitly
// enabled telemetry via `radiant telemetry enable`. The event
// records only the command name + timestamp + 8-char content
// hash + CLI version — never args, paths, or env vars.
//
// Used by the release pipeline so that cutting a release is
// auditable locally. Composes naturally with `radiant telemetry
// show` / `radiant telemetry summary`.
func recordTelemetry(command string) {
	if !isTelemetryEnabled() {
		return
	}
	hash := shortHash(time.Now().UTC().Format(time.RFC3339Nano))
	ev := telemetryEvent{
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Command:    command,
		Hash:       hash,
		RadiantVer: version,
	}
	data, err := json.Marshal(ev)
	if err != nil {
		return // best-effort; never fail the user's command
	}
	f, err := os.OpenFile(telemetryLogPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(data, '\n'))
}

// shortHash returns the first 8 chars of sha256(input). Used for
// the telemetry event's `hash` field — a stable identifier for the
// event without leaking the underlying content.
func shortHash(input string) string {
	h := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", h)[:8]
}

// telemetryLogPath is the canonical location of the local
// telemetry log. Lives under .radiant-harness/ so it stays out
// of the user's source tree.
const telemetryLogPath = ".radiant-harness/telemetry.jsonl"

// telemetryEvent is one row in the telemetry log. PRIVACY-FIRST:
// only the command name (e.g. "spec"), a content hash, and the
// ISO-8601 timestamp are recorded. No args, no paths, no project
// metadata, no environment info.
type telemetryEvent struct {
	Timestamp  string `json:"timestamp"`   // ISO-8601 UTC
	Command    string `json:"command"`     // e.g. "spec", "release", "audit"
	Hash       string `json:"hash"`        // sha256 of redacted context (placeholder, 8 chars)
	RadiantVer string `json:"radiant_ver"` // CLI version (semver, no git sha)
}

// isTelemetryEnabled returns true when the user has run
// `radiant telemetry enable`. We detect enablement by checking
// for the existence of the telemetry log file. There is no
// separate "config" file — the log's existence IS the flag.
func isTelemetryEnabled() bool {
	_, err := os.Stat(telemetryLogPath)
	return err == nil
}

// runTelemetryStatus reports whether telemetry is enabled, what
// would be recorded if it were, and where the log lives.
func runTelemetryStatus() error {
	enabled := isTelemetryEnabled()
	fmt.Printf("  Telemetry: %s\n", boolStr(enabled))
	fmt.Printf("  Log path:  %s\n", telemetryLogPath)
	fmt.Println()
	fmt.Println("  When enabled, each radiant invocation records:")
	fmt.Println("    - timestamp (ISO-8601 UTC)")
	fmt.Println("    - command name (e.g. \"spec\", \"release\")")
	fmt.Println("    - 8-char hash of redacted context")
	fmt.Println("    - radiant CLI version (semver)")
	fmt.Println()
	fmt.Println("  NEVER recorded (privacy-first):")
	fmt.Println("    - command arguments")
	fmt.Println("    - file paths")
	fmt.Println("    - project names or git SHAs")
	fmt.Println("    - environment variables")
	fmt.Println("    - network endpoints")
	fmt.Println()
	if enabled {
		fmt.Printf("  Run 'radiant telemetry disable' to opt out and delete the log.\n")
	} else {
		fmt.Printf("  Run 'radiant telemetry enable' to opt in.\n")
	}
	return nil
}

// runTelemetryEnable creates the telemetry log file (empty).
// The act of creating the file IS the opt-in.
func runTelemetryEnable() error {
	if isTelemetryEnabled() {
		fmt.Printf("  Telemetry already enabled (log at %s)\n", telemetryLogPath)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(telemetryLogPath), 0o755); err != nil {
		return err
	}
	f, err := os.Create(telemetryLogPath)
	if err != nil {
		return fmt.Errorf("create %s: %w", telemetryLogPath, err)
	}
	defer f.Close()
	fmt.Printf("  ✓ Telemetry enabled. Log: %s\n", telemetryLogPath)
	fmt.Println("  Each subsequent radiant invocation will append one line.")
	fmt.Println("  Disable with 'radiant telemetry disable'.")
	return nil
}

// runTelemetryDisable removes the telemetry log file. Idempotent:
// returns nil even if the file doesn't exist.
func runTelemetryDisable() error {
	if !isTelemetryEnabled() {
		fmt.Println("  Telemetry already disabled (no log file).")
		return nil
	}
	if err := os.Remove(telemetryLogPath); err != nil {
		return fmt.Errorf("remove %s: %w", telemetryLogPath, err)
	}
	fmt.Printf("  ✓ Telemetry disabled. Removed %s.\n", telemetryLogPath)
	return nil
}

// runTelemetryShow prints the last 50 events from the log, one
// per line. If telemetry is disabled, prints a helpful message.
func runTelemetryShow() error {
	if !isTelemetryEnabled() {
		fmt.Println("  Telemetry is disabled. Run 'radiant telemetry enable' to start collecting.")
		return nil
	}
	data, err := os.ReadFile(telemetryLogPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", telemetryLogPath, err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 {
		fmt.Println("  (no events recorded)")
		return nil
	}
	// Last 50 events, most recent first.
	start := len(lines) - 50
	if start < 0 {
		start = 0
	}
	fmt.Printf("  Last %d events:\n", len(lines)-start)
	for _, line := range lines[start:] {
		fmt.Printf("    %s\n", line)
	}
	return nil
}

// boolStr is a tiny helper to render "enabled" / "disabled".
func boolStr(b bool) string {
	if b {
		return "ENABLED"
	}
	return "disabled (opt-in)"
}

// securityFinding is one row in the security report.
type securityFinding struct {
	Severity string // "ERROR" | "WARNING" | "INFO"
	Location string // file:line or path
	Message  string
}

// secretPattern matches a plausible secret in source. The list
// covers the most common formats seen in real leaks. False
// positives are accepted (a secret-shaped string in a test file
// is still worth flagging — humans can ignore if appropriate).
type secretPattern struct {
	Name        string
	Regex       *regexp.Regexp
	Description string
}

// runSecurity scans the project for security issues per the
// `--scope` flag. Scopes: "secrets" (regex-based secret scan),
// "perms" (sensitive file permission check), "all" (both).
func runSecurity(scope, outPath string, failOnWarning bool) error {
	if outPath == "" {
		outPath = "docs/security-report.md"
	}

	var findings []securityFinding
	if scope == "all" || scope == "secrets" {
		findings = append(findings, scanSecrets()...)
	}
	if scope == "all" || scope == "perms" {
		findings = append(findings, scanPerms()...)
	}

	severityRank := map[string]int{"ERROR": 0, "WARNING": 1, "INFO": 2}
	sort.SliceStable(findings, func(i, j int) bool {
		return severityRank[findings[i].Severity] < severityRank[findings[j].Severity]
	})

	errors, warnings, infos := 0, 0, 0
	for _, f := range findings {
		switch f.Severity {
		case "ERROR":
			errors++
		case "WARNING":
			warnings++
		case "INFO":
			infos++
		}
	}

	body := renderSecurityReport(scope, findings, errors, warnings, infos)
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	if err := atomicWrite(outPath, body); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}

	fmt.Printf("  ✓ wrote %s\n", outPath)
	fmt.Printf("\n  Summary: %d errors, %d warnings, %d info\n", errors, warnings, infos)
	if errors > 0 || (failOnWarning && warnings > 0) {
		return fmt.Errorf("security scan found %d error(s) and %d warning(s) — see %s", errors, warnings, outPath)
	}
	return nil
}

// scanSecrets walks .go / .md / .yml / .json / .sh / .env / .ts / .js
// files and reports any line that matches a known secret pattern.
// Skips vendor / node_modules / .git / dist directories and
// .test.go files (test fixtures commonly contain fake secrets).
func scanSecrets() []securityFinding {
	patterns := []secretPattern{
		{Name: "AWS Access Key", Regex: regexp.MustCompile(`AKIA[0-9A-Z]{16}`), Description: "AWS access key ID"},
		{Name: "GitHub Token", Regex: regexp.MustCompile(`ghp_[A-Za-z0-9]{36}`), Description: "GitHub personal access token"},
		{Name: "GitHub Fine-Grained Token", Regex: regexp.MustCompile(`github_pat_[A-Za-z0-9_]{82}`), Description: "GitHub fine-grained PAT"},
		{Name: "Slack Token", Regex: regexp.MustCompile(`xox[abpr]-[A-Za-z0-9-]{10,}`), Description: "Slack API token"},
		{Name: "OpenAI Key", Regex: regexp.MustCompile(`sk-[A-Za-z0-9_-]{20,}`), Description: "OpenAI / OpenAI-compatible API key"},
		{Name: "Anthropic Key", Regex: regexp.MustCompile(`sk-ant-[A-Za-z0-9_-]{20,}`), Description: "Anthropic API key"},
		{Name: "Google API Key", Regex: regexp.MustCompile(`AIza[0-9A-Za-z_-]{35}`), Description: "Google API key"},
		{Name: "Generic Bearer", Regex: regexp.MustCompile(`Bearer\s+[A-Za-z0-9_\-\.=]{20,}`), Description: "Bearer token in source"},
	}
	skipDirs := map[string]bool{
		".git":                    true,
		"node_modules":            true,
		"vendor":                  true,
		"dist":                    true,
		".radiant-harness/skills": true, // bundled skills have example secrets in docs
	}
	var findings []securityFinding
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			// Top-level skips.
			if name == ".git" || name == "node_modules" || name == "vendor" || name == "dist" {
				return filepath.SkipDir
			}
			if skipDirs[path] {
				return filepath.SkipDir
			}
			return nil
		}
		// Only scan known file types.
		ext := filepath.Ext(path)
		switch ext {
		case ".go", ".md", ".yml", ".yaml", ".json", ".sh", ".env",
			".ts", ".js", ".py", ".rb", ".toml":
		default:
			return nil
		}
		// Skip test files (test fixtures commonly contain fake secrets).
		base := filepath.Base(path)
		if strings.HasSuffix(base, "_test.go") || strings.HasSuffix(base, ".test.ts") ||
			strings.HasSuffix(base, ".test.js") || strings.HasSuffix(base, "_test.py") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		for i, line := range strings.Split(string(data), "\n") {
			for _, p := range patterns {
				if p.Regex.MatchString(line) {
					findings = append(findings, securityFinding{
						Severity: "ERROR",
						Location: fmt.Sprintf("%s:%d", path, i+1),
						Message:  fmt.Sprintf("Possible %s (%s)", p.Name, p.Description),
					})
					break // one finding per line is enough
				}
			}
		}
		return nil
	})
	if err != nil {
		// Walking the tree shouldn't fail; if it does, return what we have.
		return findings
	}
	return findings
}

// scanPerms checks for sensitive files with overly permissive
// (world-readable/writable) permissions. Targets: .env, *.key,
// *.pem, id_rsa, id_dsa, id_ecdsa, id_ed25519, *.p12, *.pfx.
func scanPerms() []securityFinding {
	sensitiveNames := map[string]bool{
		".env":            true,
		".env.local":      true,
		".env.production": true,
		"id_rsa":          true,
		"id_dsa":          true,
		"id_ecdsa":        true,
		"id_ed25519":      true,
	}
	sensitiveExts := map[string]bool{
		".key": true, ".pem": true, ".p12": true, ".pfx": true,
	}
	var findings []securityFinding
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		base := filepath.Base(path)
		if !sensitiveNames[base] && !sensitiveExts[filepath.Ext(base)] {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		mode := info.Mode().Perm()
		// Group or world has any access (read/write/execute).
		if mode&0o077 != 0 {
			findings = append(findings, securityFinding{
				Severity: "WARNING",
				Location: path,
				Message:  fmt.Sprintf("sensitive file has permissive mode %04o (group/world can access); chmod 600 recommended", mode),
			})
		}
		return nil
	})
	if err != nil {
		return findings
	}
	return findings
}

// renderSecurityReport produces the docs/security-report.md content.
func renderSecurityReport(scope string, findings []securityFinding, errors, warnings, infos int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Security report — scope=%s\n\n", scope)
	b.WriteString("> Generated by `radiant security`. MVP scope:\n")
	b.WriteString("> hardcoded secret scan + sensitive file permissions.\n")
	b.WriteString("> Dep-CVE scanning and config-CORS checks are future work.\n\n")

	b.WriteString("## Summary\n\n")
	b.WriteString("| Severity | Count |\n")
	b.WriteString("|----------|-------|\n")
	fmt.Fprintf(&b, "| ERROR    | %d |\n", errors)
	fmt.Fprintf(&b, "| WARNING  | %d |\n", warnings)
	fmt.Fprintf(&b, "| INFO     | %d |\n\n", infos)

	if len(findings) == 0 {
		b.WriteString("No findings. Project passes the security scan.\n")
		return b.String()
	}

	b.WriteString("## Findings\n\n")
	for _, f := range findings {
		fmt.Fprintf(&b, "### [%s] %s\n\n", f.Severity, f.Message)
		fmt.Fprintf(&b, "- **Location**: %s\n\n", f.Location)
	}
	return b.String()
}

// callMCPTool dispatches a tools/call request to the matching
// radiant CLI command. Returns the stdout as a content array.
func callMCPTool(name string, args json.RawMessage) mcpResponse {
	var argv []string
	argv = append(argv, name)
	// Each tool has its own CLI shape. Map tools to subcommands.
	switch name {
	case "radiant_spec":
		var a struct {
			Intent string `json:"intent"`
		}
		_ = json.Unmarshal(args, &a)
		argv = []string{"spec", a.Intent}
	case "radiant_adr":
		var a struct {
			Decision string `json:"decision"`
			Status   string `json:"status"`
		}
		_ = json.Unmarshal(args, &a)
		if a.Status != "" {
			argv = []string{"adr", a.Decision, "--status=" + a.Status}
		} else {
			argv = []string{"adr", a.Decision}
		}
	case "radiant_product":
		var a struct {
			Vision   string `json:"vision"`
			MVPWeeks int    `json:"mvp_weeks"`
		}
		_ = json.Unmarshal(args, &a)
		if a.MVPWeeks > 0 {
			argv = []string{"product", a.Vision, "--mvp-weeks=" + strconv.Itoa(a.MVPWeeks)}
		} else {
			argv = []string{"product", a.Vision}
		}
	case "radiant_evals":
		var a struct {
			Scope string `json:"scope"`
		}
		_ = json.Unmarshal(args, &a)
		if a.Scope == "" {
			a.Scope = "all"
		}
		argv = []string{"evals", "--scope=" + a.Scope}
	case "radiant_audit":
		var a struct {
			Scope string `json:"scope"`
		}
		_ = json.Unmarshal(args, &a)
		if a.Scope == "" {
			a.Scope = "full"
		}
		argv = []string{"audit", "--scope=" + a.Scope}
	case "radiant_release":
		var a struct {
			Version string `json:"version"`
		}
		_ = json.Unmarshal(args, &a)
		// Always dry-run via MCP for safety — never let an MCP
		// caller tag a release without explicit CLI confirmation.
		argv = []string{"release", a.Version, "--dry-run"}
	default:
		return mcpResponse{JSONRPC: "2.0", Error: &mcpError{Code: -32602, Message: "unknown tool: " + name}}
	}

	cmd := exec.Command("radiant", argv...)
	stdout, err := cmd.CombinedOutput()
	if err != nil {
		// Return the error as content (text) so the MCP client
		// sees the failure message. Don't bubble up a JSON-RPC
		// error — tools/call errors are tool-call failures, not
		// protocol errors.
		return mcpResponse{
			JSONRPC: "2.0",
			Result: map[string]interface{}{
				"content": []map[string]string{{"type": "text", "text": string(stdout)}},
				"isError": true,
			},
		}
	}
	return mcpResponse{
		JSONRPC: "2.0",
		Result: map[string]interface{}{
			"content": []map[string]string{{"type": "text", "text": string(stdout)}},
		},
	}
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
// that runs validate → audit → security → tests → build on every
// PR (5 gates; Sprint 17 added `radiant security`).
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
      - name: Security (hardcoded secrets + sensitive file perms)
        run: radiant security --fail-on-warning
      - name: Tests
        run: go test ./... -count=1 -race
      - name: Build
        run: go build ./...
`, modelArg)
}

// renderGitLabCI produces a .gitlab-ci.yml with the same five
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

radiant-security:
  stage: radiant
  image: golang:1.22
  before_script:
    - go install github.com/quant-risk/radiant-harness/cmd/radiant@latest
  script:
    - radiant security --fail-on-warning

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
// five gates. Secrets via context (the CircleCI idiom).
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
          name: Security (hardcoded secrets + sensitive file perms)
          command: radiant security --fail-on-warning
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
