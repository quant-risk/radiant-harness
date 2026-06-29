//go:build !light_only

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	radctx "github.com/quant-risk/radiant-harness/internal/context"
	"github.com/quant-risk/radiant-harness/internal/ontology"
	"github.com/spf13/cobra"
)

func registerContextCmds(root *cobra.Command) {
	// ── context (Sprint 33) ──────────────────────────────────────────────────
	contextCmd := &cobra.Command{
		Use:   "context",
		Short: "Manage project context (detect domain, assemble minimal CONTEXT.md, compress)",
	}

	contextDetectCmd := &cobra.Command{
		Use:   "detect",
		Short: "Detect project domain, tier and recommended skills",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			jsonMode, _ := cmd.Flags().GetBool("json")

			r, err := radctx.Detect(cwd)
			if err != nil {
				return err
			}

			if jsonMode {
				b, err := json.MarshalIndent(r, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(b))
				return nil
			}

			fmt.Printf("Domain:  %s\n", r.Domain)
			fmt.Printf("Tier:    %s\n", r.Tier)
			fmt.Printf("Project: %s\n", r.ProjectName)
			if r.ActiveSpec != "" {
				fmt.Printf("Spec:    %s\n", r.ActiveSpec)
			}
			fmt.Printf("Signals: %s\n", strings.Join(r.Signals, ", "))
			fmt.Printf("Skills:  %s\n", strings.Join(r.RecommendedSkills, ", "))
			return nil
		},
	}
	contextDetectCmd.Flags().Bool("json", false, "Output JSON")

	contextAssembleCmd := &cobra.Command{
		Use:   "assemble",
		Short: "Build .radiant-harness/CONTEXT.md with only skills relevant to this project",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			budget, _ := cmd.Flags().GetInt("budget")
			withSpec, _ := cmd.Flags().GetBool("with-spec")
			extra, _ := cmd.Flags().GetStringSlice("skills")

			r, err := radctx.Detect(cwd)
			if err != nil {
				return fmt.Errorf("detect: %w", err)
			}

			outPath, tokens, err := radctx.Assemble(cwd, r, radctx.AssembleOptions{
				BudgetTokens:      budget,
				IncludeActiveSpec: withSpec,
				ExtraSkills:       extra,
			})
			if err != nil {
				return err
			}

			fmt.Printf("✓ %s (%d skills, ~%d tokens)\n",
				outPath, len(r.RecommendedSkills)+len(extra), tokens)
			return nil
		},
	}
	contextAssembleCmd.Flags().Int("budget", 0, "Token budget (0 = no limit)")
	contextAssembleCmd.Flags().Bool("with-spec", false, "Include active spec tasks.md in context")
	contextAssembleCmd.Flags().StringSlice("skills", nil, "Additional skills to include")

	contextCompressCmd := &cobra.Command{
		Use:   "compress",
		Short: "Compress .radiant-harness/CONTEXT.md to fit a token budget",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			budget, _ := cmd.Flags().GetInt("budget")
			if budget <= 0 {
				return fmt.Errorf("--budget is required and must be > 0")
			}

			path := filepath.Join(cwd, ".radiant-harness", "CONTEXT.md")
			result, err := radctx.CompressFile(path, budget)
			if err != nil {
				return err
			}

			if result.Original == result.Compressed {
				fmt.Printf("✓ Already within budget (%d tokens)\n", result.Original)
				return nil
			}

			fmt.Printf("✓ Compressed %d → %d tokens (%.0f%% reduction)\n",
				result.Original, result.Compressed,
				(1-result.Ratio)*100)
			if result.Truncated {
				fmt.Println("⚠ Content was hard-truncated to fit budget")
			}
			return nil
		},
	}
	contextCompressCmd.Flags().Int("budget", 0, "Target token budget (required)")

	contextSummarizeCmd := &cobra.Command{
		Use:   "summarize --phase=<phase>",
		Short: "Compress a completed phase's context to ≤20% of original tokens",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			phase, _ := cmd.Flags().GetString("phase")
			if phase == "" {
				return fmt.Errorf("--phase is required (e.g. --phase=execute)")
			}

			// Read CONTEXT.md as the content to summarize
			path := filepath.Join(cwd, ".radiant-harness", "CONTEXT.md")
			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read CONTEXT.md: %w (run `radiant context assemble` first)", err)
			}

			result := radctx.SummarizePhase(phase, string(data))

			// Write summarized content back
			if err := os.WriteFile(path, []byte(result.Content), 0o644); err != nil {
				return fmt.Errorf("write CONTEXT.md: %w", err)
			}

			fmt.Printf("✓ Phase %q summarized: %d → %d tokens (%.0f%% of original)\n",
				phase, result.Original, result.Summarized, result.Ratio*100)
			if len(result.KeyFacts) > 0 {
				fmt.Printf("  Preserved %d key facts\n", len(result.KeyFacts))
			}
			return nil
		},
	}
	contextSummarizeCmd.Flags().String("phase", "", "Phase whose context to summarize (discover|plan|execute|verify|persist)")

	contextCmd.AddCommand(contextDetectCmd, contextAssembleCmd, contextCompressCmd, contextSummarizeCmd)
	root.AddCommand(contextCmd)

	// ── ontology (Sprint 41) ─────────────────────────────────────────────────
	ontologyCmd := &cobra.Command{
		Use:   "ontology",
		Short: "Inspect the harness world model (entities, relations, axioms)",
	}

	ontologyExportCmd := &cobra.Command{
		Use:   "export",
		Short: "Export the world model for any LLM to reason over",
		RunE: func(cmd *cobra.Command, args []string) error {
			o := ontology.Default()
			compact, _ := cmd.Flags().GetBool("compact")
			if compact {
				fmt.Print(o.ExportCompact())
			} else {
				fmt.Print(o.Export())
			}
			return nil
		},
	}
	ontologyExportCmd.Flags().Bool("compact", false, "Token-budgeted form (~300 tokens) for LLM context")

	ontologyValidateCmd := &cobra.Command{
		Use:   "validate",
		Short: "Check the world model for axiom violations",
		RunE: func(cmd *cobra.Command, args []string) error {
			o := ontology.Default()
			violations := o.Violations()
			if len(violations) == 0 {
				fmt.Println("✓ ontology consistent — 0 axiom violations")
				return nil
			}
			fmt.Printf("✗ %d axiom violation(s):\n", len(violations))
			for _, v := range violations {
				fmt.Printf("  - %s\n", v)
			}
			return fmt.Errorf("ontology has %d violation(s)", len(violations))
		},
	}

	ontologySkillsCmd := &cobra.Command{
		Use:   "skills <domain>",
		Short: "List skills that govern a domain (semantic skill routing)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o := ontology.Default()
			skills := o.SkillsForDomain("domain:" + args[0])
			if len(skills) == 0 {
				fmt.Printf("no skills govern domain %q\n", args[0])
				return nil
			}
			fmt.Printf("skills governing %q:\n", args[0])
			for _, s := range skills {
				fmt.Printf("  - %s\n", s)
			}
			return nil
		},
	}

	ontologyCmd.AddCommand(ontologyExportCmd, ontologyValidateCmd, ontologySkillsCmd)
	root.AddCommand(ontologyCmd)
}
