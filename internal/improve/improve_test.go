package improve

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ── Analyzer tests ────────────────────────────────────────────────────────────

func TestAnalyzeTraces_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	result, err := AnalyzeTraces(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Traces != 0 {
		t.Errorf("empty dir: expected 0 traces, got %d", result.Traces)
	}
	if len(result.Patterns) != 0 {
		t.Errorf("empty dir: expected no patterns, got %d", len(result.Patterns))
	}
}

func TestAnalyzeTraces_DetectsFailures(t *testing.T) {
	dir := t.TempDir()
	writeTrace(t, dir, "run-001", []traceEvent{
		{Phase: "execute", Action: "write_file", Result: "ok", RunID: "run-001"},
		{Phase: "verify", Action: "check", Result: "failed", Evidence: "no evidence found", RunID: "run-001"},
		{Phase: "verify", Action: "check2", Result: "failed", Evidence: "no test results", RunID: "run-001"},
	})

	result, err := AnalyzeTraces(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Failures == 0 {
		t.Error("should detect failures")
	}
	if len(result.Patterns) == 0 {
		t.Error("should detect patterns from failures")
	}
}

func TestAnalyzeTraces_MissingVerificationCategory(t *testing.T) {
	dir := t.TempDir()
	writeTrace(t, dir, "run-002", []traceEvent{
		{Phase: "verify", Action: "review", Result: "failed", Evidence: "no evidence cited", RunID: "run-002"},
		{Phase: "verify", Action: "test", Result: "failed", Evidence: "no test found", RunID: "run-002"},
		{Phase: "verify", Action: "audit", Result: "failed", Evidence: "unverified claim", RunID: "run-002"},
	})

	result, err := AnalyzeTraces(dir, "")
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, p := range result.Patterns {
		if p.Category == CategoryMissingVerification {
			found = true
			if p.Count < 2 {
				t.Errorf("missing-verification count = %d, want ≥2", p.Count)
			}
		}
	}
	if !found {
		t.Errorf("expected CategoryMissingVerification pattern, got: %+v", result.Patterns)
	}
}

func TestAnalyzeTraces_RepeatFailure(t *testing.T) {
	dir := t.TempDir()
	// Same action fails in two different runs
	writeTrace(t, dir, "run-003", []traceEvent{
		{Phase: "execute", Action: "run_tests", Result: "failed", Evidence: "timeout", RunID: "run-003"},
	})
	writeTrace(t, dir, "run-004", []traceEvent{
		{Phase: "execute", Action: "run_tests", Result: "failed", Evidence: "timeout again", RunID: "run-004"},
	})

	result, err := AnalyzeTraces(dir, "")
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, p := range result.Patterns {
		if p.Category == CategoryRepeatFailure {
			found = true
		}
	}
	if !found {
		t.Errorf("expected CategoryRepeatFailure pattern for same action across runs, got: %+v", result.Patterns)
	}
}

func TestAnalyzeTraces_SkillFilter(t *testing.T) {
	dir := t.TempDir()
	writeTrace(t, dir, "run-005", []traceEvent{
		{Phase: "execute", Action: "nova-feature", Result: "failed",
			Evidence: "incomplete", RunID: "run-005",
			Meta: map[string]string{"skill": "nova-feature"}},
		{Phase: "execute", Action: "validar", Result: "failed",
			Evidence: "no test", RunID: "run-005",
			Meta: map[string]string{"skill": "validar"}},
	})

	result, err := AnalyzeTraces(dir, "nova-feature")
	if err != nil {
		t.Fatal(err)
	}
	// Only 1 event should match the filter
	if result.Events > 1 {
		t.Errorf("filter should reduce events; got %d", result.Events)
	}
}

func TestFormatAnalysis_NoTraces(t *testing.T) {
	r := &AnalysisResult{AnalyzedAt: time.Now()}
	out := FormatAnalysis(r)
	if !strings.Contains(out, "No traces found") {
		t.Errorf("expected 'No traces found', got: %s", out)
	}
}

func TestFormatAnalysis_WithPatterns(t *testing.T) {
	r := &AnalysisResult{
		Traces:   2,
		Events:   10,
		Failures: 4,
		Patterns: []FailurePattern{
			{Category: CategoryMissingVerification, Count: 3, Confidence: 0.85,
				Examples: []string{"no evidence found"}},
		},
	}
	out := FormatAnalysis(r)
	if !strings.Contains(out, "missing-verification") {
		t.Errorf("expected pattern category in output, got: %s", out)
	}
	if !strings.Contains(out, "85%") {
		t.Errorf("expected confidence in output, got: %s", out)
	}
}

// ── Proposer tests ────────────────────────────────────────────────────────────

func TestProposeEdits_Empty(t *testing.T) {
	proposals := ProposeEdits(nil, t.TempDir())
	if len(proposals) != 0 {
		t.Errorf("expected 0 proposals for empty patterns, got %d", len(proposals))
	}
}

func TestProposeEdits_LowConfidenceSkipped(t *testing.T) {
	patterns := []FailurePattern{
		{Category: CategoryMissingVerification, Count: 1, Confidence: 0.20},
	}
	proposals := ProposeEdits(patterns, t.TempDir())
	if len(proposals) != 0 {
		t.Errorf("low confidence pattern should be skipped, got %d proposals", len(proposals))
	}
}

func TestProposeEdits_GeneratesProposal(t *testing.T) {
	patterns := []FailurePattern{
		{Category: CategoryMissingVerification, Count: 3, Confidence: 0.85,
			Skill: "validar"},
	}
	proposals := ProposeEdits(patterns, t.TempDir())
	if len(proposals) == 0 {
		t.Fatal("expected ≥1 proposal")
	}
	p := proposals[0]
	if p.Category != CategoryMissingVerification {
		t.Errorf("category = %q, want missing-verification", p.Category)
	}
	if p.After == "" {
		t.Error("proposal.After should contain the suggested change")
	}
}

func TestFormatProposals_Empty(t *testing.T) {
	out := FormatProposals(nil)
	if !strings.Contains(out, "No proposals") {
		t.Errorf("expected 'No proposals', got: %s", out)
	}
}

func TestFormatProposals_WithContent(t *testing.T) {
	proposals := []Proposal{
		{Skill: "validar", File: "SKILL.md", Category: CategoryMissingVerification,
			Confidence: 0.85, Description: "Strengthen verification"},
	}
	out := FormatProposals(proposals)
	if !strings.Contains(out, "validar") {
		t.Errorf("expected skill name in output, got: %s", out)
	}
	if !strings.Contains(out, "85%") {
		t.Errorf("expected confidence in output, got: %s", out)
	}
}

// ── Validator tests ───────────────────────────────────────────────────────────

func TestValidateProposal_NoEvents(t *testing.T) {
	proposal := Proposal{Skill: "validar", Category: CategoryMissingVerification, Confidence: 0.8}
	analysis := &AnalysisResult{Events: 0, Failures: 0}
	result := ValidateProposal(proposal, analysis)
	if result.Passed {
		t.Error("should not pass when no events")
	}
}

func TestValidateProposal_PassesWhenImprovementLarge(t *testing.T) {
	proposal := Proposal{
		Skill:      "validar",
		Category:   CategoryMissingVerification,
		Confidence: 0.90,
	}
	analysis := &AnalysisResult{
		Events:   100,
		Failures: 30,
		Patterns: []FailurePattern{
			{Category: CategoryMissingVerification, Count: 25, Confidence: 0.90},
		},
	}
	result := ValidateProposal(proposal, analysis)
	if !result.Passed {
		t.Errorf("should pass when improvement is large (%.1fpp)", result.DeltaPP)
	}
	if result.DeltaPP < 5.0 {
		t.Errorf("DeltaPP = %.1f, want ≥5.0", result.DeltaPP)
	}
}

func TestValidateProposal_FailsWhenImprovementSmall(t *testing.T) {
	proposal := Proposal{
		Skill:      "validar",
		Category:   CategoryMissingVerification,
		Confidence: 0.50,
	}
	analysis := &AnalysisResult{
		Events:   1000,
		Failures: 5, // very low failure rate
		Patterns: []FailurePattern{
			{Category: CategoryMissingVerification, Count: 1, Confidence: 0.50},
		},
	}
	result := ValidateProposal(proposal, analysis)
	if result.Passed {
		t.Errorf("should fail when improvement is tiny (%.1fpp < 5pp)", result.DeltaPP)
	}
}

func TestFormatValidationResult_Pass(t *testing.T) {
	result := ValidationResult{
		Proposal: Proposal{Skill: "validar", File: "SKILL.md", Category: CategoryMissingVerification},
		Passed:   true,
		OldScore: 0.70,
		NewScore: 0.85,
		DeltaPP:  15.0,
		Evidence: "addresses 3 of 10 failures",
	}
	out := FormatValidationResult(result)
	if !strings.Contains(out, "[PASS]") {
		t.Errorf("expected [PASS] in output, got: %s", out)
	}
	if !strings.Contains(out, "+15.0pp") {
		t.Errorf("expected delta in output, got: %s", out)
	}
}

func TestPersistAndReadHistory(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".radiant-harness"), 0o755)

	record := ImprovementRecord{
		Skill:       "validar",
		File:        "SKILL.md",
		Category:    "missing-verification",
		Description: "Strengthen verification",
		OldScore:    0.70,
		NewScore:    0.85,
		DeltaPP:     15.0,
		AppliedAt:   time.Now(),
	}

	if err := PersistRecord(record, dir); err != nil {
		t.Fatal(err)
	}

	records, err := ReadHistory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Errorf("expected 1 record, got %d", len(records))
	}
	if records[0].Skill != "validar" {
		t.Errorf("skill = %q, want validar", records[0].Skill)
	}
	if records[0].DeltaPP != 15.0 {
		t.Errorf("DeltaPP = %.1f, want 15.0", records[0].DeltaPP)
	}
}

func TestReadHistory_Empty(t *testing.T) {
	dir := t.TempDir()
	records, err := ReadHistory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if records != nil {
		t.Errorf("empty dir should return nil, got %v", records)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func writeTrace(t *testing.T, dir, runID string, events []traceEvent) {
	t.Helper()
	tracesDir := filepath.Join(dir, ".radiant-harness", "traces")
	os.MkdirAll(tracesDir, 0o755)
	path := filepath.Join(tracesDir, runID+".jsonl")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, e := range events {
		b, _ := json.Marshal(e)
		f.WriteString(string(b) + "\n")
	}
}
