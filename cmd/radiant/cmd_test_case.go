package main

// `radiant test-case <path.zip>` — drive the full possession flow
// against a real subprocess (`radiant mcp serve`) using a synthetic
// host that simulates realistic per-call sampling latency. Records the
// outcome + per-phase timing in a Markdown report so a human (or CI)
// can audit whether the harness end-to-end works under the exact
// conditions that broke real agents in production.

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/quant-risk/radiant-harness/v3/internal/casetest"
	"github.com/spf13/cobra"
)

// registerTestCaseCmd adds `radiant test-case` to the root command. It
// is NOT gated by RADIANT_INTERNAL: it's a public diagnostic, on par
// with `radiant mcp self-test`.
func registerTestCaseCmd(root *cobra.Command) {
	var (
		flagColdStart    int
		flagJitter       int
		flagSamplingTO   time.Duration
		flagProfile      string
		flagReportPath   string
		flagUnpackedKeep bool
		flagTimeout      time.Duration
	)

	cmd := &cobra.Command{
		Use:   "test-case <path.zip|dir>",
		Short: "Drive a full possession flow against a real case with simulated latency",
		Long: `Boots a child 'radiant mcp serve' subprocess, sends the full
sampling/createMessage possession flow end-to-end, with a configurable
per-call cold start to simulate real LLM-backed host latency (Hermes,
Codex, Claude Code). The synthetic host is deterministic but exercises
the full harness state machine: bootstrap, discover, plan, execute,
verify, and final.

Use cases:
  - Sanity-check the harness before testing in a real agent.
  - Reproduce the failure modes observed in 2026-06-29 (Hermes
    mimo / Codex GPT-5 cold-start) offline.
  - CI integration: every PR proves the harness does not regress
    on the "real agents are slow" path.

Output: exits 0 if the harness exits 'success' within the configured
sampling timeout. Non-zero on critical_failure, transport error, or
timeout. A Markdown report is written to --report (default
.radiant-harness/reports/<case>-<timestamp>.md).`,
		Args: cobra.ExactArgs(1),
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			src := args[0]
			bin, _ := os.Executable()
			reportPath := strings.TrimSpace(flagReportPath)

			// Materialize the case dir (extract .zip, or symlink dir).
			caseDir, keep, err := materializeCase(src, flagUnpackedKeep)
			if err != nil {
				return err
			}
			if !keep {
				defer func() { _ = os.RemoveAll(caseDir) }()
			}

			c, err := casetest.LoadFromDir(caseDir)
			if err != nil {
				return fmt.Errorf("load case: %w", err)
			}
			if c.UserPrompt == "" {
				return fmt.Errorf("case %q has an empty user prompt", src)
			}

			cfg := casetest.Config{
				Binary:      bin,
				Workdir:     caseDir,
				ColdStartMs: flagColdStart,
				JitterMs:    flagJitter,
				SamplingTO:  flagSamplingTO,
				Profile:     flagProfile,
			}
			ctx, cancel := context.WithTimeout(context.Background(), flagTimeout)
			defer cancel()

			rep, err := casetest.Run(ctx, c, cfg)
			if err != nil {
				// Even on harness failure, dump whatever we observed
				// so the user can see WHICH phase failed.
				if reportPath == "" {
					reportPath = defaultReportPath(c.Name)
				}
				writeBestEffortReport(rep, reportPath)
				return fmt.Errorf("test-case: %w", err)
			}

			if reportPath == "" {
				reportPath = defaultReportPath(c.Name)
			}
			f, err := os.Create(reportPath)
			if err != nil {
				return fmt.Errorf("write report: %w", err)
			}
			defer f.Close()
			if err := rep.WriteMarkdown(f); err != nil {
				return fmt.Errorf("render report: %w", err)
			}

			fmt.Fprintf(os.Stdout,
				"result: %s\nsampling calls: %d\nelapsed: %s\nreport: %s\n",
				rep.Outcome,
				rep.SamplingCallCount(),
				rep.FinishedAt.Sub(rep.StartedAt).Round(time.Millisecond),
				reportPath,
			)
			return nil
		},
	}

	cmd.Flags().IntVar(&flagColdStart, "cold-start-ms", 25000,
		"simulated per-call sampling latency in milliseconds (Hermes mimo cold-start is ~20-40s; default replicates that).")
	cmd.Flags().IntVar(&flagJitter, "jitter-ms", 5000,
		"± random spread on each call (subtracts/added to --cold-start-ms).")
	cmd.Flags().DurationVar(&flagSamplingTO, "sampling-timeout", 120*time.Second,
		"per-call sampling timeout the harness enforces. Must exceed --cold-start-ms.")
	cmd.Flags().StringVar(&flagProfile, "profile", "standard",
		"harness profile: lean | standard | thorough")
	cmd.Flags().StringVar(&flagReportPath, "report", "",
		"path to write the Markdown report (default .radiant-harness/reports/<case>-<timestamp>.md)")
	cmd.Flags().BoolVar(&flagUnpackedKeep, "keep-unpacked", false,
		"don't auto-clean the extracted case dir (useful for inspecting post-run).")
	cmd.Flags().DurationVar(&flagTimeout, "timeout", 8*time.Minute,
		"overall test-case timeout.")

	root.AddCommand(cmd)
}

// materializeCase returns a path to a directory containing the case
// contents. For a directory, it returns the path directly. For a .zip,
// it extracts into a temp dir and returns that. When keep is false the
// caller is expected to remove the dir when done.
func materializeCase(src string, keep bool) (string, bool, error) {
	info, err := os.Stat(src)
	if err != nil {
		return "", false, fmt.Errorf("stat %s: %w", src, err)
	}
	if info.IsDir() {
		return src, true, nil
	}
	if !strings.HasSuffix(strings.ToLower(src), ".zip") {
		return "", false, fmt.Errorf("not a .zip or directory: %s", src)
	}
	out, err := casetest.LoadFromZip(src)
	if err != nil {
		return "", false, fmt.Errorf("extract zip: %w", err)
	}
	return out, keep, nil
}

// defaultReportPath returns a sensible default location for the
// Markdown report when the user didn't specify --report.
func defaultReportPath(caseName string) string {
	dir := ".radiant-harness/reports"
	_ = os.MkdirAll(dir, 0o755)
	return filepath.Join(dir,
		fmt.Sprintf("%s-%s.md", caseName, time.Now().UTC().Format("2006-01-02T15-04Z")))
}

// writeBestEffortReport writes the partial report even on harness
// failure; the report is the user's primary debug artifact.
func writeBestEffortReport(rep *casetest.Report, path string) {
	if rep == nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "warning: report write failed:", err)
		return
	}
	defer f.Close()
	defer f.Close()
	_ = rep.WriteMarkdown(f)
}

// keep io imported so future flag-handler edits can reach for io.Args
// without an import-cycle.
var _ io.Reader = (*os.File)(nil)
