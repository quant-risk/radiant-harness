package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	radctx "github.com/quant-risk/radiant-harness/internal/context"
	"github.com/quant-risk/radiant-harness/internal/fleet"
	"github.com/quant-risk/radiant-harness/internal/improve"
	"github.com/quant-risk/radiant-harness/internal/llm"
	"github.com/quant-risk/radiant-harness/internal/loop"
	"github.com/quant-risk/radiant-harness/internal/worktree"
	"github.com/spf13/cobra"
)

func registerFleetCmds(root *cobra.Command) {
	// ── worktree (Sprint 42) ─────────────────────────────────────────────────
	worktreeCmd := &cobra.Command{
		Use:   "worktree",
		Short: "Manage isolated git worktrees for parallel agents",
	}

	worktreeAddCmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Create an isolated worktree on its own branch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			m, err := worktree.NewManager(cwd)
			if err != nil {
				return err
			}
			wt, err := m.Add(args[0])
			if err != nil {
				return err
			}
			fmt.Printf("✓ worktree created\n  path:   %s\n  branch: %s\n", wt.Path, wt.Branch)
			return nil
		},
	}

	worktreeListCmd := &cobra.Command{
		Use:   "list",
		Short: "List all worktrees",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			m, err := worktree.NewManager(cwd)
			if err != nil {
				return err
			}
			list, err := m.List()
			if err != nil {
				return err
			}
			for _, w := range list {
				branch := w.Branch
				if branch == "" {
					branch = "(detached)"
				}
				fmt.Printf("  %-50s %s\n", w.Path, branch)
			}
			return nil
		},
	}

	worktreeRemoveCmd := &cobra.Command{
		Use:   "remove <path>",
		Short: "Remove a worktree",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			m, err := worktree.NewManager(cwd)
			if err != nil {
				return err
			}
			force, _ := cmd.Flags().GetBool("force")
			return m.Remove(worktree.Worktree{Path: args[0]}, force)
		},
	}
	worktreeRemoveCmd.Flags().Bool("force", false, "Discard uncommitted changes")

	worktreePruneCmd := &cobra.Command{
		Use:   "prune",
		Short: "Remove administrative entries for deleted worktrees",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			m, err := worktree.NewManager(cwd)
			if err != nil {
				return err
			}
			return m.Prune()
		},
	}

	worktreeCmd.AddCommand(worktreeAddCmd, worktreeListCmd, worktreeRemoveCmd, worktreePruneCmd)
	root.AddCommand(worktreeCmd)

	// ── fleet (Sprint 39) ────────────────────────────────────────────────────
	fleetCmd := &cobra.Command{
		Use:   "fleet",
		Short: "Multi-agent coordination (Planner + Implementer + Verifier + Summarizer)",
	}

	fleetStartCmd := &cobra.Command{
		Use:   "start <goal>",
		Short: "Start a multi-agent fleet run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			goal := args[0]
			agentCount, _ := cmd.Flags().GetInt("agents")

			runID := fmt.Sprintf("fleet-%d", time.Now().Unix())
			store, err := fleet.NewStore(cwd, runID, goal)
			if err != nil {
				return fmt.Errorf("init store: %w", err)
			}

			coord := fleet.NewCoordinator(store, agentCount)

			// Register agents by role
			roles := []fleet.AgentRole{fleet.RolePlanner}
			for i := 0; i < agentCount-1; i++ {
				roles = append(roles, fleet.RoleImplementer)
			}
			roles = append(roles, fleet.RoleVerifier, fleet.RoleSummarizer)

			for i, role := range roles {
				coord.RegisterAgent(fmt.Sprintf("agent-%02d", i+1), role)
			}

			fmt.Printf("✓ Fleet started\n")
			fmt.Printf("  Run ID:  %s\n", runID)
			fmt.Printf("  Goal:    %s\n", goal)
			fmt.Printf("  Agents:  %d\n\n", len(roles))

			roles2 := fleet.DefaultRoleConfigs()
			for role, cfg := range roles2 {
				fmt.Printf("  %s\n", fleet.FormatRoleConfig(fleet.RoleConfig{
					Role:          role,
					TokenBudget:   cfg.TokenBudget,
					MaxIterations: cfg.MaxIterations,
				}))
			}
			fmt.Printf("\n  Store:   .radiant-harness/fleet/%s.json\n", runID)
			fmt.Printf("\nNext: `radiant fleet status %s`\n", runID)
			return nil
		},
	}
	fleetStartCmd.Flags().Int("agents", 3, "Number of implementer agents to run in parallel")

	fleetStatusCmd := &cobra.Command{
		Use:   "status <run-id>",
		Short: "Show multi-agent fleet status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			runID := args[0]
			store, err := fleet.LoadStore(cwd, runID)
			if err != nil {
				return fmt.Errorf("load fleet %q: %w", runID, err)
			}
			coord := fleet.NewCoordinator(store, 0)
			fmt.Print(fleet.FormatStatus(coord.Status()))
			return nil
		},
	}

	// ── fleet plan ───────────────────────────────────────────────────────────
	fleetPlanCmd := &cobra.Command{
		Use:   "plan <run-id>",
		Short: "Decompose the fleet goal into tasks (heuristic or LLM-assisted)",
		Long: `Plan reads the goal from an existing fleet run and decomposes it into
2–6 independently-executable tasks, writing them to the store.

When --model is provided the LLM is asked to produce the task list.
Without --model (or on LLM failure) a deterministic 3-task skeleton is
used: research → implement → verify.

Run plan before dispatch:
  radiant fleet start "goal" --agents 3
  radiant fleet plan   <run-id> --model claude-sonnet-4-6
  radiant fleet dispatch <run-id> --model claude-sonnet-4-6 --auto-route`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			runID := args[0]
			modelID, _ := cmd.Flags().GetString("model")

			store, err := fleet.LoadStore(cwd, runID)
			if err != nil {
				return fmt.Errorf("load fleet %q: %w", runID, err)
			}

			snap := store.Snapshot()
			goal := snap.Goal
			if goal == "" {
				return fmt.Errorf("fleet run %q has no goal stored", runID)
			}

			var client fleet.PlannerClient
			if modelID != "" {
				apiKey, _ := cmd.Flags().GetString("api-key")
				client = llm.NewClient(llm.Model{Model: modelID, APIKey: apiKey})
			}

			fmt.Printf("Planning fleet %s\n  Goal: %s\n", runID, goal)
			if client != nil {
				fmt.Printf("  Model: %s\n", modelID)
			} else {
				fmt.Printf("  Mode: heuristic (no model specified)\n")
			}
			fmt.Println()

			tasks, err := fleet.Plan(cmd.Context(), goal, client)
			if err != nil {
				return fmt.Errorf("plan: %w", err)
			}

			if err := store.SetTasks(tasks); err != nil {
				return fmt.Errorf("save tasks: %w", err)
			}

			fmt.Printf("✓ %d tasks written\n\n", len(tasks))
			for _, t := range tasks {
				fmt.Printf("  [%s] %s\n", t.ID, t.Title)
				fmt.Printf("        done when: %s\n", t.DoneWhen)
			}
			fmt.Printf("\nNext: `radiant fleet dispatch %s`\n", runID)
			return nil
		},
	}
	fleetPlanCmd.Flags().String("model", "", "LLM model for decomposition (optional; heuristic used when omitted)")
	fleetPlanCmd.Flags().String("api-key", "", "API key for the model (reads from env if omitted)")

	// ── fleet dispatch ───────────────────────────────────────────────────────
	fleetDispatchCmd := &cobra.Command{
		Use:   "dispatch <run-id>",
		Short: "Spawn one agent process per pending task in isolated worktrees",
		Long: `Dispatch claims all pending tasks from a fleet run and spawns one
radiant loop process per task in an isolated git worktree. Each agent
receives the task's DoneWhen criterion as its goal.

Flags like --model and --auto-route are forwarded to every agent subprocess
so all agents share the same model configuration.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			runID := args[0]
			modelID, _ := cmd.Flags().GetString("model")
			autoRoute, _ := cmd.Flags().GetBool("auto-route")
			timeoutMins, _ := cmd.Flags().GetInt("timeout")

			store, err := fleet.LoadStore(cwd, runID)
			if err != nil {
				return fmt.Errorf("load fleet %q: %w", runID, err)
			}

			iso, err := fleet.NewIsolator(store, cwd)
			if err != nil {
				return fmt.Errorf("init isolator: %w", err)
			}

			cfg := fleet.DispatchConfig{}
			if timeoutMins > 0 {
				cfg.Timeout = time.Duration(timeoutMins) * time.Minute
			}

			d, err := fleet.NewDispatcher(iso, cfg)
			if err != nil {
				return fmt.Errorf("init dispatcher: %w", err)
			}

			// Build extra args forwarded to each `loop start` subprocess.
			var extraArgs []string
			if modelID != "" {
				extraArgs = append(extraArgs, "--model", modelID)
			}
			if autoRoute {
				extraArgs = append(extraArgs, "--auto-route")
			}

			snap := store.Snapshot()
			pending := 0
			for _, t := range snap.Tasks {
				if t.Status == fleet.TaskPending {
					pending++
				}
			}
			fmt.Printf("✓ Dispatching fleet %s\n", runID)
			fmt.Printf("  Pending tasks: %d\n", pending)
			if modelID != "" {
				fmt.Printf("  Model:         %s\n", modelID)
			}
			if autoRoute {
				fmt.Printf("  Auto-route:    enabled\n")
			}
			if cfg.Timeout > 0 {
				fmt.Printf("  Timeout/agent: %v\n", cfg.Timeout)
			}
			fmt.Println()

			results, err := d.RunAll(context.Background(), extraArgs)
			if err != nil {
				return fmt.Errorf("dispatch: %w", err)
			}

			success, failed := 0, 0
			for _, r := range results {
				if r.ExitCode == 0 && r.Err == nil {
					success++
				} else {
					failed++
				}
			}
			fmt.Printf("✓ Dispatch complete: %d succeeded, %d failed\n", success, failed)
			fmt.Printf("  Run `radiant fleet status %s` to review results.\n", runID)
			return nil
		},
	}
	fleetDispatchCmd.Flags().String("model", "", "Model forwarded to each agent (default: agent uses RADIANT_MODEL or claude-sonnet-4-6)")
	fleetDispatchCmd.Flags().Bool("auto-route", false, "Forward --auto-route to each agent (research→top-tier, plan→mid, execute→anchor)")
	fleetDispatchCmd.Flags().Int("timeout", 0, "Per-agent timeout in minutes (0 = no timeout)")

	fleetCmd.AddCommand(fleetStartCmd, fleetStatusCmd, fleetPlanCmd, fleetDispatchCmd)
	root.AddCommand(fleetCmd)

	// ── improve (Sprint 38) ──────────────────────────────────────────────────
	improveCmd := &cobra.Command{
		Use:   "improve",
		Short: "Self-improvement engine — analyze traces, propose and apply skill edits",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			skill, _ := cmd.Flags().GetString("skill")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			apply, _ := cmd.Flags().GetBool("apply")
			fromTraces, _ := cmd.Flags().GetBool("from-traces")

			if !fromTraces && !apply {
				return fmt.Errorf("specify --from-traces to analyze, or --apply to apply validated proposals")
			}

			// Step 1: analyze traces
			fmt.Println("Analyzing traces...")
			analysis, err := improve.AnalyzeTraces(cwd, skill)
			if err != nil {
				return fmt.Errorf("analyze traces: %w", err)
			}
			fmt.Print(improve.FormatAnalysis(analysis))

			if len(analysis.Patterns) == 0 {
				return nil
			}

			// Step 2: propose edits
			proposals := improve.ProposeEdits(analysis.Patterns, cwd)
			fmt.Print(improve.FormatProposals(proposals))

			if dryRun || !apply {
				fmt.Println("\n(dry-run — pass --apply to apply validated proposals)")
				return nil
			}

			// Step 3: validate and apply
			applied := 0
			for _, proposal := range proposals {
				vr := improve.ValidateProposal(proposal, analysis)
				fmt.Print(improve.FormatValidationResult(vr))
				if !vr.Passed {
					continue
				}

				backupPath, err := improve.ApplyProposal(proposal, cwd)
				if err != nil {
					fmt.Printf("  ✗ Failed to apply: %v\n", err)
					continue
				}
				fmt.Printf("  ✓ Applied (backup: %s)\n", backupPath)

				record := improve.ImprovementRecord{
					Skill:       proposal.Skill,
					File:        proposal.File,
					Category:    string(proposal.Category),
					Description: proposal.Description,
					RunIDs:      proposal.RunIDs,
					OldScore:    vr.OldScore,
					NewScore:    vr.NewScore,
					DeltaPP:     vr.DeltaPP,
				}
				improve.PersistRecord(record, cwd)
				applied++
			}

			if applied == 0 {
				fmt.Println("\nNo proposals passed validation threshold (≥5pp improvement required).")
			} else {
				fmt.Printf("\n✓ Applied %d improvement(s). History: .radiant-harness/improvements.jsonl\n", applied)
			}
			return nil
		},
	}
	improveCmd.Flags().String("skill", "", "Filter analysis to a specific skill name")
	improveCmd.Flags().Bool("from-traces", false, "Analyze loop traces for failure patterns")
	improveCmd.Flags().Bool("dry-run", false, "Show proposals without applying")
	improveCmd.Flags().Bool("apply", false, "Apply validated proposals to skill files")

	improveHistoryCmd := &cobra.Command{
		Use:   "history",
		Short: "Show the improvement history log",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			records, err := improve.ReadHistory(cwd)
			if err != nil {
				return err
			}
			if len(records) == 0 {
				fmt.Println("No improvement history. Run `radiant improve --from-traces --apply` to start.")
				return nil
			}
			for _, r := range records {
				fmt.Printf("[%s] %s/%s — %s → +%.1fpp (%.0f%%→%.0f%%)\n",
					r.AppliedAt.Format("2006-01-02"),
					r.Skill, r.File,
					r.Category,
					r.DeltaPP, r.OldScore*100, r.NewScore*100)
			}
			return nil
		},
	}

	improveCmd.AddCommand(improveHistoryCmd)
	root.AddCommand(improveCmd)

	// ── budget (Sprint 37) ───────────────────────────────────────────────────
	budgetCmd := &cobra.Command{
		Use:   "budget",
		Short: "Token budget estimation and reporting",
	}

	budgetEstimateCmd := &cobra.Command{
		Use:   "estimate [spec-dir-or-file]",
		Short: "Estimate token consumption per phase before running",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			profile, _ := cmd.Flags().GetString("profile")
			prof := radctx.GetProfile(profile)

			var content string
			if len(args) > 0 {
				data, err := os.ReadFile(args[0])
				if err != nil {
					return fmt.Errorf("read %s: %w", args[0], err)
				}
				content = string(data)
			}

			estimates := radctx.EstimateSpec(content, prof)
			fmt.Print(radctx.FormatEstimate(estimates, prof))
			return nil
		},
	}
	budgetEstimateCmd.Flags().String("profile", "standard", "Budget profile: lean|standard|thorough")

	budgetReportCmd := &cobra.Command{
		Use:   "report <run-id>",
		Short: "Show post-run token usage report from trace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			runID := args[0]
			tracePath := filepath.Join(cwd, ".radiant-harness", "traces", runID+".jsonl")

			events, err := loop.ReadTrace(tracePath)
			if err != nil {
				return fmt.Errorf("trace %q not found: %w", runID, err)
			}

			// Aggregate tokens per phase
			phaseIn := map[string]int{}
			phaseOut := map[string]int{}
			for _, e := range events {
				phaseIn[string(e.Phase)] += e.TokensIn
				phaseOut[string(e.Phase)] += e.TokensOut
			}

			fmt.Printf("Token report — run: %s (%d events)\n\n", runID, len(events))
			fmt.Printf("%-12s %10s %10s %10s\n", "Phase", "Tokens In", "Tokens Out", "Total")
			fmt.Println("------------ ---------- ---------- ----------")
			totalIn, totalOut := 0, 0
			for _, phase := range []string{"discover", "plan", "execute", "verify", "persist", "unknown"} {
				in, out := phaseIn[phase], phaseOut[phase]
				if in+out == 0 {
					continue
				}
				fmt.Printf("%-12s %10d %10d %10d\n", phase, in, out, in+out)
				totalIn += in
				totalOut += out
			}
			fmt.Println("------------ ---------- ---------- ----------")
			fmt.Printf("%-12s %10d %10d %10d\n", "TOTAL", totalIn, totalOut, totalIn+totalOut)
			return nil
		},
	}

	budgetCmd.AddCommand(budgetEstimateCmd, budgetReportCmd)
	root.AddCommand(budgetCmd)

}
