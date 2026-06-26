// Package improve implements the self-improvement engine.
// It analyzes loop failure traces, proposes instruction edits for skills,
// validates proposals on held-out tasks, and persists a history of improvements.
//
// Design: proposals are only applied when they demonstrably improve success rate.
// Regressions are rejected. The original skill is always backed up before patching.
package improve

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FailureCategory classifies why a loop iteration failed.
type FailureCategory string

const (
	// CategoryPrematureExit: loop exited before completing all ACs.
	CategoryPrematureExit FailureCategory = "premature-exit"
	// CategoryWrongScope: executor acted outside the defined goal.
	CategoryWrongScope FailureCategory = "wrong-scope"
	// CategoryMissingVerification: verifier had no evidence to evaluate.
	CategoryMissingVerification FailureCategory = "missing-verification"
	// CategoryRepeatFailure: same error across multiple iterations.
	CategoryRepeatFailure FailureCategory = "repeat-failure"
	// CategoryBudgetWaste: high token consumption with no output.
	CategoryBudgetWaste FailureCategory = "budget-waste"
	// CategoryUnknown: failure doesn't match a known pattern.
	CategoryUnknown FailureCategory = "unknown"
)

// FailurePattern is a detected pattern across one or more trace events.
type FailurePattern struct {
	Category   FailureCategory
	Count      int
	Examples   []string // excerpts from trace events
	Skill      string   // which skill was in use when failure occurred
	RunIDs     []string // which runs exhibited this pattern
	Confidence float64  // 0.0–1.0
}

// AnalysisResult is the output of analyzing a set of traces.
type AnalysisResult struct {
	Traces     int
	Events     int
	Failures   int
	Patterns   []FailurePattern
	AnalyzedAt time.Time
}

// AnalyzeTraces reads all trace files in projectDir and extracts failure patterns.
// If skillFilter is non-empty, only events mentioning that skill are analyzed.
func AnalyzeTraces(projectDir, skillFilter string) (*AnalysisResult, error) {
	tracesDir := filepath.Join(projectDir, ".radiant-harness", "traces")
	entries, err := os.ReadDir(tracesDir)
	if os.IsNotExist(err) {
		return &AnalysisResult{AnalyzedAt: time.Now()}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read traces dir: %w", err)
	}

	var allEvents []traceEvent
	traceCount := 0

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		events, err := readTraceFile(filepath.Join(tracesDir, e.Name()))
		if err != nil {
			continue
		}
		traceCount++
		for _, ev := range events {
			if skillFilter != "" && !strings.Contains(ev.Action, skillFilter) &&
				!strings.Contains(ev.Meta["skill"], skillFilter) {
				continue
			}
			allEvents = append(allEvents, ev)
		}
	}

	failures := filterFailures(allEvents)
	patterns := detectPatterns(failures, allEvents)

	return &AnalysisResult{
		Traces:     traceCount,
		Events:     len(allEvents),
		Failures:   len(failures),
		Patterns:   patterns,
		AnalyzedAt: time.Now(),
	}, nil
}

// FormatAnalysis renders the analysis result as a human-readable report.
func FormatAnalysis(r *AnalysisResult) string {
	if r.Traces == 0 {
		return "No traces found. Start a loop with `radiant loop start \"<goal>\"` first.\n"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Self-Improvement Analysis\n"))
	sb.WriteString(fmt.Sprintf("  Traces: %d | Events: %d | Failures: %d\n\n", r.Traces, r.Events, r.Failures))

	if len(r.Patterns) == 0 {
		sb.WriteString("No failure patterns detected. Skill performance looks healthy.\n")
		return sb.String()
	}

	sb.WriteString(fmt.Sprintf("Detected %d failure pattern(s):\n\n", len(r.Patterns)))
	for i, p := range r.Patterns {
		sb.WriteString(fmt.Sprintf("%d. [%s] — %d occurrence(s), confidence %.0f%%\n",
			i+1, p.Category, p.Count, p.Confidence*100))
		if p.Skill != "" {
			sb.WriteString(fmt.Sprintf("   Skill: %s\n", p.Skill))
		}
		for _, ex := range p.Examples {
			sb.WriteString(fmt.Sprintf("   Example: %s\n", ex))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// ── internals ─────────────────────────────────────────────────────────────────

type traceEvent struct {
	Phase     string            `json:"phase"`
	Action    string            `json:"action"`
	Result    string            `json:"result"`
	Evidence  string            `json:"evidence,omitempty"`
	TokensIn  int               `json:"tokens_in,omitempty"`
	TokensOut int               `json:"tokens_out,omitempty"`
	RunID     string            `json:"run"`
	Meta      map[string]string `json:"meta,omitempty"`
}

func readTraceFile(path string) ([]traceEvent, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var events []traceEvent
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var e traceEvent
		if json.Unmarshal([]byte(line), &e) == nil {
			events = append(events, e)
		}
	}
	return events, nil
}

func filterFailures(events []traceEvent) []traceEvent {
	var out []traceEvent
	for _, e := range events {
		if e.Result == "failed" || e.Phase == "failed" {
			out = append(out, e)
		}
	}
	return out
}

func detectPatterns(failures, all []traceEvent) []FailurePattern {
	if len(failures) == 0 {
		return nil
	}

	categoryCount := map[FailureCategory][]traceEvent{}
	for _, f := range failures {
		cat := classifyFailure(f)
		categoryCount[cat] = append(categoryCount[cat], f)
	}

	// Check for repeat failures (same action failing in multiple runs)
	actionRuns := map[string][]string{}
	for _, f := range failures {
		actionRuns[f.Action] = append(actionRuns[f.Action], f.RunID)
	}
	for action, runs := range actionRuns {
		if len(runs) >= 2 {
			unique := dedup(runs)
			if len(unique) >= 2 {
				categoryCount[CategoryRepeatFailure] = append(categoryCount[CategoryRepeatFailure],
					traceEvent{Action: action, RunID: strings.Join(unique, ",")})
			}
		}
	}

	var patterns []FailurePattern
	for cat, events := range categoryCount {
		if len(events) == 0 {
			continue
		}
		pattern := FailurePattern{
			Category:   cat,
			Count:      len(events),
			Confidence: calcConfidence(cat, len(events), len(all)),
		}
		// Attach run IDs and evidence snippets
		for _, e := range events {
			if e.RunID != "" && !containsStr(pattern.RunIDs, e.RunID) {
				pattern.RunIDs = append(pattern.RunIDs, e.RunID)
			}
			if e.Evidence != "" && len(pattern.Examples) < 3 {
				ex := e.Evidence
				if len(ex) > 80 {
					ex = ex[:77] + "..."
				}
				pattern.Examples = append(pattern.Examples, ex)
			}
			if e.Meta["skill"] != "" {
				pattern.Skill = e.Meta["skill"]
			}
		}
		patterns = append(patterns, pattern)
	}
	return patterns
}

func classifyFailure(e traceEvent) FailureCategory {
	ev := strings.ToLower(e.Evidence + " " + e.Action)
	switch {
	case strings.Contains(ev, "premature") || strings.Contains(ev, "incomplete") ||
		strings.Contains(ev, "not all ac"):
		return CategoryPrematureExit
	case strings.Contains(ev, "out of scope") || strings.Contains(ev, "wrong file") ||
		strings.Contains(ev, "unrelated"):
		return CategoryWrongScope
	case strings.Contains(ev, "no evidence") || strings.Contains(ev, "no test") ||
		strings.Contains(ev, "unverified"):
		return CategoryMissingVerification
	case e.TokensIn > 5000 && e.TokensOut < 10:
		return CategoryBudgetWaste
	default:
		return CategoryUnknown
	}
}

func calcConfidence(cat FailureCategory, count, total int) float64 {
	if total == 0 {
		return 0
	}
	base := float64(count) / float64(total)
	// Known categories get a confidence boost
	if cat != CategoryUnknown {
		base = base*0.7 + 0.3
	}
	if base > 1.0 {
		return 1.0
	}
	return base
}

func dedup(ss []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func containsStr(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
