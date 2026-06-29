//go:build !light_only

package loop

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

// ── traceCall JSONL output ─────────────────────────────────────────────────

func TestTraceCall_LogJSON_EmitsEntry(t *testing.T) {
	var buf strings.Builder
	traceCall(nil, &buf, "r1", PhaseExecute, "executor", "claude-sonnet-4-6",
		"prompt", "response", 200, nil)

	raw := strings.TrimSpace(buf.String())
	if raw == "" {
		t.Fatal("expected JSONL output, got empty string")
	}
	var entry map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &entry); err != nil {
		t.Fatalf("invalid JSON: %v — got: %q", err, raw)
	}
	if entry["event"] != "loop.llm_call" {
		t.Errorf("expected event 'loop.llm_call', got %q", entry["event"])
	}
	if entry["level"] != "info" {
		t.Errorf("expected level 'info', got %q", entry["level"])
	}
	if entry["run_id"] != "r1" {
		t.Errorf("expected run_id 'r1', got %q", entry["run_id"])
	}
	if entry["phase"] != string(PhaseExecute) {
		t.Errorf("expected phase %q, got %q", PhaseExecute, entry["phase"])
	}
}

func TestTraceCall_LogJSON_ErrorSetsLevel(t *testing.T) {
	var buf strings.Builder
	traceCall(nil, &buf, "r", PhaseVerify, "verifier", "claude-opus-4-8",
		"p", "resp", 100, fmt.Errorf("LLM timeout"))

	var entry map[string]interface{}
	_ = json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &entry)
	if entry["level"] != "error" {
		t.Errorf("expected level 'error' on failure, got %q", entry["level"])
	}
	if entry["result"] != "failed" {
		t.Errorf("expected result 'failed', got %q", entry["result"])
	}
}

func TestTraceCall_NilLogJSON_NoOutput(t *testing.T) {
	// Should not panic and trace goes to tracer only.
	dir := t.TempDir()
	tr, _ := NewTracer(dir, "r-nil")
	defer tr.Close()
	traceCall(tr, nil, "r-nil", PhaseExecute, "executor", "claude-sonnet-4-6",
		"p", "resp", 50, nil)
	// Verify tracer received the event.
	events, _ := ReadTrace(tr.Path())
	if len(events) != 1 {
		t.Errorf("expected 1 event in tracer, got %d", len(events))
	}
}

func TestTraceCall_LogJSON_HasTimestamp(t *testing.T) {
	var buf strings.Builder
	before := time.Now().UTC().Add(-time.Second)
	traceCall(nil, &buf, "r", PhasePlan, "planner", "claude-sonnet-4-6",
		"p", "resp", 100, nil)

	var entry map[string]interface{}
	_ = json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &entry)
	ts, ok := entry["time"].(string)
	if !ok || ts == "" {
		t.Fatalf("expected 'time' field in JSON: %v", entry)
	}
	parsed, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		t.Fatalf("invalid time format: %q", ts)
	}
	if parsed.Before(before) {
		t.Errorf("timestamp %v is before test start %v", parsed, before)
	}
}

func TestTraceCall_LogJSON_KnownModel_HasCost(t *testing.T) {
	var buf strings.Builder
	traceCall(nil, &buf, "r", PhaseExecute, "executor", "claude-sonnet-4-6",
		"p", "resp", 1000, nil)

	var entry map[string]interface{}
	_ = json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &entry)
	cost, ok := entry["cost_usd"].(float64)
	if !ok || cost <= 0 {
		t.Errorf("expected positive cost_usd for known model, got %v", entry["cost_usd"])
	}
}

func TestTraceCall_BothTracerAndLogJSON(t *testing.T) {
	dir := t.TempDir()
	tr, _ := NewTracer(dir, "r-both")
	defer tr.Close()

	var buf strings.Builder
	traceCall(tr, &buf, "r-both", PhaseExecute, "executor", "claude-sonnet-4-6",
		"p", "resp", 200, nil)

	// Tracer received event.
	events, _ := ReadTrace(tr.Path())
	if len(events) != 1 {
		t.Errorf("expected 1 event in tracer, got %d", len(events))
	}
	// JSON output also received.
	if strings.TrimSpace(buf.String()) == "" {
		t.Error("expected JSONL output, got empty string")
	}
}
