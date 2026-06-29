// Package benchmark provides a comparison harness for SDD frameworks. The
// goal is to measure concrete numbers (duration, tokens, AC coverage,
// test pass rate) when running the same feature through different
// frameworks — useful for justifying the choice of radiant-harness over
// alternatives and for tracking regressions across releases.
package benchmark

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Framework represents an SDD framework to benchmark against.
type Framework struct {
	Name        string
	Description string
	Type        string // "harness", "skill", "template"
	Command     string // shell command that triggers a run
}

// KnownFrameworks is the list of frameworks we benchmark against. Vendor-
// neutral: ordered alphabetically by name. Add a framework by appending to
// this slice — nothing else needs to change.
var KnownFrameworks = []Framework{
	{Name: "github-spec-kit", Description: "GitHub spec-driven framework", Type: "template", Command: "npx -y @github/spec-kit run"},
	{Name: "openspec", Description: "OpenSpec framework", Type: "template", Command: "npx -y @fission-ai/openspec run"},
	{Name: "radiant-harness", Description: "Go harness with orchestrator + feedback loop", Type: "harness", Command: "radiant run"},
	{Name: "superpowers", Description: "Superpowers AI coding skill", Type: "skill", Command: "skill"},
	{Name: "tlc-spec-driven", Description: "Tech Leads Club spec-driven skill", Type: "skill", Command: "skill"},
}

// BenchmarkResult is the result of a single benchmark run.
type BenchmarkResult struct {
	Framework   string        `json:"framework"`
	Feature     string        `json:"feature"`
	Duration    time.Duration `json:"duration_ns"`
	TokensUsed  int           `json:"tokens_used"`
	ACsCovered  int           `json:"acs_covered"`
	ACsTotal    int           `json:"acs_total"`
	TestsPassed int           `json:"tests_passed"`
	TestsTotal  int           `json:"tests_total"`
	Score       float64       `json:"score"`
	Errors      []string      `json:"errors,omitempty"`
}

// BenchmarkSuite runs benchmarks across frameworks.
type BenchmarkSuite struct {
	Results []BenchmarkResult
}

// NewBenchmarkSuite creates a new benchmark suite.
func NewBenchmarkSuite() *BenchmarkSuite {
	return &BenchmarkSuite{}
}

// RunRadiantHarness runs `radiant run <specDir>` as a subprocess and
// captures duration + tokens. The convenience method exists because
// radiant-harness is our reference; other frameworks use RunCommand
// directly with their own command string.
func (bs *BenchmarkSuite) RunRadiantHarness(ctx context.Context, specDir string, extraArgs ...string) (*BenchmarkResult, error) {
	return bs.RunCommand(ctx, KnownFrameworks[2], append([]string{specDir}, extraArgs...)...)
}

// RunCommand runs an arbitrary command for a framework and captures
// duration + estimated token usage. The command's stdout is scanned for
// patterns like `Feature: foo (3 ACs, 5 tasks)` to extract AC/task counts.
//
// Even when the command fails (binary missing, non-zero exit, timeout), the
// result is appended to the suite with the error recorded — failures are
// data, not exceptions. The returned error is the underlying exec error so
// the caller can fail fast if they want.
func (bs *BenchmarkSuite) RunCommand(ctx context.Context, fw Framework, args ...string) (*BenchmarkResult, error) {
	if fw.Command == "" {
		return nil, fmt.Errorf("framework %q has no command", fw.Name)
	}
	start := time.Now()

	// Split command into program + args so we can use exec.CommandContext
	// (which takes the program as a separate string).
	parts := strings.Fields(fw.Command)
	program := parts[0]
	cmdArgs := append(parts[1:], args...)

	cmd := exec.CommandContext(ctx, program, cmdArgs...)
	out, err := cmd.CombinedOutput()
	duration := time.Since(start)

	// Feature name = first positional arg if it looks like a feature path,
	// otherwise empty. Most frameworks' commands take the spec dir first.
	feature := ""
	if len(cmdArgs) > 0 {
		feature = filepath.Base(cmdArgs[0])
	}

	result := &BenchmarkResult{
		Framework: fw.Name,
		Feature:   feature,
		Duration:  duration,
	}

	if err != nil {
		result.Errors = append(result.Errors, err.Error())
	}

	// Extract AC/task counts from output. Best-effort — if the framework's
	// output format is different the fields stay at 0 and the operator can
	// fill them manually in the JSON later.
	result.ACsCovered, result.ACsTotal = parseACCount(string(out))
	result.TokensUsed = estimateTokens(string(out))

	if result.ACsTotal > 0 {
		result.Score = float64(result.ACsCovered) / float64(result.ACsTotal)
	}

	bs.Results = append(bs.Results, *result)
	return &bs.Results[len(bs.Results)-1], err
}

// Summary returns a Markdown table of all benchmark results, sorted by
// score descending. Useful for pasting into a PR description or research
// doc.
func (bs *BenchmarkSuite) Summary() string {
	if len(bs.Results) == 0 {
		return "No benchmark results"
	}

	sorted := append([]BenchmarkResult(nil), bs.Results...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Score != sorted[j].Score {
			return sorted[i].Score > sorted[j].Score
		}
		return sorted[i].Duration < sorted[j].Duration
	})

	var sb strings.Builder
	sb.WriteString("| Framework | Feature | Duration | Tokens | ACs | Score | Errors |\n")
	sb.WriteString("|-----------|---------|----------|--------|-----|-------|--------|\n")
	for _, r := range sorted {
		errs := ""
		if len(r.Errors) > 0 {
			errs = fmt.Sprintf("%d", len(r.Errors))
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %d | %d/%d | %.2f | %s |\n",
			r.Framework,
			r.Feature,
			r.Duration.Round(time.Millisecond),
			r.TokensUsed,
			r.ACsCovered,
			r.ACsTotal,
			r.Score,
			errs,
		))
	}
	return sb.String()
}

// SaveResults saves benchmark results to a JSON file. Useful for
// tracking regressions across runs and feeding into a CI report.
func (bs *BenchmarkSuite) SaveResults(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(bs.Results, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// LoadResults loads benchmark results from a JSON file.
func LoadResults(path string) ([]BenchmarkResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var results []BenchmarkResult
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, err
	}

	return results, nil
}

// Compare returns a human-readable comparison of two results.
func Compare(a, b BenchmarkResult) string {
	var sb strings.Builder
	if a.Score > b.Score {
		sb.WriteString(fmt.Sprintf("%s scores higher (%.2f vs %.2f)\n", a.Framework, a.Score, b.Score))
	} else if b.Score > a.Score {
		sb.WriteString(fmt.Sprintf("%s scores higher (%.2f vs %.2f)\n", b.Framework, a.Score, b.Score))
	} else {
		sb.WriteString(fmt.Sprintf("Equal scores (%.2f)\n", a.Score))
	}

	if a.Duration < b.Duration {
		sb.WriteString(fmt.Sprintf("%s is faster (%s vs %s)\n",
			a.Framework, a.Duration.Round(time.Millisecond), b.Duration.Round(time.Millisecond)))
	} else if b.Duration < a.Duration {
		sb.WriteString(fmt.Sprintf("%s is faster (%s vs %s)\n",
			b.Framework, a.Duration.Round(time.Millisecond), b.Duration.Round(time.Millisecond)))
	}

	return sb.String()
}

// acCountRe matches "N ACs" or "N/M ACs" in framework output.
var acCountRe = regexp.MustCompile(`(\d+)/(\d+)\s*ACs?`)

// parseACCount extracts covered/total AC counts from a framework's output.
// Returns (covered, total). If the pattern isn't found, both are 0.
func parseACCount(output string) (int, int) {
	m := acCountRe.FindStringSubmatch(output)
	if len(m) != 3 {
		return 0, 0
	}
	covered, total := 0, 0
	if _, err := fmt.Sscanf(m[1], "%d", &covered); err != nil {
		return 0, 0
	}
	if _, err := fmt.Sscanf(m[2], "%d", &total); err != nil {
		return 0, 0
	}
	return covered, total
}

// estimateTokens is a coarse 1-token-per-4-characters heuristic. Good
// enough for relative comparisons between frameworks; absolute numbers
// would need a real tokenizer (tiktoken, sentencepiece) per vendor.
func estimateTokens(s string) int {
	return len(s) / 4
}
