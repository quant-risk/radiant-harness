package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	radctx "github.com/quant-risk/radiant-harness/internal/context"
	"github.com/quant-risk/radiant-harness/internal/fleet"
	"github.com/quant-risk/radiant-harness/internal/improve"
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

	fleetCmd.AddCommand(fleetStartCmd, fleetStatusCmd)
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
