package quality

import (
	"bufio"
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	radiant "github.com/quant-risk/radiant-harness/internal"
	"github.com/quant-risk/radiant-harness/internal/policy"
)

// gateTimeout is defined in gate_unix.go / gate_windows.go.

// allowedGateBinaries is re-exported from internal/policy so existing
// package-local references in validate.go and tests keep working
// without a sweeping rename. The canonical definition lives in
// internal/policy.
var allowedGateBinaries = policy.GateBinaries

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
		out, runErr := runShellGate(ctx, projectDir, gate, 0) // 0 = package default (10 MiB)
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

// runShellGate is implemented in gate_unix.go / gate_windows.go so the
// shell binary matches the host OS. This wrapper just unifies the
// signature.
var _ = errors.New // keep import in case future variants need it

// gateRowRe matches a single markdown table row from tasks.md with a
// Gate (command) column. We accept optional surrounding whitespace and
// backtick fences around the command.
var gateRowRe = regexp.MustCompile("`([^`]+)`")

// extractGates pulls unique gate commands out of tasks.md. The convention is
// `| <task> | <gate> | ...` per row; we capture anything inside backticks.
// Single-token commands like `true` and `pwd` are valid gates too, so we
// don't filter by whitespace — validation is the allowlist's job.
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
			if cmd == "" {
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

// validateGateCommand is a thin delegation to internal/policy.
func validateGateCommand(gate string) error {
	return policy.ValidateGateCommand(gate)
}

// splitOnLogicalOps is a thin delegation to internal/policy.
func splitOnLogicalOps(s string) []string {
	return policy.SplitOnLogicalOps(s)
}

// splitShellTokens is a thin delegation to internal/policy.
func splitShellTokens(cmd string) []string {
	return policy.SplitShellTokens(cmd)
}

// isShellOp is a thin delegation to internal/policy.
func isShellOp(s string) bool {
	return policy.IsShellOp(s)
}

// ACDetail holds parsed AC information.
type ACDetail struct {
	ID    string
	Title string
	Given string
	When  string
	Then  string
}

// extractACDetails extracts full AC details from spec content. Handles
// "And" clauses by appending to the most recent non-empty Given/When/Then.
func extractACDetails(content string) []ACDetail {
	var acs []ACDetail
	scanner := bufio.NewScanner(strings.NewReader(content))

	acRe := regexp.MustCompile("^###\\s+(AC-\\d+):\\s*(.+)$")
	// Regexes are permissive: accept with or without markdown bold (`**`),
	// with or without trailing colon, and case-insensitive. Matches both
	// "- **Given** x" and "- Given: x".
	givenRe := regexp.MustCompile("(?i)^[-\\s]*\\*?\\*?Given\\*?\\*?\\s*[:：]?\\s*(.+)$")
	whenRe := regexp.MustCompile("(?i)^[-\\s]*\\*?\\*?When\\*?\\*?\\s*[:：]?\\s*(.+)$")
	thenRe := regexp.MustCompile("(?i)^[-\\s]*\\*?\\*?Then\\*?\\*?\\s*[:：]?\\s*(.+)$")
	andRe := regexp.MustCompile("(?i)^[-\\s]*\\*?\\*?And\\*?\\*?\\s*[:：]?\\s*(.+)$")

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
				continue
			}
			if m := whenRe.FindStringSubmatch(line); m != nil {
				current.When = strings.TrimSpace(m[1])
				continue
			}
			if m := thenRe.FindStringSubmatch(line); m != nil {
				current.Then = strings.TrimSpace(m[1])
				continue
			}
			if m := andRe.FindStringSubmatch(line); m != nil {
				extra := " and " + strings.TrimSpace(m[1])
				switch {
				case current.Then != "":
					current.Then += extra
				case current.When != "":
					current.When += extra
				case current.Given != "":
					current.Given += extra
				}
				continue
			}
		}
	}

	if current != nil {
		acs = append(acs, *current)
	}

	return acs
}
