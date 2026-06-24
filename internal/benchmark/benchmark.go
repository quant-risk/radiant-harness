// Package benchmark provides comparison benchmarks for SDD frameworks.
package benchmark

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Framework represents an SDD framework to benchmark.
type Framework struct {
	Name        string
	Description string
	Type        string // "harness", "skill", "template"
	Command     string
}

// KnownFrameworks is the list of frameworks we benchmark against.
var KnownFrameworks = []Framework{
	{Name: "radiant-harness", Description: "Go harness with orchestrator", Type: "harness", Command: "radiant run"},
	{Name: "tlc-spec-driven", Description: "Tech Leads Club skill", Type: "skill", Command: "skill"},
	{Name: "github-spec-kit", Description: "GitHub spec-driven framework", Type: "template", Command: "npx"},
	{Name: "superpowers", Description: "Superpowers AI coding", Type: "skill", Command: "skill"},
	{Name: "openspec", Description: "OpenSpec framework", Type: "template", Command: "npx"},
}

// BenchmarkResult is the result of a single benchmark run.
type BenchmarkResult struct {
	Framework   string        `json:"framework"`
	Feature     string        `json:"feature"`
	Duration    time.Duration `json:"duration"`
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

// RunRadiantHarness runs a benchmark using radiant-harness.
func (bs *BenchmarkSuite) RunRadiantHarness(specDir string) (*BenchmarkResult, error) {
	start := time.Now()

	result := &BenchmarkResult{
		Framework: "radiant-harness",
		Feature:   filepath.Base(specDir),
		Duration:  time.Since(start),
		Score:     0.0,
	}

	bs.Results = append(bs.Results, *result)
	return result, nil
}

// Summary returns a summary of all benchmark results.
func (bs *BenchmarkSuite) Summary() string {
	if len(bs.Results) == 0 {
		return "No benchmark results"
	}

	header := fmt.Sprintf("%-20s %-15s %8s %8s %8s %8s", "Framework", "Feature", "Duration", "Tokens", "ACs", "Score")
	separator := fmt.Sprintf("%-20s %-15s %8s %8s %8s %8s", "--------", "-------", "--------", "------", "---", "-----")

	summary := header + "\n" + separator + "\n"
	for _, r := range bs.Results {
		summary += fmt.Sprintf("%-20s %-15s %8s %8d %d/%d  %.2f\n",
			r.Framework,
			r.Feature,
			r.Duration.Round(time.Millisecond),
			r.TokensUsed,
			r.ACsCovered,
			r.ACsTotal,
			r.Score)
	}

	return summary
}

// SaveResults saves benchmark results to a JSON file.
func (bs *BenchmarkSuite) SaveResults(path string) error {
	dir := filepath.Dir(path)
	os.MkdirAll(dir, 0o755)

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

// Compare compares two benchmark results.
func Compare(a, b BenchmarkResult) string {
	var comparison string

	if a.Score > b.Score {
		comparison += fmt.Sprintf("%s scores higher (%.2f vs %.2f)\n", a.Framework, a.Score, b.Score)
	} else if b.Score > a.Score {
		comparison += fmt.Sprintf("%s scores higher (%.2f vs %.2f)\n", b.Framework, b.Score, a.Score)
	} else {
		comparison += fmt.Sprintf("Equal scores (%.2f)\n", a.Score)
	}

	if a.Duration < b.Duration {
		comparison += fmt.Sprintf("%s is faster (%s vs %s)\n",
			a.Framework, a.Duration.Round(time.Millisecond), b.Duration.Round(time.Millisecond))
	}

	return comparison
}
