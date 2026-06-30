// Package possess — async gate primitives for synchronous hosts.
//
// See docs/PROPOSAL-v3.7.2-async-primitives.md for the design.
//
// v3.7.1 closed the Codex hollow-stub case (driver surfaces -32601
// mid-run → self-driven scaffold fallback). Hermes TUI is a different
// failure mode: synchronous `wait_for_tool_result` blocks sampling
// callbacks, so any `radiant_possess` call hangs for the host's tool-call
// timeout (120s on Hermes) without populating the workdir with real
// execution.
//
// v3.7.2 decomposes `radiant_possess` into async primitives:
//
//   - AsyncGate — runs ONE phase (discover|plan|execute|verify) in a
//     subprocess, returns a ticket the host polls. NO sampling callback
//     round-trip — phase runs offline, state persisted to state.json.
//   - PossessAsync — fires the full 4-phase loop as a subprocess, returns
//     a ticket immediately. Host polls via phase_status until done.
//
// These primitives are stubs in v3.7.2-prep (this commit). Real
// subprocess wiring lands in PR-B (per the proposal).
package possess

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// ErrAsyncInDevelopment is returned by all AsyncGate / PossessAsync
// entry points in v3.7.2-prep. Replace with real implementation in
// v3.7.2 PR-B.
var ErrAsyncInDevelopment = errors.New("async gate primitives are v3.7.2 in-development (see docs/PROPOSAL-v3.7.2-async-primitives.md)")

// Phase is one of the four radiant-possess phases.
type Phase string

const (
	PhaseDiscover Phase = "discover"
	PhasePlan     Phase = "plan"
	PhaseExecute  Phase = "execute"
	PhaseVerify   Phase = "verify"
)

// ValidPhase reports whether p is a known phase.
func ValidPhase(p Phase) bool {
	switch p {
	case PhaseDiscover, PhasePlan, PhaseExecute, PhaseVerify:
		return true
	default:
		return false
	}
}

// GateTicket identifies one async gate run. Format: 16-char hex prefix
// (matches the possess task_id convention).
type GateTicket string

// GateHandle is the return value of AsyncGate.Spawn — host polls
// `Status()` and reads `state.json` at `StatePath`.
type GateHandle struct {
	Ticket    GateTicket
	Phase     Phase
	StatePath string
	StartedAt time.Time
}

// Status is the current observable state of a GateHandle.
type Status struct {
	Ticket    GateTicket
	Phase     Phase
	Current   string // phase name (same as Phase for single-gate runs)
	State     string // "in_progress" | "done" | "error"
	Error     string // populated when State == "error"
	StartedAt time.Time
	EndedAt   time.Time
}

// AsyncGate runs ONE phase in the background. The interface is what
// the v3.7.2 PR-B will implement via subprocess + state.json polling.
//
// In v3.7.2-prep the only implementation returns ErrAsyncInDevelopment.
type AsyncGate interface {
	Spawn(phase Phase, task, workdir string) (GateHandle, error)
	Status(ticket GateTicket, workdir string) (Status, error)
	Cancel(ticket GateTicket, workdir string) error
}

// PossessAsync runs the FULL 4-phase loop in the background. Returns
// a ticket the host polls via `radiant_phase_status`. The MCP tool call
// itself takes <500ms (just subprocess spawn + ticket return).
//
// In v3.7.2-prep the only implementation returns ErrAsyncInDevelopment.
type PossessAsync interface {
	Spawn(task, workdir, profile string) (GateHandle, error)
	Status(ticket GateTicket, workdir string) (Status, error)
	Cancel(ticket GateTicket, workdir string) error
}

// StubAsyncGate is the v3.7.2-prep placeholder. Every method returns
// ErrAsyncInDevelopment. Replaced by the real subprocess-based
// implementation in v3.7.2 PR-B.
type StubAsyncGate struct{}

// Spawn returns ErrAsyncInDevelopment.
func (StubAsyncGate) Spawn(phase Phase, task, workdir string) (GateHandle, error) {
	return GateHandle{}, fmt.Errorf("AsyncGate.Spawn(%q): %w", phase, ErrAsyncInDevelopment)
}

// Status returns ErrAsyncInDevelopment.
func (StubAsyncGate) Status(ticket GateTicket, workdir string) (Status, error) {
	return Status{}, fmt.Errorf("AsyncGate.Status(%q): %w", ticket, ErrAsyncInDevelopment)
}

// Cancel returns ErrAsyncInDevelopment.
func (StubAsyncGate) Cancel(ticket GateTicket, workdir string) error {
	return fmt.Errorf("AsyncGate.Cancel(%q): %w", ticket, ErrAsyncInDevelopment)
}

// StubPossessAsync is the v3.7.2-prep placeholder.
type StubPossessAsync struct{}

// Spawn returns ErrAsyncInDevelopment.
func (StubPossessAsync) Spawn(task, workdir, profile string) (GateHandle, error) {
	return GateHandle{}, fmt.Errorf("PossessAsync.Spawn: %w", ErrAsyncInDevelopment)
}

// Status returns ErrAsyncInDevelopment.
func (StubPossessAsync) Status(ticket GateTicket, workdir string) (Status, error) {
	return Status{}, fmt.Errorf("PossessAsync.Status(%q): %w", ticket, ErrAsyncInDevelopment)
}

// Cancel returns ErrAsyncInDevelopment.
func (StubPossessAsync) Cancel(ticket GateTicket, workdir string) error {
	return fmt.Errorf("PossessAsync.Cancel(%q): %w", ticket, ErrAsyncInDevelopment)
}

// NewTicket returns a fresh 16-char hex ticket matching the
// `possess-<task-id>` convention used elsewhere in the harness.
// CRYPTO-RANDOM so collision across concurrent runs is negligible
// (16 hex chars = 64 bits of entropy).
func NewTicket() GateTicket {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Fall back to time-based if /dev/urandom is unreachable (should
		// never happen on Linux/macOS/Windows). The format is preserved.
		return GateTicket(fmt.Sprintf("%016x", time.Now().UnixNano()))
	}
	return GateTicket(hex.EncodeToString(b[:]))
}

// StatePathFor returns the standard `.radiant-harness/state/<ticket>/`
// path for a given ticket inside workdir. Matches the existing
// `savePossessState` layout.
func StatePathFor(workdir string, ticket GateTicket) string {
	return fmt.Sprintf("%s/.radiant-harness/state/%s/state.json", workdir, ticket)
}