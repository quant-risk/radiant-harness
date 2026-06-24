package harness

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	radiant "github.com/quant-risk/radiant-harness/internal"
)

// ── Context Window Edge Cases ──

func TestContextWindowExactBoundary(t *testing.T) {
	cw := NewContextWindow(100)

	// Exactly at smart zone boundary (40%)
	cw.AddTokens(40)
	if cw.IsSmartZone() {
		t.Error("40/100 is at boundary, should NOT be in smart zone")
	}

	// One token over
	cw.AddTokens(1)
	if cw.IsSmartZone() {
		t.Error("41/100 should exceed smart zone")
	}
}

func TestContextWindowExactDumbBoundary(t *testing.T) {
	cw := NewContextWindow(100)

	// Exactly at dumb zone boundary (60%)
	cw.AddTokens(60)
	if cw.IsDumbZone() {
		t.Error("60/100 should be at dumb zone boundary (not exceeding)")
	}

	// One token over
	cw.AddTokens(1)
	if !cw.IsDumbZone() {
		t.Error("61/100 should be in dumb zone")
	}
}

func TestContextWindowZeroMax(t *testing.T) {
	cw := NewContextWindow(0)
	if cw.Usage() != 0 {
		t.Error("zero max should have zero usage")
	}
	if !cw.IsSmartZone() {
		t.Error("zero max should be in smart zone")
	}
	if cw.IsDumbZone() {
		t.Error("zero max should not be in dumb zone")
	}
}

func TestContextWindowOverflow(t *testing.T) {
	cw := NewContextWindow(100)
	cw.AddTokens(200) // Over max
	if cw.Usage() <= 1.0 {
		t.Error("usage should exceed 1.0 when over max")
	}
	if !cw.IsDumbZone() {
		t.Error("over max should be in dumb zone")
	}
}

func TestContextWindowReset(t *testing.T) {
	cw := NewContextWindow(100)
	cw.AddTokens(50)
	cw.Reset()
	if cw.UsedTokens != 0 {
		t.Error("reset should zero tokens")
	}
}

func TestContextWindowShouldSplit(t *testing.T) {
	cw := NewContextWindow(100)
	cw.AddTokens(30) // 30%

	// Adding 5 more = 35% < 40% → should not split
	if cw.ShouldSplit(5) {
		t.Error("35% should not trigger split")
	}

	// Adding 15 more = 45% > 40% → should split
	if !cw.ShouldSplit(15) {
		t.Error("45% should trigger split")
	}
}

// ── RPI Budget Edge Cases ──

func TestRPIBudgetPhaseIsolation(t *testing.T) {
	budget := NewRPIBudget(100000)

	// Fill research phase
	budget.Research.AddTokens(30000) // 100% of research
	if !budget.Research.IsDumbZone() {
		t.Error("research should be full")
	}

	// Plan and implement should be unaffected
	if !budget.Plan.IsSmartZone() {
		t.Error("plan should be empty")
	}
	if !budget.Implement.IsSmartZone() {
		t.Error("implement should be empty")
	}
}

func TestRPIBudgetTotalTracking(t *testing.T) {
	budget := NewRPIBudget(100000)

	budget.Research.AddTokens(10000)
	budget.Plan.AddTokens(5000)
	budget.Implement.AddTokens(20000)

	budget.Total.AddTokens(10000 + 5000 + 20000)

	if budget.Total.UsedTokens != 35000 {
		t.Errorf("expected 35000, got %d", budget.Total.UsedTokens)
	}
}

// ── Token Estimator Edge Cases ──

func TestTokenEstimatorEmpty(t *testing.T) {
	te := NewTokenEstimator()
	if te.EstimateTokens("") != 0 {
		t.Error("empty text should be 0 tokens")
	}
}

func TestTokenEstimatorSingleWord(t *testing.T) {
	te := NewTokenEstimator()
	tokens := te.EstimateTokens("hello")
	if tokens < 1 || tokens > 3 {
		t.Errorf("single word should be 1-3 tokens, got %d", tokens)
	}
}

func TestTokenEstimatorCodeVsProse(t *testing.T) {
	te := NewTokenEstimator()
	code := "func calculateTotalPrice(items []Item) float64 {"
	prose := "The system calculates the total price for all items"

	codeTokens := te.EstimateTokens(code)
	proseTokens := te.EstimateTokens(prose)

	// Code should have more tokens due to special chars and identifiers
	if codeTokens < proseTokens {
		t.Errorf("code (%d) should have >= tokens than prose (%d)", codeTokens, proseTokens)
	}
}

func TestTokenEstimatorCJK(t *testing.T) {
	te := NewTokenEstimator()
	cjk := "这是一个测试文本"
	tokens := te.EstimateTokens(cjk)
	if tokens < 4 {
		t.Errorf("CJK text should have reasonable tokens, got %d", tokens)
	}
}

func TestTokenEstimatorMultiline(t *testing.T) {
	te := NewTokenEstimator()
	text := "line one\nline two\nline three\n"
	tokens := te.EstimateTokens(text)
	if tokens < 6 {
		t.Errorf("multiline should have reasonable tokens, got %d", tokens)
	}
}

// ── State Machine Edge Cases ──

func TestStateTransitionChain(t *testing.T) {
	dir := t.TempDir()
	state := NewState(dir)

	// Valid chain: idle → implement → validate → done
	transitions := []radiant.HarnessState{
		radiant.StateImplement,
		radiant.StateValidate,
		radiant.StateDone,
	}

	for _, target := range transitions {
		if err := state.Transition(target); err != nil {
			t.Errorf("transition to %s failed: %v", target, err)
		}
	}

	if state.CurrentState() != radiant.StateDone {
		t.Errorf("expected done, got %s", state.CurrentState())
	}
}

func TestStateTransitionRetryLoop(t *testing.T) {
	dir := t.TempDir()
	state := NewState(dir)

	// implement → correcting → implement (retry loop)
	state.Transition(radiant.StateImplement)
	state.Transition(radiant.StateCorrecting)
	state.Transition(radiant.StateImplement)
	state.Transition(radiant.StateCorrecting)
	state.Transition(radiant.StateImplement)
	state.Transition(radiant.StateValidate)
	state.Transition(radiant.StateDone)

	if state.CurrentState() != radiant.StateDone {
		t.Errorf("expected done after retries, got %s", state.CurrentState())
	}
}

func TestStateTransitionInvalidSkip(t *testing.T) {
	dir := t.TempDir()
	state := NewState(dir)

	// Can't skip from idle to done
	if err := state.Transition(radiant.StateDone); err == nil {
		t.Error("idle → done should be invalid")
	}

	// Can't go backwards from done to implement
	state.Transition(radiant.StateImplement)
	state.Transition(radiant.StateValidate)
	state.Transition(radiant.StateDone)
	if err := state.Transition(radiant.StateImplement); err == nil {
		t.Error("done → implement should be invalid")
	}
}

func TestStateConcurrentAccess(t *testing.T) {
	dir := t.TempDir()
	state := NewState(dir)

	state.SetTotalTasks(10)
	state.Transition(radiant.StateImplement)

	// Concurrent task operations
	done := make(chan bool, 10)
	for i := 1; i <= 10; i++ {
		go func(id int) {
			state.StartTask(id)
			state.CompleteTask(id)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// Should not panic or corrupt state
	if state.Progress() < 0 || state.Progress() > 1 {
		t.Errorf("invalid progress: %f", state.Progress())
	}
}

func TestStateSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	state := NewState(dir)

	state.SetFeature("test-feature")
	state.SetTotalTasks(5)
	state.Transition(radiant.StateImplement)
	state.StartTask(1)
	state.CompleteTask(1)
	state.StartTask(2)

	if err := state.Save(); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	// Load in fresh state
	loaded := NewState(dir)
	if loaded.data.Feature != "test-feature" {
		t.Errorf("feature mismatch: %s", loaded.data.Feature)
	}
	if loaded.data.TotalTasks != 5 {
		t.Errorf("total tasks mismatch: %d", loaded.data.TotalTasks)
	}
	if loaded.data.State != radiant.StateImplement {
		t.Errorf("state mismatch: %s", loaded.data.State)
	}
}

func TestStateProgressEdgeCases(t *testing.T) {
	dir := t.TempDir()
	state := NewState(dir)

	// Zero tasks
	if state.Progress() != 0 {
		t.Error("zero tasks should have zero progress")
	}

	// All completed
	state.SetTotalTasks(3)
	state.StartTask(1)
	state.CompleteTask(1)
	state.StartTask(2)
	state.CompleteTask(2)
	state.StartTask(3)
	state.CompleteTask(3)

	if state.Progress() != 1.0 {
		t.Errorf("all completed should be 1.0, got %f", state.Progress())
	}
}

// ── Orchestrator Edge Cases ──

func TestOrchestratorEmptySpecDir(t *testing.T) {
	dir := t.TempDir()
	orch := NewWithNoDetect(dir, 0)

	_, err := orch.Run(context.Background(), filepath.Join(dir, "nonexistent"))
	if err == nil {
		t.Error("nonexistent spec dir should fail")
	}
}

func TestOrchestratorMalformedSpec(t *testing.T) {
	dir := t.TempDir()
	specDir := filepath.Join(dir, "specs", "0001-bad")
	os.MkdirAll(specDir, 0o755)

	// Write malformed spec (no ACs)
	os.WriteFile(filepath.Join(specDir, "spec.md"), []byte("# Bad Spec\n\nNo ACs here."), 0o644)
	os.WriteFile(filepath.Join(specDir, "tasks.md"), []byte("# Tasks\n\nNothing."), 0o644)

	orch := NewWithNoDetect(dir, 0)
	result, err := orch.Run(context.Background(), specDir)
	if err != nil {
		t.Fatalf("should not error on malformed spec: %v", err)
	}

	// Should succeed with 0 tasks
	if !result.Succeeded {
		t.Error("malformed spec with no tasks should succeed vacuously")
	}
}

func TestOrchestratorWithGate(t *testing.T) {
	dir := t.TempDir()
	specDir := filepath.Join(dir, "specs", "0001-gate")
	os.MkdirAll(specDir, 0o755)

	specContent := "---\nname: test\nalwaysApply: true\n---\n\n### AC-1: test\n- **Given** X\n- **When** Y\n- **Then** Z\n"
	os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(specContent), 0o644)

	taskContent := "| # | Task | AC | Dep | Gate | Status |\n|---|------|----|-----|------|--------|\n| 1 | Test | AC-1 | — | echo ok | todo |\n"
	os.WriteFile(filepath.Join(specDir, "tasks.md"), []byte(taskContent), 0o644)

	orch := NewWithNoDetect(dir, 0)
	result, err := orch.Run(context.Background(), specDir)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	if !result.Succeeded {
		t.Error("gate 'echo ok' should pass")
	}
}

func TestOrchestratorWithFailingGate(t *testing.T) {
	dir := t.TempDir()
	specDir := filepath.Join(dir, "specs", "0001-fail")
	os.MkdirAll(specDir, 0o755)

	specContent := "---\nname: test\nalwaysApply: true\n---\n\n### AC-1: test\n- **Given** X\n- **When** Y\n- **Then** Z\n"
	os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(specContent), 0o644)

	taskContent := "| # | Task | AC | Dep | Gate | Status |\n|---|------|----|-----|------|--------|\n| 1 | Test | AC-1 | — | false | todo |\n"
	os.WriteFile(filepath.Join(specDir, "tasks.md"), []byte(taskContent), 0o644)

	orch := NewWithNoDetect(dir, 0) // 0 retries
	result, _ := orch.Run(context.Background(), specDir)

	if result.Succeeded {
		t.Error("gate 'false' should fail")
	}
}

func TestOrchestratorWithRetry(t *testing.T) {
	dir := t.TempDir()
	specDir := filepath.Join(dir, "specs", "0001-retry")
	os.MkdirAll(specDir, 0o755)

	specContent := "---\nname: test\nalwaysApply: true\n---\n\n### AC-1: test\n- **Given** X\n- **When** Y\n- **Then** Z\n"
	os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(specContent), 0o644)

	taskContent := "| # | Task | AC | Dep | Gate | Status |\n|---|------|----|-----|------|--------|\n| 1 | Test | AC-1 | — | false | todo |\n"
	os.WriteFile(filepath.Join(specDir, "tasks.md"), []byte(taskContent), 0o644)

	orch := NewWithNoDetect(dir, 3) // 3 retries
	result, _ := orch.Run(context.Background(), specDir)

	if result.Attempts != 4 { // 1 initial + 3 retries
		t.Errorf("expected 4 attempts, got %d", result.Attempts)
	}
	if result.Succeeded {
		t.Error("should fail after all retries")
	}
}

// ── Agent Detection Edge Cases ──

func TestDetectAgentReturnsInstalled(t *testing.T) {
	id, cmd := DetectAgent()
	if cmd == "" {
		t.Skip("no AI agent installed — skipping")
	}
	if id == "" {
		t.Error("detected agent should have an ID")
	}
	t.Logf("detected: %s (%s)", id, cmd)
}

func TestIsAgentAvailable(t *testing.T) {
	// 'sh' should always be available
	if !IsAgentAvailable("sh") {
		t.Error("sh should be available")
	}

	// Nonexistent should not be available
	if IsAgentAvailable("nonexistent-agent-xyz") {
		t.Error("nonexistent agent should not be available")
	}
}

// ── Token Estimator Edge Cases ──

func TestTokenEstimatorConsistency(t *testing.T) {
	te := NewTokenEstimator()
	text := "This is a test string with multiple words and some code_like identifiers."

	t1 := te.EstimateTokens(text)
	t2 := te.EstimateTokens(text)

	if t1 != t2 {
		t.Errorf("estimator should be deterministic: %d != %d", t1, t2)
	}
}

func TestTokenEstimatorMonotonic(t *testing.T) {
	te := NewTokenEstimator()
	short := "hello world"
	long := "hello world this is a longer text with more words"

	if te.EstimateTokens(long) <= te.EstimateTokens(short) {
		t.Error("longer text should have more tokens")
	}
}

// ── Log Edge Cases ──

func TestLogDoesNotPanic(t *testing.T) {
	logger := NewConsoleLogger(LevelInfo)
	logger.Info("test", "detail")
	logger.Info("task", "task_id", 1, "action", "started")
	logger.Warn("warning", "count", 42)
	logger.Error("error", "err", "something broke")
}

func TestSetVerbose(t *testing.T) {
	SetVerbose()
	Log.Debug("debug message")
}
