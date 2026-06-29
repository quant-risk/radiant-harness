//go:build !light_only

package loop

import (
	"context"
	"strings"
	"testing"

	"github.com/quant-risk/radiant-harness/internal/llm"
)

// ── RunConfig.AutoRoute defaults ───────────────────────────────────────────

func TestRunConfigAutoRouteDefaultFalse(t *testing.T) {
	cfg := RunConfig{}
	if cfg.AutoRoute {
		t.Error("AutoRoute should be false by default")
	}
}

func TestRunConfigAutoRouteAssignable(t *testing.T) {
	cfg := RunConfig{AutoRoute: true}
	if !cfg.AutoRoute {
		t.Error("AutoRoute not set")
	}
}

// ── AutoRoute model derivation (unit-level, no LLM) ──────────────────────

// autoRoutedModels simulates the model-derivation logic from Run() so we
// can test it without invoking a real LLM.
func autoRoutedModels(anchor llm.Model, autoRoute bool) (exec, ver, plan llm.Model) {
	exec = anchor
	ver = anchor
	plan = anchor

	if !autoRoute {
		return
	}

	if routed := llm.AutoRoute(anchor.Model, llm.PhaseResearch); routed != anchor.Model {
		ver = llm.Model{Model: routed, APIKey: anchor.APIKey, BaseURL: anchor.BaseURL}
	}
	if routed := llm.AutoRoute(anchor.Model, llm.PhasePlan); routed != anchor.Model {
		plan = llm.Model{Model: routed, APIKey: anchor.APIKey, BaseURL: anchor.BaseURL}
	}
	return
}

func TestAutoRouteDisabled_AllModelsEqualAnchor(t *testing.T) {
	anchor := llm.Model{Model: "claude-sonnet-4-6", APIKey: "k"}
	exec, ver, plan := autoRoutedModels(anchor, false)

	if exec.Model != "claude-sonnet-4-6" {
		t.Errorf("exec should be anchor, got %q", exec.Model)
	}
	if ver.Model != "claude-sonnet-4-6" {
		t.Errorf("ver should be anchor when auto-route off, got %q", ver.Model)
	}
	if plan.Model != "claude-sonnet-4-6" {
		t.Errorf("plan should be anchor when auto-route off, got %q", plan.Model)
	}
}

func TestAutoRouteEnabled_SonnetAnchor_VerifierUpgradesToOpus(t *testing.T) {
	anchor := llm.Model{Model: "claude-sonnet-4-6", APIKey: "k"}
	exec, ver, plan := autoRoutedModels(anchor, true)

	// Execute stays on anchor.
	if exec.Model != "claude-sonnet-4-6" {
		t.Errorf("exec should stay on anchor, got %q", exec.Model)
	}
	// Verifier should upgrade to claude-opus (top-tier research model).
	if !strings.HasPrefix(ver.Model, "claude-opus") {
		t.Errorf("verifier should be claude-opus when anchor is sonnet, got %q", ver.Model)
	}
	// Plan stays mid-tier (sonnet IS mid — AutoRoute returns anchor for PhasePlan).
	if !strings.HasPrefix(plan.Model, "claude-sonnet") {
		t.Errorf("plan should stay sonnet (already mid-tier), got %q", plan.Model)
	}
	// API key propagated.
	if ver.APIKey != "k" {
		t.Errorf("verifier should inherit API key, got %q", ver.APIKey)
	}
}

func TestAutoRouteEnabled_OpusAnchor_StaysOnOpus(t *testing.T) {
	anchor := llm.Model{Model: "claude-opus-4-8", APIKey: "k"}
	exec, ver, plan := autoRoutedModels(anchor, true)

	// Opus is already top-tier — AutoRoute returns anchor for research.
	if exec.Model != "claude-opus-4-8" {
		t.Errorf("exec should be opus, got %q", exec.Model)
	}
	if ver.Model != "claude-opus-4-8" {
		t.Errorf("ver should stay on opus (already top-tier), got %q", ver.Model)
	}
	// Plan should pick mid-tier (sonnet) from the claude family.
	if !strings.HasPrefix(plan.Model, "claude-sonnet") {
		t.Errorf("plan should downgrade to sonnet from opus anchor, got %q", plan.Model)
	}
}

func TestAutoRouteEnabled_UnknownAnchor_AllStayOnAnchor(t *testing.T) {
	anchor := llm.Model{Model: "totally-custom-model", APIKey: "k"}
	exec, ver, plan := autoRoutedModels(anchor, true)

	// Unknown family → AutoRoute returns anchor for all phases.
	if exec.Model != "totally-custom-model" {
		t.Errorf("exec wrong: %q", exec.Model)
	}
	if ver.Model != "totally-custom-model" {
		t.Errorf("ver should stay on anchor for unknown family, got %q", ver.Model)
	}
	if plan.Model != "totally-custom-model" {
		t.Errorf("plan should stay on anchor for unknown family, got %q", plan.Model)
	}
}

func TestAutoRouteEnabled_BaseURLPropagated(t *testing.T) {
	anchor := llm.Model{Model: "claude-sonnet-4-6", APIKey: "k", BaseURL: "https://custom.host"}
	_, ver, _ := autoRoutedModels(anchor, true)

	if ver.BaseURL != "https://custom.host" {
		t.Errorf("verifier should inherit BaseURL, got %q", ver.BaseURL)
	}
}

// ── Run() with AutoRoute=true — fail-open (no API key) ───────────────────

func TestRunWithAutoRouteEnabled(t *testing.T) {
	dir := t.TempDir()
	cfg := RunConfig{
		Budget:    BudgetConfig{MaxIter: 1},
		AutoRoute: true,
		ExecutorModel: llm.Model{Model: "claude-sonnet-4-6"},
	}
	// No API key → all LLM calls fail → fail-open → Run returns result, not error.
	result, err := Run(context.Background(), dir, "run-autoroute", "goal", cfg)
	if err != nil {
		t.Fatalf("Run with AutoRoute=true should not error on LLM failure: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestRunWithAutoRouteDisabled(t *testing.T) {
	dir := t.TempDir()
	cfg := RunConfig{
		Budget:    BudgetConfig{MaxIter: 1},
		AutoRoute: false,
		ExecutorModel: llm.Model{Model: "claude-sonnet-4-6"},
	}
	result, err := Run(context.Background(), dir, "run-noautoroute", "goal", cfg)
	if err != nil {
		t.Fatalf("Run with AutoRoute=false should not error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}
