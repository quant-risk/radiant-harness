package benchmark

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestKnownFrameworksHasRadiantHarness(t *testing.T) {
	found := false
	for _, fw := range KnownFrameworks {
		if fw.Name == "radiant-harness" {
			found = true
			if fw.Type != "harness" {
				t.Errorf("radiant-harness type should be 'harness', got %q", fw.Type)
			}
			if fw.Command == "" {
				t.Error("radiant-harness command should not be empty")
			}
		}
	}
	if !found {
		t.Error("radiant-harness missing from KnownFrameworks")
	}
}

func TestKnownFrameworksIsSortedAlphabetically(t *testing.T) {
	for i := 1; i < len(KnownFrameworks); i++ {
		if KnownFrameworks[i-1].Name > KnownFrameworks[i].Name {
			t.Errorf("KnownFrameworks not sorted at %d: %q > %q",
				i, KnownFrameworks[i-1].Name, KnownFrameworks[i].Name)
		}
	}
}

func TestParseACCount(t *testing.T) {
	cases := []struct {
		in      string
		covered int
		total   int
	}{
		{"Feature: foo (3/5 ACs)", 3, 5},
		{"3/5 ACs covered", 3, 5},
		{"0 ACs covered", 0, 0}, // matches but is 0/0
		{"no ac info here", 0, 0},
	}
	for _, c := range cases {
		covered, total := parseACCount(c.in)
		if covered != c.covered || total != c.total {
			t.Errorf("parseACCount(%q) = (%d, %d), want (%d, %d)",
				c.in, covered, total, c.covered, c.total)
		}
	}
}

func TestEstimateTokens(t *testing.T) {
	if got := estimateTokens("hello world"); got != 2 {
		t.Errorf("estimateTokens('hello world') = %d, want ~2", got)
	}
	if got := estimateTokens(strings.Repeat("a", 400)); got != 100 {
		t.Errorf("estimateTokens(400 chars) = %d, want 100", got)
	}
}

func TestCompare(t *testing.T) {
	a := BenchmarkResult{Framework: "a", Score: 0.9, Duration: 5 * time.Second}
	b := BenchmarkResult{Framework: "b", Score: 0.7, Duration: 8 * time.Second}
	c := Compare(a, b)
	if !strings.Contains(c, "a scores higher") {
		t.Errorf("expected 'a scores higher' in comparison: %q", c)
	}
	if !strings.Contains(c, "a is faster") {
		t.Errorf("expected 'a is faster' in comparison: %q", c)
	}
}

func TestSaveAndLoadResults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "results.json")
	suite := NewBenchmarkSuite()
	suite.Results = []BenchmarkResult{
		{Framework: "a", Feature: "feat", Duration: 5 * time.Second, Score: 0.8, ACsCovered: 4, ACsTotal: 5},
	}
	if err := suite.SaveResults(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadResults(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 result, got %d", len(loaded))
	}
	if loaded[0].Framework != "a" {
		t.Errorf("loaded framework = %q, want 'a'", loaded[0].Framework)
	}
	if loaded[0].Score != 0.8 {
		t.Errorf("loaded score = %f, want 0.8", loaded[0].Score)
	}
}

func TestSaveResultsCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "results.json")
	suite := NewBenchmarkSuite()
	suite.Results = []BenchmarkResult{{Framework: "x", Feature: "y"}}
	if err := suite.SaveResults(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file not created: %v", err)
	}
}

func TestSummarySortsByScoreDesc(t *testing.T) {
	suite := NewBenchmarkSuite()
	suite.Results = []BenchmarkResult{
		{Framework: "low", Score: 0.5, ACsCovered: 1, ACsTotal: 2, Duration: 1 * time.Second},
		{Framework: "high", Score: 0.9, ACsCovered: 9, ACsTotal: 10, Duration: 5 * time.Second},
	}
	summary := suite.Summary()
	// "high" must come before "low" in the markdown table.
	hiIdx := strings.Index(summary, "| high |")
	loIdx := strings.Index(summary, "| low |")
	if hiIdx < 0 || loIdx < 0 {
		t.Fatalf("missing rows in summary: %q", summary)
	}
	if hiIdx > loIdx {
		t.Errorf("expected high before low in summary, got: %q", summary)
	}
}

func TestRunCommandRejectsEmptyCommand(t *testing.T) {
	suite := NewBenchmarkSuite()
	_, err := suite.RunCommand(context.Background(), Framework{Name: "broken"})
	if err == nil {
		t.Error("expected error for empty command")
	}
}

func TestRunCommandWithMissingBinary(t *testing.T) {
	suite := NewBenchmarkSuite()
	// Use a binary that's definitely not on $PATH.
	_, err := suite.RunCommand(context.Background(),
		Framework{Name: "fake", Command: "this-binary-does-not-exist-xyz123"})
	if err == nil {
		t.Error("expected error for missing binary")
	}
	// The result should still be appended with the error.
	if len(suite.Results) != 1 {
		t.Errorf("expected 1 result (with error), got %d", len(suite.Results))
	}
	if len(suite.Results[0].Errors) == 0 {
		t.Error("expected error to be recorded in result")
	}
}

func TestBenchmarkResultJSONRoundTrip(t *testing.T) {
	original := BenchmarkResult{
		Framework:  "radiant-harness",
		Feature:    "0001-test",
		Duration:   5 * time.Second,
		TokensUsed: 12345,
		ACsCovered: 4,
		ACsTotal:   5,
		Score:      0.8,
		Errors:     []string{"none really"},
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	var got BenchmarkResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Framework != original.Framework || got.Score != original.Score {
		t.Errorf("round trip mismatch: %+v vs %+v", got, original)
	}
}
