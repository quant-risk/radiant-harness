//go:build !light_only

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	radiant "github.com/quant-risk/radiant-harness/internal"
	"github.com/quant-risk/radiant-harness/internal/benchmark"
	"github.com/quant-risk/radiant-harness/internal/engine"
	"github.com/quant-risk/radiant-harness/internal/llm"
	"github.com/quant-risk/radiant-harness/internal/loop"
	"github.com/quant-risk/radiant-harness/internal/mcpbridge"
	"github.com/quant-risk/radiant-harness/internal/quality"
	"github.com/quant-risk/radiant-harness/internal/routing"
	"github.com/quant-risk/radiant-harness/internal/scaffold"
	"github.com/spf13/cobra"
)

func registerRunCmds(root *cobra.Command) {
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
	var runDisableTools bool
	var runMCPBridges []string
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
			// Sprint 69 / v2.38.0: wire the structured tool-use registry
			// into the engine so ```tool_call``` fences in the LLM
			// output are dispatched through it. Falls back to legacy
			// code-block emission when the LLM doesn't emit tool calls.
			// Use a nil check so a future "disable tools" flag is a
			// one-liner.
			if !runDisableTools {
				// Sprint 72 / v2.41.0: register MCP bridges before
				// passing the registry to the engine. Failures here
				// surface as a clear CLI error so operators see
				// which bridge failed and why.
				mcpBridges := make([]loop.MCPSpec, 0, len(runMCPBridges))
				for _, spec := range runMCPBridges {
					name, command, args, err := mcpbridge.ParseSpec(spec)
					if err != nil {
						return fmt.Errorf("invalid --mcp-bridge %q: %w", spec, err)
					}
					mcpBridges = append(mcpBridges, loop.MCPSpec{
						Name: name, Command: command, Args: args,
					})
				}
				registry, err := loop.RealRegistry(projectDir, mcpBridges...)
				if err != nil {
					return err
				}
				e.ToolRegistry = registry
			}
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
	runCmd.Flags().BoolVar(&runDisableTools, "no-tools", false, "disable structured tool-use; force the legacy code-block emission path (v2.37.0 behaviour)")
	runCmd.Flags().StringArrayVar(&runMCPBridges, "mcp-bridge", nil, "register an MCP server as a tool source (format: \"name:command args...\"). Repeatable. Example: --mcp-bridge \"github:npx -y @modelcontextprotocol/server-github\"")
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

	// ── config ──
	var configProvider string
	var configModel string

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

	// ── models route ──
	var routeAgent, routeAnchor string
	var routeDryRun, routeApply, routeJSON bool

	modelsRouteCmd := &cobra.Command{
		Use:   "route",
		Short: "Show or apply agent-aware model routing for this project",
		Long: `Detects which agent is hosting this session and computes the optimal
model for each development phase (research, plan, implement, verify, summarize).

Use --dry-run (default) to preview, --apply to write config files,
or --json for machine-readable output.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectDir := "."

			// Resolve anchor: flag > first detected family mid-tier > default.
			anchor := routeAnchor
			if anchor == "" {
				anchor = "claude-sonnet-4-6"
			}

			// Resolve agent: flag > auto-detect.
			agentID := routing.AgentID(routeAgent)
			strategy := routing.StrategySingleModelAdvisory
			if routeAgent != "" {
				strategy = routing.AgentStrategy(agentID)
			} else {
				agentID, strategy = routing.DetectAgent(projectDir)
			}

			plan := routing.Resolve(anchor, agentID, strategy)

			if routeJSON {
				data, err := json.MarshalIndent(plan, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(data))
				return nil
			}

			fmt.Print(routing.FormatPlan(plan))

			if routeApply {
				written, err := routing.Emit(projectDir, plan)
				if err != nil {
					return fmt.Errorf("emit routing: %w", err)
				}
				if len(written) > 0 {
					fmt.Println()
					for _, f := range written {
						rel, _ := filepath.Rel(projectDir, f)
						fmt.Printf("  ✓ Written: %s\n", rel)
					}
				} else {
					fmt.Println("\n  (no files to write — direct_api strategy routes at runtime)")
				}
			}
			return nil
		},
	}
	modelsRouteCmd.Flags().StringVar(&routeAgent, "agent", "", "agent ID (claude, codex, gemini, cursor, copilot, windsurf, hermes, opencode)")
	modelsRouteCmd.Flags().StringVar(&routeAnchor, "anchor", "", "anchor model preset (default: claude-sonnet-4-6)")
	modelsRouteCmd.Flags().BoolVar(&routeDryRun, "dry-run", true, "show plan without writing files (default)")
	modelsRouteCmd.Flags().BoolVar(&routeApply, "apply", false, "write routing config files")
	modelsRouteCmd.Flags().BoolVar(&routeJSON, "json", false, "output as JSON")
	modelsCmd.AddCommand(modelsRouteCmd)

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
}
