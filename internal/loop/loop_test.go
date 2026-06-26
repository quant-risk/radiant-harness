package loop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ── Budget tests ──────────────────────────────────────────────────────────────

func TestBudget_NewFromProfile(t *testing.T) {
	b := NewBudget(BudgetConfig{Profile: ProfileStandard})
	if b.MaxTokens() != 50_000 {
		t.Errorf("standard profile: MaxTokens = %d, want 50000", b.MaxTokens())
	}
	if b.MaxIter() <= 0 {
		t.Errorf("MaxIter should be positive, got %d", b.MaxIter())
	}
}

func TestBudget_NewFromExplicit(t *testing.T) {
	b := NewBudget(BudgetConfig{MaxTokens: 1000, MaxIter: 5})
	if b.MaxTokens() != 1000 {
		t.Errorf("MaxTokens = %d, want 1000", b.MaxTokens())
	}
	if b.MaxIter() != 5 {
		t.Errorf("MaxIter = %d, want 5", b.MaxIter())
	}
}

func TestBudget_ConsumeAndStatus(t *testing.T) {
	b := NewBudget(BudgetConfig{MaxTokens: 1000, MaxIter: 10, WarnRatio: 0.7})

	if b.Status() != BudgetOK {
		t.Errorf("fresh budget should be OK, got %s", b.Status())
	}

	b.Consume(750, PhaseExecute)
	if b.Status() != BudgetWarning {
		t.Errorf("at 75%% should be Warning, got %s", b.Status())
	}

	b.Consume(300, PhaseVerify)
	if b.Status() != BudgetExceeded {
		t.Errorf("at 105%% should be Exceeded, got %s", b.Status())
	}
}

func TestBudget_IterLimit(t *testing.T) {
	b := NewBudget(BudgetConfig{MaxTokens: 100_000, MaxIter: 3})
	for i := 0; i < 3; i++ {
		b.IncrIter()
	}
	if b.Status() != BudgetExceeded {
		t.Errorf("after max iterations should be Exceeded, got %s", b.Status())
	}
}

func TestBudget_Remaining(t *testing.T) {
	b := NewBudget(BudgetConfig{MaxTokens: 1000})
	b.Consume(300, PhaseDiscover)
	rem := b.Remaining()
	if rem != 700 {
		t.Errorf("remaining = %d, want 700", rem)
	}
}

func TestBudget_UnlimitedRemaining(t *testing.T) {
	b := NewBudget(BudgetConfig{MaxIter: 5}) // no token limit
	if b.Remaining() != -1 {
		t.Errorf("unlimited budget: Remaining() should be -1, got %d", b.Remaining())
	}
}

func TestBudget_PhaseBreakdown(t *testing.T) {
	b := NewBudget(BudgetConfig{MaxTokens: 10_000})
	b.Consume(100, PhaseDiscover)
	b.Consume(500, PhaseExecute)
	b.Consume(200, PhaseVerify)

	bd := b.PhaseBreakdown()
	if bd[PhaseDiscover] != 100 {
		t.Errorf("Discover tokens = %d, want 100", bd[PhaseDiscover])
	}
	if bd[PhaseExecute] != 500 {
		t.Errorf("Execute tokens = %d, want 500", bd[PhaseExecute])
	}
}

func TestBudget_Snapshot(t *testing.T) {
	b := NewBudget(BudgetConfig{MaxTokens: 5000, MaxIter: 10})
	b.Consume(1000, PhasePlan)

	snap := b.Snapshot()
	if snap.UsedTokens != 1000 {
		t.Errorf("snapshot UsedTokens = %d, want 1000", snap.UsedTokens)
	}
	if snap.MaxTokens != 5000 {
		t.Errorf("snapshot MaxTokens = %d, want 5000", snap.MaxTokens)
	}
}

func TestBudget_Summary(t *testing.T) {
	b := NewBudget(BudgetConfig{MaxTokens: 1000, MaxIter: 5})
	b.Consume(200, PhaseExecute)
	b.IncrIter()

	s := b.Summary()
	if !strings.Contains(s, "200") {
		t.Errorf("summary should contain used tokens, got: %s", s)
	}
	if !strings.Contains(s, "ok") {
		t.Errorf("summary should contain status, got: %s", s)
	}
}

// ── Cycle tests ───────────────────────────────────────────────────────────────

func TestCycle_NewAndTransition(t *testing.T) {
	dir := t.TempDir()
	b := NewBudget(BudgetConfig{MaxTokens: 50_000, MaxIter: 10})
	c := NewCycle(dir, "run-001", "implement login", b)

	if c.State().Phase != PhaseIdle {
		t.Errorf("initial phase = %s, want idle", c.State().Phase)
	}

	if err := c.Transition(PhaseDiscover, "starting"); err != nil {
		t.Fatal(err)
	}
	if c.State().Phase != PhaseDiscover {
		t.Errorf("phase = %s, want discover", c.State().Phase)
	}
}

func TestCycle_InvalidTransition(t *testing.T) {
	dir := t.TempDir()
	b := NewBudget(BudgetConfig{MaxTokens: 50_000})
	c := NewCycle(dir, "run-002", "goal", b)

	// idle → execute is invalid (must go through discover→plan first)
	err := c.Transition(PhaseExecute, "skip")
	if err == nil {
		t.Error("expected error for invalid transition idle → execute")
	}
}

func TestCycle_Persistence(t *testing.T) {
	dir := t.TempDir()
	b := NewBudget(BudgetConfig{MaxTokens: 50_000, MaxIter: 5})
	c := NewCycle(dir, "run-003", "my goal", b)

	if err := c.Transition(PhaseDiscover, "start"); err != nil {
		t.Fatal(err)
	}
	if err := c.Transition(PhasePlan, "discovered"); err != nil {
		t.Fatal(err)
	}

	// Loop.json must exist
	loopJSON := filepath.Join(dir, ".radiant-harness", "loop.json")
	if _, err := os.Stat(loopJSON); os.IsNotExist(err) {
		t.Fatal("loop.json not written")
	}

	// Load from disk
	c2, err := LoadCycle(dir)
	if err != nil {
		t.Fatal(err)
	}
	if c2.State().Phase != PhasePlan {
		t.Errorf("loaded phase = %s, want plan", c2.State().Phase)
	}
	if c2.State().Goal != "my goal" {
		t.Errorf("loaded goal = %q, want %q", c2.State().Goal, "my goal")
	}
}

func TestCycle_LoadMissing(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadCycle(dir)
	if err == nil {
		t.Error("expected error loading cycle from empty dir")
	}
}

func TestCycle_ConsecFailures(t *testing.T) {
	dir := t.TempDir()
	b := NewBudget(BudgetConfig{MaxTokens: 50_000, MaxIter: 20})
	c := NewCycle(dir, "run-004", "goal", b)

	c.Transition(PhaseDiscover, "")
	c.Transition(PhasePlan, "")
	c.Transition(PhaseExecute, "")
	c.Transition(PhaseFailed, "first fail")
	c.Transition(PhaseDiscover, "retry")
	c.Transition(PhasePlan, "")
	c.Transition(PhaseExecute, "")
	c.Transition(PhaseFailed, "second fail")
	c.Transition(PhaseDiscover, "retry")
	c.Transition(PhasePlan, "")
	c.Transition(PhaseExecute, "")
	c.Transition(PhaseFailed, "third fail")

	ok, reason := c.ShouldContinue(b)
	if ok {
		t.Error("should NOT continue after 3 consecutive failures")
	}
	if reason != ExitCritical {
		t.Errorf("exit reason = %s, want critical_failure", reason)
	}
}

func TestCycle_ShouldContinue_BudgetExceeded(t *testing.T) {
	dir := t.TempDir()
	b := NewBudget(BudgetConfig{MaxTokens: 100, MaxIter: 10})
	b.Consume(101, PhaseExecute) // exceed
	c := NewCycle(dir, "run-005", "goal", b)

	ok, reason := c.ShouldContinue(b)
	if ok {
		t.Error("should NOT continue when budget exceeded")
	}
	if reason != ExitBudget {
		t.Errorf("exit reason = %s, want budget_exhausted", reason)
	}
}

func TestCycle_ShouldContinue_MaxIter(t *testing.T) {
	dir := t.TempDir()
	b := NewBudget(BudgetConfig{MaxTokens: 100_000, MaxIter: 3})
	for i := 0; i < 3; i++ {
		b.IncrIter()
	}
	c := NewCycle(dir, "run-006", "goal", b)

	ok, reason := c.ShouldContinue(b)
	if ok {
		t.Error("should NOT continue when max iter reached")
	}
	if reason != ExitBudget {
		t.Errorf("exit reason = %s, want budget_exhausted (iter limit)", reason)
	}
}

func TestCycle_SetExit(t *testing.T) {
	dir := t.TempDir()
	b := NewBudget(BudgetConfig{MaxTokens: 50_000})
	c := NewCycle(dir, "run-007", "goal", b)

	c.Transition(PhaseDiscover, "")
	c.Transition(PhasePlan, "")
	c.Transition(PhaseExecute, "")
	c.Transition(PhaseVerify, "")
	c.Transition(PhasePersist, "done")
	c.SetExit(ExitSuccess, "all tests pass")

	state := c.State()
	if state.ExitReason != ExitSuccess {
		t.Errorf("exit reason = %s, want success", state.ExitReason)
	}
}

func TestCycle_LogPreservation(t *testing.T) {
	dir := t.TempDir()
	b := NewBudget(BudgetConfig{MaxTokens: 50_000})
	c := NewCycle(dir, "run-008", "goal", b)

	c.Transition(PhaseDiscover, "note1")
	c.Transition(PhasePlan, "note2")

	state := c.State()
	if len(state.Log) < 2 {
		t.Errorf("expected ≥2 log entries, got %d", len(state.Log))
	}
}

func TestCycle_FormatStatus(t *testing.T) {
	dir := t.TempDir()
	b := NewBudget(BudgetConfig{MaxTokens: 50_000, MaxIter: 10})
	c := NewCycle(dir, "run-test", "build login feature", b)
	c.Transition(PhaseDiscover, "detecting project")

	status := FormatStatus(c.State())
	if !strings.Contains(status, "run-test") {
		t.Errorf("status should contain run ID, got:\n%s", status)
	}
	if !strings.Contains(status, "build login feature") {
		t.Errorf("status should contain goal, got:\n%s", status)
	}
	if !strings.Contains(status, "discover") {
		t.Errorf("status should contain current phase, got:\n%s", status)
	}
}

func TestCycle_FormatStatus_Empty(t *testing.T) {
	status := FormatStatus(LoopState{})
	if !strings.Contains(status, "No active loop") {
		t.Errorf("empty state should say no active loop, got: %s", status)
	}
}

// ── Trace tests ───────────────────────────────────────────────────────────────

func TestTracer_RecordAndRead(t *testing.T) {
	dir := t.TempDir()
	tracer, err := NewTracer(dir, "run-trace-001")
	if err != nil {
		t.Fatal(err)
	}
	defer tracer.Close()

	events := []TraceEvent{
		{Phase: PhaseExecute, Action: "write_file", Result: "ok", Evidence: "file written"},
		{Phase: PhaseVerify, Action: "review", Result: "ok", Evidence: "tests pass"},
		{Phase: PhaseFailed, Action: "gate", Result: "failed", Evidence: "lint errors"},
	}
	for _, e := range events {
		if err := tracer.Record(e); err != nil {
			t.Fatalf("record: %v", err)
		}
	}
	tracer.Close()

	read, err := ReadTrace(tracer.Path())
	if err != nil {
		t.Fatal(err)
	}
	if len(read) != 3 {
		t.Errorf("expected 3 events, got %d", len(read))
	}
	if read[0].Action != "write_file" {
		t.Errorf("first event action = %q, want write_file", read[0].Action)
	}
	if read[0].RunID != "run-trace-001" {
		t.Errorf("RunID = %q, want run-trace-001", read[0].RunID)
	}
}

func TestTracer_JSONValidity(t *testing.T) {
	dir := t.TempDir()
	tracer, err := NewTracer(dir, "run-json")
	if err != nil {
		t.Fatal(err)
	}
	defer tracer.Close()

	tracer.Record(TraceEvent{
		Phase:    PhaseExecute,
		Action:   "test",
		Result:   "ok",
		TokensIn: 100, TokensOut: 50,
		Meta: map[string]string{"key": "value"},
	})
	tracer.Close()

	data, _ := os.ReadFile(tracer.Path())
	var event TraceEvent
	if err := json.Unmarshal(data[:len(data)-1], &event); err != nil {
		t.Errorf("trace line is not valid JSON: %v", err)
	}
}

func TestTracer_TimestampSet(t *testing.T) {
	dir := t.TempDir()
	before := time.Now().UTC()
	tracer, _ := NewTracer(dir, "run-ts")
	defer tracer.Close()
	tracer.Record(TraceEvent{Phase: PhaseExecute, Action: "a", Result: "ok"})
	tracer.Close()
	after := time.Now().UTC()

	events, _ := ReadTrace(tracer.Path())
	if len(events) == 0 {
		t.Fatal("no events")
	}
	ts := events[0].Timestamp
	if ts.Before(before) || ts.After(after) {
		t.Errorf("timestamp %v outside expected range [%v, %v]", ts, before, after)
	}
}

func TestListTraces(t *testing.T) {
	dir := t.TempDir()
	for _, id := range []string{"run-a", "run-b", "run-c"} {
		tr, _ := NewTracer(dir, id)
		tr.Close()
	}

	ids, err := ListTraces(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 3 {
		t.Errorf("expected 3 traces, got %d", len(ids))
	}
}

func TestListTraces_Empty(t *testing.T) {
	dir := t.TempDir()
	ids, err := ListTraces(dir)
	if err != nil {
		t.Fatal(err)
	}
	if ids != nil {
		t.Errorf("expected nil for empty dir, got %v", ids)
	}
}

func TestFormatTrace(t *testing.T) {
	events := []TraceEvent{
		{Timestamp: time.Now(), Phase: PhaseExecute, Action: "write", Result: "ok"},
		{Timestamp: time.Now(), Phase: PhaseVerify, Action: "review", Result: "failed", Evidence: "test failed"},
	}
	out := FormatTrace(events)
	if !strings.Contains(out, "✓") {
		t.Errorf("approved event should show ✓, got:\n%s", out)
	}
	if !strings.Contains(out, "✗") {
		t.Errorf("failed event should show ✗, got:\n%s", out)
	}
	if !strings.Contains(out, "test failed") {
		t.Errorf("evidence should be included, got:\n%s", out)
	}
}

func TestFormatTrace_Empty(t *testing.T) {
	out := FormatTrace(nil)
	if !strings.Contains(out, "empty") {
		t.Errorf("empty trace should say 'empty', got: %s", out)
	}
}

// ── Verifier tests ────────────────────────────────────────────────────────────

func TestBuildVerifierPrompt_ContainsGoal(t *testing.T) {
	cfg := DefaultVerifierConfig()
	prompt := BuildVerifierPrompt("implement login API", "output here", cfg)
	if !strings.Contains(prompt, "implement login API") {
		t.Error("prompt should contain the goal")
	}
	if !strings.Contains(prompt, "output here") {
		t.Error("prompt should contain executor output")
	}
}

func TestBuildVerifierPrompt_StrictMode(t *testing.T) {
	strict := DefaultVerifierConfig()
	strict.StrictMode = true
	prompt := BuildVerifierPrompt("goal", "output", strict)
	if !strings.Contains(prompt, "Default to REJECTED") {
		t.Error("strict mode prompt should contain 'Default to REJECTED'")
	}
}

func TestParseVerifyResponse_Approved(t *testing.T) {
	cfg := DefaultVerifierConfig()
	response := `VERDICT: APPROVED
SCORE: 0.92
EVIDENCE: All 4 acceptance criteria are covered by passing tests
ISSUES:`

	result := ParseVerifyResponse(response, cfg)
	if !result.Approved {
		t.Error("expected approved verdict")
	}
	if result.Score < 0.9 {
		t.Errorf("score = %.2f, want ≥0.90", result.Score)
	}
	if result.Evidence == "" {
		t.Error("evidence should be set")
	}
}

func TestParseVerifyResponse_Rejected(t *testing.T) {
	cfg := DefaultVerifierConfig()
	response := `VERDICT: REJECTED
SCORE: 0.30
EVIDENCE: Tests pass but only cover happy path
ISSUES:
- Missing error handling for invalid input
- No test for concurrent access`

	result := ParseVerifyResponse(response, cfg)
	if result.Approved {
		t.Error("expected rejected verdict")
	}
	if len(result.Issues) != 2 {
		t.Errorf("expected 2 issues, got %d: %v", len(result.Issues), result.Issues)
	}
}

func TestParseVerifyResponse_ScoreBelowThreshold(t *testing.T) {
	cfg := DefaultVerifierConfig()
	cfg.MinScore = 0.80
	response := `VERDICT: APPROVED
SCORE: 0.65
EVIDENCE: Mostly works
ISSUES:`

	result := ParseVerifyResponse(response, cfg)
	if result.Approved {
		t.Error("approved with score below threshold should be forced to rejected")
	}
}

func TestParseVerifyResponse_MalformedDefault(t *testing.T) {
	cfg := DefaultVerifierConfig()
	cfg.StrictMode = true
	response := "I think it looks good but I'm not sure"

	result := ParseVerifyResponse(response, cfg)
	if result.Approved {
		t.Error("malformed response in strict mode should default to rejected")
	}
}

func TestShouldRetry(t *testing.T) {
	// Should retry on rejection with issues
	rejected := VerifyResult{Approved: false, Issues: []string{"missing test"}}
	if !ShouldRetry(rejected) {
		t.Error("should retry when rejected with issues")
	}

	// Should NOT retry on approval
	approved := VerifyResult{Approved: true}
	if ShouldRetry(approved) {
		t.Error("should NOT retry when approved")
	}
}

func TestFormatVerifyResult(t *testing.T) {
	result := VerifyResult{
		Approved: false,
		Score:    0.45,
		Evidence: "tests fail",
		Issues:   []string{"no coverage", "lint errors"},
	}
	out := FormatVerifyResult(result)
	if !strings.Contains(out, "REJECTED") {
		t.Errorf("expected REJECTED in output, got: %s", out)
	}
	if !strings.Contains(out, "no coverage") {
		t.Errorf("expected issue in output, got: %s", out)
	}
}
