//go:build with_full

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/quant-risk/radiant-harness/internal/boot"
	"github.com/quant-risk/radiant-harness/internal/ontology"
	"github.com/quant-risk/radiant-harness/internal/skill"
	"github.com/spf13/cobra"
)

func registerSkillsCmds(root *cobra.Command) {
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

	// ── boot (Sprint 34) ──────────────────────────────────────────────────────
	bootCmd := &cobra.Command{
		Use:   "boot",
		Short: "Print a minimal project manifest for any LLM or IDE",
		Long: `Bootstrap Protocol — universal entry point for any agent.

Outputs a <500-token manifest describing the project domain, recommended skills,
and the loop command to start autonomous work. Run this first in any session.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			jsonMode, _ := cmd.Flags().GetBool("json")
			agentFlavor, _ := cmd.Flags().GetString("agent")
			profile, _ := cmd.Flags().GetString("profile")

			m, err := boot.Generate(cwd, boot.Options{
				Flavor:        boot.AgentFlavor(agentFlavor),
				JSON:          jsonMode,
				BudgetProfile: profile,
			})
			if err != nil {
				return err
			}

			if jsonMode {
				out, err := boot.RenderJSON(m)
				if err != nil {
					return err
				}
				fmt.Println(out)
				return nil
			}

			fmt.Print(boot.RenderMarkdown(m, boot.AgentFlavor(agentFlavor)))

			if withWM, _ := cmd.Flags().GetBool("world-model"); withWM {
				fmt.Print("\n```\n")
				fmt.Print(ontology.Default().ExportCompact())
				fmt.Print("```\n")
			}
			return nil
		},
	}
	bootCmd.Flags().Bool("json", false, "Output machine-readable JSON manifest")
	bootCmd.Flags().String("agent", "generic", "Tailor output for agent: claude|cursor|copilot|gemini|windsurf|codex")
	bootCmd.Flags().String("profile", "standard", "Budget profile: lean|standard|thorough")
	bootCmd.Flags().Bool("world-model", false, "Append the compact ontology world model (~300 tokens)")
	root.AddCommand(bootCmd)

}
