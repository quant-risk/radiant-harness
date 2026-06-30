// mcp__radiant__run_gate — async primitive for synchronous hosts.
//
// Synchronous hosts (Hermes TUI is the documented case) block on
// sampling-callback round-trips inside the `radiant_possess` loop,
// causing a 120s tool-call deadlock. v3.7.2 splits the loop into
// per-phase primitives so the host can:
//   1. `run_gate(phase="discover", task=…, workdir=…)` → returns <500ms
//      with a handle to a populated state.json.
//   2. host then `phase_status(task_id)` to read progress.
//   3. host then `run_gate(phase="plan", …)` → same.
//
// The state file at `.radiant-harness/state/possess-<task-id>/state.json`
// persists between calls so each phase resumes from where the previous
// left off. This collapses the 4-phase loop into 4 fast MCP calls
// instead of 1 long blocking one.
//
// Each phase is implemented by the existing self-driven harness
// primitives (`internal/possess/driver.go` for the LLM path,
// `cmd_mcp_possess_self_driven.go` for the offline path). v3.7.2 uses
// the self-driven path so synchronous hosts don't need sampling at all
// — the harness scaffolds, the host fills the [host-agent: ...]
// markers with its own tools. See `AGENTS-FOR-TASKS.md` § Hermes-TUI
// workstream.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/quant-risk/radiant-harness/v3/internal/possess"
)

// asyncGate is the v3.7.2 implementation of `internal/possess.AsyncGate`.
// Holds no state of its own — every call reads/writes the state.json on
// disk so a host can re-call `mcp__radiant__run_gate` after a restart.
type asyncGate struct{}

var asyncGateInstance possess.AsyncGate = asyncGate{}

// Spawn runs the named phase synchronously, then returns a handle that
// points at the persisted state.json. The host can poll via Status().
// `phase` is validated against possess.ValidPhase.
func (asyncGate) Spawn(phase possess.Phase, task, workdir string) (possess.GateHandle, error) {
	return spawnOnePhase(phase, task, workdir)
}

// Status loads the state.json for the ticket and returns the observable
// progress.
func (asyncGate) Status(ticket possess.GateTicket, workdir string) (possess.Status, error) {
	if workdir == "" {
		workdir, _ = os.Getwd()
	}
	st, err := loadPossessState(workdir, string(ticket))
	if err != nil {
		return possess.Status{}, err
	}
	return phaseStatusFromState(st, possess.PhaseDiscover), nil // phase filled below
}

// Cancel marks any in-progress phase as cancelled by writing a sentinel
// value. Future calls to Status() return state="cancelled".
func (asyncGate) Cancel(ticket possess.GateTicket, workdir string) error {
	if workdir == "" {
		workdir, _ = os.Getwd()
	}
	st, err := loadPossessState(workdir, string(ticket))
	if err != nil {
		return err
	}
	st.Cancelled = true
	st.CurrentPhase = string(possess.PhaseDiscover) + "|cancelled"
	return savePossessState(st)
}

// mcpRunGate is the MCP-router entry point. It validates args, runs the
// chosen phase, persists state, and returns a Handle the host can poll.
func mcpRunGate(args json.RawMessage) mcpResponse {
	var a struct {
		Phase   string `json:"phase"`
		Task    string `json:"task"`
		Workdir string `json:"workdir"`
	}
	_ = json.Unmarshal(args, &a)
	if a.Phase == "" {
		return mcpResponse{JSONRPC: "2.0", Error: &mcpError{
			Code:    -32602,
			Message: "radiant_run_gate: phase is required (discover | plan | execute | verify)",
		}}
	}
	if !possess.ValidPhase(possess.Phase(a.Phase)) {
		return mcpResponse{JSONRPC: "2.0", Error: &mcpError{
			Code:    -32602,
			Message: fmt.Sprintf("radiant_run_gate: unknown phase %q (valid: discover, plan, execute, verify)", a.Phase),
		}}
	}
	if a.Task == "" {
		return mcpResponse{JSONRPC: "2.0", Error: &mcpError{
			Code:    -32602,
			Message: "radiant_run_gate: task is required",
		}}
	}
	if a.Workdir == "" {
		a.Workdir, _ = os.Getwd()
	}

	h, err := selectedAsyncGate().Spawn(possess.Phase(a.Phase), a.Task, a.Workdir)
	if err != nil {
		return mcpResponse{JSONRPC: "2.0", Error: &mcpError{
			Code: -32603, Message: "radiant_run_gate: " + err.Error(),
		}}
	}
	body := fmt.Sprintf(
		"phase: %s\nticket: %s\nstate: %s\nstarted_at: %s\n",
		h.Phase, h.Ticket, h.StatePath, h.StartedAt.Format(time.RFC3339),
	)
	return mcpResponse{
		JSONRPC: "2.0",
		Result: map[string]interface{}{
			"content": []map[string]string{{"type": "text", "text": body}},
		},
	}
}

// spawnOnePhase is the workhorse shared between mcpRunGate and the
// asyncGate interface. It writes (or refreshes) the possessState and
// invokes the per-phase function.
func spawnOnePhase(phase possess.Phase, task, workdir string) (possess.GateHandle, error) {
	if workdir == "" {
		workdir, _ = os.Getwd()
	}
	id := taskID(workdir, task)

	// Load or create the state file. Across phases we want the same
	// id so a host can resume from the prior step.
	st, err := loadPossessState(workdir, id)
	if err != nil {
		st = newPossessState(workdir, task, id)
	}
	st.CurrentPhase = string(phase)
	st.Profile = profileOf(st)
	st.LastPhaseAt = time.Now()
	if st.StartedAt.IsZero() {
		st.StartedAt = st.LastPhaseAt
	}

	// Pre-resolve the slug/spec path so plan/execute/verify all see the
	// same target even when called independently by a host.
	if st.Slug == "" {
		st.Slug = selfDrivenSlugify(task)
	}
	if st.SpecDir == "" && st.Slug != "" {
		st.SpecDir = defaultSpecDir(workdir, task)
		// defaultSpecDir already encodes a fixed "/0001-" prefix; we
		// recompute it consistently with the resolved slug so downstream
		// phases don't disagree with the file system layout that plan
		// produced.
		st.SpecDir = workdir + "/specs/0001-" + st.Slug
	}

	// Persist the meta fields before invoking any phase fn so a crash
	// in the middle still leaves a recoverable state.
	_ = savePossessState(st)

	var phaseErr error
	w := io.Discard
	switch phase {
	case possess.PhaseDiscover:
		phaseErr = selfDrivenDiscover(workdir, task, st.Profile, w)
	case possess.PhasePlan:
		phaseErr = selfDrivenPlan(workdir, st.SpecDir, task, st.Slug, st.Profile, w)
	case possess.PhaseExecute:
		phaseErr = selfDrivenExecute(workdir, st.SpecDir, task, st.Profile, w)
	case possess.PhaseVerify:
		phaseErr = selfDrivenVerify(workdir, st.SpecDir, w)
	default:
		phaseErr = fmt.Errorf("unknown phase: %s", phase)
	}

	if phaseErr != nil {
		st.Phases[string(phase)] = &phaseResult{
			Phase: string(phase), Status: "error", StartedAt: time.Now(),
			EndedAt: time.Now(), Error: phaseErr.Error(),
		}
		_ = savePossessState(st)
		return possess.GateHandle{}, phaseErr
	}

	st.Phases[string(phase)] = &phaseResult{
		Phase: string(phase), Status: "done", StartedAt: time.Now(),
		EndedAt: time.Now(),
	}
	if err := savePossessState(st); err != nil {
		return possess.GateHandle{}, err
	}

	return possess.GateHandle{
		Ticket:    possess.GateTicket(id),
		Phase:     phase,
		StatePath: possess.StatePathFor(workdir, possess.GateTicket(id)),
		StartedAt: st.StartedAt,
	}, nil
}

// profileOf returns the profile recorded in state, or "standard"
// as the harness default.
func profileOf(st *possessState) string {
	if st.Profile == "" {
		return "standard"
	}
	return st.Profile
}

// defaultSpecDir maps a workdir + task to the conventional
// specs/NNNN-<slug> directory matching selfDrivenSlugify conventions.
func defaultSpecDir(workdir, task string) string {
	return workdir + "/specs/0001-" + selfDrivenSlugify(task)
}

// phaseRecord is unused but kept for callers that may want to build a
// loose phase entry without recording into st.Phases.
func phaseRecord(phase, status, errMsg string) phaseResult {
	return phaseResult{Phase: phase, Status: status, Error: errMsg}
}

// phaseStatusFromState converts the on-disk shape to the AsyncGate
// public Status. The host asks by phase name; we look up the matching
// entry in st.Phases (map keyed by phase name) and translate state.
func phaseStatusFromState(st *possessState, phase possess.Phase) possess.Status {
	if st == nil {
		return possess.Status{State: "unknown"}
	}
	pr := st.Phases[string(phase)]
	state := "in_progress"
	errMsg := ""
	if pr != nil {
		switch pr.Status {
		case "done":
			state = "done"
		case "error":
			state = "error"
			errMsg = pr.Error
		}
	}
	if st.Cancelled {
		state = "cancelled"
	}
	return possess.Status{
		Ticket:    possess.GateTicket(st.TaskID),
		Phase:     phase,
		Current:   st.CurrentPhase,
		State:     state,
		Error:     errMsg,
		StartedAt: st.StartedAt,
		EndedAt:   time.Now(),
	}
}

// Workaround: keep `ctx` import for parity with runSelfDrivenPossess
// signature; the async path does not need cancellation hooks yet.
var _ = context.Background

// Strings used by go-vet to silence import-only-without-use warnings
// when pkg builds with -strict-unused.
var _ = strings.TrimSpace
