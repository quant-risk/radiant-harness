//go:build with_full

package loop

import (
	"strings"
	"testing"
	"time"

	"github.com/quant-risk/radiant-harness/internal/llm"
)

// ── RunConfig defaults ────────────────────────────────────────────────────────

func TestRunConfigZeroVerifierUsesDefault(t *testing.T) {
	cfg := RunConfig{}
	// Verifier with zero MinScore should trigger DefaultVerifierConfig inside Run.
	// We test this indirectly: build the verifier config as Run would.
	verCfg := cfg.Verifier
	if verCfg.MinScore == 0 {
		verCfg = DefaultVerifierConfig()
		verCfg.Quorum = cfg.Verifier.Quorum
	}
	if verCfg.MinScore <= 0 {
		t.Fatal("expected positive MinScore from default")
	}
}

func TestRunConfigVerifierPreservedWhenNonZero(t *testing.T) {
	cfg := RunConfig{Verifier: VerifierConfig{MinScore: 0.9}}
	verCfg := cfg.Verifier
	if verCfg.MinScore == 0 {
		verCfg = DefaultVerifierConfig()
	}
	if verCfg.MinScore != 0.9 {
		t.Fatalf("expected 0.9, got %f", verCfg.MinScore)
	}
}

// ── ReviewPanel.maxRestarts ───────────────────────────────────────────────────

func TestReviewPanelMaxRestartsDefault(t *testing.T) {
	rp := ReviewPanel{}
	if rp.maxRestarts() != DefaultReviewPanel().MaxRestarts {
		t.Fatalf("expected default %d, got %d", DefaultReviewPanel().MaxRestarts, rp.maxRestarts())
	}
}

func TestReviewPanelMaxRestartsCustom(t *testing.T) {
	rp := ReviewPanel{MaxRestarts: 7}
	if rp.maxRestarts() != 7 {
		t.Fatalf("expected 7, got %d", rp.maxRestarts())
	}
}

// ── estimateTokens ────────────────────────────────────────────────────────────

func TestEstimateTokensEmpty(t *testing.T) {
	if n := estimateTokens("", ""); n != 0 {
		t.Fatalf("expected 0, got %d", n)
	}
}

func TestEstimateTokensApprox(t *testing.T) {
	// 400 ASCII runes → (400*10+34)/35 = 115 tokens (≈3.5 chars/token)
	prompt := strings.Repeat("a", 200)
	resp := strings.Repeat("b", 200)
	if n := estimateTokens(prompt, resp); n != 115 {
		t.Fatalf("expected 115, got %d", n)
	}
}

func TestEstimateTokensRounding(t *testing.T) {
	// 5 runes → (5*10+34)/35 = 84/35 = 2
	if n := estimateTokens("hello", ""); n != 2 {
		t.Fatalf("expected 2, got %d", n)
	}
}

// ── StallBrake nil-safe reset ─────────────────────────────────────────────────

func TestStallBrakeNilReset(t *testing.T) {
	var s *StallBrake
	// Should not panic.
	s.reset()
}

func TestStallBrakeResetClearsState(t *testing.T) {
	s := NewStallBrake(2)
	s.Record("action-a")
	s.Record("action-a")
	// Should have stalled; reset clears it.
	s.reset()
	// After reset, first record should not trigger stall.
	if stalled := s.Record("action-a"); stalled {
		t.Fatal("expected no stall immediately after reset")
	}
}

// ── buildExecutorPrompt ───────────────────────────────────────────────────────

func TestBuildExecutorPromptGoalOnly(t *testing.T) {
	out := buildExecutorPrompt("fix the bug", "", "", nil)
	if !strings.Contains(out, "fix the bug") {
		t.Fatal("goal missing from prompt")
	}
	if strings.Contains(out, "GROUNDING") {
		t.Fatal("unexpected grounding block")
	}
}

func TestBuildExecutorPromptWithGround(t *testing.T) {
	out := buildExecutorPrompt("fix the bug", "## Grounding\n- commit abc", "", nil)
	if !strings.Contains(out, "commit abc") {
		t.Fatal("grounding block missing")
	}
}

func TestBuildExecutorPromptWithFindings(t *testing.T) {
	findings := []string{"missing test", "wrong return type"}
	out := buildExecutorPrompt("fix the bug", "", "", findings)
	if !strings.Contains(out, "missing test") {
		t.Fatal("finding 1 missing")
	}
	if !strings.Contains(out, "wrong return type") {
		t.Fatal("finding 2 missing")
	}
}

func TestBuildExecutorPromptNoFindingsSection(t *testing.T) {
	out := buildExecutorPrompt("goal", "", "", nil)
	if strings.Contains(out, "PRIOR REVIEW") {
		t.Fatal("unexpected review section when findings is nil")
	}
}

func TestBuildExecutorPromptEmptyFindingsNoSection(t *testing.T) {
	out := buildExecutorPrompt("goal", "", "", []string{})
	if strings.Contains(out, "PRIOR REVIEW") {
		t.Fatal("unexpected review section when findings is empty")
	}
}

// ── system prompt content ─────────────────────────────────────────────────────

func TestExecutorSystemPromptNotEmpty(t *testing.T) {
	if executorSystemPrompt("") == "" {
		t.Fatal("executor system prompt is empty")
	}
}

func TestVerifierSystemPromptNotEmpty(t *testing.T) {
	if verifierSystemPrompt() == "" {
		t.Fatal("verifier system prompt is empty")
	}
}

func TestReviewerSystemPromptNotEmpty(t *testing.T) {
	if reviewerSystemPrompt() == "" {
		t.Fatal("reviewer system prompt is empty")
	}
}

func TestVerifierSystemPromptDefaultsRejected(t *testing.T) {
	if !strings.Contains(verifierSystemPrompt(), "REJECTED") {
		t.Fatal("verifier prompt must mention REJECTED as default stance")
	}
}

// ── buildResult ───────────────────────────────────────────────────────────────

func TestBuildResultFields(t *testing.T) {
	b := NewBudget(BudgetConfig{MaxTokens: 1000, MaxIter: 5})
	c := NewCycle(t.TempDir(), "run-test", "my goal", b)
	_ = c.Transition(PhaseDiscover, "start")
	started := time.Now().Add(-2 * time.Second)
	r := buildResult("run-test", "my goal", ExitSuccess, c, b, started)

	if r.RunID != "run-test" {
		t.Errorf("RunID wrong: %s", r.RunID)
	}
	if r.Goal != "my goal" {
		t.Errorf("Goal wrong: %s", r.Goal)
	}
	if r.ExitReason != ExitSuccess {
		t.Errorf("ExitReason wrong: %s", r.ExitReason)
	}
	if r.Elapsed < 2*time.Second {
		t.Errorf("Elapsed too short: %v", r.Elapsed)
	}
}

// ── RunConfig + llm.Model zero-value ─────────────────────────────────────────

func TestRunConfigVerifierModelFallback(t *testing.T) {
	cfg := RunConfig{
		ExecutorModel: llm.Model{Model: "claude-sonnet-4-6"},
		// VerifierModel intentionally left zero
	}
	verModel := cfg.VerifierModel
	if verModel.Model == "" {
		verModel = cfg.ExecutorModel
	}
	if verModel.Model != "claude-sonnet-4-6" {
		t.Fatalf("verifier fallback failed: %s", verModel.Model)
	}
}

func TestRunConfigVerifierModelExplicit(t *testing.T) {
	cfg := RunConfig{
		ExecutorModel: llm.Model{Model: "claude-haiku-4-5"},
		VerifierModel: llm.Model{Model: "claude-opus-4-8"},
	}
	verModel := cfg.VerifierModel
	if verModel.Model == "" {
		verModel = cfg.ExecutorModel
	}
	if verModel.Model != "claude-opus-4-8" {
		t.Fatalf("expected opus, got %s", verModel.Model)
	}
}
