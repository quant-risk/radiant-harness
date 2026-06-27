package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/quant-risk/radiant-harness/internal/skill"
	"github.com/spf13/cobra"
)

func registerOpsCmds(root *cobra.Command) {
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
}
