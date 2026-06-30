// mcp__radiant__run_gate — async primitive for synchronous hosts.
//
// See docs/PROPOSAL-v3.7.2-async-primitives.md for the design.
//
// v3.7.2-prep stub: returns ErrAsyncInDevelopment wrapped in a structured
// response so callers can detect "this is in dev" without crashing.
// Real subprocess plumbing lands in v3.7.2 PR-B.

package main

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/quant-risk/radiant-harness/internal/possess"
)

// mcpRunGate handles mcp__radiant__run_gate — run ONE phase
// (discover | plan | execute | verify) in the background and return
// a ticket the host polls via radiant_phase_status.
//
// In v3.7.2-prep this is a stub. The real implementation (PR-B) will:
//   - Spawn a subprocess running the phase (offline, no sampling callback)
//   - Persist state to .radiant-harness/state/<ticket>/state.json
//   - Return immediately with handle {ticket, phase, state_path}
//   - Poll via `Status()` until phase reaches done | error
//
// v3.7.2-prep behaviour: returns a structured "in development" response
// so callers can detect this without parsing error strings.
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

	// v3.7.2-prep stub. Real subprocess wiring in PR-B.
	body := "radiant_run_gate is v3.7.2 in-development.\n" +
		"This stub confirms your call routed correctly (phase=" + a.Phase + ").\n" +
		"Real implementation lands in v3.7.2 PR-B (see docs/PROPOSAL-v3.7.2-async-primitives.md).\n" +
		"Until then, use the bounded-primitive hybrid pattern documented in CHANGELOG [3.7.2-prep]:\n" +
		"  1. radiant_skill_list / radiant_skill_load\n" +
		"  2. radiant_init / radiant_create_spec\n" +
		"  3. Python / bash directly to fill [host-agent: ...] markers\n"

	return mcpResponse{
		JSONRPC: "2.0",
		Result: map[string]interface{}{
			"content": []map[string]string{{"type": "text", "text": body}},
			"isError": true, // signal "not yet implemented" without crashing
		},
	}
}

// Compile-time guard: possess.ErrAsyncInDevelopment is part of the public
// surface — keep import alive so future refactors don't silently drop it.
var _ = errors.Is(possess.ErrAsyncInDevelopment, possess.ErrAsyncInDevelopment)