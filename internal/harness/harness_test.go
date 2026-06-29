package harness

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	radiant "github.com/quant-risk/radiant-harness/internal"
)

func TestContextWindow(t *testing.T) {
	cw := NewContextWindow(200000)

	if !cw.IsSmartZone() {
		t.Error("empty context should be in smart zone")
	}

	cw.AddTokens(80000)
	if cw.IsDumbZone() {
		t.Error("40% should not be dumb zone")
	}

	cw.AddTokens(50000)
	if !cw.IsDumbZone() {
		t.Error("65% should be dumb zone")
	}
}

func TestContextWindowStatus(t *testing.T) {
	cw := NewContextWindow(200000)
	cw.AddTokens(50000)

	status := cw.Status()
	if status == "" {
		t.Error("status should not be empty")
	}
}

func TestRPIBudget(t *testing.T) {
	budget := NewRPIBudget(200000)

	if budget.Research.MaxTokens != 60000 {
		t.Errorf("expected 60000, got %d", budget.Research.MaxTokens)
	}
	if budget.Plan.MaxTokens != 40000 {
		t.Errorf("expected 40000, got %d", budget.Plan.MaxTokens)
	}
	if budget.Implement.MaxTokens != 100000 {
		t.Errorf("expected 100000, got %d", budget.Implement.MaxTokens)
	}
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		text string
		min  int
		max  int
	}{
		{"hello", 1, 5},
		{"a longer text with multiple words", 5, 20},
		{"", 0, 1},
	}
	for _, tt := range tests {
		got := EstimateTokens(tt.text)
		if got < tt.min || got > tt.max {
			t.Errorf("EstimateTokens(%q) = %d, want %d-%d", tt.text, got, tt.min, tt.max)
		}
	}
}

func TestStateTransitions(t *testing.T) {
	dir := t.TempDir()
	state := NewState(dir)

	if err := state.Transition(radiant.StateImplement); err != nil {
		t.Errorf("idle to implement should be valid: %v", err)
	}

	if err := state.Transition(radiant.StateValidate); err != nil {
		t.Errorf("implement to validate should be valid: %v", err)
	}

	if err := state.Transition(radiant.StateDone); err != nil {
		t.Errorf("validate to done should be valid: %v", err)
	}
}

func TestStateInvalidTransition(t *testing.T) {
	dir := t.TempDir()
	state := NewState(dir)

	err := state.Transition(radiant.StateDone)
	if err == nil {
		t.Error("idle to done should be invalid")
	}
}

func TestStateProgress(t *testing.T) {
	dir := t.TempDir()
	state := NewState(dir)

	state.SetTotalTasks(4)
	state.StartTask(1)
	state.CompleteTask(1)
	state.StartTask(2)
	state.CompleteTask(2)

	progress := state.Progress()
	if progress != 0.5 {
		t.Errorf("expected 0.5, got %f", progress)
	}
}

func TestStateSaveLoad(t *testing.T) {
	dir := t.TempDir()
	state := NewState(dir)

	state.SetFeature("test-feature")
	state.SetTotalTasks(3)
	state.Transition(radiant.StateImplement)

	if err := state.Save(); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	state2 := NewState(dir)
	if state2.data.Feature != "test-feature" {
		t.Errorf("expected 'test-feature', got '%s'", state2.data.Feature)
	}
	if state2.data.TotalTasks != 3 {
		t.Errorf("expected 3, got %d", state2.data.TotalTasks)
	}
}

func TestDetectAgent(t *testing.T) {
	_, cmd := DetectAgent()
	t.Logf("detected agent: %s", cmd)
}

func TestOrchestratorDryRun(t *testing.T) {
	dir := t.TempDir()

	specDir := filepath.Join(dir, "specs", "0001-test")
	os.MkdirAll(specDir, 0o755)

	specContent := "---\nname: test\ndescription: Test\nalwaysApply: true\n---\n\n# Spec\n\n## Summary\nTest.\n\n### AC-1: test\n- **Given** X\n- **When** Y\n- **Then** Z\n"
	os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(specContent), 0o644)

	taskContent := "| # | Task | Covers AC | Depends on | Gate | Status |\n|---|------|-----------|------------|------|--------|\n| 1 | Test | AC-1 | — | echo ok | todo |\n"
	os.WriteFile(filepath.Join(specDir, "tasks.md"), []byte(taskContent), 0o644)

	orch := NewWithNoDetect(dir, 0)
	result, err := orch.Run(context.Background(), specDir)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	if !result.Succeeded {
		t.Error("dry-run should succeed")
	}
	if result.Attempts == 0 {
		t.Error("should have at least 1 attempt")
	}
}
