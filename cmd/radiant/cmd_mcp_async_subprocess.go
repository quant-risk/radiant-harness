// Subprocess-backed AsyncGate (v3.7.7+).
//
// The existing inline AsyncGate (`asyncGateInstance` in
// `cmd_mcp_run_gate.go`) runs the per-phase self-driven helpers
// in the same Go process. For most cases that is correct: a
// 4-phase offline run completes in well under 500 ms and the
// inline path has zero fork/exec overhead.
//
// The subprocess-backed gate below covers the cases the inline
// path cannot:
//
//   - Hosts that gate tool-call completion on subprocess exit
//     (e.g. a synchronous TUI that doesn't release the tool
//     call until the inner process exits). The inline path
//     happens to satisfy this today because phases complete
//     synchronously, but a future host that required real
//     sampling-backed phases would deadlock — the inline path
//     cannot decouple its work from the tool call.
//   - Long-running async ops (cross-process worktree ops in
//     Fleet mode, ad-hoc background runs from a CI host).
//
// Selection: the env var `RADIANT_ASYNC_SUBPROCESS=1` swaps
// `asyncGateInstance` for `subprocessAsyncGateInstance` at
// process start. The default stays inline (no behaviour change
// for existing deployments). To turn it on, set the env var
// in the `radiant mcp serve` process:
//
//   RADIANT_ASYNC_SUBPROCESS=1 radiant setup-mcp --agent=hermes --global
//
// Per-phase flow:
//
//   1. parent `mcpRunGate` validates args, computes the ticket
//      (16-char hex of `taskID(workdir, task)`).
//   2. parent spawns `radiant async-runner --phase=…` via
//      `os/exec`. Returns ~immediately with the ticket + the
//      child pid; the child writes `.radiant-harness/pids/<ticket>.pid`
//      so a `phase_status` call can poll liveness.
//   3. parent waits for the child to exit. On a synchronous
//      host this is what gives the tool-call its bounded
//      wall-clock: the child does its work, exits, and the
//      parent's `cmd.Wait` returns. Total wall-clock for an
//      offline run is bounded by `selfDriven<Phase>` + fork/exec
//      overhead (~50 ms cold).
//   4. parent reads the child's exit code; if non-zero, surface
//      it in the MCP response. If zero, the child has already
//      persisted state.json so the host can `phase_status`
//      immediately.

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/quant-risk/radiant-harness/v3/internal/possess"
)

// subprocessAsyncGateInstance is the subprocess-backed impl of
// `internal/possess.AsyncGate`. Wired into `asyncGateInstance`
// only when RADIANT_ASYNC_SUBPROCESS=1 is set at process start
// (see `selectAsyncGate` in cmd_mcp_runtime.go).
type subprocessAsyncGate struct{}

// Spawn runs the named phase as a child `radiant async-runner`
// subprocess, blocks on its completion, and returns the handle.
// The handle's Ticket + StatePath match the inline path so the
// downstream MCP code does not need to branch.
func (subprocessAsyncGate) Spawn(phase possess.Phase, task, workdir string) (possess.GateHandle, error) {
	if workdir == "" {
		var err error
		workdir, err = os.Getwd()
		if err != nil {
			return possess.GateHandle{}, fmt.Errorf("resolve workdir: %w", err)
		}
	}
	if !possess.ValidPhase(phase) {
		return possess.GateHandle{}, fmt.Errorf("invalid phase: %s", phase)
	}

	id := taskID(workdir, task)
	statePath := possess.StatePathFor(workdir, possess.GateTicket(id))
	startedAt := time.Now()

	bin, err := asyncRunnerSelfPath()
	if err != nil {
		return possess.GateHandle{}, fmt.Errorf("locate radiant binary: %w", err)
	}

	// Best-effort: ensure the .radiant-harness/pids/ dir exists
	// before the child writes its pid file.
	_ = os.MkdirAll(filepath.Join(workdir, ".radiant-harness", "pids"), 0o755)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, "async-runner",
		"--phase", string(phase),
		"--ticket", id,
		"--workdir", workdir,
		"--task", task,
	)
	// Forward the parent's env so the subprocess picks up the
	// same RADIANT_* knobs (model hints, sampling timeout, etc).
	// Plus set RADIANT_INTERNAL=1 because async-runner is an
	// internal worker primitive — without it, the gatekeeper
	// in main.go rejects the call before our RunE runs.
	cmd.Env = append(os.Environ(), "RADIANT_INTERNAL=1")
	cmd.Stdout = os.Stderr // surface async-runner progress to the parent's stderr
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// exec.ExitError means the child exited non-zero (phase
		// failed). Other errors (e.g. ctx deadline exceeded,
		// binary not found) propagate as-is.
		if exitErr, ok := err.(*exec.ExitError); ok {
			return possess.GateHandle{}, fmt.Errorf("async-runner phase %s exited %d", phase, exitErr.ExitCode())
		}
		return possess.GateHandle{}, fmt.Errorf("async-runner phase %s: %w", phase, err)
	}

	return possess.GateHandle{
		Ticket:    possess.GateTicket(id),
		Phase:     phase,
		StatePath: statePath,
		StartedAt: startedAt,
	}, nil
}

// Status / Cancel defer to the inline implementation — state.json
// is the canonical source of truth, not the pid file, so reading
// or cancelling works identically regardless of which path
// produced it.
func (subprocessAsyncGate) Status(ticket possess.GateTicket, workdir string) (possess.Status, error) {
	return asyncGateInstance.Status(ticket, workdir)
}

func (subprocessAsyncGate) Cancel(ticket possess.GateTicket, workdir string) error {
	// Mark the ticket cancelled in state.json (the inline path
	// already does this). Also best-effort kill the recorded
	// pid so a long-running detached child stops.
	if err := asyncGateInstance.Cancel(ticket, workdir); err != nil {
		return err
	}
	if workdir == "" {
		var err error
		workdir, err = os.Getwd()
		if err != nil {
			return nil
		}
	}
	if alive, pid := asyncRunnerLiveness(workdir, string(ticket)); alive && pid > 0 {
		if proc, err := os.FindProcess(pid); err == nil {
			// SIGTERM first; the async-runner body has a
			// 60-second budget so this is enough for normal
			// phases to drain. A SIGKILL fallback is the
			// caller's responsibility (or a follow-up).
			_ = proc.Signal(os.Interrupt)
		}
	}
	return nil
}

// subprocessPossessAsyncInstance is the subprocess-backed impl
// of `internal/possess.PossessAsync`. Same selection mechanism
// as the gate above: opt-in via RADIANT_ASYNC_SUBPROCESS=1.
type subprocessPossessAsync struct{}

// Spawn fires all four phases back-to-back in subprocess mode.
// Each phase runs as its own `radiant async-runner` invocation
// (no sharing of state between child processes besides the
// state.json file). Returns once the fourth phase exits.
func (subprocessPossessAsync) Spawn(task, workdir, profile string) (possess.GateHandle, error) {
	if workdir == "" {
		var err error
		workdir, err = os.Getwd()
		if err != nil {
			return possess.GateHandle{}, fmt.Errorf("resolve workdir: %w", err)
		}
	}
	if profile == "" {
		profile = "standard"
	}
	id := taskID(workdir, task)
	statePath := possess.StatePathFor(workdir, possess.GateTicket(id))
	startedAt := time.Now()

	gate := subprocessAsyncGate{}
	for _, phase := range []possess.Phase{
		possess.PhaseDiscover,
		possess.PhasePlan,
		possess.PhaseExecute,
		possess.PhaseVerify,
	} {
		if _, err := gate.Spawn(phase, task, workdir); err != nil {
			return possess.GateHandle{Ticket: possess.GateTicket(id), Phase: phase, StatePath: statePath, StartedAt: startedAt},
				fmt.Errorf("possess_async phase %s: %w", phase, err)
		}
	}
	return possess.GateHandle{
		Ticket:    possess.GateTicket(id),
		Phase:     possess.PhaseVerify,
		StatePath: statePath,
		StartedAt: startedAt,
	}, nil
}

func (subprocessPossessAsync) Status(ticket possess.GateTicket, workdir string) (possess.Status, error) {
	return asyncGateInstance.Status(ticket, workdir)
}

func (subprocessPossessAsync) Cancel(ticket possess.GateTicket, workdir string) error {
	return subprocessAsyncGate{}.Cancel(ticket, workdir)
}