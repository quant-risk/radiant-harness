//go:build with_full

package loop

import (
	"strings"
	"testing"
)

// ---- GeometricMean tests ----

func TestGeometricMean_AllOnes(t *testing.T) {
	dims := []VerifyDimension{
		{Name: "correctness", Score: 1.0},
		{Name: "completeness", Score: 1.0},
	}
	got := GeometricMean(dims)
	if got < 0.999 || got > 1.001 {
		t.Errorf("expected ~1.0, got %f", got)
	}
}

func TestGeometricMean_ZeroDrivesToZero(t *testing.T) {
	dims := []VerifyDimension{
		{Name: "correctness", Score: 1.0},
		{Name: "completeness", Score: 0.0}, // zero dimension
		{Name: "test_quality", Score: 0.9},
	}
	got := GeometricMean(dims)
	if got != 0.0 {
		t.Errorf("expected 0.0 when any dimension is zero, got %f", got)
	}
}

func TestGeometricMean_MixedScores(t *testing.T) {
	dims := []VerifyDimension{
		{Name: "a", Score: 0.5},
		{Name: "b", Score: 0.5},
	}
	got := GeometricMean(dims)
	// geometric mean of [0.5, 0.5] = 0.5
	if got < 0.499 || got > 0.501 {
		t.Errorf("expected ~0.5, got %f", got)
	}
}

func TestGeometricMean_Empty(t *testing.T) {
	got := GeometricMean(nil)
	if got != 0.0 {
		t.Errorf("expected 0 for empty dims, got %f", got)
	}
}

func TestGeometricMean_LowerThanArithmetic(t *testing.T) {
	// geo mean is always <= arithmetic mean (equality only when all equal)
	dims := []VerifyDimension{
		{Name: "a", Score: 0.9},
		{Name: "b", Score: 0.1},
	}
	geo := GeometricMean(dims)
	arith := (0.9 + 0.1) / 2 // 0.5
	if geo >= arith {
		t.Errorf("geometric mean (%f) should be < arithmetic mean (%f)", geo, arith)
	}
}

func TestGeometricMean_NegativeDrivesToZero(t *testing.T) {
	dims := []VerifyDimension{
		{Name: "a", Score: -0.1},
		{Name: "b", Score: 0.9},
	}
	got := GeometricMean(dims)
	if got != 0.0 {
		t.Errorf("expected 0 for negative score, got %f", got)
	}
}

// ---- Quorum tests ----

func TestRunQuorum_AllPass(t *testing.T) {
	judges := []VerifyResult{
		{Approved: true, Score: 0.9},
		{Approved: true, Score: 0.8},
		{Approved: true, Score: 0.85},
	}
	result := RunQuorum(QuorumConfig{K: 2, N: 3}, judges)
	if !result.Met {
		t.Errorf("expected quorum met: %s", result.Reason)
	}
	if result.Passed != 3 {
		t.Errorf("expected 3 passing, got %d", result.Passed)
	}
}

func TestRunQuorum_BelowThreshold(t *testing.T) {
	judges := []VerifyResult{
		{Approved: true, Score: 0.9},
		{Approved: false, Score: 0.3},
		{Approved: false, Score: 0.2},
	}
	result := RunQuorum(QuorumConfig{K: 2, N: 3}, judges)
	if result.Met {
		t.Error("expected quorum NOT met (only 1 of 3 passed, need 2)")
	}
}

func TestRunQuorum_ExactlyK(t *testing.T) {
	judges := []VerifyResult{
		{Approved: true, Score: 0.75},
		{Approved: true, Score: 0.80},
		{Approved: false, Score: 0.4},
	}
	result := RunQuorum(QuorumConfig{K: 2, N: 3}, judges)
	if !result.Met {
		t.Error("expected quorum met with exactly K=2 passing")
	}
}

func TestRunQuorum_ConfidenceIsMeanOfPassing(t *testing.T) {
	judges := []VerifyResult{
		{Approved: true, Score: 0.8},
		{Approved: true, Score: 0.6},
		{Approved: false, Score: 0.2},
	}
	result := RunQuorum(QuorumConfig{K: 2, N: 3}, judges)
	want := (0.8 + 0.6) / 2 // 0.7
	if result.Confidence < 0.699 || result.Confidence > 0.701 {
		t.Errorf("expected confidence ~0.7, got %f", result.Confidence)
	}
	_ = want
}

func TestRunQuorum_ReasonMentionsKN(t *testing.T) {
	judges := []VerifyResult{{Approved: true, Score: 0.9}}
	result := RunQuorum(QuorumConfig{K: 1, N: 1}, judges)
	if !strings.Contains(result.Reason, "1/1") {
		t.Errorf("reason should mention 1/1, got: %s", result.Reason)
	}
}

// ---- ReviewPanel tests ----

func TestBuildReviewPrompt_ContainsGoal(t *testing.T) {
	p := BuildReviewPrompt("add pagination", "done", nil)
	if !strings.Contains(p, "add pagination") {
		t.Error("review prompt should contain the goal")
	}
}

func TestBuildReviewPrompt_ContainsDimensions(t *testing.T) {
	p := BuildReviewPrompt("goal", "output", nil)
	for _, dim := range []string{"correctness", "completeness", "test_quality", "regression_risk"} {
		if !strings.Contains(p, dim) {
			t.Errorf("review prompt missing dimension: %s", dim)
		}
	}
}

func TestBuildReviewPrompt_ThreadsPriorFindings(t *testing.T) {
	findings := []string{"missing error handling", "test deleted"}
	p := BuildReviewPrompt("goal", "output", findings)
	if !strings.Contains(p, "missing error handling") {
		t.Error("prior findings should appear in review prompt")
	}
	if !strings.Contains(p, "PRIOR REVIEW FINDINGS") {
		t.Error("prior findings section header missing")
	}
}

func TestBuildReviewPrompt_NoPriorFindingsSection(t *testing.T) {
	p := BuildReviewPrompt("goal", "output", nil)
	if strings.Contains(p, "PRIOR REVIEW FINDINGS") {
		t.Error("should not include prior findings section when none provided")
	}
}

func TestParseReviewResponse_Pass(t *testing.T) {
	response := `REVIEW: PASS
SCORE: 0.92
EVIDENCE: all acceptance criteria met and tested
FINDINGS:`
	result := ParseReviewResponse(response)
	if !result.Pass {
		t.Error("expected Pass=true")
	}
	if result.Score < 0.91 || result.Score > 0.93 {
		t.Errorf("expected score ~0.92, got %f", result.Score)
	}
}

func TestParseReviewResponse_Fail(t *testing.T) {
	response := `REVIEW: FAIL
SCORE: 0.4
EVIDENCE: pagination skips page 0
FINDINGS:
- page 0 returns 404 instead of empty list
- no test for edge case`
	result := ParseReviewResponse(response)
	if result.Pass {
		t.Error("expected Pass=false")
	}
	if len(result.Findings) != 2 {
		t.Errorf("expected 2 findings, got %d", len(result.Findings))
	}
}

func TestDefaultReviewPanel_MaxRestarts(t *testing.T) {
	rp := DefaultReviewPanel()
	if rp.maxRestarts() != 3 {
		t.Errorf("expected default MaxRestarts=3, got %d", rp.maxRestarts())
	}
}

func TestReviewPanel_ZeroMaxRestartsDefaults(t *testing.T) {
	rp := ReviewPanel{MaxRestarts: 0}
	if rp.maxRestarts() != 3 {
		t.Errorf("zero MaxRestarts should default to 3, got %d", rp.maxRestarts())
	}
}

// ---- GroundingBlock tests ----

func TestGroundingBlock_ReturnsStringOrEmpty(t *testing.T) {
	// Run from the actual repo dir — has commits.
	block, err := GroundingBlock("../..", 5)
	if err != nil {
		t.Fatalf("GroundingBlock error: %v", err)
	}
	// If the repo has commits (it does), should return a non-empty block.
	if block == "" {
		t.Error("expected non-empty grounding block from a repo with commits")
	}
}

func TestGroundingBlock_ContainsHeader(t *testing.T) {
	block, err := GroundingBlock("../..", 3)
	if err != nil {
		t.Fatalf("GroundingBlock error: %v", err)
	}
	if !strings.Contains(block, "## Recent work") {
		t.Errorf("grounding block should contain '## Recent work', got:\n%s", block[:min(200, len(block))])
	}
}

func TestGroundingBlock_ZeroMaxDefaultsTen(t *testing.T) {
	// Just ensure it doesn't panic with max=0
	_, err := GroundingBlock("../..", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGroundingBlock_InvalidDir(t *testing.T) {
	// Non-git dir should return empty string, not error.
	block, err := GroundingBlock(t.TempDir(), 5)
	if err != nil {
		t.Fatalf("expected no error for non-git dir, got: %v", err)
	}
	if block != "" {
		t.Error("expected empty block for non-git dir")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
