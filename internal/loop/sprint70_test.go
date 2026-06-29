//go:build !light_only

package loop

import (
	"strings"
	"testing"
	"time"
)

// ── ExportTrace ────────────────────────────────────────────────────────────

func TestExportTrace_BasicFields(t *testing.T) {
	ts := time.Now()
	events := []TraceEvent{
		{Timestamp: ts, Phase: PhaseExecute, Action: "write", Result: "ok",
			TokensIn: 500, TokensOut: 250},
	}
	exp := ExportTrace("run-1", "claude-sonnet-4-6", events)
	if exp.RunID != "run-1" {
		t.Errorf("RunID: got %q", exp.RunID)
	}
	if exp.ModelID != "claude-sonnet-4-6" {
		t.Errorf("ModelID: got %q", exp.ModelID)
	}
	if exp.EventCount != 1 {
		t.Errorf("EventCount: got %d", exp.EventCount)
	}
	if exp.TokensIn != 500 {
		t.Errorf("TokensIn: got %d", exp.TokensIn)
	}
	if exp.TokensOut != 250 {
		t.Errorf("TokensOut: got %d", exp.TokensOut)
	}
}

func TestExportTrace_CostPopulated(t *testing.T) {
	ts := time.Now()
	events := []TraceEvent{
		{Timestamp: ts, Phase: PhaseExecute, Action: "w", Result: "ok",
			TokensIn: 1000, TokensOut: 500},
	}
	exp := ExportTrace("r", "claude-sonnet-4-6", events)
	if exp.CostUSD <= 0 {
		t.Errorf("expected positive CostUSD for known model, got %f", exp.CostUSD)
	}
}

func TestExportTrace_NoCost_UnknownModel(t *testing.T) {
	events := []TraceEvent{
		{Timestamp: time.Now(), Phase: PhaseExecute, Action: "w", Result: "ok",
			TokensIn: 1000, TokensOut: 500},
	}
	exp := ExportTrace("r", "totally-unknown", events)
	if exp.CostUSD != 0 {
		t.Errorf("expected 0 CostUSD for unknown model, got %f", exp.CostUSD)
	}
}

func TestExportTrace_TimestampsBounded(t *testing.T) {
	t0 := time.Now()
	t1 := t0.Add(time.Second)
	events := []TraceEvent{
		{Timestamp: t0, Phase: PhaseExecute, Action: "a", Result: "ok"},
		{Timestamp: t1, Phase: PhaseVerify, Action: "b", Result: "ok"},
	}
	exp := ExportTrace("r", "", events)
	if !exp.StartedAt.Equal(t0) {
		t.Errorf("StartedAt should be first event: %v", exp.StartedAt)
	}
	if !exp.UpdatedAt.Equal(t1) {
		t.Errorf("UpdatedAt should be last event: %v", exp.UpdatedAt)
	}
}

func TestExportTrace_EmptyEvents(t *testing.T) {
	exp := ExportTrace("r", "claude-sonnet-4-6", nil)
	if exp.EventCount != 0 {
		t.Errorf("expected 0 events, got %d", exp.EventCount)
	}
	if exp.CostUSD != 0 {
		t.Errorf("expected 0 cost for empty events, got %f", exp.CostUSD)
	}
}

// ── ExportTraceMarkdown ────────────────────────────────────────────────────

func TestExportTraceMarkdown_HasHeader(t *testing.T) {
	exp := ExportTrace("my-run", "claude-sonnet-4-6", []TraceEvent{
		{Timestamp: time.Now(), Phase: PhaseExecute, Action: "write", Result: "ok",
			TokensIn: 100, TokensOut: 50},
	})
	md := ExportTraceMarkdown(exp)
	if !strings.Contains(md, "# Loop Run: my-run") {
		t.Errorf("missing header: %q", md)
	}
	if !strings.Contains(md, "claude-sonnet-4-6") {
		t.Errorf("missing model: %q", md)
	}
	if !strings.Contains(md, "## Events") {
		t.Errorf("missing Events section: %q", md)
	}
}

func TestExportTraceMarkdown_ShowsCost(t *testing.T) {
	exp := ExportTrace("r", "claude-sonnet-4-6", []TraceEvent{
		{Timestamp: time.Now(), Phase: PhaseExecute, Action: "w", Result: "ok",
			TokensIn: 1000, TokensOut: 500},
	})
	md := ExportTraceMarkdown(exp)
	if !strings.Contains(md, "Cost") {
		t.Errorf("expected Cost in markdown: %q", md)
	}
	if !strings.Contains(md, "$") {
		t.Errorf("expected $ in cost line: %q", md)
	}
}

func TestExportTraceMarkdown_EachEventListed(t *testing.T) {
	ts := time.Now()
	exp := ExportTrace("r", "", []TraceEvent{
		{Timestamp: ts, Phase: PhaseDiscover, Action: "scan", Result: "ok", Evidence: "found 3 files"},
		{Timestamp: ts.Add(time.Second), Phase: PhaseExecute, Action: "write", Result: "failed"},
	})
	md := ExportTraceMarkdown(exp)
	if !strings.Contains(md, "scan") {
		t.Errorf("missing event action 'scan': %q", md)
	}
	if !strings.Contains(md, "found 3 files") {
		t.Errorf("missing evidence: %q", md)
	}
	if !strings.Contains(md, "write") {
		t.Errorf("missing event action 'write': %q", md)
	}
}
