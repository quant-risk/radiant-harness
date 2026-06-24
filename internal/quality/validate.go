package quality

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	radiant "github.com/quant-risk/radiant-harness/internal"
)

// gateTimeout caps any single gate (test runner, type-checker) execution so a
// hung gate can't stall the whole validate run. 5 minutes is generous for any
// realistic test suite; anything longer is almost always a deadlock.
const gateTimeout = 5 * time.Minute

// allowedGateBinaries mirrors the allowlist enforced by the harness agent
// runner. Validation is stricter than the agent (we're static-checking
// tasks.md) so we use the same closed set — adding a binary here means
// opening it up at both the orchestration and validation layers.
var allowedGateBinaries = map[string]struct{}{
	"node": {}, "npm": {}, "pnpm": {}, "yarn": {}, "bun": {}, "deno": {},
	"go": {}, "make": {},
	"pytest": {}, "python": {}, "python3": {}, "pip": {},
	"cargo": {}, "rustc": {},
	"jest": {}, "vitest": {},
	"tsc": {}, "eslint": {},
	"shellcheck": {},
}

// ValidateFeature runs full UAT validation on a feature directory. Pure: does
// NOT execute gates. Call RunGates separately to also exercise the task gates.
func ValidateFeature(specDir string) radiant.ValidationResult {
	specFile := filepath.Join(specDir, "spec.md")
	taskFile := filepath.Join(specDir, "tasks.md")

	specText, err := os.ReadFile(specFile)
	if err != nil {
		return radiant.ValidationResult{Passed: false, Errors: []string{"cannot read spec.md: " + err.Error()}}
	}

	taskText, err := os.ReadFile(taskFile)
	if err != nil {
		return radiant.ValidationResult{Passed: false, Errors: []string{"cannot read tasks.md: " + err.Error()}}
	}

	specContent := string(specText)
	taskContent := string(taskText)

	specACs := extractACDetails(specContent)

	var acResults []radiant.ACResult
	var errors []string

	for _, ac := range specACs {
		result := radiant.ACResult{AC: ac.ID}

		if strings.Contains(taskContent, ac.ID) {
			result.TestFile = "tasks.md"
		} else {
			result.Passed = false
			errors = append(errors, ac.ID+" has no task coverage")
			acResults = append(acResults, result)
			continue
		}

		if ac.Given == "" || ac.When == "" || ac.Then == "" {
			errors = append(errors, ac.ID+" has incomplete Given/When/Then")
			result.Passed = false
		} else {
			result.Passed = true
		}

		acResults = append(acResults, result)
	}

	specDevRe := regexp.MustCompile("SPEC_DEVIATION")
	if specDevRe.MatchString(taskContent) {
		errors = append(errors, "open SPEC_DEVIATION in tasks.md")
	}

	score := 1.0
	if len(acResults) > 0 {
		passed := 0
		for _, r := range acResults {
			if r.Passed {
				passed++
			}
		}
		score = float64(passed) / float64(len(acResults))
	}

	return radiant.ValidationResult{
		Passed:    len(errors) == 0,
		Score:     score,
		ACResults: acResults,
		Errors:    errors,
	}
}

// GateResult captures the outcome of a single task gate execution.
type GateResult struct {
	Command string
	Passed  bool
	Output  string
	Skipped bool   // true when the gate was rejected by the allowlist
	Reason  string // populated when Skipped is true
}

// RunGates parses tasks.md, deduplicates gates, validates each against the
// allowlist, and executes the allowed ones sequentially with a timeout.
// Gates that reference a binary outside the allowlist are reported but not
// executed, so a developer notices the misconfiguration without the run
// stalling on a forbidden command.
func RunGates(projectDir, specDir string) []GateResult {
	taskFile := filepath.Join(specDir, "tasks.md")
	taskText, err := os.ReadFile(taskFile)
	if err != nil {
		return []GateResult{{Command: taskFile, Passed: false, Reason: "cannot read tasks.md: " + err.Error()}}
	}

	gates := extractGates(string(taskText))
	if len(gates) == 0 {
		return nil
	}

	results := make([]GateResult, 0, len(gates))
	for _, gate := range gates {
		if err := validateGateCommand(gate); err != nil {
			results = append(results, GateResult{
				Command: gate, Passed: false, Skipped: true,
				Reason: err.Error(),
			})
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), gateTimeout)
		out, runErr := runShellGate(ctx, projectDir, gate)
		cancel()

		if runErr != nil {
			results = append(results, GateResult{
				Command: gate, Passed: false,
				Output: out,
				Reason: runErr.Error(),
			})
			continue
		}
		results = append(results, GateResult{Command: gate, Passed: true, Output: out})
	}
	return results
}

// runShellGate executes the gate as `sh -c <gate>` with the projectDir as
// cwd. Returns the combined stdout/stderr and any error from execution or
// context cancellation.
func runShellGate(ctx context.Context, projectDir, gate string) (string, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", gate)
	cmd.Dir = projectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return string(out), fmt.Errorf("gate timeout after %s", gateTimeout)
		}
		return string(out), fmt.Errorf("gate failed: %w", err)
	}
	return string(out), nil
}

// gateRowRe matches a single markdown table row from tasks.md with a
// Gate (command) column. We accept optional surrounding whitespace and
// backtick fences around the command.
var gateRowRe = regexp.MustCompile("`([^`]+)`")

// extractGates pulls unique gate commands out of tasks.md. The convention is
// `| <task> | <gate> | ...` per row; we capture anything inside backticks
// that looks like a shell command (has at least one word boundary).
func extractGates(content string) []string {
	seen := map[string]struct{}{}
	var out []string
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		// Skip the header/separator rows. Table separator looks like |---|---|
		if strings.HasPrefix(strings.TrimSpace(line), "|---") {
			continue
		}
		// Only consider lines that look like table rows with backticked commands.
		if !strings.Contains(line, "`") {
			continue
		}
		matches := gateRowRe.FindAllStringSubmatch(line, -1)
		for _, m := range matches {
			cmd := strings.TrimSpace(m[1])
			if cmd == "" || !strings.ContainsAny(cmd, " \t") {
				continue
			}
			if _, dup := seen[cmd]; dup {
				continue
			}
			seen[cmd] = struct{}{}
			out = append(out, cmd)
		}
	}
	return out
}

// validateGateCommand mirrors harness.validateGateCommand; duplicated here
// so quality.RunGates can be called independently of the orchestrator (e.g.
// by `radiant validate --gates` in CI).
func validateGateCommand(gate string) error {
	gate = strings.TrimSpace(gate)
	if gate == "" {
		return nil
	}
	parts := splitShellTokens(gate)
	for _, part := range parts {
		if isShellOp(part) || strings.HasPrefix(part, "-") || strings.Contains(part, "=") {
			continue
		}
		base := part
		if idx := strings.LastIndexAny(base, "/\\"); idx >= 0 {
			base = base[idx+1:]
		}
		if _, ok := allowedGateBinaries[base]; !ok {
			return fmt.Errorf("binary %q is not in the gate allowlist", base)
		}
	}
	return nil
}

func splitShellTokens(cmd string) []string {
	repl := strings.NewReplacer(
		"&&", " ", "||", " ", "|", " ",
		";", " ", ">", " ", "<", " ",
		"(", " ", ")", " ",
	)
	return strings.Fields(repl.Replace(cmd))
}

func isShellOp(s string) bool {
	switch s {
	case "&&", "||", "|", ";", "&", ">", ">>", "<", "<<", "(", ")":
		return true
	}
	return false
}

// ACDetail holds parsed AC information.
type ACDetail struct {
	ID    string
	Title string
	Given string
	When  string
	Then  string
}

// extractACDetails extracts full AC details from spec content.
func extractACDetails(content string) []ACDetail {
	var acs []ACDetail
	scanner := bufio.NewScanner(strings.NewReader(content))

	acRe := regexp.MustCompile("^###\\s+(AC-\\d+):\\s*(.+)$")
	givenRe := regexp.MustCompile("(?i)^[-\\s]*\\*\\*Given\\*\\*\\s+(.+)$")
	whenRe := regexp.MustCompile("(?i)^[-\\s]*\\*\\*When\\*\\*\\s+(.+)$")
	thenRe := regexp.MustCompile("(?i)^[-\\s]*\\*\\*Then\\*\\*\\s+(.+)$")

	var current *ACDetail

	for scanner.Scan() {
		line := scanner.Text()

		if m := acRe.FindStringSubmatch(line); m != nil {
			if current != nil {
				acs = append(acs, *current)
			}
			current = &ACDetail{ID: m[1], Title: m[2]}
			continue
		}

		if current != nil {
			if m := givenRe.FindStringSubmatch(line); m != nil {
				current.Given = strings.TrimSpace(m[1])
			}
			if m := whenRe.FindStringSubmatch(line); m != nil {
				current.When = strings.TrimSpace(m[1])
			}
			if m := thenRe.FindStringSubmatch(line); m != nil {
				current.Then = strings.TrimSpace(m[1])
			}
		}
	}

	if current != nil {
		acs = append(acs, *current)
	}

	return acs
}
