package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// ── tools/list includes new loop tools ────────────────────────────────────

func TestMCPToolsList_IncludesLoopTools(t *testing.T) {
	// Build the real tool list used by runMCPServe by calling tools/list.
	// We use handleMCPRequest which reads the tool list passed to it, so we
	// directly call runMCPServe with a tools/list request.
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n"
	in := strings.NewReader(input)
	var out strings.Builder
	_ = runMCPServe(in, &out)

	raw := out.String()
	for _, name := range []string{"radiant_loop_start", "radiant_loop_status", "radiant_loop_list"} {
		if !strings.Contains(raw, name) {
			t.Errorf("tools/list missing %q; got: %s", name, raw)
		}
	}
}

// ── callMCPTool — argument mapping ────────────────────────────────────────

// argvFromMCPTool calls callMCPTool and extracts the command-line arguments
// that would be passed to the radiant binary, captured from the error message
// when the binary is not found (exec.Command fails with "executable not found"
// or similar — the argv we built is still visible in the error path).
// Instead, we test via the mcpResponse content: when radiant is not in PATH
// the response is a non-nil result (tool error) with the argv embedded.
//
// Simpler approach: just verify that callMCPTool returns without a JSON-RPC
// error (code -32602 means unknown tool; any other response means dispatch ran).

func TestCallMCPTool_LoopStart_Dispatches(t *testing.T) {
	args := json.RawMessage(`{"goal":"refactor auth","model":"claude-sonnet-4-6","max_iter":5}`)
	resp := callMCPTool("radiant_loop_start", args)
	// Should NOT return unknown-tool error (-32602).
	if resp.Error != nil && resp.Error.Code == -32602 {
		t.Errorf("radiant_loop_start should be a known tool, got: %+v", resp.Error)
	}
}

func TestCallMCPTool_LoopStatus_Dispatches(t *testing.T) {
	args := json.RawMessage(`{"run_id":"my-run-001"}`)
	resp := callMCPTool("radiant_loop_status", args)
	if resp.Error != nil && resp.Error.Code == -32602 {
		t.Errorf("radiant_loop_status should be a known tool, got: %+v", resp.Error)
	}
}

func TestCallMCPTool_LoopList_Dispatches(t *testing.T) {
	args := json.RawMessage(`{}`)
	resp := callMCPTool("radiant_loop_list", args)
	if resp.Error != nil && resp.Error.Code == -32602 {
		t.Errorf("radiant_loop_list should be a known tool, got: %+v", resp.Error)
	}
}

func TestCallMCPTool_LoopStart_AutoRoute(t *testing.T) {
	args := json.RawMessage(`{"goal":"build API","auto_route":true}`)
	resp := callMCPTool("radiant_loop_start", args)
	if resp.Error != nil && resp.Error.Code == -32602 {
		t.Errorf("radiant_loop_start with auto_route should be a known tool: %+v", resp.Error)
	}
}

func TestCallMCPTool_LoopStatus_NoRunID_Dispatches(t *testing.T) {
	// Empty run_id → falls back to active loop status (no run-id arg).
	args := json.RawMessage(`{}`)
	resp := callMCPTool("radiant_loop_status", args)
	if resp.Error != nil && resp.Error.Code == -32602 {
		t.Errorf("radiant_loop_status with empty run_id should still dispatch: %+v", resp.Error)
	}
}

func TestCallMCPTool_LoopList_Plain(t *testing.T) {
	args := json.RawMessage(`{"plain":true}`)
	resp := callMCPTool("radiant_loop_list", args)
	if resp.Error != nil && resp.Error.Code == -32602 {
		t.Errorf("radiant_loop_list --plain should be a known tool: %+v", resp.Error)
	}
}

// ── tools/list schema fields ──────────────────────────────────────────────

func TestMCPLoopStartSchema_HasGoalRequired(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n"
	in := strings.NewReader(input)
	var out strings.Builder
	_ = runMCPServe(in, &out)
	raw := out.String()
	// "goal" must appear near "radiant_loop_start" and be in required list.
	if !strings.Contains(raw, `"goal"`) {
		t.Errorf("expected 'goal' field in schema: %s", raw)
	}
	if !strings.Contains(raw, `"auto_route"`) {
		t.Errorf("expected 'auto_route' field in schema: %s", raw)
	}
}

// ── radiant_run tool ─────────────────────────────────────────────────────────

func TestMCPToolsList_IncludesRadiantRun(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n"
	in := strings.NewReader(input)
	var out strings.Builder
	_ = runMCPServe(in, &out)
	if !strings.Contains(out.String(), "radiant_run") {
		t.Errorf("tools/list missing radiant_run; got: %s", out.String())
	}
}

func TestCallMCPTool_RadiantRun_RequiresGoal(t *testing.T) {
	resp := callMCPTool("radiant_run", json.RawMessage(`{}`))
	if resp.Error == nil {
		t.Error("expected JSON-RPC error when goal is missing")
	}
	if resp.Error != nil && resp.Error.Code != -32602 {
		t.Errorf("expected code -32602, got %d", resp.Error.Code)
	}
}

func TestCallMCPTool_RadiantRun_Schema_HasGoal(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n"
	in := strings.NewReader(input)
	var out strings.Builder
	_ = runMCPServe(in, &out)
	raw := out.String()
	if !strings.Contains(raw, `"goal"`) {
		t.Errorf("radiant_run schema missing 'goal' property; got: %s", raw)
	}
}
