//go:build !light_only

package loop

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ---- StallBrake tests ----

func TestStallBrake_NoStallWithDiverseActions(t *testing.T) {
	b := NewStallBrake(3)
	actions := []string{"write file.go", "run tests", "fix lint"}
	for _, a := range actions {
		if b.Record(a) {
			t.Errorf("unexpected stall after action %q", a)
		}
	}
}

func TestStallBrake_StallAfterPatientIdentical(t *testing.T) {
	b := NewStallBrake(3)
	b.Record("same action")
	b.Record("same action")
	if b.Record("same action") != true {
		t.Error("expected stall after 3 identical actions")
	}
}

func TestStallBrake_NoStallBelowPatience(t *testing.T) {
	b := NewStallBrake(3)
	b.Record("same")
	if b.Record("same") {
		t.Error("should not stall with only 2 entries (patience=3)")
	}
}

func TestStallBrake_ResetClearsHistory(t *testing.T) {
	b := NewStallBrake(3)
	b.Record("x")
	b.Record("x")
	b.Record("x") // stalled
	b.Reset()
	// After reset, same 3 identical must stall again from scratch
	b.Record("x")
	b.Record("x")
	if b.Record("x") != true {
		t.Error("expected stall again after reset + 3 identical")
	}
}

func TestStallBrake_DifferentActionBreaksStreak(t *testing.T) {
	b := NewStallBrake(3)
	b.Record("same")
	b.Record("same")
	b.Record("different") // breaks streak
	if b.Record("same") {
		t.Error("streak was broken; should not stall")
	}
}

func TestStallBrake_DefaultPatience(t *testing.T) {
	b := NewStallBrake(0) // 0 → defaults to 3
	if b.Patience() != 3 {
		t.Errorf("expected default patience 3, got %d", b.Patience())
	}
}

func TestStallBrake_PatternNotDependentOnContent(t *testing.T) {
	b := NewStallBrake(2)
	// Two different actions that hash identically won't happen, but test
	// that hash-equality is the comparison mechanism via Record returning bool.
	if b.Record("alpha") {
		t.Error("first record should never stall")
	}
	if b.Record("beta") {
		t.Error("two different actions should not stall")
	}
}

// ---- Pricing tests ----

func TestPriceFor_KnownModel(t *testing.T) {
	price, ok := PriceFor("claude-sonnet-4-6")
	if !ok {
		t.Fatal("claude-sonnet-4-6 should be in pricing table")
	}
	if price <= 0 {
		t.Errorf("expected positive price, got %f", price)
	}
}

func TestPriceFor_UnknownModel(t *testing.T) {
	_, ok := PriceFor("nonexistent-model-xyz")
	if ok {
		t.Error("unknown model should return ok=false")
	}
}

func TestKnownModels_NotEmpty(t *testing.T) {
	models := KnownModels()
	if len(models) == 0 {
		t.Error("KnownModels should return at least one entry")
	}
}

// ---- Budget time + cost tests ----

func TestBudget_CheckTime_NotExceeded(t *testing.T) {
	cfg := BudgetConfig{MaxTokens: 1000, MaxIter: 10, MaxDuration: 5 * time.Minute}
	b := NewBudget(cfg)
	exceeded, _ := b.CheckTime(time.Now())
	if exceeded {
		t.Error("should not exceed time limit immediately after creation")
	}
}

func TestBudget_CheckTime_Exceeded(t *testing.T) {
	cfg := BudgetConfig{MaxTokens: 1000, MaxIter: 10, MaxDuration: time.Millisecond}
	b := NewBudget(cfg)
	exceeded, elapsed := b.CheckTime(time.Now().Add(time.Second))
	if !exceeded {
		t.Errorf("should exceed 1ms limit after 1s; elapsed=%v", elapsed)
	}
}

func TestBudget_CheckTime_ZeroMeansUnlimited(t *testing.T) {
	cfg := BudgetConfig{MaxTokens: 1000, MaxIter: 10}
	b := NewBudget(cfg)
	exceeded, _ := b.CheckTime(time.Now().Add(24 * time.Hour))
	if exceeded {
		t.Error("zero MaxDuration should never exceed")
	}
}

func TestBudget_EstimatedCostUSD(t *testing.T) {
	cfg := BudgetConfig{MaxTokens: 100_000, MaxIter: 10, CostPer1K: 0.003}
	b := NewBudget(cfg)
	b.Consume(1000, PhasePlan)
	cost := b.EstimatedCostUSD()
	if cost != 0.003 {
		t.Errorf("expected $0.003 for 1K tokens at $0.003/1K, got %f", cost)
	}
}

func TestBudget_CheckCost_Exceeded(t *testing.T) {
	cfg := BudgetConfig{MaxTokens: 100_000, MaxIter: 10, CostPer1K: 0.01, MaxCostUSD: 0.005}
	b := NewBudget(cfg)
	b.Consume(1000, PhaseExecute) // $0.01 — exceeds $0.005 ceiling
	if !b.CheckCost() {
		t.Error("expected cost limit to be exceeded")
	}
}

func TestBudget_CheckCost_NotExceeded(t *testing.T) {
	cfg := BudgetConfig{MaxTokens: 100_000, MaxIter: 10, CostPer1K: 0.001, MaxCostUSD: 1.0}
	b := NewBudget(cfg)
	b.Consume(500, PhaseExecute) // $0.0005 — well under $1.00
	if b.CheckCost() {
		t.Error("cost should not be exceeded yet")
	}
}

func TestBudget_CheckCost_NoCostPer1K(t *testing.T) {
	cfg := BudgetConfig{MaxTokens: 1000, MaxIter: 10, MaxCostUSD: 1.0}
	b := NewBudget(cfg)
	b.Consume(999_999, PhaseExecute)
	if b.CheckCost() {
		t.Error("no CostPer1K → cost brake should never trigger")
	}
}

func TestBudget_SummaryCostAppended(t *testing.T) {
	cfg := BudgetConfig{MaxTokens: 10_000, MaxIter: 5, CostPer1K: 0.003, MaxCostUSD: 0.10}
	b := NewBudget(cfg)
	b.Consume(2000, PhaseExecute)
	s := b.Summary()
	if len(s) == 0 {
		t.Error("Summary should not be empty")
	}
	// must mention cost
	found := false
	for i := range s {
		if s[i] == '$' {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Summary should mention cost, got: %s", s)
	}
}

// ---- VerifyResult Escalate tests ----

func TestParseVerifyResponse_EscalateTrue(t *testing.T) {
	response := `VERDICT: REJECTED
SCORE: 0.2
EVIDENCE: test was deleted
ESCALATE: true
ISSUES:
- test file was removed`
	cfg := DefaultVerifierConfig()
	result := ParseVerifyResponse(response, cfg)
	if !result.Escalate {
		t.Error("expected Escalate=true")
	}
	if result.Approved {
		t.Error("should be rejected")
	}
}

func TestParseVerifyResponse_EscalateFalse(t *testing.T) {
	response := `VERDICT: REJECTED
SCORE: 0.4
EVIDENCE: missing error handling
ESCALATE: false
ISSUES:
- no error check on file open`
	cfg := DefaultVerifierConfig()
	result := ParseVerifyResponse(response, cfg)
	if result.Escalate {
		t.Error("expected Escalate=false")
	}
}

func TestParseVerifyResponse_EscalateMissing(t *testing.T) {
	response := `VERDICT: APPROVED
SCORE: 0.9
EVIDENCE: all tests pass`
	cfg := DefaultVerifierConfig()
	result := ParseVerifyResponse(response, cfg)
	if result.Escalate {
		t.Error("missing ESCALATE line should default to false")
	}
}

// ---- Inbox tests ----

func TestWriteAndListInboxItem(t *testing.T) {
	dir := t.TempDir()
	c := NewCycle(dir, "run-test-001", "add rate limiting", NewBudget(BudgetConfig{MaxIter: 5}))
	_ = c.Transition(PhaseDiscover, "")
	_ = c.Transition(PhasePlan, "")
	_ = c.Transition(PhaseExecute, "")
	_ = c.Transition(PhaseVerify, "")

	result := VerifyResult{
		Approved: false,
		Score:    0.1,
		Evidence: "test was deleted",
		Issues:   []string{"anti-cheat: test removed"},
		Escalate: true,
	}
	id, err := c.WriteInboxItem(result)
	if err != nil {
		t.Fatalf("WriteInboxItem: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty ID")
	}

	items, err := ListInboxItems(dir)
	if err != nil {
		t.Fatalf("ListInboxItems: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 inbox item, got %d", len(items))
	}
	if items[0].Evidence != result.Evidence {
		t.Errorf("evidence mismatch: got %q", items[0].Evidence)
	}
}

func TestResolveInboxItem(t *testing.T) {
	dir := t.TempDir()
	inboxDir := filepath.Join(dir, ".radiant-harness", "inbox")
	_ = os.MkdirAll(inboxDir, 0o755)
	_ = os.WriteFile(filepath.Join(inboxDir, "test-id.json"), []byte(`{"id":"test-id"}`), 0o644)

	if err := ResolveInboxItem(dir, "test-id"); err != nil {
		t.Fatalf("ResolveInboxItem: %v", err)
	}
	items, _ := ListInboxItems(dir)
	if len(items) != 0 {
		t.Error("expected inbox to be empty after resolve")
	}
}

func TestResolveInboxItem_NonExistent(t *testing.T) {
	dir := t.TempDir()
	// Should not error when item doesn't exist
	if err := ResolveInboxItem(dir, "ghost-id"); err != nil {
		t.Errorf("expected no error for non-existent item, got: %v", err)
	}
}

// ---- ExitReason completeness ----

func TestNewExitReasons_Defined(t *testing.T) {
	reasons := []ExitReason{
		ExitNeedsHuman,
		ExitStalled,
		ExitTimeLimitReached,
		ExitCostLimitReached,
	}
	for _, r := range reasons {
		if string(r) == "" {
			t.Errorf("ExitReason %v must not be empty", r)
		}
	}
}
