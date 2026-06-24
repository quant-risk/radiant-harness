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

var version = "0.4.2"

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
