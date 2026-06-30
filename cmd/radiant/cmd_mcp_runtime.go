// MCP runtime. runMCPServe here is the sole MCP server entry point.
// It dispatches every tool call through `callMCPTool`, which routes
// inference via MCP sampling/createMessage back to the host agent.

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/quant-risk/radiant-harness/v3/internal/llm"
	"github.com/quant-risk/radiant-harness/v3/internal/skill"
)

// MCP server. Seven bounded primitives + one driver-driven possess entry
// point. No HTTP LLM, no API keys.
func runMCPServe(in io.Reader, out io.Writer, samplingMode bool, samplingTimeout time.Duration, modelHint string) error {
	d := &mcpDispatcher{}
	if samplingMode {
		d.sampling = llm.NewSamplingBackend(llm.SamplingOptions{
			ModelHint: modelHint,
			MaxTokens: 8192,
			Timeout:   samplingTimeout,
			Out:       out,
		})
	}

	tools := []mcpTool{
		// ----- 4 new bounded primitives (added in v3.3.0) ------
		{Name: "radiant_possess", Description: "Take ownership of the agent for a bounded-phase run of the user's task. The harness drives the agent through discover → plan → execute → verify, one sampling round-trip per phase, state persisted to .radiant-harness/state/possess-<task-id>.json. Use this whenever the user asks for non-trivial work in a project that has radiant-harness installed.", InputSchema: mcpInputSchema{
			Type: "object",
			Properties: map[string]mcpPropertyDef{
				"task":    {Type: "string", Description: "The user's original task prompt (verbatim). Required. This is what the harness reads and decomposes."},
				"workdir": {Type: "string", Description: "Project directory the task runs in. Defaults to the agent's CWD."},
				"profile": {Type: "string", Description: "Execution profile: lean | standard | thorough (default: standard)."},
			},
			Required: []string{"task"},
		}},
		// ----- v3.7.2 async/offline primitives ------
		{Name: "radiant_run_gate", Description: "Run ONE possess phase (discover | plan | execute | verify) through the self-driven offline path and return a ticket the host polls via radiant_phase_status. NO sampling/createMessage round-trip — phase work is persisted to .radiant-harness/state/<ticket>/state.json. Designed to unblock synchronous TUI hosts where the full radiant_possess cannot process sampling callbacks during a tool call.", InputSchema: mcpInputSchema{
			Type: "object",
			Properties: map[string]mcpPropertyDef{
				"phase":   {Type: "string", Description: "Phase to run: discover | plan | execute | verify. Required."},
				"task":    {Type: "string", Description: "The user's original task prompt (verbatim). Required."},
				"workdir": {Type: "string", Description: "Project directory. Defaults to agent's CWD."},
			},
			Required: []string{"phase", "task"},
		}},
		{Name: "radiant_possess_async", Description: "Offline wrapper around the full 4-phase possess loop. Runs discover, plan, execute, and verify through self-driven scaffolding, persists state after each phase, and returns a ticket for radiant_phase_status. Use when a synchronous host cannot safely run sampling-backed radiant_possess.", InputSchema: mcpInputSchema{
			Type: "object",
			Properties: map[string]mcpPropertyDef{
				"task":    {Type: "string", Description: "The user's original task prompt (verbatim). Required."},
				"workdir": {Type: "string", Description: "Project directory. Defaults to agent's CWD."},
				"profile": {Type: "string", Description: "Execution profile: lean | standard | thorough (default: standard)."},
			},
			Required: []string{"task"},
		}},
		{Name: "radiant_phase_status", Description: "Return the current state of a radiant_possess run by task_id (the 16-char prefix shown in the harness trace).", InputSchema: mcpInputSchema{
			Type: "object",
			Properties: map[string]mcpPropertyDef{
				"task_id": {Type: "string", Description: "16-char task ID returned by radiant_possess."},
				"workdir": {Type: "string", Description: "Project directory (default: agent's CWD)."},
			},
			Required: []string{"task_id"},
		}},
		{Name: "radiant_skill_list", Description: "Enumerate the bundled domain skills (e.g. credit-risk, nova-feature, regulatory, ml). Always call this once before invoking radiant_possess on a non-trivial task so the harness picks the right methodology.", InputSchema: mcpInputSchema{
			Type: "object",
			Properties: map[string]mcpPropertyDef{
				"filter": {Type: "string", Description: "Optional substring filter against skill name + description."},
			},
		}},
		{Name: "radiant_skill_load", Description: "Return the full text (SKILL.md + frontmatter) of one bundled skill. Read it before deciding which skills to apply during the discover phase.", InputSchema: mcpInputSchema{
			Type: "object",
			Properties: map[string]mcpPropertyDef{
				"name": {Type: "string", Description: "Skill name (e.g. credit-risk, ml, regulatory). Required."},
			},
			Required: []string{"name"},
		}},
		// ----- v3.7.9 fleet async primitives (parity with loop) ------
		mcpFleetStatusTool,
		mcpFleetResumeTool,
	}

	var encMu sync.Mutex
	enc := json.NewEncoder(out)
	encode := func(v mcpResponse) {
		encMu.Lock()
		_ = enc.Encode(v)
		encMu.Unlock()
	}

	if samplingMode {
		d.sampling.SetWriteMu(&encMu)
	}

	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		if samplingMode && llm.IsSamplingResponse(line) {
			d.sampling.Dispatch(line)
			continue
		}
		var req mcpRequest
		if err := json.Unmarshal(line, &req); err != nil {
			encode(mcpResponse{JSONRPC: "2.0", Error: &mcpError{Code: -32700, Message: "parse error"}})
			continue
		}
		if samplingMode && req.Method == "tools/call" {
			go func(req mcpRequest) {
				encode(handleMCPRequestLight(req, tools, d))
			}(req)
			continue
		}
		resp := handleMCPRequestLight(req, tools, d)
		if resp.suppress {
			continue // notification — do not encode a response
		}
		encode(resp)
	}
	return scanner.Err()
}

func handleMCPRequestLight(req mcpRequest, tools []mcpTool, d *mcpDispatcher) mcpResponse {
	switch req.Method {
	case "notifications/initialized", "notifications/cancelled", "notifications/progress":
		// Per JSON-RPC 2.0, a notification (no `id`) MUST NOT be answered.
		// The MCP initialize handshake sends `notifications/initialized`
		// right after the server replies to `initialize`. Returning any
		// response here (even an error) causes the well-behaved client to
		// throw away the next genuine response (tools/list, etc.). We
		// therefore send back a zero-value `mcpResponse{}` and the caller
		// (`runMCPServe`) treats it as "do not encode".
		return mcpResponse{JSONRPC: "2.0", suppress: true}
	case "initialize":
		return mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"serverInfo": map[string]string{
					"name":    "radiant-harness",
					"version": version,
				},
				"capabilities": map[string]interface{}{"tools": map[string]interface{}{}},
			},
		}
	case "tools/list":
		return mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]interface{}{"tools": tools},
		}
	case "tools/call":
		var params struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return mcpResponse{JSONRPC: "2.0", ID: req.ID, Error: &mcpError{Code: -32602, Message: "invalid params"}}
		}
		result := callMCPToolLight(params.Name, params.Arguments, d)
		result.ID = req.ID
		return result
	default:
		return mcpResponse{JSONRPC: "2.0", ID: req.ID, Error: &mcpError{Code: -32601, Message: "method not found: " + req.Method}}
	}
}

func callMCPToolLight(name string, args json.RawMessage, d *mcpDispatcher) mcpResponse {
	switch name {
	case "radiant_possess":
		return mcpPossessWithBackend(args, d.backend())
	case "radiant_possess_async":
		return mcpPossessAsync(args)
	case "radiant_run_gate":
		return mcpRunGate(args)
	case "radiant_phase_status":
		return mcpPhaseStatus(args)
	case "radiant_skill_list":
		return mcpSkillList(args)
	case "radiant_skill_load":
		return mcpSkillLoad(args)
	case "radiant_fleet_status":
		return mcpFleetStatus(args)
	case "radiant_fleet_resume":
		return mcpFleetResume(args)
	}
	return mcpResponse{JSONRPC: "2.0", Error: &mcpError{Code: -32602, Message: "unknown tool: " + name}}
}

// mcpPossessWithBackend implements the new mcp__radiant__possess tool —
// bounded-phase possession of the host agent for the user's task. Each
// phase = exactly one sampling/createMessage round-trip; state is
// persisted between phases so a timeout/crash can resume.
func mcpPossessWithBackend(args json.RawMessage, backend llm.Backend) mcpResponse {
	var a struct {
		Task    string `json:"task"`
		Workdir string `json:"workdir"`
		Profile string `json:"profile"`
	}
	_ = json.Unmarshal(args, &a)
	if a.Task == "" {
		return mcpResponse{JSONRPC: "2.0", Error: &mcpError{Code: -32602, Message: "task is required (pass the user's original prompt verbatim)"}}
	}
	if a.Workdir == "" {
		a.Workdir, _ = os.Getwd()
	}
	if a.Profile == "" {
		a.Profile = "standard"
	}

	id := taskID(a.Workdir, a.Task)
	state, err := runPossessWithBackend(context.Background(), a.Workdir, a.Task, a.Profile, backend, io.Discard)
	out := strings.Builder{}
	out.WriteString(fmt.Sprintf("Run id:        %s\n", id))
	out.WriteString(fmt.Sprintf("Workdir:       %s\n", a.Workdir))
	out.WriteString(fmt.Sprintf("Task:          %s\n", a.Task))
	out.WriteString(fmt.Sprintf("Profile:       %s\n", a.Profile))
	out.WriteString(fmt.Sprintf("Phases done:   %s\n", state.CurrentPhase))
	if state.RunMode != "" {
		out.WriteString(fmt.Sprintf("Mode:          %s\n", state.RunMode))
	}
	if err != nil {
		out.WriteString(fmt.Sprintf("Exit:          %v\n", err))
		return mcpResponse{
			JSONRPC: "2.0",
			Result: map[string]interface{}{
				"content": []map[string]string{{"type": "text", "text": out.String()}},
				"isError": true,
			},
		}
	}
	out.WriteString(fmt.Sprintf("Exit:          success\n"))
	out.WriteString(fmt.Sprintf("Artifacts:     %v\n", state.Artifacts))
	out.WriteString(fmt.Sprintf("State file:    %s\n", possessStatePath(a.Workdir, id)))
	if state.RunMode == "self-driven" {
		out.WriteString(selfDrivenMCPGuidance(a.Workdir, state))
	}

	return mcpResponse{
		JSONRPC: "2.0",
		Result: map[string]interface{}{
			"content": []map[string]string{{"type": "text", "text": out.String()}},
			"isError": false,
		},
	}
}

func selfDrivenMCPGuidance(workdir string, st *possessState) string {
	pending := countHostAgentMarkers(workdir)
	var b strings.Builder
	b.WriteString("\nSelf-driven handoff:\n")
	b.WriteString("  Radiant scaffolded the run because this host does not provide MCP sampling.\n")
	b.WriteString("  The host agent must now use its native tools to replace [host-agent: fill in] markers.\n")
	if st.SpecDir != "" {
		b.WriteString(fmt.Sprintf("  Spec dir:      %s\n", st.SpecDir))
	}
	b.WriteString("  Read first:    .radiant-harness/CONTEXT.md\n")
	b.WriteString("  Then update:   spec.md, tasks.md, scripts/run.sh, docs/README.md, .radiant-harness/handoff.md\n")
	b.WriteString("  Verify with:   scripts/run.sh and radiant_phase_status\n")
	b.WriteString(fmt.Sprintf("  Pending marks: %d\n", pending))
	return b.String()
}

func countHostAgentMarkers(workdir string) int {
	count := 0
	_ = filepath.WalkDir(workdir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if d != nil && d.IsDir() && d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.Contains(path, string(filepath.Separator)+".git"+string(filepath.Separator)) {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		count += strings.Count(string(data), "[host-agent: fill in")
		return nil
	})
	return count
}

// phaseStatusSummary is the v3.7.6 actionable view of a possess run.
// It is included alongside the raw state.json dump so a host agent can
// drive a single `radiant_phase_status` call without having to parse
// the on-disk shape itself. Fields:
//
//   - Status: "done" | "in_progress" | "error" | "cancelled" | "pending"
//   - NextStep: concrete next action the host should take.
//   - ResumeCommand: an exact MCP call (mcp__radiant__possess) the
//     host can issue to resume the run from the failed/pending phase.
//   - PendingFiles: relative paths under workdir that the harness has
//     templated but not yet filled (only populated for self-driven
//     runs that left [host-agent: fill in] markers behind).
//   - PendingMarkerCount: number of [host-agent: fill in] markers
//     still on disk (drives the "host agent must fill before verify"
//     decision).
//   - LastGate: most recent gate (scripts/run.sh or any *_test.go
//     exit) the harness ran. ExitCode + At let the host distinguish
//     "ran green" from "ran red" without re-reading every output.
//   - Phases: compact per-phase status, easier than re-parsing the
//     raw state.json dump.
type phaseStatusSummary struct {
	TaskID             string                `json:"task_id"`
	RunMode            string                `json:"run_mode,omitempty"`
	CurrentPhase       string                `json:"current_phase"`
	Status             string                `json:"status"`
	NextStep           string                `json:"next_step"`
	ResumeCommand      string                `json:"resume_command,omitempty"`
	PendingFiles       []string              `json:"pending_files,omitempty"`
	PendingMarkerCount int                   `json:"pending_marker_count,omitempty"`
	LastGate           *phaseGateSummary     `json:"last_gate,omitempty"`
	Phases             map[string]*phaseMini `json:"phases"`
	StartedAt          string                `json:"started_at,omitempty"`
	LastUpdateAt       string                `json:"last_update_at,omitempty"`
	// Subprocess liveness (v3.7.8+) — populated from
	// `.radiant-harness/pids/<ticket>.pid`. When the run mode is
	// subprocess-backed (RADIANT_ASYNC_SUBPROCESS=1), the parent
	// `radiant mcp serve` process is NOT the one running the phase
	// work — a child `radiant async-runner` is. A status of
	// `in_progress` with SubprocessAlive=false indicates the child
	// died without writing an error to state.json (crash); the
	// host should re-call `radiant_run_gate` to resume.
	SubprocessAlive bool   `json:"subprocess_alive,omitempty"`
	SubprocessPid   int    `json:"subprocess_pid,omitempty"`
}

type phaseGateSummary struct {
	Name     string `json:"name"`
	ExitCode int    `json:"exit_code"`
	At       string `json:"at,omitempty"`
}

type phaseMini struct {
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
	StartedAt string `json:"started_at,omitempty"`
	EndedAt   string `json:"ended_at,omitempty"`
}

// phasePhaseOrder is the canonical discover→plan→execute→verify order
// shared with the run-gate primitives. Kept in lock-step with
// `internal/possess.Phase*` constants.
var phasePhaseOrder = []string{"discover", "plan", "execute", "verify"}

// buildPhaseStatusSummary derives an actionable summary from a
// possessState. The summary is what `radiant_phase_status` returns to
// a host agent so the host can drive a single call without re-parsing
// the on-disk shape. Side-effect-free — no writes.
func buildPhaseStatusSummary(st *possessState, workdir string) phaseStatusSummary {
	if st == nil {
		return phaseStatusSummary{Status: "unknown", NextStep: "no state on disk"}
	}
	out := phaseStatusSummary{
		TaskID:       st.TaskID,
		RunMode:      st.RunMode,
		CurrentPhase: st.CurrentPhase,
		Phases:       map[string]*phaseMini{},
		StartedAt:    st.StartedAt.Format("2006-01-02T15:04:05Z07:00"),
		LastUpdateAt: st.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	for _, p := range phasePhaseOrder {
		pr := st.Phases[p]
		if pr == nil {
			out.Phases[p] = &phaseMini{Status: "pending"}
			continue
		}
		mini := &phaseMini{
			Status:    pr.Status,
			Error:     pr.Error,
			StartedAt: pr.StartedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
		if !pr.EndedAt.IsZero() {
			mini.EndedAt = pr.EndedAt.Format("2006-01-02T15:04:05Z07:00")
		}
		out.Phases[p] = mini
	}
	// Composite status: cancelled > error > crashed > done > in_progress > pending.
	//
	// The "crashed" branch is v3.7.8 — fires only when state.json
	// records the phase as in_progress but the subprocess pid
	// file points at a dead process. Without this check, a
	// crashed child looks like "still in progress" indefinitely.
	switch {
	case st.Cancelled:
		out.Status = "cancelled"
		out.NextStep = fmt.Sprintf(
			"run was cancelled (phase=%s). Restart by re-calling `mcp__radiant__possess` with the same task + workdir.",
			st.CurrentPhase)
		out.ResumeCommand = fmt.Sprintf(
			"mcp__radiant__possess(task=%q, workdir=%q)", st.Task, st.Workdir)
	case hasPhaseError(st):
		bad := firstErroredPhase(st)
		pr := st.Phases[bad]
		out.Status = "error"
		out.NextStep = fmt.Sprintf(
			"phase %q failed: %s. Inspect state.json then re-call `mcp__radiant__possess` to resume from %q.",
			bad, pr.Error, bad)
		out.ResumeCommand = fmt.Sprintf(
			"mcp__radiant__possess(task=%q, workdir=%q)", st.Task, st.Workdir)
	case st.CurrentPhase == "done":
		out.Status = "done"
		out.NextStep = "all four phases complete. Artifacts listed in state.json `artifacts`."
		if len(st.Artifacts) > 0 {
			out.PendingFiles = nil // explicit: nothing pending
		}
	case allPhasesDone(st):
		out.Status = "done"
		out.CurrentPhase = "done"
		st.CurrentPhase = "done"
		out.NextStep = "all four phases complete. Artifacts listed in state.json `artifacts`."
	default:
		out.Status = "in_progress"
		next := nextPendingPhase(st)
		out.NextStep = fmt.Sprintf(
			"waiting on phase %q. Re-call `mcp__radiant__run_gate(phase=%q, ...)` or the full `mcp__radiant__possess` to advance.",
			next, next)
		out.ResumeCommand = fmt.Sprintf(
			"mcp__radiant__run_gate(phase=%q, task=%q, workdir=%q)",
			next, st.Task, st.Workdir)
	}

	// Subprocess liveness (v3.7.8). If a pid file exists, populate
	// SubprocessAlive + SubprocessPid. When the recorded phase is
	// in_progress but the subprocess is dead, escalate status to
	// "crashed" so the host agent can re-call the gate to resume
	// instead of waiting for a phase that will never finish.
	if alive, pid := asyncRunnerLiveness(workdir, st.TaskID); pid > 0 {
		out.SubprocessAlive = alive
		out.SubprocessPid = pid
		if !alive && out.Status == "in_progress" {
			out.Status = "crashed"
			out.NextStep = fmt.Sprintf(
				"subprocess pid=%d died without writing an error to state.json. Re-call `mcp__radiant__run_gate(phase=%q, ...)` to resume from where the crash happened.",
				pid, st.CurrentPhase)
		} else if alive {
			out.NextStep = fmt.Sprintf(
				"%s (subprocess pid=%d alive — last polled at %s)",
				out.NextStep, pid, time.Now().UTC().Format("2006-01-02T15:04:05Z"))
		}
	}

	// Self-driven runs surface pending files + marker count so the
	// host agent knows exactly what to fill before verifying.
	if st.RunMode == "self-driven" || strings.Contains(st.RunMode, "self-driven") ||
		st.RunMode == "sync-host-async" || strings.HasPrefix(st.RunMode, "sync-host-async") {
		out.PendingMarkerCount = countHostAgentMarkers(workdir)
		out.PendingFiles = pendingSelfDrivenFiles(workdir, st)
		if out.Status == "done" && out.PendingMarkerCount > 0 {
			out.Status = "in_progress"
			out.NextStep = fmt.Sprintf(
				"all four phases scaffolded, but %d `[host-agent: fill in]` markers remain. Replace them, then re-run `scripts/run.sh` and `radiant_phase_status`.",
				out.PendingMarkerCount)
		}
	}
	// Last gate: scan executed phase Output for the gate marker the
	// self-driven Execute phase emits at the end of scripts/run.sh.
	if lastGate := extractLastGate(st); lastGate != nil {
		out.LastGate = lastGate
	}
	return out
}

func hasPhaseError(st *possessState) bool {
	for _, p := range phasePhaseOrder {
		if pr := st.Phases[p]; pr != nil && pr.Status == "error" {
			return true
		}
	}
	return false
}

func firstErroredPhase(st *possessState) string {
	for _, p := range phasePhaseOrder {
		if pr := st.Phases[p]; pr != nil && pr.Status == "error" {
			return p
		}
	}
	return st.CurrentPhase
}

func allPhasesDone(st *possessState) bool {
	for _, p := range phasePhaseOrder {
		if pr := st.Phases[p]; pr == nil || pr.Status != "done" {
			return false
		}
	}
	return true
}

func nextPendingPhase(st *possessState) string {
	for _, p := range phasePhaseOrder {
		if pr := st.Phases[p]; pr == nil || pr.Status == "pending" || pr.Status == "in_progress" {
			return p
		}
	}
	return "done"
}

// pendingSelfDrivenFiles lists the canonical files the self-driven
// scaffold writes that the host agent is expected to edit. We list the
// files that exist on disk AND still contain at least one
// [host-agent: fill in marker so a status caller sees only what is
// actually pending, not the full template list.
func pendingSelfDrivenFiles(workdir string, st *possessState) []string {
	candidates := []string{
		filepath.Join(workdir, ".radiant-harness", "CONTEXT.md"),
		filepath.Join(workdir, ".radiant-harness", "handoff.md"),
		filepath.Join(workdir, ".radiant-harness", "verify.md"),
	}
	if st.SpecDir != "" {
		candidates = append(candidates,
			filepath.Join(st.SpecDir, "spec.md"),
			filepath.Join(st.SpecDir, "tasks.md"),
		)
	}
	if st.Slug != "" {
		candidates = append(candidates,
			filepath.Join(workdir, "scripts", "run.sh"),
			filepath.Join(workdir, "docs", "README.md"),
		)
	}
	var out []string
	for _, p := range candidates {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if strings.Contains(string(data), "[host-agent: fill in") {
			out = append(out, p)
		}
	}
	return out
}

// extractLastGate scans the executed phase Output for the gate marker
// the self-driven Execute phase emits at the end of scripts/run.sh.
// Returns nil when no gate evidence is on disk (typical for runs that
// never reached Execute, or for async/host-driven runs where the host
// owns the gate step).
func extractLastGate(st *possessState) *phaseGateSummary {
	pr := st.Phases["execute"]
	if pr == nil || pr.Output == "" {
		return nil
	}
	out := pr.Output
	// Self-driven Execute phase writes a sentinel line:
	//   "[execute] gate: scripts/run.sh exit=<n> at=<RFC3339>"
	// Parse it back out for the status summary.
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "[execute] gate:") {
			continue
		}
		body := strings.TrimPrefix(line, "[execute] gate:")
		body = strings.TrimSpace(body)
		g := &phaseGateSummary{}
		for _, kv := range strings.Fields(body) {
			eq := strings.Index(kv, "=")
			if eq < 0 {
				continue
			}
			k := kv[:eq]
			v := kv[eq+1:]
			switch k {
			case "name":
				g.Name = v
			case "exit":
				n, _ := strconv.Atoi(v)
				g.ExitCode = n
			case "at":
				g.At = v
			}
		}
		if g.Name != "" {
			return g
		}
	}
	return nil
}

// formatPhaseStatusSummary renders a human-readable multi-line summary
// suitable for `content[1].text`. Kept separate from the structured
// JSON so the text format can evolve without breaking callers that
// parse the JSON `summary` field.
func formatPhaseStatusSummary(s phaseStatusSummary) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("status:       %s\n", s.Status))
	if s.RunMode != "" {
		b.WriteString(fmt.Sprintf("run mode:     %s\n", s.RunMode))
	}
	b.WriteString(fmt.Sprintf("current:      %s\n", s.CurrentPhase))
	if s.SubprocessPid > 0 {
		// Subprocess liveness line — only shown when a pid file
		// is recorded (i.e. the run went through the subprocess
		// gate path).
		live := "dead"
		if s.SubprocessAlive {
			live = "alive"
		}
		b.WriteString(fmt.Sprintf("subprocess:   pid=%d %s\n", s.SubprocessPid, live))
	}
	b.WriteString(fmt.Sprintf("next step:    %s\n", s.NextStep))
	if s.ResumeCommand != "" {
		b.WriteString(fmt.Sprintf("resume with:  %s\n", s.ResumeCommand))
	}
	if s.LastGate != nil {
		b.WriteString(fmt.Sprintf("last gate:    %s (exit=%d", s.LastGate.Name, s.LastGate.ExitCode))
		if s.LastGate.At != "" {
			b.WriteString(fmt.Sprintf(", at=%s", s.LastGate.At))
		}
		b.WriteString(")\n")
	}
	if s.PendingMarkerCount > 0 {
		b.WriteString(fmt.Sprintf("pending markers: %d\n", s.PendingMarkerCount))
	}
	if len(s.PendingFiles) > 0 {
		b.WriteString("pending files:\n")
		for _, p := range s.PendingFiles {
			b.WriteString(fmt.Sprintf("  - %s\n", p))
		}
	}
	b.WriteString("phases:\n")
	for _, p := range phasePhaseOrder {
		mini := s.Phases[p]
		if mini == nil {
			continue
		}
		icon := "○"
		switch mini.Status {
		case "done":
			icon = "✓"
		case "error":
			icon = "✗"
		case "in_progress":
			icon = "▶"
		}
		fmt.Fprintf(&b, "  %s %-9s %s", icon, p, mini.Status)
		if mini.Error != "" {
			fmt.Fprintf(&b, "  (%s)", truncate(mini.Error, 80))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// mcpPhaseStatus returns the persisted progress of a previous possess run.
// v3.7.6: returns a `summary` field with the actionable view (next step,
// resume command, pending files/markers, last gate, error/cancel state)
// in addition to the raw state.json dump that earlier versions returned.
// The raw dump is preserved in `content[0].text` for backwards
// compatibility with callers that parse it directly.
func mcpPhaseStatus(args json.RawMessage) mcpResponse {
	var a struct {
		TaskID  string `json:"task_id"`
		Workdir string `json:"workdir"`
	}
	_ = json.Unmarshal(args, &a)
	if a.TaskID == "" {
		return mcpResponse{JSONRPC: "2.0", Error: &mcpError{Code: -32602, Message: "task_id required"}}
	}
	if a.Workdir == "" {
		a.Workdir, _ = os.Getwd()
	}
	st, err := loadPossessState(a.Workdir, a.TaskID)
	if err != nil {
		return mcpResponse{JSONRPC: "2.0", Error: &mcpError{Code: -32604,
			Message: fmt.Sprintf("no run with task_id %s in %s (have you run radiant_possess yet?)", a.TaskID, a.Workdir)}}
	}
	data, _ := json.MarshalIndent(st, "", "  ")
	summary := buildPhaseStatusSummary(st, a.Workdir)
	return mcpResponse{
		JSONRPC: "2.0",
		Result: map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": string(data)},
				{"type": "text", "text": formatPhaseStatusSummary(summary)},
			},
			"summary": summary,
		},
	}
}

// mcpSkillList enumerates the bundled skills visible to the harness.
func mcpSkillList(args json.RawMessage) mcpResponse {
	var a struct {
		Filter string `json:"filter"`
	}
	_ = json.Unmarshal(args, &a)

	infos, err := skill.Bundle()
	if err != nil {
		return mcpResponse{JSONRPC: "2.0", Error: &mcpError{Code: -32603, Message: "load skills: " + err.Error()}}
	}
	out := strings.Builder{}
	out.WriteString(fmt.Sprintf("%d bundled skills:\n\n", len(infos)))
	for _, info := range infos {
		if a.Filter != "" {
			if !strings.Contains(strings.ToLower(info.Name), strings.ToLower(a.Filter)) &&
				!strings.Contains(strings.ToLower(info.Description), strings.ToLower(a.Filter)) {
				continue
			}
		}
		out.WriteString(fmt.Sprintf("- %-32s  v%-8s  %s\n", info.Name, info.Version, truncate(info.Description, 80)))
	}
	if a.Filter != "" && out.Len() == 0 {
		out.WriteString(fmt.Sprintf("(no skills match filter %q)\n", a.Filter))
	}
	out.WriteString("\nTo inspect one: call radiant_skill_load(name=\"credit-risk\").\n")
	out.WriteString("Then call radiant_possess(task=..., workdir=...) to drive the run.\n")
	return mcpResponse{
		JSONRPC: "2.0",
		Result: map[string]interface{}{
			"content": []map[string]string{{"type": "text", "text": out.String()}},
		},
	}
}

// mcpSkillLoad returns the full SKILL.md + frontmatter of one skill.
func mcpSkillLoad(args json.RawMessage) mcpResponse {
	var a struct {
		Name string `json:"name"`
	}
	_ = json.Unmarshal(args, &a)
	if a.Name == "" {
		return mcpResponse{JSONRPC: "2.0", Error: &mcpError{Code: -32602, Message: "name is required"}}
	}
	infos, _ := skill.Bundle()
	found := false
	for _, info := range infos {
		if info.Name == a.Name {
			found = true
			break
		}
	}
	if !found {
		out := strings.Builder{}
		out.WriteString(fmt.Sprintf("skill %q not found. Available skills:\n\n", a.Name))
		for _, info := range infos {
			out.WriteString(fmt.Sprintf("- %s\n", info.Name))
		}
		return mcpResponse{JSONRPC: "2.0", Error: &mcpError{Code: -32604, Message: out.String()}}
	}

	// Load via the internal/skill package, which owns the embedded FS.
	sk, err := skill.LoadFromFS(skill.BundledFS(), "skills/"+a.Name)
	if err != nil {
		return mcpResponse{JSONRPC: "2.0", Error: &mcpError{Code: -32603,
			Message: fmt.Sprintf("skill %q found but its SKILL.md could not be loaded: %v", a.Name, err)}}
	}
	body := strings.Builder{}
	body.WriteString(fmt.Sprintf("---\nname: %s\nversion: %s\ndescription: %s\n---\n\n",
		sk.Name, sk.Version, sk.Description))
	fsys := skill.BundledFS()
	if data, err := fs.ReadFile(fsys, "skills/"+a.Name+"/SKILL.md"); err == nil {
		body.Write(data)
	}
	return mcpResponse{
		JSONRPC: "2.0",
		Result: map[string]interface{}{
			"content": []map[string]string{{"type": "text", "text": body.String()}},
		},
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
