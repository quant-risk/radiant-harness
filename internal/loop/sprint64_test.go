//go:build with_full

package loop

import (
	"strings"
	"testing"
	"time"
)

// ── ListTraceInfos ────────────────────────────────────────────────────────

func TestListTraceInfos_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	infos, err := ListTraceInfos(dir)
	if err != nil {
		t.Fatalf("unexpected error on empty dir: %v", err)
	}
	if len(infos) != 0 {
		t.Errorf("expected 0 infos, got %d", len(infos))
	}
}

func TestListTraceInfos_SingleTrace(t *testing.T) {
	dir := t.TempDir()
	tr, err := NewTracer(dir, "run-info-1")
	if err != nil {
		t.Fatal(err)
	}
	_ = tr.Record(TraceEvent{
		Timestamp: time.Now(), Phase: PhaseExecute,
		Action: "write file", Result: "ok", TokensIn: 10, TokensOut: 5,
	})
	tr.Close()

	infos, err := ListTraceInfos(dir)
	if err != nil {
		t.Fatalf("ListTraceInfos: %v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("expected 1 info, got %d", len(infos))
	}
	info := infos[0]
	if info.RunID != "run-info-1" {
		t.Errorf("RunID = %q", info.RunID)
	}
	if info.EventCount != 1 {
		t.Errorf("EventCount = %d, want 1", info.EventCount)
	}
	if info.LastPhase != PhaseExecute {
		t.Errorf("LastPhase = %q", info.LastPhase)
	}
	if info.LastResult != "ok" {
		t.Errorf("LastResult = %q", info.LastResult)
	}
	if info.LastAction != "write file" {
		t.Errorf("LastAction = %q", info.LastAction)
	}
}

func TestListTraceInfos_MultipleTraces_NewestFirst(t *testing.T) {
	dir := t.TempDir()

	// Write older trace.
	tr1, _ := NewTracer(dir, "run-old")
	_ = tr1.Record(TraceEvent{
		Timestamp: time.Now().Add(-10 * time.Minute),
		Phase: PhaseDiscover, Action: "old", Result: "ok",
	})
	tr1.Close()

	// Write newer trace.
	tr2, _ := NewTracer(dir, "run-new")
	_ = tr2.Record(TraceEvent{
		Timestamp: time.Now(),
		Phase: PhaseVerify, Action: "new", Result: "ok",
	})
	tr2.Close()

	infos, err := ListTraceInfos(dir)
	if err != nil {
		t.Fatalf("ListTraceInfos: %v", err)
	}
	if len(infos) != 2 {
		t.Fatalf("expected 2 infos, got %d", len(infos))
	}
	if infos[0].RunID != "run-new" {
		t.Errorf("expected newest first, got %q", infos[0].RunID)
	}
	if infos[1].RunID != "run-old" {
		t.Errorf("expected oldest second, got %q", infos[1].RunID)
	}
}

func TestListTraceInfos_MultipleEvents_LastEventUsed(t *testing.T) {
	dir := t.TempDir()
	tr, _ := NewTracer(dir, "run-multi-ev")
	ts := time.Now()
	_ = tr.Record(TraceEvent{Timestamp: ts, Phase: PhaseDiscover, Action: "first", Result: "ok"})
	_ = tr.Record(TraceEvent{Timestamp: ts.Add(time.Second), Phase: PhaseExecute, Action: "second", Result: "failed"})
	tr.Close()

	infos, _ := ListTraceInfos(dir)
	if len(infos) != 1 {
		t.Fatalf("expected 1 info, got %d", len(infos))
	}
	info := infos[0]
	if info.EventCount != 2 {
		t.Errorf("EventCount = %d, want 2", info.EventCount)
	}
	if info.LastAction != "second" {
		t.Errorf("LastAction = %q, want 'second'", info.LastAction)
	}
	if info.LastResult != "failed" {
		t.Errorf("LastResult = %q, want 'failed'", info.LastResult)
	}
}

// ── FormatTraceList ───────────────────────────────────────────────────────

func TestFormatTraceList_Empty(t *testing.T) {
	out := FormatTraceList(nil)
	if !strings.Contains(out, "No traces found") {
		t.Errorf("expected empty message: %q", out)
	}
}

func TestFormatTraceList_ShowsRunID(t *testing.T) {
	infos := []TraceInfo{{RunID: "my-run-abc", EventCount: 5, LastPhase: PhasePlan, LastResult: "ok"}}
	out := FormatTraceList(infos)
	if !strings.Contains(out, "my-run-abc") {
		t.Errorf("expected run ID in output: %q", out)
	}
}

func TestFormatTraceList_ShowsEventCount(t *testing.T) {
	infos := []TraceInfo{{RunID: "r", EventCount: 42, LastPhase: PhaseExecute, LastResult: "ok"}}
	out := FormatTraceList(infos)
	if !strings.Contains(out, "42") {
		t.Errorf("expected event count 42 in output: %q", out)
	}
}

func TestFormatTraceList_ShowsPhaseAndResult(t *testing.T) {
	infos := []TraceInfo{{RunID: "r", EventCount: 1, LastPhase: PhaseVerify, LastResult: "failed"}}
	out := FormatTraceList(infos)
	if !strings.Contains(out, string(PhaseVerify)) {
		t.Errorf("expected phase in output: %q", out)
	}
	if !strings.Contains(out, "failed") {
		t.Errorf("expected result in output: %q", out)
	}
}

func TestFormatTraceList_LongRunIDTruncated(t *testing.T) {
	long := strings.Repeat("a", 50)
	infos := []TraceInfo{{RunID: long, EventCount: 1}}
	out := FormatTraceList(infos)
	if strings.Contains(out, long) {
		t.Errorf("expected long run ID to be truncated in output")
	}
	if !strings.Contains(out, "...") {
		t.Errorf("expected '...' truncation marker: %q", out)
	}
}

func TestFormatTraceList_ShowsTimestamp(t *testing.T) {
	infos := []TraceInfo{{
		RunID:     "r",
		EventCount: 1,
		UpdatedAt: time.Date(2026, 6, 27, 14, 30, 0, 0, time.UTC),
	}}
	out := FormatTraceList(infos)
	if !strings.Contains(out, "2026-06-27") {
		t.Errorf("expected date in output: %q", out)
	}
}

func TestFormatTraceList_HasHeader(t *testing.T) {
	infos := []TraceInfo{{RunID: "r", EventCount: 1}}
	out := FormatTraceList(infos)
	if !strings.Contains(out, "RUN-ID") {
		t.Errorf("expected header row: %q", out)
	}
}
