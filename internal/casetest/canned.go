package casetest

import (
	"fmt"
	"os"
	"strings"
)

// CannedResponse returns a phase-correct canned assistant message for
// the synthetic host. Each response is bounded (the sampling-backend
// passes the full text to the model, so keep it readable) and explicit
// enough that the harness's verifier phase can run deterministically.
//
// The detect-once `## radiant-phase: <name>` prompt marker (added in
// v3.3.0) makes this mapping 1:1: the host reads the user's text and
// returns the canned body for the marked phase.
func CannedResponse(phase Phase, c *Case) string {
	taskSummary := strings.SplitN(c.UserPrompt, "\n", 2)[0]
	if len(taskSummary) > 80 {
		taskSummary = taskSummary[:80] + "…"
	}

	switch phase {
	case PhaseDiscover:
		return strings.Join([]string{
			"## Project layout (synthetic host)",
			"",
			"- Project: case dir, files match the menu_flex risk case shape",
			"- Manifests present: " + manifestSummary(c.Path),
			"- Specs: 0 found",
			"- Bundled skills exposed: credit-risk, nova-feature, actuarial, stats, ml",
			"- Locale: pt-BR",
			"- Operator note: ramp of " + quote(taskSummary) + " fits the credit-risk skill; risky if scoped wider.",
			"",
			"## Skills to apply",
			"- credit-risk",
			"- nova-feature",
		}, "\n")

	case PhasePlan:
		return strings.Join([]string{
			"## Plan",
			"",
			"## Acceptance Criteria",
			"AC1: Read the case dir and surface key data fields.",
			"AC2: Split the dataset temporally (train/val/test).",
			"AC3: Train a baseline classifier (logistic regression).",
			"AC4: Output predictions + decision policy to case/output/.",
			"AC5: Persistence across restart — no in-memory state.",
			"",
			"## Tasks",
			"1. Read CONTEXT.md and enumerate files.",
			"2. Write data loader + temporal split.",
			"3. Train baseline + capture metrics.",
			"4. Write policies (approve / review / reject) to output/.",
			"5. Verify all gates pass with `go test ./...`.",
		}, "\n")

	case PhaseExecute:
		return strings.Join([]string{
			"## Execute",
			"",
			"Implementation summary:",
			"- Wrote `main.go` and `main_test.go` in the case dir.",
			"- Data loader reads `data/*.csv`.",
			"- Temporal split uses 70/15/15 by month.",
			"- Baseline is logistic regression with L2 penalty.",
			"- Decisions written to `output/decisions.csv`.",
			"- Gates run:",
			"  - `go build ./...` → PASS",
			"  - `go vet ./...` → PASS",
			"  - `go test ./...` → PASS",
			"  - `gofmt -l .` → empty",
			"- Iterations: 1.",
		}, "\n")

	case PhaseVerify:
		return strings.Join([]string{
			"VERDICT: APPROVED",
			"SCORE: 1.00",
			"EVIDENCE: AC1–AC5 all implemented; gates PASS; output files present.",
			"ESCALATE: false",
			"ISSUES:",
		}, "\n")
	}
	return fmt.Sprintf("(unknown phase %q)", phase)
}

// manifestSummary reads the case dir's top-level files and returns
// a short list of any package manifests (go.mod, package.json, …).
func manifestSummary(casePath string) string {
	candidates := []string{
		"go.mod", "package.json", "pyproject.toml", "Cargo.toml",
		"requirements.txt", "Pipfile", "pom.xml", "build.gradle",
	}
	var found []string
	for _, c := range candidates {
		if fileExists(casePath + "/" + c) {
			found = append(found, c)
		}
	}
	if fileExists(casePath + "/.git") {
		found = append(found, ".git/")
	}
	if len(found) == 0 {
		return "none detected"
	}
	return strings.Join(found, ", ")
}

func fileExists(p string) bool {
	if len(p) == 0 {
		return false
	}
	_, err := os.Stat(p)
	return err == nil
}

func quote(s string) string {
	if len(s) > 60 {
		s = s[:60] + "…"
	}
	s = strings.ReplaceAll(s, "`", "'")
	return "`" + s + "`"
}
