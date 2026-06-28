package loop

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ── traceCall nil-safety ──────────────────────────────────────────────────────

func TestTraceCallNilTracer(t *testing.T) {
	// Must not panic when tr == nil.
	traceCall(nil, nil, "run-1", PhaseExecute, "executor", "claude-sonnet-4-6", "prompt", "response", 100, nil)
}

func TestTraceCallNilTracerWithError(t *testing.T) {
	traceCall(nil, nil, "run-1", PhaseExecute, "executor", "gpt-4o", "prompt", "", 0, errors.New("timeout"))
}

// ── traceCall with real Tracer ────────────────────────────────────────────────

func TestTraceCallRecordsOkEvent(t *testing.T) {
	dir := t.TempDir()
	tr, err := NewTracer(dir, "run-trace-ok")
	if err != nil {
		t.Fatal(err)
	}
	traceCall(tr, nil, "run-trace-ok", PhaseExecute, "executor", "claude-sonnet-4-6", "my prompt", "my response", 80, nil)
	tr.Close()

	events, err := ReadTrace(tr.Path())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	e := events[0]
	if e.Result != "ok" {
		t.Errorf("expected result=ok, got %s", e.Result)
	}
	if e.Agent != "executor" {
		t.Errorf("expected agent=executor, got %s", e.Agent)
	}
	if e.Phase != PhaseExecute {
		t.Errorf("expected phase=execute, got %s", e.Phase)
	}
	if e.Meta["model"] != "claude-sonnet-4-6" {
		t.Errorf("expected model in meta, got %v", e.Meta)
	}
	if e.PromptHash == "" {
		t.Error("expected non-empty prompt hash")
	}
	if len(e.PromptHash) != 8 {
		t.Errorf("expected 8-char hash, got %d: %s", len(e.PromptHash), e.PromptHash)
	}
	if e.TokensIn+e.TokensOut != 80 {
		t.Errorf("expected tokens 80 total, got in=%d out=%d", e.TokensIn, e.TokensOut)
	}
}

func TestTraceCallRecordsFailedEvent(t *testing.T) {
	dir := t.TempDir()
	tr, err := NewTracer(dir, "run-trace-fail")
	if err != nil {
		t.Fatal(err)
	}
	traceCall(tr, nil, "run-trace-fail", PhaseVerify, "verifier", "gpt-4o", "prompt", "", 0, errors.New("connection reset"))
	tr.Close()

	events, _ := ReadTrace(tr.Path())
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	e := events[0]
	if e.Result != "failed" {
		t.Errorf("expected result=failed, got %s", e.Result)
	}
	if e.Evidence != "connection reset" {
		t.Errorf("expected evidence to contain error message, got %s", e.Evidence)
	}
}

func TestTraceCallMultipleEvents(t *testing.T) {
	dir := t.TempDir()
	tr, err := NewTracer(dir, "run-trace-multi")
	if err != nil {
		t.Fatal(err)
	}
	traceCall(tr, nil, "run-trace-multi", PhaseExecute, "executor", "model-a", "p1", "r1", 40, nil)
	traceCall(tr, nil, "run-trace-multi", PhaseVerify, "verifier", "model-b", "p2", "r2", 60, nil)
	traceCall(tr, nil, "run-trace-multi", PhaseVerify, "reviewer", "model-b", "p3", "r3", 50, nil)
	tr.Close()

	events, _ := ReadTrace(tr.Path())
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
	if events[0].Agent != "executor" || events[1].Agent != "verifier" || events[2].Agent != "reviewer" {
		t.Error("event order or agent names wrong")
	}
}

func TestTraceCallHashDiffersForDifferentPrompts(t *testing.T) {
	dir := t.TempDir()
	tr, _ := NewTracer(dir, "run-hash-test")
	traceCall(tr, nil, "run-hash-test", PhaseExecute, "executor", "m", "prompt-A", "out", 10, nil)
	traceCall(tr, nil, "run-hash-test", PhaseExecute, "executor", "m", "prompt-B", "out", 10, nil)
	tr.Close()

	events, _ := ReadTrace(tr.Path())
	if events[0].PromptHash == events[1].PromptHash {
		t.Error("different prompts should produce different hashes")
	}
}

// ── RunConfig.Trace field ─────────────────────────────────────────────────────

func TestRunConfigTraceFieldDefault(t *testing.T) {
	cfg := RunConfig{}
	if cfg.Trace != nil {
		t.Error("Trace should be nil by default")
	}
}

func TestRunConfigTraceFieldAssignable(t *testing.T) {
	dir := t.TempDir()
	tr, _ := NewTracer(dir, "run-cfg-trace")
	cfg := RunConfig{Trace: tr}
	if cfg.Trace != tr {
		t.Error("Trace field not assignable")
	}
	tr.Close()
}

// ── Tracer auto-created in Run() ─────────────────────────────────────────────

func TestRunCreatesTraceFile(t *testing.T) {
	// Run() with dry config (missing API key) still creates the trace file
	// before the LLM call fails. We verify the file appears on disk.
	// We use a zero LLM model which will fail — but the tracer is opened first.
	dir := t.TempDir()
	runID := "run-trace-auto"

	// Confirm trace file doesn't exist yet.
	tracePath := filepath.Join(dir, ".radiant-harness", "traces", runID+".jsonl")
	if _, err := os.Stat(tracePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("trace file should not exist before Run()")
	}

	// Run will fail fast (no API key) but should still create the file.
	ctx := context.Background()
	cfg := RunConfig{
		Budget: BudgetConfig{MaxIter: 1},
	}
	_, _ = Run(ctx, dir, runID, "test goal", cfg)

	// File must now exist (tracer was opened).
	if _, err := os.Stat(tracePath); err != nil {
		t.Errorf("trace file not created by Run(): %v", err)
	}
}

// ── Tracer timestamps ─────────────────────────────────────────────────────────

func TestTraceCallTimestamp(t *testing.T) {
	before := time.Now().UTC().Add(-time.Millisecond)
	dir := t.TempDir()
	tr, _ := NewTracer(dir, "run-ts")
	traceCall(tr, nil, "run-ts", PhaseExecute, "executor", "m", "p", "r", 10, nil)
	tr.Close()
	after := time.Now().UTC().Add(time.Millisecond)

	events, _ := ReadTrace(tr.Path())
	if len(events) == 0 {
		t.Fatal("no events")
	}
	ts := events[0].Timestamp
	if ts.Before(before) || ts.After(after) {
		t.Errorf("timestamp %v outside window [%v, %v]", ts, before, after)
	}
}
