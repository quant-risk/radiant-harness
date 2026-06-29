//go:build with_full

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/quant-risk/radiant-harness/internal/config"
	"github.com/quant-risk/radiant-harness/internal/llm"
	"github.com/quant-risk/radiant-harness/internal/loop"
	"github.com/quant-risk/radiant-harness/internal/schedule"
	"github.com/quant-risk/radiant-harness/internal/webhook"
	"github.com/spf13/cobra"
)

func registerLoopCmds(root *cobra.Command) {
	// ── loop (Sprint 35) ─────────────────────────────────────────────────────
	loopCmd := &cobra.Command{
		Use:   "loop",
		Short: "Manage the autonomous feedback loop (start, status, resume)",
	}

	loopStartCmd := &cobra.Command{
		Use:   "start <goal>",
		Short: "Start an autonomous Discover→Plan→Execute→Verify→Persist loop",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			goal := args[0]

			// Load project config — provides defaults for unset flags.
			cfg, _ := config.Load(cwd)
			if cfg == nil {
				cfg = &config.Config{}
			}

			budget, _ := cmd.Flags().GetInt("budget")
			maxIter, _ := cmd.Flags().GetInt("max-iter")
			if maxIter == 0 && cfg.MaxIter > 0 {
				maxIter = cfg.MaxIter
			}
			profile, _ := cmd.Flags().GetString("profile")
			if profile == "standard" && cfg.Profile != "" {
				profile = cfg.Profile
			}

			runID := fmt.Sprintf("run-%d", time.Now().Unix())

			maxTime, _ := cmd.Flags().GetDuration("max-time")
			maxCost, _ := cmd.Flags().GetFloat64("max-cost")
			modelID, _ := cmd.Flags().GetString("model")
			verifierModelID, _ := cmd.Flags().GetString("verifier-model")
			plannerModelID, _ := cmd.Flags().GetString("planner-model")
			baseURL, _ := cmd.Flags().GetString("base-url")
			stall, _ := cmd.Flags().GetInt("stall-patience")
			quorumK, _ := cmd.Flags().GetInt("quorum-k")
			quorumN, _ := cmd.Flags().GetInt("quorum-n")
			ground, _ := cmd.Flags().GetBool("ground")
			reviewRestarts, _ := cmd.Flags().GetInt("review-restarts")
			contextBudget, _ := cmd.Flags().GetInt("context-budget")
			stream, _ := cmd.Flags().GetBool("stream")
			plan, _ := cmd.Flags().GetBool("plan")
			autoRoute, _ := cmd.Flags().GetBool("auto-route")
			dryRun, _ := cmd.Flags().GetBool("dry-run")

			// `radiant loop start` is always Full mode — the harness calls
			// LLM HTTP endpoints directly with the operator's API key. For
			// Light mode (MCP sampling), use `radiant mcp-serve` and let the
			// host agent drive the loop. No flag/env/config to set; the
			// behaviour emerges from which subcommand you ran.

			// Resolve API key from env (vendor-neutral order).
			apiKey, resolvedBaseURL := resolveLoopLLMCreds(baseURL)
			if apiKey == "" && !dryRun {
				return fmt.Errorf(
					"no LLM API key found — set one of: OPENROUTER_API_KEY, OPENAI_API_KEY, ANTHROPIC_API_KEY\n" +
						"  or use --dry-run to register the goal without calling an LLM")
			}

			// Resolve model: flag > env > config > provider default.
			if modelID == "" {
				modelID = os.Getenv("RADIANT_MODEL")
			}
			if modelID == "" && cfg.Model != "" {
				modelID = cfg.Model
			}
			if modelID == "" {
				modelID = "claude-sonnet-4-6"
			}
			if !autoRoute && cfg.AutoRoute {
				autoRoute = true
			}
			if verifierModelID == "" {
				verifierModelID = modelID
			}

			execModel := llm.Model{Model: modelID, APIKey: apiKey, BaseURL: resolvedBaseURL}
			verModel := llm.Model{Model: verifierModelID, APIKey: apiKey, BaseURL: resolvedBaseURL}
			if plannerModelID == "" {
				plannerModelID = modelID
			}
			planModel := llm.Model{Model: plannerModelID, APIKey: apiKey, BaseURL: resolvedBaseURL}

			// Resolve cost-per-1K from model pricing table.
			costPer1K, _ := loop.PriceFor(modelID)

			if quorumN == 0 && quorumK > 0 {
				quorumN = quorumK + 1
			}

			intensityFlag, _ := cmd.Flags().GetString("intensity")
			runCfg := loop.RunConfig{
				ExecutorModel: execModel,
				VerifierModel: verModel,
				PlannerModel:  planModel,
				Budget: loop.BudgetConfig{
					MaxTokens:   budget,
					MaxIter:     maxIter,
					Profile:     loop.BudgetProfile(profile),
					MaxDuration: maxTime,
					MaxCostUSD:  maxCost,
					CostPer1K:   costPer1K,
				},
				StallPatience: stall,
				Verifier: loop.VerifierConfig{
					Quorum: loop.QuorumConfig{K: quorumK, N: quorumN},
				},
				Review:              loop.ReviewPanel{MaxRestarts: reviewRestarts},
				Ground:              ground,
				MaxGroundCommits:    0,
				ContextBudgetTokens: contextBudget,
				Stream:              stream,
				Plan:                plan,
				AutoRoute:           autoRoute,
				Intensity:           intensityFlag,
			}

			fmt.Printf("✓ Loop starting\n")
			fmt.Printf("  Run ID:  %s\n", runID)
			fmt.Printf("  Goal:    %s\n", goal)
			fmt.Printf("  Model:   %s\n", modelID)
			if modelID != verifierModelID {
				fmt.Printf("  Verifier: %s\n", verifierModelID)
			}
			if maxTime > 0 {
				fmt.Printf("  Max time: %s\n", maxTime)
			}
			if maxCost > 0 {
				fmt.Printf("  Max cost: $%.2f\n", maxCost)
			}
			if stall > 0 {
				fmt.Printf("  Stall brake: %d identical outputs\n", stall)
			}
			if quorumK > 0 {
				fmt.Printf("  Quorum: %d-of-%d judges\n", quorumK, quorumN)
			}
			if ground {
				fmt.Printf("  Grounding: enabled\n")
			}
			fmt.Println()

			if dryRun {
				fmt.Println("(dry-run: goal registered, no LLM calls made)")
				return nil
			}

			webhookURL, _ := cmd.Flags().GetString("webhook-url")
			if webhookURL == "" {
				webhookURL = cfg.WebhookURL
			}

			logJSONFlag, _ := cmd.Flags().GetBool("log-json")
			if logJSONFlag {
				runCfg.LogJSON = os.Stdout
			}

			result, err := loop.Run(context.Background(), cwd, runID, goal, runCfg)
			if err != nil {
				_ = webhook.Send(context.Background(), webhookURL, webhook.Payload{
					Event: webhook.EventLoopFailed,
					RunID: runID,
					Data:  map[string]any{"error": err.Error(), "goal": goal},
				})
				return fmt.Errorf("loop: %w", err)
			}

			fmt.Printf("✓ Loop finished\n")
			fmt.Printf("  Exit:       %s\n", result.ExitReason)
			fmt.Printf("  Iterations: %d\n", result.Iterations)
			fmt.Printf("  Elapsed:    %s\n", result.Elapsed.Round(time.Second))
			fmt.Printf("  Tokens:     %d\n", result.TokensUsed)
			if result.CostUSD > 0 {
				fmt.Printf("  Cost:       $%.4f\n", result.CostUSD)
			}
			if result.ExitReason == loop.ExitNeedsHuman {
				fmt.Printf("\nAction required: radiant loop review\n")
			}

			_ = webhook.Send(context.Background(), webhookURL, webhook.Payload{
				Event: webhook.EventLoopDone,
				RunID: runID,
				Data: map[string]any{
					"goal":       goal,
					"exit":       string(result.ExitReason),
					"iterations": result.Iterations,
					"tokens":     result.TokensUsed,
					"cost_usd":   result.CostUSD,
				},
			})
			return nil
		},
	}
	loopStartCmd.Flags().String("webhook-url", "", "URL to POST a JSON event when the loop finishes")
	loopStartCmd.Flags().Bool("log-json", false, "Emit structured JSONL to stdout for each LLM call (one entry per line)")
	loopStartCmd.Flags().Int("budget", 0, "Token budget (0 = use profile default)")
	loopStartCmd.Flags().Int("max-iter", 0, "Max iterations (0 = use default 20)")
	loopStartCmd.Flags().String("profile", "standard", "Budget profile: lean|standard|thorough")
	loopStartCmd.Flags().Duration("max-time", 0, "Wall-clock time limit per run (e.g. 30m, 2h). 0 = unlimited")
	loopStartCmd.Flags().Float64("max-cost", 0, "Dollar cost ceiling for the run (e.g. 0.50). 0 = unlimited")
	loopStartCmd.Flags().String("model", "", "Model ID for cost tracking (e.g. claude-sonnet-4-6)")
	loopStartCmd.Flags().Int("stall-patience", 0, "No-progress brake: halt after N identical actions (0 = disabled)")
	loopStartCmd.Flags().Int("quorum-k", 0, "Minimum passing judges for quorum verification (0 = disabled)")
	loopStartCmd.Flags().Int("quorum-n", 0, "Total judges for quorum (default = quorum-k+1)")
	loopStartCmd.Flags().Bool("ground", false, "Inject recent commit log into each iteration prompt")
	loopStartCmd.Flags().Int("review-restarts", 0, "Post-convergence review panel max restarts (0 = default 3)")
	loopStartCmd.Flags().String("verifier-model", "", "Separate model for verification (default = same as --model)")
	loopStartCmd.Flags().String("base-url", "", "Override LLM base URL (e.g. http://localhost:11434/v1)")
	loopStartCmd.Flags().Bool("dry-run", false, "Register goal and print config without calling any LLM")
	loopStartCmd.Flags().Int("context-budget", 0, "Inject assembled CONTEXT.md into executor prompt (tokens cap, e.g. 6000). 0 = disabled")
	loopStartCmd.Flags().Bool("stream", false, "Stream executor output to stdout chunk by chunk (verifier stays non-streaming)")
	loopStartCmd.Flags().Bool("plan", false, "Call the LLM in the Plan phase to decompose the goal before each executor call")
	loopStartCmd.Flags().String("planner-model", "", "Model used for planning (default = same as --model; a cheaper model like haiku is often sufficient)")
	loopStartCmd.Flags().Bool("auto-route", false, "Auto-select per-phase models from the anchor's preset family (research→top-tier, plan→mid, execute→anchor)")
	loopStartCmd.Flags().String("intensity", "", "Executor intensity for lazy-executor skill: lite|full|ultra|off. Default: full. lite = build what was asked, suggest lazy alt. full = ladder enforced. ultra = YAGNI extremist. off = no skill injected.")

	loopStatusCmd := &cobra.Command{
		Use:   "status [run-id]",
		Short: "Show loop progress — live state or trace summary for a run-id",
		Long: `Without a run-id, shows the current active loop state.
With a run-id, reads the JSONL trace and shows iteration, phase, token totals, and last action.

Examples:
  radiant loop status                       # active loop
  radiant loop status my-run-2026-06-27     # trace summary`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			asJSON, _ := cmd.Flags().GetBool("json")

			if len(args) == 1 {
				runID := args[0]
				path := loop.TracePath(cwd, runID)
				events, err := loop.ReadTrace(path)
				if err != nil {
					return fmt.Errorf("read trace for %q: %w", runID, err)
				}
				modelID, _ := cmd.Flags().GetString("model")
				if asJSON {
					infos, _ := loop.ListTraceInfos(cwd)
					for _, info := range infos {
						if info.RunID == runID {
							enc := json.NewEncoder(os.Stdout)
							enc.SetIndent("", "  ")
							return enc.Encode(info)
						}
					}
					// Fallback: synthesise from events inline.
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(map[string]interface{}{
						"run_id":      runID,
						"model":       modelID,
						"event_count": len(events),
					})
				}
				fmt.Print(loop.FormatProgress(runID, modelID, events))
				return nil
			}

			c, err := loop.LoadCycle(cwd)
			if err != nil {
				fmt.Println("No active loop. Start one with: radiant loop start \"<goal>\"")
				return nil
			}
			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(c.State())
			}
			fmt.Print(loop.FormatStatus(c.State()))
			return nil
		},
	}

	loopResumeCmd := &cobra.Command{
		Use:   "resume",
		Short: "Resume an interrupted loop from its last persisted state",
		Long: `Load the last persisted loop state and continue running from where it left off.
Accepts the same LLM flags as 'loop start'. When no flags are given, uses
the same env-var resolution as 'start' (OPENROUTER_API_KEY, etc.).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			c, err := loop.LoadCycle(cwd)
			if err != nil {
				return fmt.Errorf("no loop state found (run `radiant loop start` first): %w", err)
			}
			state := c.State()

			if state.ExitReason != "" && state.ExitReason != loop.ExitNeedsHuman {
				return fmt.Errorf("loop already finished with exit=%s — start a new one with `radiant loop start`", state.ExitReason)
			}

			modelID, _ := cmd.Flags().GetString("model")
			verifierModelID, _ := cmd.Flags().GetString("verifier-model")
			plannerModelID, _ := cmd.Flags().GetString("planner-model")
			baseURL, _ := cmd.Flags().GetString("base-url")
			plan, _ := cmd.Flags().GetBool("plan")
			autoRoute, _ := cmd.Flags().GetBool("auto-route")
			dryRun, _ := cmd.Flags().GetBool("dry-run")

			apiKey, resolvedBaseURL := resolveLoopLLMCreds(baseURL)
			if apiKey == "" && !dryRun {
				return fmt.Errorf(
					"no LLM API key found — set one of: OPENROUTER_API_KEY, OPENAI_API_KEY, ANTHROPIC_API_KEY\n" +
						"  or use --dry-run to inspect state without resuming")
			}

			if modelID == "" {
				modelID = os.Getenv("RADIANT_MODEL")
			}
			if modelID == "" {
				modelID = "claude-sonnet-4-6"
			}
			if verifierModelID == "" {
				verifierModelID = modelID
			}
			if plannerModelID == "" {
				plannerModelID = modelID
			}

			execModel := llm.Model{Model: modelID, APIKey: apiKey, BaseURL: resolvedBaseURL}
			verModel := llm.Model{Model: verifierModelID, APIKey: apiKey, BaseURL: resolvedBaseURL}
			planModel := llm.Model{Model: plannerModelID, APIKey: apiKey, BaseURL: resolvedBaseURL}
			costPer1K, _ := loop.PriceFor(modelID)

			// Restore budget config from persisted snapshot.
			snap := state.Budget
			runCfg := loop.RunConfig{
				ExecutorModel: execModel,
				VerifierModel: verModel,
				PlannerModel:  planModel,
				Budget: loop.BudgetConfig{
					MaxTokens:  snap.MaxTokens,
					MaxIter:    state.MaxIter,
					MaxCostUSD: snap.MaxCostUSD,
					CostPer1K:  costPer1K,
				},
				Plan:      plan,
				AutoRoute: autoRoute,
			}

			fmt.Printf("✓ Resuming loop %s\n", state.RunID)
			fmt.Printf("  Goal:   %s\n", state.Goal)
			fmt.Printf("  Phase:  %s  (iter %d/%d)\n", state.Phase, state.Iteration, state.MaxIter)
			fmt.Printf("  Model:  %s\n", modelID)
			fmt.Println()

			if dryRun {
				fmt.Println("(dry-run: state loaded, no LLM calls made)")
				return nil
			}

			result, err := loop.Run(context.Background(), cwd, state.RunID, state.Goal, runCfg)
			if err != nil {
				return fmt.Errorf("loop resume: %w", err)
			}

			fmt.Printf("✓ Loop finished\n")
			fmt.Printf("  Exit:       %s\n", result.ExitReason)
			fmt.Printf("  Iterations: %d\n", result.Iterations)
			fmt.Printf("  Elapsed:    %s\n", result.Elapsed.Round(time.Second))
			fmt.Printf("  Tokens:     %d\n", result.TokensUsed)
			if result.CostUSD > 0 {
				fmt.Printf("  Cost:       $%.4f\n", result.CostUSD)
			}
			if result.ExitReason == loop.ExitNeedsHuman {
				fmt.Printf("\nAction required: radiant loop review\n")
			}
			return nil
		},
	}
	loopResumeCmd.Flags().String("model", "", "Model ID for the resumed run (default = claude-sonnet-4-6)")
	loopResumeCmd.Flags().String("verifier-model", "", "Separate model for verification")
	loopResumeCmd.Flags().String("planner-model", "", "Model used for planning (default = same as --model)")
	loopResumeCmd.Flags().String("base-url", "", "Override LLM base URL")
	loopResumeCmd.Flags().Bool("plan", false, "Call the LLM in the Plan phase to decompose the goal")
	loopResumeCmd.Flags().Bool("auto-route", false, "Auto-select per-phase models from the anchor's preset family")
	loopResumeCmd.Flags().Bool("dry-run", false, "Inspect persisted state without calling any LLM")

	loopScheduleCmd := &cobra.Command{
		Use:   "schedule",
		Short: "Evaluate work signals and decide whether to dispatch a loop run",
		Long: `The Schedule stage of the loop cycle: reads work signals from the repo
(new commits, pending TODO/FIXME, optionally a failing gate) and decides under a
policy whether to re-dispatch an autonomous run. With --check it only reports the
decision; without it, a RUN decision advances and persists scheduler state.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			checkOnly, _ := cmd.Flags().GetBool("check")
			gateFailing, _ := cmd.Flags().GetBool("gate-failing")
			minInterval, _ := cmd.Flags().GetDuration("min-interval")
			maxRuns, _ := cmd.Flags().GetInt("max-per-day")

			state, err := schedule.LoadState(cwd)
			if err != nil {
				return fmt.Errorf("load schedule state: %w", err)
			}

			policy := schedule.DefaultPolicy()
			if minInterval > 0 {
				policy.MinInterval = minInterval
			}
			if maxRuns > 0 {
				policy.MaxRunsPerDay = maxRuns
			}

			signals := schedule.DetectSignals(cwd, state)
			if gateFailing {
				signals = append(signals, schedule.Signal{
					Kind: schedule.TriggerFailingGate, Detail: "reported via --gate-failing", Value: 1,
				})
			}

			now := time.Now().UTC()
			decision := schedule.Evaluate(policy, state, signals, now)
			fmt.Print(schedule.FormatDecision(decision, now))

			if decision.ShouldRun && !checkOnly {
				newState := schedule.RecordRun(state, schedule.CurrentCommit(cwd), now)
				if err := schedule.SaveState(cwd, newState); err != nil {
					return fmt.Errorf("persist schedule state: %w", err)
				}
				fmt.Printf("\nDispatch: `radiant loop start \"<goal>\"` — scheduler state advanced (run %d today).\n", newState.RunsToday)
			}
			return nil
		},
	}
	loopScheduleCmd.Flags().Bool("check", false, "Only report the decision; do not advance scheduler state")
	loopScheduleCmd.Flags().Bool("gate-failing", false, "Signal that a gate is currently red")
	loopScheduleCmd.Flags().Duration("min-interval", 0, "Override min interval between runs (e.g. 15m)")
	loopScheduleCmd.Flags().Int("max-per-day", 0, "Override max runs per day")

	loopReviewCmd := &cobra.Command{
		Use:   "review",
		Short: "List and resolve escalated items waiting for human review",
		Long: `When the verifier sets escalate=true, the loop stops with status needs_human
and writes an item to .radiant-harness/inbox/. Use this command to list pending
items and approve or reject each one.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			approveID, _ := cmd.Flags().GetString("approve")
			rejectID, _ := cmd.Flags().GetString("reject")

			if approveID != "" {
				if err := loop.ResolveInboxItem(cwd, approveID); err != nil {
					return err
				}
				fmt.Printf("✓ Approved and resolved: %s\n", approveID)
				fmt.Printf("  Resume the loop with: radiant loop resume\n")
				return nil
			}
			if rejectID != "" {
				if err := loop.ResolveInboxItem(cwd, rejectID); err != nil {
					return err
				}
				fmt.Printf("✗ Rejected and resolved: %s\n", rejectID)
				fmt.Printf("  The loop will not resume for this item.\n")
				return nil
			}

			items, err := loop.ListInboxItems(cwd)
			if err != nil {
				return fmt.Errorf("list inbox: %w", err)
			}
			if len(items) == 0 {
				fmt.Println("Inbox is empty — no items waiting for review.")
				return nil
			}
			fmt.Printf("Pending review (%d item(s)):\n\n", len(items))
			for _, item := range items {
				fmt.Printf("  ID:        %s\n", item.ID)
				fmt.Printf("  Run:       %s  (iter %d)\n", item.RunID, item.Iteration)
				fmt.Printf("  Goal:      %s\n", item.Goal)
				fmt.Printf("  Evidence:  %s\n", item.Evidence)
				for _, issue := range item.Issues {
					fmt.Printf("  Issue:     %s\n", issue)
				}
				fmt.Printf("  Created:   %s\n", item.CreatedAt.Format("2006-01-02 15:04 UTC"))
				fmt.Println()
			}
			fmt.Printf("To approve: radiant loop review --approve <id>\n")
			fmt.Printf("To reject:  radiant loop review --reject <id>\n")
			return nil
		},
	}
	loopReviewCmd.Flags().String("approve", "", "Resolve item as approved (resumes loop)")
	loopReviewCmd.Flags().String("reject", "", "Resolve item as rejected (abandons loop run)")

	loopListCmd := &cobra.Command{
		Use:   "list",
		Short: "List all loop runs with event count, phase and last result",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			plain, _ := cmd.Flags().GetBool("plain")
			if plain {
				ids, err := loop.ListTraces(cwd)
				if err != nil {
					return err
				}
				for _, id := range ids {
					fmt.Println(id)
				}
				return nil
			}
			infos, err := loop.ListTraceInfos(cwd)
			if err != nil {
				return err
			}
			fmt.Print(loop.FormatTraceList(infos))
			return nil
		},
	}
	loopListCmd.Flags().Bool("plain", false, "Output bare run IDs only (one per line)")
	loopStatusCmd.Flags().Bool("json", false, "Output as JSON")

	// ── loop export (Sprint 70) ──────────────────────────────────────────────
	loopExportCmd := &cobra.Command{
		Use:   "export <run-id>",
		Short: "Export a loop run as JSON or Markdown",
		Long: `Export reads the JSONL trace for the given run and emits a structured
document — JSON by default, Markdown with --format=md.

Examples:
  radiant loop export my-run-2026-06-27
  radiant loop export my-run-2026-06-27 --format md > report.md
  radiant loop export my-run-2026-06-27 --model claude-sonnet-4-6`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			runID := args[0]
			modelID, _ := cmd.Flags().GetString("model")
			format, _ := cmd.Flags().GetString("format")

			path := loop.TracePath(cwd, runID)
			events, err := loop.ReadTrace(path)
			if err != nil {
				return fmt.Errorf("read trace %q: %w", runID, err)
			}
			exp := loop.ExportTrace(runID, modelID, events)

			switch format {
			case "md", "markdown":
				fmt.Print(loop.ExportTraceMarkdown(exp))
			default:
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(exp)
			}
			return nil
		},
	}
	loopExportCmd.Flags().String("format", "json", "Output format: json or md")
	loopExportCmd.Flags().String("model", "", "Model ID for cost estimation")

	// ── loop diff (Sprint 72) ────────────────────────────────────────────────
	loopDiffCmd := &cobra.Command{
		Use:   "diff <run-id>",
		Short: "Show files changed during a loop run (git diff vs base branch)",
		Long: `Diff runs git diff between the loop's branch and the base branch (default: main).
The loop dispatcher creates a branch named after the run-id in its worktree.
If the branch was pruned, diff falls back to listing trace events that recorded
file modifications.

Examples:
  radiant loop diff my-run-2026-06-27
  radiant loop diff my-run-2026-06-27 --base main
  radiant loop diff my-run-2026-06-27 --stat`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			runID := args[0]
			base, _ := cmd.Flags().GetString("base")
			statOnly, _ := cmd.Flags().GetBool("stat")
			if base == "" {
				base = "main"
			}

			// Try git diff against the run branch.
			gitArgs := []string{"diff", base + "..." + runID}
			if statOnly {
				gitArgs = []string{"diff", "--stat", base + "..." + runID}
			}
			out, err := runGitInDir(cwd, gitArgs...)
			if err == nil {
				if out == "" {
					fmt.Printf("No changes between %s and %s.\n", base, runID)
				} else {
					fmt.Print(out)
				}
				return nil
			}

			// Branch not found — fall back to trace events.
			path := loop.TracePath(cwd, runID)
			events, readErr := loop.ReadTrace(path)
			if readErr != nil {
				return fmt.Errorf("branch %q not found and trace unreadable: %w", runID, readErr)
			}
			fmt.Printf("Branch %q not found; files referenced in trace:\n", runID)
			seen := map[string]bool{}
			for _, e := range events {
				if e.Evidence != "" && !seen[e.Evidence] {
					seen[e.Evidence] = true
					fmt.Printf("  %s\n", e.Evidence)
				}
			}
			if len(seen) == 0 {
				fmt.Println("  (no file references in trace)")
			}
			return nil
		},
	}
	loopDiffCmd.Flags().String("base", "main", "Base branch to diff against")
	loopDiffCmd.Flags().Bool("stat", false, "Show --stat summary instead of full diff")

	// ── loop cancel (Sprint 77) ──────────────────────────────────────────────
	loopCancelCmd := &cobra.Command{
		Use:   "cancel <run-id>",
		Short: "Gracefully stop a running loop by sending SIGTERM",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			runID := args[0]
			if err := loop.CancelRun(cwd, runID); err != nil {
				return err
			}
			fmt.Printf("SIGTERM sent to loop %q — it will stop after the current iteration.\n", runID)
			return nil
		},
	}

	// ── loop history (Sprint 80) ─────────────────────────────────────────────
	loopHistoryCmd := &cobra.Command{
		Use:   "history",
		Short: "Show aggregate stats for past loop runs",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			asJSON, _ := cmd.Flags().GetBool("json")
			infos, err := loop.ListTraceInfos(cwd)
			if err != nil {
				return err
			}
			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(infos)
			}
			if len(infos) == 0 {
				fmt.Println("No loop runs found.")
				return nil
			}
			var totalCost float64
			var totalTokens int
			ok, failed := 0, 0
			for _, i := range infos {
				totalCost += i.CostUSD
				totalTokens += i.TokensIn + i.TokensOut
				if i.LastResult == "ok" {
					ok++
				} else if i.LastResult == "failed" {
					failed++
				}
			}
			fmt.Printf("Loop history — %d run(s)\n", len(infos))
			fmt.Printf("  Success:      %d\n", ok)
			fmt.Printf("  Failed:       %d\n", failed)
			fmt.Printf("  Total tokens: %d\n", totalTokens)
			if totalCost > 0 {
				fmt.Printf("  Total cost:   $%.4f\n", totalCost)
			}
			fmt.Println()
			fmt.Print(loop.FormatTraceList(infos))
			return nil
		},
	}
	loopHistoryCmd.Flags().Bool("json", false, "Output as JSON array")

	loopCmd.AddCommand(loopStartCmd, loopStatusCmd, loopResumeCmd, loopScheduleCmd, loopReviewCmd, loopListCmd, loopExportCmd, loopDiffCmd, loopCancelCmd, loopHistoryCmd)
	root.AddCommand(loopCmd)

	// ── trace (Sprint 35) ────────────────────────────────────────────────────
	traceCmd := &cobra.Command{
		Use:   "trace",
		Short: "Inspect reasoning traces from loop runs",
	}

	traceShowCmd := &cobra.Command{
		Use:   "show <run-id>",
		Short: "Display the reasoning trace for a run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			runID := args[0]
			jsonMode, _ := cmd.Flags().GetBool("json")

			tracePath := filepath.Join(cwd, ".radiant-harness", "traces", runID+".jsonl")
			events, err := loop.ReadTrace(tracePath)
			if err != nil {
				return fmt.Errorf("run %q not found: %w", runID, err)
			}

			if jsonMode {
				b, _ := json.MarshalIndent(events, "", "  ")
				fmt.Println(string(b))
				return nil
			}

			fmt.Printf("Trace: %s (%d events)\n\n", runID, len(events))
			fmt.Print(loop.FormatTrace(events))
			return nil
		},
	}
	traceShowCmd.Flags().Bool("json", false, "Output raw JSON")

	traceListCmd := &cobra.Command{
		Use:   "list",
		Short: "List all trace runs with event count, phase and last result",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			plain, _ := cmd.Flags().GetBool("plain")
			if plain {
				ids, err := loop.ListTraces(cwd)
				if err != nil {
					return err
				}
				for _, id := range ids {
					fmt.Println(id)
				}
				return nil
			}
			infos, err := loop.ListTraceInfos(cwd)
			if err != nil {
				return err
			}
			fmt.Print(loop.FormatTraceList(infos))
			return nil
		},
	}
	traceListCmd.Flags().Bool("plain", false, "Output bare run IDs only (one per line)")

	traceCmd.AddCommand(traceShowCmd, traceListCmd)
	root.AddCommand(traceCmd)
}
