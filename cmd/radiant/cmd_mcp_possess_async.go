// mcp__radiant__possess_async — full 4-phase loop as a fire-and-forget
// MCP call. Returns a ticket in <500ms; host polls via
// `radiant_phase_status(task_id)`.
//
// Why this exists: synchronous TUI hosts (Hermes is the documented one)
// cannot satisfy nested `sampling/createMessage` callbacks — the
// existing `radiant_possess` MCP call deadlocks at 120s. v3.7.2
// decomposes the loop into per-phase `radiant_run_gate` calls so the
// host can drive each phase in its own short MCP round-trip. This tool
// is the convenience wrapper for hosts that want one-shot semantics
// anyway: it runs all four phases back-to-back (still offline, no
// sampling) and yields the same shape that `radiant_possess` would have
// produced.
//
// The four phases are implemented by the existing self-driven harness
// primitives — no sampling involved, so synchronous hosts work fine.
// See `AGENTS-FOR-TASKS.md` § Hermes-TUI workstream for the workflow.

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/quant-risk/radiant-harness/v3/internal/possess"
)

// asyncPossess is the v3.7.2 implementation of
// `internal/possess.PossessAsync`.
type asyncPossess struct{}

// subprocessAsyncEnabled reports whether the parent `radiant mcp
// serve` should run the gate primitives as a `radiant async-runner`
// subprocess instead of inline. Opt-in via env var so the default
// inline behaviour (v3.7.0-v3.7.6) is unchanged for existing
// deployments.
func subprocessAsyncEnabled() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("RADIANT_ASYNC_SUBPROCESS")))
	return v == "1" || v == "true" || v == "yes"
}

// selectedPossessAsync returns the inline `asyncPossess` impl by
// default, or the subprocess-backed `subprocessPossessAsync` when
// RADIANT_ASYNC_SUBPROCESS=1 is set. The two impls are drop-in
// replacements for `internal/possess.PossessAsync`.
func selectedPossessAsync() possess.PossessAsync {
	if subprocessAsyncEnabled() {
		return subprocessPossessAsync{}
	}
	return asyncPossess{}
}

// selectedAsyncGate returns the inline or subprocess AsyncGate impl
// based on the same env var. Drop-in replacement for
// `internal/possess.AsyncGate`.
func selectedAsyncGate() possess.AsyncGate {
	if subprocessAsyncEnabled() {
		return subprocessAsyncGate{}
	}
	return asyncGate{}
}

// Spawn runs all four phases against the given workdir. Each phase
// persists its state.json synchronously so the host sees progress
// between back-to-back calls.
func (asyncPossess) Spawn(task, workdir, profile string) (possess.GateHandle, error) {
	if workdir == "" {
		workdir, _ = os.Getwd()
	}
	if profile == "" {
		profile = "standard"
	}
	id := taskID(workdir, task)

	for _, phase := range []possess.Phase{
		possess.PhaseDiscover,
		possess.PhasePlan,
		possess.PhaseExecute,
		possess.PhaseVerify,
	} {
		// Resume support: skip phases already marked done on disk.
		// We must re-check the disk for each iteration because
		// `spawnOnePhase` reloads/saves the state file inside itself.
		if st, err := loadPossessState(workdir, id); err == nil && alreadyDone(st, string(phase)) {
			continue
		}
		h, err := spawnOnePhase(phase, task, workdir)
		if err != nil {
			return h, fmt.Errorf("possess_async phase %s: %w", phase, err)
		}
		_ = h // each call already persisted state
	}

	// Reload the latest state from disk before appending the
	// async-loop record. spawnOnePhase rewrote the file inside each
	// loop body, so any local copy here would be stale and overwrite
	// all four phase results with the initial pending entries.
	st, err := loadPossessState(workdir, id)
	if err != nil {
		return possess.GateHandle{}, fmt.Errorf("reload state after phases: %w", err)
	}
	st.Profile = profile
	st.Phases["async-loop"] = &phaseResult{
		Phase: "async-loop", Status: "done", StartedAt: time.Now(), EndedAt: time.Now(),
	}
	if err := savePossessState(st); err != nil {
		return possess.GateHandle{}, err
	}
	return possess.GateHandle{
		Ticket:    possess.GateTicket(id),
		Phase:     possess.PhaseVerify,
		StatePath: possess.StatePathFor(workdir, possess.GateTicket(id)),
		StartedAt: st.StartedAt,
	}, nil
}

// Status forwards to the AsyncGate implementation (same on-disk layout).
func (asyncPossess) Status(ticket possess.GateTicket, workdir string) (possess.Status, error) {
	return asyncGateInstance.Status(ticket, workdir)
}

// Cancel forwards to the AsyncGate implementation.
func (asyncPossess) Cancel(ticket possess.GateTicket, workdir string) error {
	return asyncGateInstance.Cancel(ticket, workdir)
}

// mcpPossessAsync is the MCP-router entry point for
// `mcp__radiant__possess_async`. Runs all four phases offline, returns
// a ticket, persists final state.
func mcpPossessAsync(args json.RawMessage) mcpResponse {
	var a struct {
		Task    string `json:"task"`
		Workdir string `json:"workdir"`
		Profile string `json:"profile"`
	}
	_ = json.Unmarshal(args, &a)
	if a.Task == "" {
		return mcpResponse{JSONRPC: "2.0", Error: &mcpError{
			Code: -32602,
			Message: "radiant_possess_async: task is required (pass the user's original prompt verbatim)",
		}}
	}

	h, err := selectedPossessAsync().Spawn(a.Task, a.Workdir, a.Profile)
	if err != nil {
		return mcpResponse{JSONRPC: "2.0", Error: &mcpError{
			Code: -32603, Message: "radiant_possess_async: " + err.Error(),
		}}
	}
	body := fmt.Sprintf(
		"task:    %s\nprofile: %s\nworkdir: %s\nticket:  %s\nstate:   %s\nstarted: %s\n",
		a.Task, a.Profile, a.Workdir, h.Ticket, h.StatePath, h.StartedAt.Format("2006-01-02T15:04:05Z07:00"),
	)
	return mcpResponse{
		JSONRPC: "2.0",
		Result: map[string]interface{}{
			"content": []map[string]string{{"type": "text", "text": body}},
		},
	}
}

// alreadyDone reports whether a phase was recorded as done in st.Phases
// (map keyed by phase name).
func alreadyDone(st *possessState, phase string) bool {
	pr, ok := st.Phases[phase]
	if !ok || pr == nil {
		return false
	}
	return pr.Status == "done"
}

// runAsyncPossessForBackend is the in-process entry that callers
// inside cmd_mcp_possess (NOT the MCP router) use when they need to
// run the full 4-phase offline loop instead of the synchronous
// driver. Used by the sync-host auto-routing in
// `runPossessWithBackend` (Hermes TUI) so the harness can return a
// populated *possessState without sampling/createMessage round-
// trips. Returns the canonical state shape so callers can read
// st.CurrentPhase / st.Phases exactly as if the synchronous loop
// had completed.
func runAsyncPossessForBackend(workdir, task, profile string) (*possessState, error) {
	if workdir == "" {
		workdir, _ = os.Getwd()
	}
	h, err := selectedPossessAsync().Spawn(task, workdir, profile)
	if err != nil {
		return nil, err
	}
	id := taskID(workdir, task)
	st, err := loadPossessState(workdir, id)
	if err != nil {
		return nil, fmt.Errorf("sync-host auto-routing: %w", err)
	}
	// Tag the mode so a later audit can spot it.
	st.RunMode = fmt.Sprintf("sync-host-async %s", h.Ticket)
	st.LastPhaseAt = time.Now()
	if err := savePossessState(st); err != nil {
		return nil, err
	}
	return st, nil
}
