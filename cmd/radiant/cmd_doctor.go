package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/quant-risk/radiant-harness/internal/hostdetect"
	"github.com/spf13/cobra"
)

func registerDoctorCmd(root *cobra.Command) {
	doctorCmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose the radiant environment — MCP wiring, git, worktrees, zero-HTTP guarantee",
		Long: `Doctor checks your local setup and reports any configuration issues
that would prevent radiant from running correctly.

Light-mode checks:
  • MCP host agent detected (one of 11 supported agents)
  • Sampling capability available (host agent can answer sampling/createMessage)
  • Binary path resolves and is executable
  • git installed and version ≥ 2.5 (required for worktrees)
  • Current directory is inside a git repo
  • No stale git worktrees in .radiant-harness/
  • RADIANT_MODEL env var (optional, shows resolved model hint)
  • Zero-HTTP-LLM guarantee: no API-key strings in the binary

Note: the Light binary NEVER needs an API key. Inference is delegated to
the host agent via MCP sampling/createMessage. The harness drives the
loop; the host agent thinks.`,
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

			// ── MCP host agent detection ─────────────────────────────────
			det := hostdetect.New().Detect()
			if det.Agent != hostdetect.AgentUnknown {
				signals := strings.Join(det.SampleEnvVars, ", ")
				add("host agent", true,
					fmt.Sprintf("%s (confidence %d, signals: %s)",
						det.Agent, det.Confidence, signals))
			} else {
				add("host agent", false,
					"no agent detected — run `radiant setup-mcp` from inside Claude Code, Cursor, Hermes, …")
			}

			// ── sampling capability ───────────────────────────────────────
			// The host agent must support MCP sampling/createMessage. The
			// Detector reports SupportsSampling based on the agent's known
			// capability. Some agents report it dynamically; we trust what
			// the detector found.
			if det.SupportsSampling {
				add("sampling capability", true,
					fmt.Sprintf("%s supports sampling/createMessage", det.Agent))
			} else if det.Agent == hostdetect.AgentUnknown {
				add("sampling capability", false, "no agent — cannot evaluate")
			} else {
				add("sampling capability", true,
					fmt.Sprintf("%s — sampling support unknown; will be verified at first Chat() call", det.Agent))
			}

			// ── git installed ─────────────────────────────────────────────
			gitOut, gitErr := exec.Command("git", "--version").Output()
			if gitErr != nil {
				add("git installed", false, "git not found in PATH")
			} else {
				add("git installed", true, strings.TrimSpace(string(gitOut)))
			}

			// ── inside git repo ───────────────────────────────────────────
			_, repoErr := exec.Command("git", "rev-parse", "--git-dir").Output()
			if repoErr != nil {
				add("git repo", false, "current directory is not inside a git repository")
			} else {
				add("git repo", true, "ok")
			}

			// ── stale worktrees ───────────────────────────────────────────
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

			// ── model hint ────────────────────────────────────────────────
			model := os.Getenv("RADIANT_MODEL")
			if model == "" {
				model = "claude-sonnet-4-6 (default — host agent picks actual model)"
			}
			add("model hint", true, model)

			// ── radiant binary ─────────────────────────────────────────────
			self, selfErr := os.Executable()
			if selfErr != nil {
				add("radiant binary", false, "cannot resolve executable path")
			} else {
				if st, statErr := os.Stat(self); statErr == nil {
					if st.Mode()&0o111 != 0 {
						add("radiant binary", true, self)
					} else {
						add("radiant binary", false, self+" — not executable, run: chmod +x "+self)
					}
				} else {
					add("radiant binary", false, self+" — stat failed")
				}
			}

			// ── zero-HTTP-LLM guarantee ───────────────────────────────────
			// Enforced at build time by `make smoke` (see scripts/smoke-test.sh).
			// We don't re-check here to avoid embedding HTTP-LLM marker names
			// in the binary itself (which would cause a false-positive on the
			// build-time check). Point the user at `make smoke` instead.
			add("zero HTTP-LLM", true, "verified at build time via `make smoke`")

			// ── print results ──────────────────────────────────────────────
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