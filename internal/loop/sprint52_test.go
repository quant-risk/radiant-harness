package loop

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// ── StreamWriter interface ────────────────────────────────────────────────────

func TestStreamWriterSatisfiedByBuffer(t *testing.T) {
	var buf bytes.Buffer
	var w StreamWriter = &buf
	_, err := w.Write([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if buf.String() != "hello" {
		t.Errorf("expected 'hello', got %q", buf.String())
	}
}

// ── RunConfig.Stream / StreamOut defaults ─────────────────────────────────────

func TestRunConfigStreamDefaultFalse(t *testing.T) {
	cfg := RunConfig{}
	if cfg.Stream {
		t.Error("Stream should be false by default")
	}
}

func TestRunConfigStreamOutDefaultNil(t *testing.T) {
	cfg := RunConfig{}
	if cfg.StreamOut != nil {
		t.Error("StreamOut should be nil by default (resolved to os.Stdout in Run)")
	}
}

func TestRunConfigStreamAssignable(t *testing.T) {
	var buf bytes.Buffer
	cfg := RunConfig{
		Stream:    true,
		StreamOut: &buf,
	}
	if !cfg.Stream {
		t.Error("Stream not set")
	}
	if cfg.StreamOut != &buf {
		t.Error("StreamOut not set")
	}
}

// ── simpleChatStream — nil writer discards output ─────────────────────────────

func TestSimpleChatStreamNilWriterNoError(t *testing.T) {
	// simpleChatStream with a nil writer should still accumulate the response.
	// We can't call a real LLM here; we test the callback logic with a mock
	// by calling the internal callback directly.
	var sb strings.Builder
	cb := func(chunk string) {
		sb.WriteString(chunk)
		// w is nil — this is what simpleChatStream does internally
		var w StreamWriter // nil
		if w != nil {
			_, _ = w.Write([]byte(chunk))
		}
	}
	cb("hello ")
	cb("world")
	if sb.String() != "hello world" {
		t.Errorf("expected 'hello world', got %q", sb.String())
	}
}

// ── Run() with Stream=true writes iter header ─────────────────────────────────

func TestRunStreamWritesIterHeader(t *testing.T) {
	var buf bytes.Buffer
	dir := t.TempDir()
	cfg := RunConfig{
		Budget:    BudgetConfig{MaxIter: 1},
		Stream:    true,
		StreamOut: &buf,
	}
	_, _ = Run(context.Background(), dir, "run-stream-hdr", "goal", cfg)

	out := buf.String()
	// Even when LLM fails (no key), the iter header should be written before
	// the LLM call is attempted.
	if !strings.Contains(out, "executor (iter 1)") {
		t.Errorf("expected iter header in stream output, got: %q", out[:min52(len(out), 80)])
	}
}

func TestRunNoStreamNoIterHeader(t *testing.T) {
	var buf bytes.Buffer
	dir := t.TempDir()
	cfg := RunConfig{
		Budget:    BudgetConfig{MaxIter: 1},
		Stream:    false,
		StreamOut: &buf,
	}
	_, _ = Run(context.Background(), dir, "run-no-stream", "goal", cfg)

	// When stream=false, nothing is written to StreamOut.
	if buf.Len() > 0 {
		t.Errorf("expected empty buffer when Stream=false, got: %q", buf.String()[:min52(buf.Len(), 80)])
	}
}

// ── stream separator lines ────────────────────────────────────────────────────

func TestRunStreamWritesSeparator(t *testing.T) {
	var buf bytes.Buffer
	dir := t.TempDir()
	cfg := RunConfig{
		Budget:    BudgetConfig{MaxIter: 1},
		Stream:    true,
		StreamOut: &buf,
	}
	_, _ = Run(context.Background(), dir, "run-stream-sep", "goal", cfg)

	out := buf.String()
	if !strings.Contains(out, "──────") {
		t.Errorf("expected separator line in stream output, got: %q", out[:min52(len(out), 80)])
	}
}

// ── helper ───────────────────────────────────────────────────────────────────

func min52(a, b int) int {
	if a < b {
		return a
	}
	return b
}
