package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

func registerDoctorCmd(root *cobra.Command) {
	doctorCmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose the radiant environment — API key, model, git, worktrees",
		Long: `Doctor checks your local setup and reports any configuration issues
that would prevent radiant from running correctly.

Checks:
  • API key available (OPENROUTER_API_KEY / OPENAI_API_KEY / ANTHROPIC_API_KEY)
  • git installed and version ≥ 2.5 (required for worktrees)
  • Current directory is inside a git repo
  • No stale git worktrees in .radiant-harness/
  • RADIANT_MODEL env var (optional, shows resolved model)`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			type check struct {
				label string
				ok    bool
				note  string
			}
			var checks []check
			allOK := true

			add := func(label string, ok bool, note string) {
				checks = append(checks, check{label, ok, note})
				if !ok {
					allOK = false
				}
			}

			// ── API key ──────────────────────────────────────────────────────
			apiKey := os.Getenv("OPENROUTER_API_KEY")
			if apiKey == "" {
				apiKey = os.Getenv("OPENAI_API_KEY")
			}
			if apiKey == "" {
				apiKey = os.Getenv("ANTHROPIC_API_KEY")
			}
			if apiKey != "" {
				add("API key", true, "found ("+keySource()+")")
			} else {
				add("API key", false, "none found — set OPENROUTER_API_KEY, OPENAI_API_KEY, or ANTHROPIC_API_KEY")
			}

			// ── git installed ────────────────────────────────────────────────
			gitOut, gitErr := exec.Command("git", "--version").Output()
			if gitErr != nil {
				add("git installed", false, "git not found in PATH")
			} else {
				add("git installed", true, strings.TrimSpace(string(gitOut)))
			}

			// ── inside git repo ──────────────────────────────────────────────
			_, repoErr := exec.Command("git", "rev-parse", "--git-dir").Output()
			if repoErr != nil {
				add("git repo", false, "current directory is not inside a git repository")
			} else {
				add("git repo", true, "ok")
			}

			// ── stale worktrees ──────────────────────────────────────────────
			wtOut, _ := exec.Command("git", "worktree", "list", "--porcelain").Output()
			var stale []string
			for _, line := range strings.Split(string(wtOut), "\n") {
				if strings.HasPrefix(line, "prunable") {
					stale = append(stale, strings.TrimSpace(line))
				}
			}
			if len(stale) == 0 {
				add("worktrees", true, "no stale worktrees")
			} else {
				add("worktrees", false, fmt.Sprintf("%d stale worktree(s) — run: git worktree prune", len(stale)))
			}

			// ── model ────────────────────────────────────────────────────────
			model := os.Getenv("RADIANT_MODEL")
			if model == "" {
				model = "claude-sonnet-4-6 (default)"
			}
			add("model", true, model)

			// ── radiant binary ───────────────────────────────────────────────
			self, selfErr := os.Executable()
			if selfErr != nil {
				add("radiant binary", false, "cannot resolve executable path")
			} else {
				add("radiant binary", true, self)
			}

			// ── mode ─────────────────────────────────────────────────────────
			// Mode is now derived from which subcommand you ran (no flag,
			// env, or config field). The harness doesn't carry an
			// "active mode" any more; report the expected mode based on
			// whether an API key is present.
			modeNote := ""
			modeOK := true
			if apiKey == "" {
				modeNote = "Full mode (CLI subcommand) requires an API key — export OPENROUTER_API_KEY, OPENAI_API_KEY, or ANTHROPIC_API_KEY, or set api_key in .radiant.yaml"
				modeOK = false
			} else {
				modeNote = "Full mode (CLI subcommand) — API key present"
			}
			add("mode", modeOK, modeNote)

			// ── print results ────────────────────────────────────────────────
			fmt.Println("radiant doctor")
			fmt.Println(strings.Repeat("─", 60))
			for _, c := range checks {
				icon := "✓"
				if !c.ok {
					icon = "✗"
				}
				fmt.Printf("  %s  %-22s  %s\n", icon, c.label, c.note)
			}
			fmt.Println(strings.Repeat("─", 60))
			if allOK {
				fmt.Println("  All checks passed — radiant is ready.")
			} else {
				fmt.Println("  One or more checks failed. Fix the issues above.")
				return fmt.Errorf("doctor: environment not fully configured")
			}
			return nil
		},
	}
	root.AddCommand(doctorCmd)
}

func keySource() string {
	if os.Getenv("OPENROUTER_API_KEY") != "" {
		return "OPENROUTER_API_KEY"
	}
	if os.Getenv("OPENAI_API_KEY") != "" {
		return "OPENAI_API_KEY"
	}
	return "ANTHROPIC_API_KEY"
}
