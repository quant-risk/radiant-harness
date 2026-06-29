//go:build !light_only

package loop

import (
	"strings"
	"testing"
	"time"
)

// ── EstimateCost ───────────────────────────────────────────────────────────

func TestEstimateCost_KnownModel(t *testing.T) {
	cost, ok := EstimateCost("claude-sonnet-4-6", 1000, 500)
	if !ok {
		t.Fatal("expected ok=true for known model")
	}
	if cost <= 0 {
		t.Errorf("expected positive cost, got %f", cost)
	}
}

func TestEstimateCost_UnknownModel(t *testing.T) {
	cost, ok := EstimateCost("totally-unknown-model-xyz", 1000, 500)
	if ok {
		t.Error("expected ok=false for unknown model")
	}
	if cost != 0 {
		t.Errorf("expected 0 cost for unknown model, got %f", cost)
	}
}

func TestEstimateCost_ZeroTokens(t *testing.T) {
	cost, ok := EstimateCost("claude-sonnet-4-6", 0, 0)
	if !ok {
		t.Fatal("expected ok=true for known model with 0 tokens")
	}
	if cost != 0 {
		t.Errorf("expected 0 cost for 0 tokens, got %f", cost)
	}
}

func TestEstimateCost_InputCheaperThanOutput(t *testing.T) {
	// For claude-opus-4-8: input=0.003, output=0.015 per 1K
	// 1000 input only: 0.003
	// 1000 output only: 0.015
	costIn, _ := EstimateCost("claude-opus-4-8", 1000, 0)
	costOut, _ := EstimateCost("claude-opus-4-8", 0, 1000)
	if costIn >= costOut {
		t.Errorf("input cost (%f) should be less than output cost (%f)", costIn, costOut)
	}
}

func TestEstimateCost_Accumulates(t *testing.T) {
	c1, _ := EstimateCost("claude-sonnet-4-6", 500, 250)
	c2, _ := EstimateCost("claude-sonnet-4-6", 500, 250)
	cTotal, _ := EstimateCost("claude-sonnet-4-6", 1000, 500)
	if abs(c1+c2-cTotal) > 1e-10 {
		t.Errorf("cost should be linear: c1+c2=%.6f, cTotal=%.6f", c1+c2, cTotal)
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// ── FormatCost ────────────────────────────────────────────────────────────

func TestFormatCost_Zero(t *testing.T) {
	out := FormatCost(0)
	if out != "$0.0000" {
		t.Errorf("got %q", out)
	}
}

func TestFormatCost_SmallValue(t *testing.T) {
	out := FormatCost(0.000001)
	if !strings.HasPrefix(out, "< $") {
		t.Errorf("expected '< $' prefix for tiny cost, got %q", out)
	}
}

func TestFormatCost_NormalValue(t *testing.T) {
	out := FormatCost(0.0042)
	if out != "$0.0042" {
		t.Errorf("got %q", out)
	}
}

func TestFormatCost_LargeValue(t *testing.T) {
	out := FormatCost(1.2345)
	if out != "$1.2345" {
		t.Errorf("got %q", out)
	}
}

// ── FormatProgress with modelID shows cost ─────────────────────────────────

func TestFormatProgress_WithModel_ShowsCost(t *testing.T) {
	ts := time.Now()
	events := []TraceEvent{
		{Timestamp: ts, Phase: PhaseExecute, Action: "write", Result: "ok",
			TokensIn: 1000, TokensOut: 500},
	}
	out := FormatProgress("r", "claude-sonnet-4-6", events)
	if !strings.Contains(out, "Cost") {
		t.Errorf("expected Cost line in output: %q", out)
	}
	if !strings.Contains(out, "$") {
		t.Errorf("expected '$' in cost line: %q", out)
	}
}

func TestFormatProgress_UnknownModel_NoCostLine(t *testing.T) {
	ts := time.Now()
	events := []TraceEvent{
		{Timestamp: ts, Phase: PhaseExecute, Action: "write", Result: "ok",
			TokensIn: 1000, TokensOut: 500},
	}
	out := FormatProgress("r", "unknown-custom-model", events)
	if strings.Contains(out, "Cost:") {
		t.Errorf("should not show Cost line for unknown model: %q", out)
	}
}

func TestFormatProgress_NoModel_NoCostLine(t *testing.T) {
	ts := time.Now()
	events := []TraceEvent{
		{Timestamp: ts, Phase: PhaseExecute, Action: "write", Result: "ok",
			TokensIn: 1000, TokensOut: 500},
	}
	out := FormatProgress("r", "", events)
	if strings.Contains(out, "Cost:") {
		t.Errorf("should not show Cost line when no model: %q", out)
	}
}

// ── TraceInfo includes cost ────────────────────────────────────────────────

func TestListTraceInfos_CostPopulated_WhenModelKnown(t *testing.T) {
	dir := t.TempDir()
	tr, _ := NewTracer(dir, "run-cost")
	_ = tr.Record(TraceEvent{
		Timestamp: time.Now(),
		Phase:     PhaseExecute,
		Action:    "write",
		Result:    "ok",
		TokensIn:  1000,
		TokensOut: 500,
		Meta:      map[string]string{"model": "claude-sonnet-4-6"},
	})
	tr.Close()

	infos, err := ListTraceInfos(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 1 {
		t.Fatalf("expected 1 info, got %d", len(infos))
	}
	if infos[0].CostUSD <= 0 {
		t.Errorf("expected positive CostUSD for known model, got %f", infos[0].CostUSD)
	}
}

func TestListTraceInfos_NoCost_WhenModelUnknown(t *testing.T) {
	dir := t.TempDir()
	tr, _ := NewTracer(dir, "run-nocost")
	_ = tr.Record(TraceEvent{
		Timestamp: time.Now(), Phase: PhaseExecute, Action: "w", Result: "ok",
		TokensIn: 1000, TokensOut: 500,
	})
	tr.Close()

	infos, _ := ListTraceInfos(dir)
	if len(infos) != 1 {
		t.Fatalf("expected 1 info")
	}
	if infos[0].CostUSD != 0 {
		t.Errorf("expected 0 CostUSD when model unknown, got %f", infos[0].CostUSD)
	}
}

// ── FormatTraceList shows cost column ─────────────────────────────────────

func TestFormatTraceList_ShowsCostColumn(t *testing.T) {
	infos := []TraceInfo{{RunID: "r", EventCount: 1, CostUSD: 0.0042}}
	out := FormatTraceList(infos)
	if !strings.Contains(out, "COST") {
		t.Errorf("expected COST header: %q", out)
	}
	if !strings.Contains(out, "$0.0042") {
		t.Errorf("expected formatted cost: %q", out)
	}
}

func TestFormatTraceList_NoCost_EmptyColumn(t *testing.T) {
	infos := []TraceInfo{{RunID: "r", EventCount: 1, CostUSD: 0}}
	out := FormatTraceList(infos)
	if !strings.Contains(out, "COST") {
		t.Errorf("expected COST header even when cost=0: %q", out)
	}
}
