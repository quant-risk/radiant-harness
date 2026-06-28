package loop

import (
	"strings"
	"testing"
	"time"
)

// ── TracePath ──────────────────────────────────────────────────────────────

func TestTracePath_Format(t *testing.T) {
	p := TracePath("/project", "my-run-123")
	if !strings.HasSuffix(p, ".radiant-harness/traces/my-run-123.jsonl") {
		t.Errorf("unexpected TracePath: %q", p)
	}
	if !strings.HasPrefix(p, "/project") {
		t.Errorf("expected absolute path rooted in project dir, got %q", p)
	}
}

func TestTracePath_DifferentRunIDs(t *testing.T) {
	p1 := TracePath("/p", "run-a")
	p2 := TracePath("/p", "run-b")
	if p1 == p2 {
		t.Error("different run IDs should produce different paths")
	}
}

// ── FormatProgress — empty events ─────────────────────────────────────────

func TestFormatProgress_EmptyEvents(t *testing.T) {
	out := FormatProgress("run-empty", "", nil)
	if !strings.Contains(out, "run-empty") {
		t.Errorf("expected run ID in empty output: %q", out)
	}
	if !strings.Contains(out, "no events") {
		t.Errorf("expected 'no events' message: %q", out)
	}
}

// ── FormatProgress — single event ─────────────────────────────────────────

func TestFormatProgress_SingleEvent_ShowsRunID(t *testing.T) {
	events := []TraceEvent{
		{Timestamp: time.Now(), RunID: "run-01", Phase: PhaseDiscover,
			Action: "scan", Result: "ok"},
	}
	out := FormatProgress("run-01", "", events)
	if !strings.Contains(out, "run-01") {
		t.Errorf("expected run ID in output: %q", out)
	}
}

func TestFormatProgress_SingleEvent_ShowsPhase(t *testing.T) {
	events := []TraceEvent{
		{Timestamp: time.Now(), Phase: PhaseExecute, Action: "write file", Result: "ok"},
	}
	out := FormatProgress("r", "", events)
	if !strings.Contains(out, string(PhaseExecute)) {
		t.Errorf("expected phase in output: %q", out)
	}
}

func TestFormatProgress_SingleEvent_ShowsLastAction(t *testing.T) {
	events := []TraceEvent{
		{Timestamp: time.Now(), Phase: PhasePlan, Action: "generate plan", Result: "ok"},
	}
	out := FormatProgress("r", "", events)
	if !strings.Contains(out, "generate plan") {
		t.Errorf("expected last action in output: %q", out)
	}
}

// ── FormatProgress — token totals ─────────────────────────────────────────

func TestFormatProgress_TokensAccumulated(t *testing.T) {
	ts := time.Now()
	events := []TraceEvent{
		{Timestamp: ts, Phase: PhaseDiscover, Action: "a", Result: "ok", TokensIn: 100, TokensOut: 50},
		{Timestamp: ts.Add(time.Second), Phase: PhaseExecute, Action: "b", Result: "ok", TokensIn: 200, TokensOut: 80},
	}
	out := FormatProgress("r", "", events)
	// Total = 430
	if !strings.Contains(out, "430") {
		t.Errorf("expected total tokens 430 in output: %q", out)
	}
	if !strings.Contains(out, "300") { // tokensIn
		t.Errorf("expected tokensIn 300 in output: %q", out)
	}
	if !strings.Contains(out, "130") { // tokensOut
		t.Errorf("expected tokensOut 130 in output: %q", out)
	}
}

func TestFormatProgress_NoTokens_ShowsZero(t *testing.T) {
	events := []TraceEvent{
		{Timestamp: time.Now(), Phase: PhasePlan, Action: "plan", Result: "ok"},
	}
	out := FormatProgress("r", "", events)
	if !strings.Contains(out, "0 total") {
		t.Errorf("expected '0 total' tokens when none recorded: %q", out)
	}
}

// ── FormatProgress — iteration counting ───────────────────────────────────

func TestFormatProgress_IterationCount(t *testing.T) {
	ts := time.Now()
	events := []TraceEvent{
		{Timestamp: ts, Phase: PhaseDiscover, Action: "iter 1", Result: "ok"},
		{Timestamp: ts.Add(time.Second), Phase: PhasePlan, Action: "plan", Result: "ok"},
		{Timestamp: ts.Add(2 * time.Second), Phase: PhaseDiscover, Action: "iter 2", Result: "ok"},
		{Timestamp: ts.Add(3 * time.Second), Phase: PhaseExecute, Action: "exec", Result: "ok"},
	}
	out := FormatProgress("r", "", events)
	// Two PhaseDiscover events → iteration = 2
	if !strings.Contains(out, "2") {
		t.Errorf("expected iteration 2 in output: %q", out)
	}
}

// ── FormatProgress — evidence ─────────────────────────────────────────────

func TestFormatProgress_EvidenceShown(t *testing.T) {
	events := []TraceEvent{
		{Timestamp: time.Now(), Phase: PhaseVerify, Action: "verify",
			Result: "ok", Evidence: "all tests pass"},
	}
	out := FormatProgress("r", "", events)
	if !strings.Contains(out, "all tests pass") {
		t.Errorf("expected evidence in output: %q", out)
	}
}

func TestFormatProgress_LongEvidenceTruncated(t *testing.T) {
	long := strings.Repeat("x", 100)
	events := []TraceEvent{
		{Timestamp: time.Now(), Phase: PhaseVerify, Action: "v", Result: "ok", Evidence: long},
	}
	out := FormatProgress("r", "", events)
	if strings.Contains(out, long) {
		t.Errorf("expected long evidence to be truncated in output")
	}
	if !strings.Contains(out, "...") {
		t.Errorf("expected '...' truncation marker in output: %q", out)
	}
}

// ── FormatProgress — elapsed ───────────────────────────────────────────────

func TestFormatProgress_ShowsElapsed(t *testing.T) {
	start := time.Now()
	end := start.Add(5 * time.Minute)
	events := []TraceEvent{
		{Timestamp: start, Phase: PhaseDiscover, Action: "start", Result: "ok"},
		{Timestamp: end, Phase: PhaseExecute, Action: "end", Result: "ok"},
	}
	out := FormatProgress("r", "", events)
	if !strings.Contains(out, "Elapsed") {
		t.Errorf("expected 'Elapsed' in output: %q", out)
	}
}

// ── round-trip: write trace → ReadTrace → FormatProgress ──────────────────

func TestFormatProgress_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	tr, err := NewTracer(dir, "run-rt")
	if err != nil {
		t.Fatal(err)
	}
	ts := time.Now()
	_ = tr.Record(TraceEvent{Timestamp: ts, Phase: PhaseDiscover, Action: "scan", Result: "ok", TokensIn: 10, TokensOut: 5})
	_ = tr.Record(TraceEvent{Timestamp: ts.Add(time.Second), Phase: PhaseExecute, Action: "write", Result: "ok", TokensIn: 20, TokensOut: 8})
	tr.Close()

	events, err := ReadTrace(tr.Path())
	if err != nil {
		t.Fatalf("ReadTrace: %v", err)
	}
	out := FormatProgress("run-rt", "", events)

	if !strings.Contains(out, "run-rt") {
		t.Errorf("run ID missing: %q", out)
	}
	if !strings.Contains(out, "43") { // 10+5+20+8 = 43
		t.Errorf("expected total tokens 43: %q", out)
	}
}
