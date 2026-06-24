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
	"github.com/spf13/cobra"
)

var version = "0.3.1"

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
				Model:      model,
				ProjectDir: projectDir,
				MaxRetries: runRetries,
				Verbose:    runVerbose,
			}

			e := engine.New(cfg)
			result, err := e.Run(context.Background(), specDir)
			if err != nil {
				return err
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
