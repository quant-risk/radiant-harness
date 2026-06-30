// mcp__radiant__possess_async — async wrapper around the full possess loop.
//
// See docs/PROPOSAL-v3.7.2-async-primitives.md for the design.
//
// v3.7.2-prep stub. Real subprocess plumbing lands in v3.7.2 PR-B.

package main

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/quant-risk/radiant-harness/internal/possess"
)

// mcpPossessAsync handles mcp__radiant__possess_async — fire-and-forget
// wrapper that returns a ticket in <500ms. Host polls
// `radiant_phase_status(ticket=…)` until done.
//
// The synchronous `radiant_possess` will be refactored in v3.7.2 PR-C
// to detect synchronous hosts (Hermes TUI) and internally route through
// this async primitive + polling — eliminating the 120s tool-call
// deadlock caused by sampling-callback round-trips into a blocked TUI.
//
// In v3.7.2-prep this is a stub. v3.7.2-prep behaviour: returns a
// structured "in development" response so callers can detect this
// without crashing.
func mcpPossessAsync(args json.RawMessage) mcpResponse {
	var a struct {
		Task    string `json:"task"`
		Workdir string `json:"workdir"`
		Profile string `json:"profile"`
	}
	_ = json.Unmarshal(args, &a)
	if a.Task == "" {
		return mcpResponse{JSONRPC: "2.0", Error: &mcpError{
			Code:    -32602,
			Message: "radiant_possess_async: task is required (pass the user's original prompt verbatim)",
		}}
	}
	if a.Profile == "" {
		a.Profile = "standard"
	}

	// v3.7.2-prep stub. Real subprocess wiring in PR-B.
	body := "radiant_possess_async is v3.7.2 in-development.\n" +
		"Designed for synchronous TUI hosts (Hermes) where the existing\n" +
		"radiant_possess deadlocks on sampling/createMessage callbacks.\n\n" +
		"Until PR-B lands, use the bounded-primitive hybrid pattern:\n" +
		"  1. mcp__radiant__skill_list\n" +
		"  2. mcp__radiant__skill_load(name=\"<skill>\")\n" +
		"  3. mcp__radiant__init  /  mcp__radiant__create_spec\n" +
		"  4. Python / bash directly to fill [host-agent: ...] markers\n\n" +
		"See CHANGELOG.md [3.7.2-prep] and\n" +
		"docs/PROPOSAL-v3.7.2-async-primitives.md for full design.\n"

	return mcpResponse{
		JSONRPC: "2.0",
		Result: map[string]interface{}{
			"content": []map[string]string{{
				"type": "text",
				"text": fmt.Sprintf("Task:    %s\nProfile: %s\nWorkdir: %s\n\n%s",
					a.Task, a.Profile, a.Workdir, body),
			}},
			"isError": true, // signal "not yet implemented"
		},
	}
}

// Compile-time guard.
var _ = errors.Is(possess.ErrAsyncInDevelopment, possess.ErrAsyncInDevelopment)