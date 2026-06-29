package schedule

import (
	"strings"
	"testing"
	"time"
)

var base = time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)

// ── Evaluate: rate limiting ────────────────────────────────────────────────

func TestEvaluate_FirstRunAllowed(t *testing.T) {
	p := DefaultPolicy()
	d := Evaluate(p, State{}, []Signal{{Kind: TriggerNewCommits, Value: 3}}, base)
	if !d.ShouldRun {
		t.Errorf("first run with a fired trigger should run: %s", d.Reason)
	}
}

func TestEvaluate_RateLimited(t *testing.T) {
	p := DefaultPolicy() // MinInterval 15m
	s := State{LastRunAt: base.Add(-5 * time.Minute)}
	d := Evaluate(p, s, []Signal{{Kind: TriggerNewCommits, Value: 1}}, base)
	if d.ShouldRun {
		t.Errorf("should be rate-limited 5m < 15m, got run: %s", d.Reason)
	}
	if !strings.Contains(d.Reason, "rate-limited") {
		t.Errorf("reason should mention rate-limited, got %q", d.Reason)
	}
}

func TestEvaluate_PastRateLimit(t *testing.T) {
	p := DefaultPolicy()
	s := State{LastRunAt: base.Add(-20 * time.Minute), DayStamp: "2026-06-27", RunsToday: 1}
	d := Evaluate(p, s, []Signal{{Kind: TriggerNewCommits, Value: 1}}, base)
	if !d.ShouldRun {
		t.Errorf("20m > 15m should allow run: %s", d.Reason)
	}
}

// ── Evaluate: daily cap ────────────────────────────────────────────────────

func TestEvaluate_DailyCapReached(t *testing.T) {
	p := Policy{Triggers: []TriggerKind{TriggerNewCommits}, MinInterval: time.Minute, MaxRunsPerDay: 3}
	s := State{LastRunAt: base.Add(-time.Hour), DayStamp: "2026-06-27", RunsToday: 3}
	d := Evaluate(p, s, []Signal{{Kind: TriggerNewCommits, Value: 1}}, base)
	if d.ShouldRun {
		t.Errorf("daily cap 3/3 should block, got run: %s", d.Reason)
	}
	if !strings.Contains(d.Reason, "daily cap") {
		t.Errorf("reason should mention daily cap, got %q", d.Reason)
	}
}

func TestEvaluate_DailyCapResetsNextDay(t *testing.T) {
	p := Policy{Triggers: []TriggerKind{TriggerNewCommits}, MinInterval: time.Minute, MaxRunsPerDay: 3}
	// state is from yesterday; counter must reset
	s := State{LastRunAt: base.Add(-25 * time.Hour), DayStamp: "2026-06-26", RunsToday: 3}
	d := Evaluate(p, s, []Signal{{Kind: TriggerNewCommits, Value: 1}}, base)
	if !d.ShouldRun {
		t.Errorf("new day should reset daily cap: %s", d.Reason)
	}
}

// ── Evaluate: triggers ─────────────────────────────────────────────────────

func TestEvaluate_NoSignalNoRun(t *testing.T) {
	p := DefaultPolicy()
	d := Evaluate(p, State{}, nil, base)
	if d.ShouldRun {
		t.Errorf("no signals should not run: %s", d.Reason)
	}
}

func TestEvaluate_DisabledTriggerIgnored(t *testing.T) {
	// Policy only enables failing-gate; a new-commits signal must be ignored.
	p := Policy{Triggers: []TriggerKind{TriggerFailingGate}, MinInterval: time.Minute}
	d := Evaluate(p, State{}, []Signal{{Kind: TriggerNewCommits, Value: 5}}, base)
	if d.ShouldRun {
		t.Errorf("disabled trigger should not fire: %s", d.Reason)
	}
}

func TestEvaluate_ZeroValueSignalDoesNotFire(t *testing.T) {
	p := DefaultPolicy()
	d := Evaluate(p, State{}, []Signal{{Kind: TriggerNewCommits, Value: 0}}, base)
	if d.ShouldRun {
		t.Errorf("zero-value signal should not fire: %s", d.Reason)
	}
}

func TestEvaluate_FailingGateFires(t *testing.T) {
	p := DefaultPolicy()
	d := Evaluate(p, State{}, []Signal{{Kind: TriggerFailingGate, Detail: "go test", Value: 1}}, base)
	if !d.ShouldRun {
		t.Errorf("failing gate should trigger a run: %s", d.Reason)
	}
	if !strings.Contains(d.Reason, "failing-gate") {
		t.Errorf("reason should name the trigger, got %q", d.Reason)
	}
}

func TestEvaluate_ReasonIsStable(t *testing.T) {
	p := DefaultPolicy()
	sigs := []Signal{
		{Kind: TriggerPendingWork, Value: 2},
		{Kind: TriggerNewCommits, Value: 3},
	}
	d1 := Evaluate(p, State{}, sigs, base)
	d2 := Evaluate(p, State{}, sigs, base)
	if d1.Reason != d2.Reason {
		t.Errorf("reason not stable: %q vs %q", d1.Reason, d2.Reason)
	}
	// sorted: new-commits before pending-work
	if !strings.Contains(d1.Reason, "new-commits(3), pending-work(2)") {
		t.Errorf("reason not sorted as expected: %q", d1.Reason)
	}
}

// ── RecordRun ──────────────────────────────────────────────────────────────

func TestRecordRun_IncrementsSameDay(t *testing.T) {
	s := State{DayStamp: "2026-06-27", RunsToday: 2, LastRunAt: base.Add(-time.Hour)}
	got := RecordRun(s, "abc123", base)
	if got.RunsToday != 3 {
		t.Errorf("RunsToday = %d, want 3", got.RunsToday)
	}
	if got.LastCommit != "abc123" {
		t.Errorf("LastCommit = %q, want abc123", got.LastCommit)
	}
	if !got.LastRunAt.Equal(base) {
		t.Errorf("LastRunAt not updated")
	}
}

func TestRecordRun_ResetsNewDay(t *testing.T) {
	s := State{DayStamp: "2026-06-26", RunsToday: 19}
	got := RecordRun(s, "x", base)
	if got.RunsToday != 1 {
		t.Errorf("new day RunsToday = %d, want 1", got.RunsToday)
	}
}

// ── Persistence ────────────────────────────────────────────────────────────

func TestSaveAndLoadState(t *testing.T) {
	dir := t.TempDir()
	s := State{LastRunAt: base, RunsToday: 4, DayStamp: "2026-06-27", LastCommit: "deadbeef"}
	if err := SaveState(dir, s); err != nil {
		t.Fatalf("SaveState: %v", err)
	}
	got, err := LoadState(dir)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if got.RunsToday != 4 || got.LastCommit != "deadbeef" || !got.LastRunAt.Equal(base) {
		t.Errorf("round-trip mismatch: %+v", got)
	}
}

func TestLoadState_MissingIsZero(t *testing.T) {
	got, err := LoadState(t.TempDir())
	if err != nil {
		t.Fatalf("LoadState on empty dir: %v", err)
	}
	if !got.LastRunAt.IsZero() || got.RunsToday != 0 {
		t.Errorf("missing state should be zero, got %+v", got)
	}
}

// ── FormatDecision ─────────────────────────────────────────────────────────

func TestFormatDecision_Run(t *testing.T) {
	d := Decision{ShouldRun: true, Reason: "triggered by new-commits(3)", Signals: []Signal{{Kind: TriggerNewCommits, Detail: "a..b", Value: 3}}}
	out := FormatDecision(d, base)
	if !strings.Contains(out, "● RUN") || !strings.Contains(out, "new-commits") {
		t.Errorf("unexpected format: %s", out)
	}
}

func TestFormatDecision_Skip(t *testing.T) {
	d := Decision{ShouldRun: false, Reason: "rate-limited"}
	out := FormatDecision(d, base)
	if !strings.Contains(out, "○ SKIP") {
		t.Errorf("unexpected format: %s", out)
	}
}
