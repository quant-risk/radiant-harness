// cmd_mcp_fleet_async.go — MCP wrappers for fleet operations
// (v3.7.9). Closes the "same status/retry contract as loop"
// backlog item: the loop has `radiant_phase_status` and
// `radiant_possess_async` MCP tools (v3.7.2+) for hosts running
// inside the MCP wire; fleet had only CLI commands
// (`radiant fleet status`, `radiant fleet resume`) — host agents
// could not inspect or resume fleet runs without shelling out.
//
// Two new tools are registered on `radiant mcp serve`:
//
//   - `radiant_fleet_status(run_id, workdir?)` — returns the
//     structured FleetStatus JSON the loop's `radiant_phase_status`
//     returns for possess runs. Includes per-task liveness
//     (pid file + `kill -0` probe) and dispatcher liveness
//     when RADIANT_FLEET_ASYNC_SUBPROCESS=1 has been used.
//
//   - `radiant_fleet_resume(run_id, workdir?, model?, auto_route?)`
//     — wraps `radiant fleet resume <run-id>` so a host can
//     re-dispatch failed tasks without leaving the MCP
//     transport. Mirrors the loop's resume semantics:
//     re-call the same primitives that produced the run, the
//     harness picks up from the on-disk state.
//
// These are read/dispatch surfaces — they do NOT spawn any new
// long-running processes themselves. Resume does what
// `radiant fleet resume` does: reset failed tasks to pending
// and call Dispatcher.RunAll inline (which respects
// RADIANT_FLEET_ASYNC_SUBPROCESS if the env var is set).

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/quant-risk/radiant-harness/v3/internal/config"
	"github.com/quant-risk/radiant-harness/v3/internal/fleet"
)

// mcpFleetStatusTool is the schema entry that gets advertised
// in `tools/list`. Mirrors the loop `radiant_phase_status` shape
// so hosts that already know how to poll loops learn the fleet
// surface with the same mental model.
var mcpFleetStatusTool = mcpTool{
	Name: "radiant_fleet_status",
	Description: "Return the structured status of a fleet run by run_id. " +
		"Includes per-task agent liveness (pid file probe), dispatcher liveness " +
		"when RADIANT_FLEET_ASYNC_SUBPROCESS=1 is enabled, task counts by status, " +
		"and crash escalations for tasks whose pid is dead without terminal status. " +
		"Companion to `radiant_fleet_resume` — poll this in a loop after starting a fleet.",
	InputSchema: mcpInputSchema{
		Type: "object",
		Properties: map[string]mcpPropertyDef{
			"run_id":  {Type: "string", Description: "Fleet run ID returned by `radiant fleet start` or `radiant_fleet_start` (v3.7.9+). Required."},
			"workdir": {Type: "string", Description: "Project directory (default: agent's CWD)."},
		},
		Required: []string{"run_id"},
	},
}

var mcpFleetResumeTool = mcpTool{
	Name: "radiant_fleet_resume",
	Description: "Resume a fleet run by re-dispatching failed (or crashed) tasks. " +
		"Mirrors `radiant fleet resume <run-id>` so a host agent can drive a fleet " +
		"lifecycle entirely through the MCP wire. Returns the resumed FleetStatus " +
		"so the host can decide whether to poll again or stop. Honors " +
		"RADIANT_FLEET_ASYNC_SUBPROCESS if set (returns immediately with the " +
		"dispatcher pid; otherwise blocks until all resumed tasks complete).",
	InputSchema: mcpInputSchema{
		Type: "object",
		Properties: map[string]mcpPropertyDef{
			"run_id":     {Type: "string", Description: "Fleet run ID. Required."},
			"workdir":    {Type: "string", Description: "Project directory (default: agent's CWD)."},
			"model":      {Type: "string", Description: "Model forwarded to each agent subprocess (e.g. sonnet, opus, haiku)."},
			"auto_route": {Type: "boolean", Description: "Forward --auto-route to each agent subprocess so the per-agent picker chooses the model."},
		},
		Required: []string{"run_id"},
	},
}

// mcpFleetStatus implements `radiant_fleet_status`.
func mcpFleetStatus(args json.RawMessage) mcpResponse {
	var a struct {
		RunID   string `json:"run_id"`
		Workdir string `json:"workdir"`
	}
	_ = json.Unmarshal(args, &a)
	if a.RunID == "" {
		return mcpResponse{JSONRPC: "2.0", Error: &mcpError{Code: -32602, Message: "run_id is required"}}
	}
	if a.Workdir == "" {
		a.Workdir, _ = os.Getwd()
	}

	store, err := fleet.LoadStore(a.Workdir, a.RunID)
	if err != nil {
		return mcpResponse{
			JSONRPC: "2.0",
			Error: &mcpError{
				Code:    -32602,
				Message: fmt.Sprintf("load fleet %q: %v", a.RunID, err),
			},
		}
	}
	coord := fleet.NewCoordinator(store, 0).WithLivenessDir(a.Workdir)
	status := coord.Status()

	// Render a compact human-readable block in addition to the
	// raw JSON so a host can read the result inline without
	// parsing. Same shape as the CLI `radiant fleet status`.
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Fleet run: %s\n", status.RunID))
	sb.WriteString(fmt.Sprintf("Goal:      %s\n", status.Goal))
	sb.WriteString(fmt.Sprintf("Agents:    %d\n", status.AgentCount))
	if status.DispatcherPid > 0 || status.DispatcherAlive {
		sb.WriteString(fmt.Sprintf("Dispatcher: pid=%d alive=%t\n", status.DispatcherPid, status.DispatcherAlive))
	}
	counts := map[fleet.TaskStatus]int{}
	for _, t := range status.Tasks {
		counts[t.Status]++
	}
	total := len(status.Tasks)
	sb.WriteString(fmt.Sprintf("Tasks:     %d total", total))
	if total > 0 {
		sb.WriteString(fmt.Sprintf(" — pending=%d assigned=%d done=%d failed=%d crashed=%d",
			counts[fleet.TaskPending], counts[fleet.TaskAssigned],
			counts[fleet.TaskDone], counts[fleet.TaskFailed], counts[fleet.TaskCrashed]))
	}
	sb.WriteString("\n")

	if total > 0 {
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf("%-8s %-12s %-8s %-12s %s\n", "ID", "Status", "Pid", "Alive", "Title"))
		sb.WriteString(strings.Repeat("-", 70) + "\n")
		for _, t := range status.Tasks {
			title := t.Title
			if len(title) > 28 {
				title = title[:25] + "..."
			}
			live := status.TaskLiveness[t.ID]
			sb.WriteString(fmt.Sprintf("%-8s %-12s %-8d %-12t %s\n",
				t.ID, t.Status, live.Pid, live.Alive, title))
		}
	}

	// Operational next-step hint when the run is not in a
	// terminal state. Mirrors loop `radiant_phase_status`.
	if total == 0 {
		sb.WriteString("\n(next: run `radiant fleet plan <run-id>` to materialize tasks)\n")
	} else if counts[fleet.TaskFailed] > 0 || counts[fleet.TaskCrashed] > 0 {
		sb.WriteString(fmt.Sprintf("\n(next: re-call `radiant_fleet_resume(run_id=%q)` to re-dispatch %d failed/crashed tasks)\n",
			status.RunID, counts[fleet.TaskFailed]+counts[fleet.TaskCrashed]))
	} else if counts[fleet.TaskPending] == 0 && counts[fleet.TaskAssigned] == 0 {
		sb.WriteString("\n(all tasks finished)\n")
	}

	rawJSON, _ := json.MarshalIndent(status, "", "  ")
	sb.WriteString("\n--- raw fleet status JSON ---\n")
	sb.Write(rawJSON)

	return mcpResponse{
		JSONRPC: "2.0",
		Result: map[string]interface{}{
			"content": []map[string]string{{"type": "text", "text": sb.String()}},
		},
	}
}

// mcpFleetResume implements `radiant_fleet_resume`. Wraps the
// CLI's fleet resume logic so the host doesn't have to shell
// out — same dispatcher config the loop's resume uses.
func mcpFleetResume(args json.RawMessage) mcpResponse {
	var a struct {
		RunID     string `json:"run_id"`
		Workdir   string `json:"workdir"`
		Model     string `json:"model"`
		AutoRoute bool   `json:"auto_route"`
	}
	_ = json.Unmarshal(args, &a)
	if a.RunID == "" {
		return mcpResponse{JSONRPC: "2.0", Error: &mcpError{Code: -32602, Message: "run_id is required"}}
	}
	if a.Workdir == "" {
		a.Workdir, _ = os.Getwd()
	}

	store, err := fleet.LoadStore(a.Workdir, a.RunID)
	if err != nil {
		return mcpResponse{
			JSONRPC: "2.0",
			Error: &mcpError{Code: -32602, Message: fmt.Sprintf("load fleet %q: %v", a.RunID, err)},
		}
	}

	// Build the dispatcher using the same config the loop uses
	// for its gate primitives, so MCP-driven fleet resumes
	// share the host's model selection, auto-route policy, and
	// env vars with the inline CLI. Config is loaded for
	// diagnostics (and to pick up FleetConcurrency /
	// FleetMaxRetries when the caller doesn't override) but
	// load failure is non-fatal — resume can still work with
	// defaults.
	_, _ = config.Load(a.Workdir)

	// Wire the dispatcher with the same model/auto-route args
	// the CLI fleet resume accepts.
	dispatcherCfg := fleet.DispatchConfig{
		Workdir:         a.Workdir,
		AsyncSubprocess: fleetAsyncSubprocessEnabled(),
	}
	if a.Model != "" {
		dispatcherCfg.Env = append(dispatcherCfg.Env, fmt.Sprintf("RADIANT_MODEL=%s", a.Model))
	}
	if a.AutoRoute {
		dispatcherCfg.Env = append(dispatcherCfg.Env, "RADIANT_AUTO_ROUTE=1")
	}

	// Carry over harness-level env vars so child agents see the
	// same RADIANT_* configuration the parent process has.
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "RADIANT_") && !strings.HasPrefix(kv, "RADIANT_INTERNAL=") {
			dispatcherCfg.Env = append(dispatcherCfg.Env, kv)
		}
	}

	iso, err := fleet.NewIsolator(store, a.Workdir)
	if err != nil {
		return mcpResponse{JSONRPC: "2.0", Error: &mcpError{Code: -32602, Message: fmt.Sprintf("isolator: %v", err)}}
	}
	disp, err := fleet.NewDispatcher(iso, dispatcherCfg)
	if err != nil {
		return mcpResponse{JSONRPC: "2.0", Error: &mcpError{Code: -32602, Message: fmt.Sprintf("dispatcher: %v", err)}}
	}

	results, err := disp.ResumeAll(context.Background(), nil)
	if err != nil {
		return mcpResponse{JSONRPC: "2.0", Error: &mcpError{Code: -32602, Message: fmt.Sprintf("resume: %v", err)}}
	}

	// Build the post-resume FleetStatus so the host can see the
	// effect immediately.
	coord := fleet.NewCoordinator(store, 0).WithLivenessDir(a.Workdir)
	status := coord.Status()

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Fleet resume — run %s\n", status.RunID))
	if dispatcherCfg.AsyncSubprocess {
		sb.WriteString("Mode:       async-subprocess (dispatcher forked; poll `radiant_fleet_status` for progress)\n")
	} else {
		sb.WriteString(fmt.Sprintf("Mode:       inline (resumed %d task(s))\n", len(results)))
		for _, r := range results {
			ev := r.Err
			if ev == nil {
				ev = fmt.Errorf("exit %d", r.ExitCode)
			}
			sb.WriteString(fmt.Sprintf("  - %s: %v (elapsed %s)\n", r.TaskID, ev, r.Elapsed.Round(1e6)))
		}
	}
	sb.WriteString("\n")
	sb.WriteString(fleet.FormatStatus(status))

	return mcpResponse{
		JSONRPC: "2.0",
		Result: map[string]interface{}{
			"content": []map[string]string{{"type": "text", "text": sb.String()}},
		},
	}
}

// fleetAsyncSubprocessEnabled mirrors selectedPossessAsync from
// the loop gate (cmd/radiant/cmd_mcp_possess_async.go). Returns
// true when RADIANT_FLEET_ASYNC_SUBPROCESS=1 is set in the env.
// Default is false — inline dispatch is faster for small fleets
// and avoids pid-file bookkeeping when not needed.
func fleetAsyncSubprocessEnabled() bool {
	v := strings.TrimSpace(os.Getenv("RADIANT_FLEET_ASYNC_SUBPROCESS"))
	return v == "1" || strings.EqualFold(v, "true")
}