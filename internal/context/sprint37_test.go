package context

import (
	"strings"
	"testing"
)

// ── SummarizePhase ────────────────────────────────────────────────────────────

func TestSummarizePhase_ReducesTokens(t *testing.T) {
	// Generate a chunky content block
	content := strings.Repeat("This is a detailed explanation of what happened during the execute phase. "+
		"The agent implemented the login feature with JWT tokens. "+
		"Tests were written for all edge cases. ", 40)

	result := SummarizePhase("execute", content)

	if result.Phase != "execute" {
		t.Errorf("phase = %q, want execute", result.Phase)
	}
	if result.Original <= 0 {
		t.Error("original tokens should be > 0")
	}
	if result.Summarized >= result.Original {
		t.Errorf("summarized (%d) should be < original (%d)", result.Summarized, result.Original)
	}
	if result.Ratio > 0.25 {
		t.Errorf("ratio = %.2f, want ≤ 0.25 (≤25%% of original)", result.Ratio)
	}
	if result.Content == "" {
		t.Error("summary content should not be empty")
	}
}

func TestSummarizePhase_PreservesPhaseHeader(t *testing.T) {
	content := "Some phase content with decisions: decided to use JWT. ✓ login implemented."
	result := SummarizePhase("plan", content)
	if !strings.Contains(result.Content, "Phase summary: plan") {
		t.Errorf("summary should include phase header, got:\n%s", result.Content)
	}
}

func TestSummarizePhase_ExtractsKeyFacts(t *testing.T) {
	content := `## Planning phase
✓ Architecture decision: use microservices
APPROVED: JWT auth strategy
blocker: database migration pending
implemented: user registration endpoint
Some other content that is less important...`

	result := SummarizePhase("plan", content)
	if len(result.KeyFacts) == 0 {
		t.Error("should extract key facts from content")
	}
	// At least some keywords should be found
	combined := strings.Join(result.KeyFacts, " ")
	if !strings.Contains(combined, "✓") && !strings.Contains(combined, "APPROVED") &&
		!strings.Contains(combined, "blocker") {
		t.Errorf("key facts should include important markers, got: %v", result.KeyFacts)
	}
}

func TestSummarizePhase_TinyContent(t *testing.T) {
	result := SummarizePhase("discover", "short")
	if result.Content == "" {
		t.Error("should handle tiny content without panic")
	}
}

func TestSummarizePhase_EmptyContent(t *testing.T) {
	result := SummarizePhase("verify", "")
	if result.Original < 0 {
		t.Error("original tokens should be >= 0")
	}
}

// ── SummarizeTrace ────────────────────────────────────────────────────────────

func TestSummarizeTrace_CountsEvents(t *testing.T) {
	events := []string{
		"15:00:01 [execute] ✓ write_file ok",
		"15:00:02 [verify] ✗ run_tests failed",
		"15:00:03 [execute] ✓ fix_lint ok",
	}
	out := SummarizeTrace(events)
	if !strings.Contains(out, "2 actions completed") {
		t.Errorf("should count 2 ok events, got: %s", out)
	}
	if !strings.Contains(out, "1 actions failed") {
		t.Errorf("should count 1 failed event, got: %s", out)
	}
}

func TestSummarizeTrace_Empty(t *testing.T) {
	out := SummarizeTrace(nil)
	if !strings.Contains(out, "no trace events") {
		t.Errorf("empty trace should say no events, got: %s", out)
	}
}

// ── BudgetProfiles ────────────────────────────────────────────────────────────

func TestGetProfile_Lean(t *testing.T) {
	p := GetProfile("lean")
	if p.TotalTokens != 10_000 {
		t.Errorf("lean TotalTokens = %d, want 10000", p.TotalTokens)
	}
	if p.Name != "lean" {
		t.Errorf("Name = %q, want lean", p.Name)
	}
}

func TestGetProfile_Standard(t *testing.T) {
	p := GetProfile("standard")
	if p.TotalTokens != 50_000 {
		t.Errorf("standard TotalTokens = %d, want 50000", p.TotalTokens)
	}
}

func TestGetProfile_Thorough(t *testing.T) {
	p := GetProfile("thorough")
	if p.TotalTokens != 200_000 {
		t.Errorf("thorough TotalTokens = %d, want 200000", p.TotalTokens)
	}
}

func TestGetProfile_UnknownDefaultsToStandard(t *testing.T) {
	p := GetProfile("nonexistent")
	if p.TotalTokens != 50_000 {
		t.Errorf("unknown profile should default to standard (50K), got %d", p.TotalTokens)
	}
}

func TestGetProfile_AllPhasesPresent(t *testing.T) {
	phases := []string{"discover", "plan", "execute", "verify", "persist"}
	for _, name := range []string{"lean", "standard", "thorough"} {
		p := GetProfile(name)
		for _, phase := range phases {
			if p.PerPhase[phase] <= 0 {
				t.Errorf("%s profile: phase %q has 0 token budget", name, phase)
			}
		}
	}
}

func TestGetProfile_PhaseSumMatchesTotal(t *testing.T) {
	for _, name := range []string{"lean", "standard", "thorough"} {
		p := GetProfile(name)
		sum := 0
		for _, v := range p.PerPhase {
			sum += v
		}
		if sum != p.TotalTokens {
			t.Errorf("%s profile: phase sum %d ≠ TotalTokens %d", name, sum, p.TotalTokens)
		}
	}
}

// ── EstimateSpec ─────────────────────────────────────────────────────────────

func TestEstimateSpec_AllPhasesCovered(t *testing.T) {
	spec := "# Feature: Login\n\nImplement JWT-based login endpoint.\n\n## ACs\n- User can login with email+password\n- Returns JWT token\n"
	estimates := EstimateSpec(spec, ProfileStandard)

	phases := map[string]bool{}
	for _, e := range estimates {
		phases[e.Phase] = true
	}
	for _, want := range []string{"discover", "plan", "execute", "verify", "persist"} {
		if !phases[want] {
			t.Errorf("missing estimate for phase %q", want)
		}
	}
}

func TestEstimateSpec_ExecuteLargerThanDiscover(t *testing.T) {
	spec := strings.Repeat("feature content ", 50)
	estimates := EstimateSpec(spec, ProfileStandard)

	phaseMap := map[string]PhaseEstimate{}
	for _, e := range estimates {
		phaseMap[e.Phase] = e
	}
	if phaseMap["execute"].Typical <= phaseMap["discover"].Typical {
		t.Errorf("execute (%d) should be larger than discover (%d)",
			phaseMap["execute"].Typical, phaseMap["discover"].Typical)
	}
}

func TestEstimateSpec_EmptySpecHasFallback(t *testing.T) {
	estimates := EstimateSpec("", ProfileLean)
	for _, e := range estimates {
		if e.Typical <= 0 {
			t.Errorf("phase %q: typical should be > 0 even for empty spec", e.Phase)
		}
	}
}

func TestFormatEstimate_ContainsAllPhases(t *testing.T) {
	estimates := EstimateSpec("some spec content", ProfileStandard)
	out := FormatEstimate(estimates, ProfileStandard)
	for _, phase := range []string{"discover", "plan", "execute", "verify", "persist"} {
		if !strings.Contains(out, phase) {
			t.Errorf("format output missing phase %q, got:\n%s", phase, out)
		}
	}
	if !strings.Contains(out, "TOTAL") {
		t.Errorf("format output should contain TOTAL row, got:\n%s", out)
	}
}
