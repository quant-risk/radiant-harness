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
		// ----- v3.7.2 async primitives (stubs, real impl in PR-B) ------
		{Name: "radiant_run_gate", Description: "Run ONE possess phase (discover | plan | execute | verify) asynchronously and return immediately with a ticket the host polls via radiant_phase_status. NO sampling/createMessage round-trip — phase runs offline, state persisted to .radiant-harness/state/<ticket>/state.json. Designed to unblock synchronous TUI hosts (Hermes) where the full radiant_possess deadlocks on sampling callbacks. v3.7.2-prep: stub returns structured 'in development' response; real subprocess plumbing lands in v3.7.2 PR-B.", InputSchema: mcpInputSchema{
			Type: "object",
			Properties: map[string]mcpPropertyDef{
				"phase":   {Type: "string", Description: "Phase to run: discover | plan | execute | verify. Required."},
				"task":    {Type: "string", Description: "The user's original task prompt (verbatim). Required."},
				"workdir": {Type: "string", Description: "Project directory. Defaults to agent's CWD."},
			},
			Required: []string{"phase", "task"},
		}},
		{Name: "radiant_possess_async", Description: "Fire-and-forget wrapper around the full 4-phase possess loop. Returns a ticket in <500ms; host polls radiant_phase_status(ticket=…) until done. Replaces radiant_possess for synchronous TUI hosts (Hermes) where the synchronous version deadlocks. v3.7.2-prep: stub returns structured 'in development' response; real subprocess plumbing lands in v3.7.2 PR-B.", InputSchema: mcpInputSchema{
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
		// v3.7.2-prep stub. Real subprocess wiring in PR-B.
		return mcpPossessAsync(args)
	case "radiant_run_gate":
		// v3.7.2-prep stub. Real subprocess wiring in PR-B.
		return mcpRunGate(args)
	case "radiant_phase_status":
		return mcpPhaseStatus(args)
	case "radiant_skill_list":
		return mcpSkillList(args)
	case "radiant_skill_load":
		return mcpSkillLoad(args)
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

// mcpPhaseStatus returns the persisted progress of a previous possess run.
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
	return mcpResponse{
		JSONRPC: "2.0",
		Result: map[string]interface{}{
			"content": []map[string]string{{"type": "text", "text": string(data)}},
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
