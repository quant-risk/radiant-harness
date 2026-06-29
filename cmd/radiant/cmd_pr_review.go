//go:build !light_only

package main

// PR review worker functions. Extracted from helpers.go in
// Sprint 77 (debt-reduction pattern, matches Sprint 74/76).
//
// `runReviewPR` is the body of `radiant review-pr <spec-path>`,
// which is registered as a subcommand in cmd_spec.go. The
// subcommand wiring stays put — only the worker code moves.
//
// The block here contains the helper functions and types that
// implement the PR review scaffold:
//
//   - runReviewPR            the orchestrator (entry from cmd_spec.go)
//   - parseAcceptanceCriteria   extracts ACs from spec.md
//   - parseGatesFromTasks       extracts gate commands from tasks.md
//   - countDiffFiles            counts files in a unified diff
//   - renderPRReview            renders the pr-review.md document
//
// plus two small types (acceptanceCriterion, gateResult) used
// across those functions.
//
// The semantic AC↔code matching is left to the LLM (via the
// `revisar-pr` skill); this command produces the reproducible
// scaffold that the LLM fills in.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// gateResult is one row in the pr-review.md "Gate results" table.
type gateResult struct {
	Name   string
	Passed bool
	Err    string
}

// acceptanceCriterion is a single AC pulled from spec.md.
type acceptanceCriterion struct {
	ID    string // "AC1", "AC2", ...
	Title string // first sentence after the ID
	Body  string // remaining body
}

// runReviewPR is the body of `radiant review-pr`. Parses the
// spec for ACs, tasks for gates, optionally executes the gates,
// and writes a structured review report. The semantic
// AC↔code matching is left to the LLM (via the revisar-pr skill);
// this command produces the reproducible scaffold.
func runReviewPR(specPath, diffPath string, runGates bool, outPath string) error {
	specMD := filepath.Join(specPath, "spec.md")
	tasksMD := filepath.Join(specPath, "tasks.md")

	specBody, err := os.ReadFile(specMD)
	if err != nil {
		return fmt.Errorf("read %s: %w", specMD, err)
	}
	tasksBody, err := os.ReadFile(tasksMD)
	if err != nil {
		return fmt.Errorf("read %s: %w", tasksMD, err)
	}

	acs := parseAcceptanceCriteria(string(specBody))
	gates := parseGatesFromTasks(string(tasksBody))

	var diffStats struct {
		Lines int
		Files int
	}
	if diffPath != "" {
		data, err := os.ReadFile(diffPath)
		if err != nil {
			return fmt.Errorf("read diff %s: %w", diffPath, err)
		}
		diffStats.Lines = strings.Count(string(data), "\n")
		diffStats.Files = countDiffFiles(string(data))
	}

	var results []gateResult
	if runGates {
		for _, g := range gates {
			res := gateResult{Name: g}
			cmd := exec.Command("sh", "-c", g)
			out, err := cmd.CombinedOutput()
			if err != nil {
				res.Passed = false
				res.Err = strings.TrimSpace(string(out))
				if res.Err == "" {
					res.Err = err.Error()
				}
			} else {
				res.Passed = true
			}
			results = append(results, res)
		}
	}

	slug := filepath.Base(specPath)
	report := renderPRReview(slug, acs, gates, results, diffPath, diffStats)
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	if err := atomicWrite(outPath, report); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}
	fmt.Printf("  ✓ wrote %s\n", outPath)
	fmt.Printf("  ACs found: %d\n", len(acs))
	fmt.Printf("  Gates found: %d\n", len(gates))
	if runGates {
		passed := 0
		for _, r := range results {
			if r.Passed {
				passed++
			}
		}
		fmt.Printf("  Gates executed: %d/%d passed\n", passed, len(results))
	}
	if diffPath != "" {
		fmt.Printf("  Diff: %d files, %d lines\n", diffStats.Files, diffStats.Lines)
	}
	fmt.Printf("\n  Next: open %s and fill in AC↔code semantic check (use the revisar-pr skill).\n", outPath)
	return nil
}

// parseAcceptanceCriteria extracts ACs from spec.md. Looks for
// lines starting with "### AC" (case-insensitive). Tolerates
// variations: "### AC1: title", "### AC2 — title", etc.
func parseAcceptanceCriteria(specMD string) []acceptanceCriterion {
	var out []acceptanceCriterion
	for _, line := range strings.Split(specMD, "\n") {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "### ") {
			continue
		}
		header := strings.TrimPrefix(t, "### ")
		if !strings.HasPrefix(strings.ToUpper(header), "AC") {
			continue
		}
		// Split ID from title: "AC1: foo" or "AC1 — foo" or "AC1 foo"
		parts := strings.FieldsFunc(header, func(r rune) bool {
			return r == ':' || r == '—' || r == '-' || r == ' '
		})
		if len(parts) < 2 {
			continue
		}
		id := strings.ToUpper(parts[0])
		title := strings.TrimSpace(header[len(parts[0]):])
		title = strings.TrimLeft(title, ":—- ")
		out = append(out, acceptanceCriterion{ID: id, Title: title, Body: ""})
	}
	return out
}

// parseGatesFromTasks extracts gate commands from tasks.md.
// tasks.md uses a markdown table with a "Gate" column; gate
// values are shell commands wrapped in backticks. This function
// pulls every code span from the Gate column.
func parseGatesFromTasks(tasksMD string) []string {
	var gates []string
	inGateCol := false
	for _, line := range strings.Split(tasksMD, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "|") {
			cols := strings.Split(trimmed, "|")
			for i, c := range cols {
				c = strings.TrimSpace(c)
				// Header row: detect "Gate" column index.
				if strings.EqualFold(c, "Gate") {
					inGateCol = true
					_ = i
					continue
				}
				if inGateCol && strings.HasPrefix(c, "`") && strings.HasSuffix(c, "`") && len(c) >= 2 {
					cmd := strings.Trim(c, "`")
					if cmd != "" && cmd != "—" {
						gates = append(gates, cmd)
					}
				}
			}
		}
	}
	return gates
}

// countDiffFiles counts the number of "diff --git" headers in a
// unified diff. Each one represents one file changed.
func countDiffFiles(diff string) int {
	return strings.Count(diff, "diff --git ")
}

// renderPRReview produces the pr-review.md report. The semantic
// AC↔code check is left as TODO placeholders for the LLM (via
// the revisar-pr skill) to fill in.
func renderPRReview(slug string, acs []acceptanceCriterion, gates []string, results []gateResult, diffPath string, diffStats struct {
	Lines int
	Files int
}) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# PR review: %s\n\n", slug)
	b.WriteString("> Generated by `radiant review-pr`. The semantic\n")
	b.WriteString("> AC↔code check is left as TODOs for the LLM (via\n")
	b.WriteString("> the `revisar-pr` skill) to fill in.\n\n")

	// Summary
	fmt.Fprintf(&b, "## Summary\n\n")
	b.WriteString("| Metric | Value |\n")
	b.WriteString("|--------|-------|\n")
	fmt.Fprintf(&b, "| ACs in spec | %d |\n", len(acs))
	fmt.Fprintf(&b, "| Gates in tasks | %d |\n", len(gates))
	if len(results) > 0 {
		passed := 0
		for _, r := range results {
			if r.Passed {
				passed++
			}
		}
		fmt.Fprintf(&b, "| Gates executed | %d/%d passed |\n", passed, len(results))
	}
	if diffPath != "" {
		fmt.Fprintf(&b, "| Diff | %d files, %d lines (%s) |\n", diffStats.Files, diffStats.Lines, diffPath)
	}

	// Recommendation (skeleton — LLM fills in)
	b.WriteString("\n## Recommendation\n\n")
	b.WriteString("- [ ] Approve\n")
	b.WriteString("- [ ] Request changes\n")
	b.WriteString("- [ ] Needs spec revision (SPEC_DEVIATION)\n\n")

	// AC-by-AC check
	if len(acs) > 0 {
		b.WriteString("## AC coverage\n\n")
		b.WriteString("| AC | Title | Implemented | Notes |\n")
		b.WriteString("|----|-------|-------------|-------|\n")
		for _, ac := range acs {
			fmt.Fprintf(&b, "| %s | %s | TODO | TODO |\n", ac.ID, ac.Title)
		}
		b.WriteString("\n> Each TODO above is filled in by the LLM via the\n")
		b.WriteString("> `revisar-pr` skill: search the diff for code that\n")
		b.WriteString("> implements the AC's Given/When/Then conditions.\n\n")
	}

	// Gates
	if len(gates) > 0 {
		b.WriteString("## Gate results\n\n")
		b.WriteString("| Gate | Status | Output |\n")
		b.WriteString("|------|--------|--------|\n")
		for _, g := range gates {
			// Find matching result if any
			var status, outStr string
			for _, r := range results {
				if r.Name == g {
					if r.Passed {
						status = "✓ pass"
						outStr = "(silent)"
					} else {
						status = "✗ fail"
						outStr = r.Err
						if len(outStr) > 80 {
							outStr = outStr[:77] + "..."
						}
					}
					break
				}
			}
			if status == "" {
				status = "— not run"
				outStr = "pass --run-gates to execute"
			}
			fmt.Fprintf(&b, "| `%s` | %s | %s |\n", g, status, outStr)
		}
		b.WriteString("\n")
	}

	// SPEC_DEVIATION section (empty template for LLM to fill in)
	b.WriteString("## SPEC_DEVIATION\n\n")
	b.WriteString("Document any code that diverges from the spec:\n\n")
	b.WriteString("```markdown\n")
	b.WriteString("### SPEC_DEVIATION-001: <short title>\n\n")
	b.WriteString("- **AC**: <which AC is affected>\n")
	b.WriteString("- **Files**: <files involved>\n")
	b.WriteString("- **What's missing**: <specific gap>\n")
	b.WriteString("- **Recommended action**: <extend test | revise AC | revert>\n")
	b.WriteString("```\n\n")

	// Suggested PR comment
	b.WriteString("## Suggested PR comment\n\n")
	b.WriteString("> Copy-paste into the PR conversation:\n\n")
	b.WriteString("```\n")
	if len(results) > 0 {
		passed := 0
		for _, r := range results {
			if r.Passed {
				passed++
			}
		}
		fmt.Fprintf(&b, "PR review: %d/%d gates pass. AC coverage and SPEC_DEVIATION check above.\n", passed, len(results))
	} else {
		b.WriteString("PR review: see pr-review.md for AC coverage, gate status, and SPEC_DEVIATION check.\n")
	}
	b.WriteString("```\n\n")

	// Footer
	b.WriteString("---\n\n")
	b.WriteString("_Generated by `radiant review-pr`. Use the `revisar-pr` skill for the semantic AC↔code check._\n")
	return b.String()
}
