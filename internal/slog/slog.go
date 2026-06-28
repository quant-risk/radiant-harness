// Package slog provides structured JSONL logging for loop and fleet events.
// It is intentionally minimal — one function, one struct, no dependencies
// beyond the standard library.
package slog

import (
	"encoding/json"
	"io"
	"os"
	"time"
)

// Entry is a single log line emitted as JSON.
type Entry struct {
	Time    time.Time      `json:"time"`
	Level   string         `json:"level"`
	Event   string         `json:"event"`
	RunID   string         `json:"run_id,omitempty"`
	Phase   string         `json:"phase,omitempty"`
	Action  string         `json:"action,omitempty"`
	Result  string         `json:"result,omitempty"`
	Tokens  int            `json:"tokens,omitempty"`
	CostUSD float64        `json:"cost_usd,omitempty"`
	Data    map[string]any `json:"data,omitempty"`
}

// Logger writes structured JSONL to an io.Writer.
type Logger struct {
	out io.Writer
	enc *json.Encoder
}

// New returns a Logger that writes to out (typically os.Stdout or a file).
func New(out io.Writer) *Logger {
	enc := json.NewEncoder(out)
	return &Logger{out: out, enc: enc}
}

// Discard returns a Logger that discards all output (useful as a nil-safe default).
func Discard() *Logger { return New(io.Discard) }

// Stdout returns a Logger that writes to os.Stdout.
func Stdout() *Logger { return New(os.Stdout) }

// Info logs an info-level entry.
func (l *Logger) Info(e Entry) {
	e.Level = "info"
	if e.Time.IsZero() {
		e.Time = time.Now().UTC()
	}
	_ = l.enc.Encode(e)
}

// Error logs an error-level entry.
func (l *Logger) Error(e Entry) {
	e.Level = "error"
	if e.Time.IsZero() {
		e.Time = time.Now().UTC()
	}
	_ = l.enc.Encode(e)
}
