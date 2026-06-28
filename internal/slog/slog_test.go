package slog_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/quant-risk/radiant-harness/internal/slog"
)

func TestLogger_Info_WritesJSON(t *testing.T) {
	var buf strings.Builder
	l := slog.New(&buf)
	l.Info(slog.Entry{Event: "loop.start", RunID: "r1", Phase: "execute"})
	raw := buf.String()
	var e slog.Entry
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &e); err != nil {
		t.Fatalf("invalid JSON: %v — got: %q", err, raw)
	}
	if e.Level != "info" {
		t.Errorf("expected level 'info', got %q", e.Level)
	}
	if e.Event != "loop.start" {
		t.Errorf("expected event 'loop.start', got %q", e.Event)
	}
	if e.RunID != "r1" {
		t.Errorf("expected run_id 'r1', got %q", e.RunID)
	}
}

func TestLogger_Error_SetsLevel(t *testing.T) {
	var buf strings.Builder
	l := slog.New(&buf)
	l.Error(slog.Entry{Event: "loop.failed"})
	var e slog.Entry
	_ = json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &e)
	if e.Level != "error" {
		t.Errorf("expected level 'error', got %q", e.Level)
	}
}

func TestLogger_Discard_NoOutput(t *testing.T) {
	l := slog.Discard()
	// Should not panic.
	l.Info(slog.Entry{Event: "test"})
}

func TestLogger_TimestampAutoSet(t *testing.T) {
	var buf strings.Builder
	l := slog.New(&buf)
	l.Info(slog.Entry{Event: "e"})
	var e slog.Entry
	_ = json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &e)
	if e.Time.IsZero() {
		t.Error("expected Time to be set automatically")
	}
}

func TestLogger_MultipleEntries_EachOnOwnLine(t *testing.T) {
	var buf strings.Builder
	l := slog.New(&buf)
	l.Info(slog.Entry{Event: "a"})
	l.Info(slog.Entry{Event: "b"})
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 JSONL lines, got %d: %q", len(lines), buf.String())
	}
}
