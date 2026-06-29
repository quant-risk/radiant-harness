//go:build with_full

package loop

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── assembleContextBlock ──────────────────────────────────────────────────────

func TestAssembleContextBlockNonExistentDir(t *testing.T) {
	out := assembleContextBlock("/nonexistent/path/xyz/abc", 4000)
	if out != "" {
		t.Errorf("expected empty string for non-existent dir, got %q", out[:min51(len(out), 40)])
	}
}

func TestAssembleContextBlockEmptyDir(t *testing.T) {
	dir := t.TempDir()
	// Must not panic; may return "" or content.
	out := assembleContextBlock(dir, 4000)
	_ = out
}

func TestAssembleContextBlockHasHeader(t *testing.T) {
	dir := t.TempDir()
	out := assembleContextBlock(dir, 4000)
	if out != "" && !strings.HasPrefix(out, "## PROJECT CONTEXT") {
		t.Errorf("expected '## PROJECT CONTEXT' prefix, got %q", out[:min51(len(out), 50)])
	}
}

// ── executorSystemPrompt ──────────────────────────────────────────────────────

func TestExecutorSystemPromptNoContext(t *testing.T) {
	p := executorSystemPrompt("")
	if strings.Contains(p, "PROJECT CONTEXT") {
		t.Error("empty context should not add PROJECT CONTEXT section")
	}
	if p == "" {
		t.Error("base prompt must not be empty")
	}
}

func TestExecutorSystemPromptWithContext(t *testing.T) {
	block := "## PROJECT CONTEXT\n\nsome repo info"
	p := executorSystemPrompt(block)
	if !strings.Contains(p, "PROJECT CONTEXT") {
		t.Error("context block not injected")
	}
	if !strings.Contains(p, "some repo info") {
		t.Error("context content missing from prompt")
	}
}

func TestExecutorSystemPromptBasePreserved(t *testing.T) {
	p := executorSystemPrompt("## PROJECT CONTEXT\n\ntest")
	if !strings.Contains(p, "expert software engineer") {
		t.Error("base prompt text lost when context injected")
	}
}

func TestExecutorSystemPromptContextAfterBase(t *testing.T) {
	p := executorSystemPrompt("## PROJECT CONTEXT\n\ntest")
	baseEnd := strings.Index(p, "Output the result clearly")
	ctxStart := strings.Index(p, "PROJECT CONTEXT")
	if baseEnd == -1 || ctxStart == -1 {
		t.Fatal("expected both sections present")
	}
	if ctxStart < baseEnd {
		t.Error("context block appears before base prompt text")
	}
}

// ── RunConfig.ContextBudgetTokens ────────────────────────────────────────────

func TestRunConfigContextBudgetDefault(t *testing.T) {
	cfg := RunConfig{}
	if cfg.ContextBudgetTokens != 0 {
		t.Errorf("expected 0 default, got %d", cfg.ContextBudgetTokens)
	}
}

func TestRunConfigContextBudgetSet(t *testing.T) {
	cfg := RunConfig{ContextBudgetTokens: 6000}
	if cfg.ContextBudgetTokens != 6000 {
		t.Errorf("expected 6000, got %d", cfg.ContextBudgetTokens)
	}
}

// ── Run() context assembly integration ───────────────────────────────────────

func TestRunSkipsContextWhenBudgetZero(t *testing.T) {
	dir := t.TempDir()
	cfg := RunConfig{
		Budget:              BudgetConfig{MaxIter: 1},
		ContextBudgetTokens: 0,
	}
	_, _ = Run(context.Background(), dir, "run-ctx-skip", "goal", cfg)

	ctxPath := filepath.Join(dir, ".radiant-harness", "CONTEXT.md")
	if _, err := os.Stat(ctxPath); err == nil {
		t.Error("CONTEXT.md written even with ContextBudgetTokens=0")
	}
}

func TestRunAssemblesContextWhenBudgetSet(t *testing.T) {
	dir := t.TempDir()
	cfg := RunConfig{
		Budget:              BudgetConfig{MaxIter: 1},
		ContextBudgetTokens: 4000,
	}
	_, _ = Run(context.Background(), dir, "run-ctx-set", "goal", cfg)

	ctxPath := filepath.Join(dir, ".radiant-harness", "CONTEXT.md")
	if _, err := os.Stat(ctxPath); err != nil {
		t.Error("CONTEXT.md not written even with ContextBudgetTokens=4000")
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func min51(a, b int) int {
	if a < b {
		return a
	}
	return b
}
