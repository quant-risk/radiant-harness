//go:build !light_only

package loop

import (
	"context"
	"strings"
	"testing"
)

// ── RunConfig.Plan / PlannerModel defaults ─────────────────────────────────

func TestRunConfigPlanDefaultFalse(t *testing.T) {
	cfg := RunConfig{}
	if cfg.Plan {
		t.Error("Plan should be false by default")
	}
}

func TestRunConfigPlannerModelDefaultZero(t *testing.T) {
	cfg := RunConfig{}
	if cfg.PlannerModel.Model != "" {
		t.Errorf("PlannerModel should be zero value by default, got %q", cfg.PlannerModel.Model)
	}
}

func TestRunConfigPlanAssignable(t *testing.T) {
	cfg := RunConfig{Plan: true}
	if !cfg.Plan {
		t.Error("Plan not set")
	}
}

// ── BuildPlannerPrompt ─────────────────────────────────────────────────────

func TestBuildPlannerPromptContainsGoal(t *testing.T) {
	p := BuildPlannerPrompt("add rate limiting", 0)
	if !strings.Contains(p, "add rate limiting") {
		t.Errorf("planner prompt should contain goal, got: %q", p)
	}
}

func TestBuildPlannerPromptFirstIter(t *testing.T) {
	p := BuildPlannerPrompt("my goal", 0)
	// First iteration: no "Prior attempts" language
	if strings.Contains(p, "Prior attempts") {
		t.Errorf("first-iteration prompt should not mention prior attempts, got: %q", p)
	}
}

func TestBuildPlannerPromptSubsequentIter(t *testing.T) {
	p := BuildPlannerPrompt("my goal", 2)
	if !strings.Contains(p, "iteration 2") {
		t.Errorf("subsequent-iteration prompt should mention iteration number, got: %q", p)
	}
	if !strings.Contains(p, "Prior attempts") {
		t.Errorf("subsequent-iteration prompt should mention prior attempts, got: %q", p)
	}
}

// ── buildExecutorPrompt with planOutput ────────────────────────────────────

func TestBuildExecutorPromptWithPlan(t *testing.T) {
	p := buildExecutorPrompt("goal", "", "1. Step one\n2. Step two", nil)
	if !strings.Contains(p, "PLAN:") {
		t.Errorf("executor prompt should contain PLAN: section, got: %q", p)
	}
	if !strings.Contains(p, "Step one") {
		t.Errorf("executor prompt should contain plan content, got: %q", p)
	}
}

func TestBuildExecutorPromptWithoutPlan(t *testing.T) {
	p := buildExecutorPrompt("goal", "", "", nil)
	if strings.Contains(p, "PLAN:") {
		t.Errorf("executor prompt without plan should not contain PLAN: section, got: %q", p)
	}
}

func TestBuildExecutorPromptPlanAfterGoal(t *testing.T) {
	p := buildExecutorPrompt("the goal", "", "step list", nil)
	goalIdx := strings.Index(p, "the goal")
	planIdx := strings.Index(p, "PLAN:")
	if goalIdx < 0 || planIdx < 0 {
		t.Fatal("expected both goal and plan in prompt")
	}
	if planIdx < goalIdx {
		t.Error("PLAN: section should appear after GOAL: section")
	}
}

// ── Run() with Plan=true — planner called (fail-open when no key) ──────────

func TestRunWithPlanEnabled(t *testing.T) {
	dir := t.TempDir()
	cfg := RunConfig{
		Budget: BudgetConfig{MaxIter: 1},
		Plan:   true,
	}
	// No API key → planner errors → fail-open → executor still called.
	// Run should return a result (not a fatal error) even when planner fails.
	result, err := Run(context.Background(), dir, "run-plan", "goal", cfg)
	if err != nil {
		t.Fatalf("Run with Plan=true should not return error on planner failure, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestRunWithPlanFalse(t *testing.T) {
	dir := t.TempDir()
	cfg := RunConfig{
		Budget: BudgetConfig{MaxIter: 1},
		Plan:   false,
	}
	result, err := Run(context.Background(), dir, "run-noplan", "goal", cfg)
	if err != nil {
		t.Fatalf("Run with Plan=false should not error, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}
