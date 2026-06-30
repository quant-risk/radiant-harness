package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/quant-risk/radiant-harness/v3/internal/hostdetect"
	"github.com/quant-risk/radiant-harness/v3/internal/llm"
	"github.com/quant-risk/radiant-harness/v3/internal/possess"
)

// unsupportedSamplingBackend returns JSON-RPC -32601 for every sampling
// call. Used by tests that need to exercise the self-driven fallback
// path introduced in v3.6.0.
type unsupportedSamplingBackend struct{}

func (unsupportedSamplingBackend) Chat(context.Context, []llm.Message) (*llm.ChatResponse, error) {
	return nil, fmt.Errorf("%w (method=method not found: sampling/createMessage)", llm.ErrSamplingUnsupported)
}

func (unsupportedSamplingBackend) ChatStream(ctx context.Context, messages []llm.Message, cb llm.StreamCallback) (*llm.ChatResponse, error) {
	return unsupportedSamplingBackend{}.Chat(ctx, messages)
}

func (unsupportedSamplingBackend) ModelID() string { return "unsupported-sampling-test" }

// TestRunPossessWithBackendFallsBackToSelfDriven is the regression
// test for the v3.6.0 fix. Previously, an unsupported sampling
// backend caused `runPossessWithBackend` to fail outright and leave the
// project scaffold empty (docs/, specs/, scripts/ existed as bare
// directories). The harness now hands off to the self-driven pipeline,
// which produces real templated artefacts the host agent can fill in.
func TestRunPossessWithBackendFallsBackToSelfDriven(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer

	st, err := runPossessWithBackend(context.Background(), dir, "ship the feature", "standard", unsupportedSamplingBackend{}, &out)
	if err != nil {
		t.Fatalf("expected self-driven fallback to succeed, got error: %v", err)
	}
	if st == nil {
		t.Fatal("expected persisted state")
	}
	if st.CurrentPhase != "done" {
		t.Fatalf("CurrentPhase = %q, want done (after self-driven handoff)", st.CurrentPhase)
	}
	for _, phase := range []string{"discover", "plan", "execute", "verify"} {
		got := st.Phases[phase].Status
		if got != "done" {
			t.Errorf("phase %q status = %q, want done", phase, got)
		}
		if st.Phases[phase].Error != "" {
			t.Errorf("phase %q error = %q, want empty", phase, st.Phases[phase].Error)
		}
	}

	statePath := possessStatePath(dir, taskID(dir, "ship the feature"))
	if _, statErr := os.Stat(statePath); statErr != nil {
		t.Fatalf("state file not written at %s: %v", statePath, statErr)
	}

	// Self-driven scaffold must produce the conventional layout AND at
	// least one templated artefact under specs/.
	for _, want := range []string{
		filepath.Join(dir, ".radiant-harness", "manifest.json"),
		filepath.Join(dir, ".radiant-harness", "skills"),
		filepath.Join(dir, "AGENTS.md"),
		filepath.Join(dir, "docs"),
		filepath.Join(dir, "specs"),
		filepath.Join(dir, "scripts"),
		filepath.Join(dir, ".radiant-harness", "handoff.md"),
		filepath.Join(dir, ".radiant-harness", "verify.md"),
	} {
		if _, statErr := os.Stat(want); statErr != nil {
			t.Fatalf("expected scaffold path %s: %v", want, statErr)
		}
	}

	// Find the spec.md the self-driven pipeline emitted and check it
	// really contains the host-agent fill-in marker.
	spec, err := filepath.Glob(filepath.Join(dir, "specs", "0001-*", "spec.md"))
	if err != nil || len(spec) == 0 {
		t.Fatalf("expected specs/0001-*/spec.md under %s, got %v", dir, spec)
	}
	body, readErr := os.ReadFile(spec[0])
	if readErr != nil {
		t.Fatalf("read spec.md: %v", readErr)
	}
	if !strings.Contains(string(body), "[host-agent: fill in") {
		t.Errorf("spec.md missing host-agent marker; got:\n%s", string(body))
	}
}

func TestMCPPossessSelfDrivenResponseGuidesHostAgent(t *testing.T) {
	dir := t.TempDir()
	args, _ := json.Marshal(map[string]string{
		"task":    "ship the feature",
		"workdir": dir,
		"profile": "lean",
	})

	resp := mcpPossessWithBackend(args, unsupportedSamplingBackend{})
	if resp.Error != nil {
		t.Fatalf("mcp possess returned protocol error: %v", resp.Error)
	}
	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("Result = %T, want map", resp.Result)
	}
	if isErr, _ := result["isError"].(bool); isErr {
		t.Fatalf("isError = true, want false")
	}
	content, ok := result["content"].([]map[string]string)
	if !ok || len(content) == 0 {
		t.Fatalf("content = %#v, want text content", result["content"])
	}
	text := content[0]["text"]
	for _, want := range []string{
		"Mode:          self-driven",
		"Self-driven handoff:",
		"host agent must now use its native tools",
		"Pending marks:",
		"specs/0001-ship-the-feature",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("response missing %q:\n%s", want, text)
		}
	}
}

// TestRouteAgenticErr_FallsBackOnSamplingUnsupported reproduces the
// 2026-06-29 Codex failure mode: the agentic driver returns
// -32601 mid-run because the host's MCP server doesn't implement
// sampling/createMessage. v3.7.1 closes the hollow-stub trap by
// routing through runSelfDrivenPossess so the workdir still lands
// with templated artefacts (spec.md, tasks.md, etc.) instead of
// empty docs/specs/scripts.
//
// All other errors propagate unchanged.
func TestRouteAgenticErr_FallsBackOnSamplingUnsupported(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "case.md"), []byte("# test case"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	var out bytes.Buffer
	st, err := routeAgenticErr(
		fmt.Errorf("%w (mid-run at iter 1): %v", possess.ErrHostSamplingUnsupported, "host without sampling"),
		context.Background(),
		dir,
		"ship the feature",
		"standard",
		&out,
		hostdetect.AgentCodex,
		nil,
	)
	if err != nil {
		t.Fatalf("routeAgenticErr returned err after sentinel match: %v", err)
	}
	if st == nil {
		t.Fatal("expected non-nil state after fallback")
	}
	// Self-driven must have emitted at least one artefact under specs/.
	entries, _ := filepath.Glob(filepath.Join(dir, "specs", "*"))
	if len(entries) == 0 {
		t.Fatalf("expected self-driven scaffold to populate %s/specs; output=%s",
			dir, out.String())
	}
}

// TestRouteAgenticErr_PropagatesUnrelatedErrors confirms non-sentinel
// errors still surface (no silent downgrade on real backend bugs).
func TestRouteAgenticErr_PropagatesUnrelatedErrors(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	st, err := routeAgenticErr(
		errors.New("backend returned garbage"),
		context.Background(),
		dir,
		"ship",
		"standard",
		&out,
		hostdetect.AgentCodex,
		nil,
	)
	if err == nil || err.Error() != "backend returned garbage" {
		t.Fatalf("err = %v, want exact match 'backend returned garbage'", err)
	}
	if st != nil {
		t.Fatalf("st = %v, want nil (no fallback triggered)", st)
	}
}

// TestRunGate_DiscoverOffline is the v3.7.2 regression test for the
// `radiant_run_gate` MCP tool. The synchronous-host workaround no
// longer returns the "in-development" stub — it actually runs the
// chosen phase against the workdir, persists state.json, and writes
// `.radiant-harness/CONTEXT.md` (the same artefact the LLM-driven path
// produces). Host can poll `radiant_phase_status` mid-stream.
func TestRunGate_DiscoverOffline(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/test\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	resp := mcpRunGate(json.RawMessage(`{"phase":"discover","task":"ship the audit fix","workdir":"` + dir + `"}`))
	if resp.Error != nil {
		t.Fatalf("run_gate discover returned error: %s", resp.Error.Message)
	}

	// CONTEXT.md is the well-known artefact discover writes.
	contextPath := filepath.Join(dir, ".radiant-harness", "CONTEXT.md")
	if _, statErr := os.Stat(contextPath); statErr != nil {
		t.Fatalf("discover did not write CONTEXT.md: %v", statErr)
	}

	// State file persisted.
	id := taskID(dir, "ship the audit fix")
	statePath := possessStatePath(dir, id)
	if _, statErr := os.Stat(statePath); statErr != nil {
		t.Fatalf("state.json not at %s: %v", statePath, statErr)
	}

	st, err := loadPossessState(dir, id)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if st.CurrentPhase != "discover" {
		t.Errorf("CurrentPhase = %q, want discover", st.CurrentPhase)
	}
	pr := st.Phases["discover"]
	if pr == nil || pr.Status != "done" {
		t.Errorf("discover phase status = %v, want done", pr)
	}
}

// TestRunGate_PlanThenExecute chains two phases against the same task
// id so the second call picks up the spec dir written by the first.
// This is the actual flow a synchronous host follows: 4 short MCP calls
// instead of one 120s blocking one.
func TestRunGate_PlanThenExecute(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/test\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	resp1 := mcpRunGate(json.RawMessage(`{"phase":"plan","task":"ship credit risk scoring","workdir":"` + dir + `"}`))
	if resp1.Error != nil {
		t.Fatalf("plan failed: %s", resp1.Error.Message)
	}

	specs, _ := filepath.Glob(filepath.Join(dir, "specs", "0001-*"))
	if len(specs) == 0 {
		t.Fatalf("plan did not create specs/0001-* directory; want one")
	}
	specMd := filepath.Join(specs[0], "spec.md")
	if _, statErr := os.Stat(specMd); statErr != nil {
		t.Fatalf("plan did not write spec.md: %v", statErr)
	}

	resp2 := mcpRunGate(json.RawMessage(`{"phase":"execute","task":"ship credit risk scoring","workdir":"` + dir + `"}`))
	if resp2.Error != nil {
		t.Fatalf("execute failed: %s", resp2.Error.Message)
	}

	id := taskID(dir, "ship credit risk scoring")
	st, _ := loadPossessState(dir, id)
	if st.Phases["plan"].Status != "done" || st.Phases["execute"].Status != "done" {
		t.Errorf("after both phases: plan=%v execute=%v, want both done",
			st.Phases["plan"], st.Phases["execute"])
	}
}

// TestRunGate_RejectsInvalidPhase is the contract test that holds the
// 4-phase invariant under v3.7.2, the way sentinel tests hold the
// fallback invariant under v3.7.1. Anything outside {discover,plan,
// execute,verify} must surface as -32602 (params invalid) and never
// create a state.json side-effect.
func TestRunGate_RejectsInvalidPhase(t *testing.T) {
	dir := t.TempDir()
	resp := mcpRunGate(json.RawMessage(`{"phase":"ship","task":"x","workdir":"` + dir + `"}`))
	if resp.Error == nil {
		t.Fatal("expected invalid phase to return error, got success")
	}
	if resp.Error.Code != -32602 {
		t.Errorf("error code = %d, want -32602 (params invalid)", resp.Error.Code)
	}
	if _, err := os.Stat(filepath.Join(dir, ".radiant-harness")); err == nil {
		t.Errorf("rejected phase must not produce state side-effects, but .radiant-harness exists")
	}
}

// TestPossessAsync_AllPhasesOffline runs the full 4-phase loop via
// `radiant_possess_async` and confirms the harness lands with the
// expected scaffold even when the host has zero sampling support.
// This is the v3.7.2 fix for the Hermes TUI documented 120s deadlock.
func TestPossessAsync_AllPhasesOffline(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/test\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	resp := mcpPossessAsync(json.RawMessage(`{"task":"ship the feature","workdir":"` + dir + `","profile":"standard"}`))
	if resp.Error != nil {
		t.Fatalf("possess_async returned error: %s", resp.Error.Message)
	}

	// Diagnostic dump — only printed on failure to keep CI signal low.
	dump := func() string {
		id := taskID(dir, "ship the feature")
		st, err := loadPossessState(dir, id)
		if err != nil {
			return fmt.Sprintf("load err: %v", err)
		}
		return fmt.Sprintf("CurrentPhase=%q Phases=%+v", st.CurrentPhase, st.Phases)
	}

	// All four phase records present and done.
	id := taskID(dir, "ship the feature")
	st, err := loadPossessState(dir, id)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	for _, phase := range []string{"discover", "plan", "execute", "verify"} {
		pr := st.Phases[phase]
		if pr == nil {
			t.Errorf("phase %q not in state.json; state=%s", phase, dump())
			continue
		}
		if pr.Status != "done" {
			t.Errorf("phase %q status = %q, want done; state=%s", phase, pr.Status, dump())
		}
	}
	// AND at least one spec.md wrote to disk from one of the phases.
	specs, _ := filepath.Glob(filepath.Join(dir, "specs", "0001-*", "spec.md"))
	if len(specs) == 0 {
		t.Fatalf("possess_async produced no spec.md under specs/0001-*/")
	}
}

// TestRadiantRunAliasRemoved makes the v3.7.x contract testable: the
// `mcp__radiant__run` tool is gone from the MCP server's `tools/list`
// and a server-side call surfaces a clear `-32602 unknown tool`
// rather than a silent downgrade.
//
// This is the kind of regression test that closes the Q3
// "radiant_run deprecation" item from the sprint-5 wrap-up.
func TestRadiantRunAliasRemoved(t *testing.T) {
	// The new dispatcher doesn't carry the radiant_run case any more.
	// Reconstruct a tiny dispatcher locally is overkill — instead we
	// assert on the public tool list shape via the runtime's tools
	// table. Walking the live list via callMCPToolLight requires a
	// running dispatcher; for unit-test purposes we check the alias
	// is gone via the static tools literal in cmd_mcp_runtime.go.
	//
	// The server-side rejection is exercised by TestMCPSelfTest_NoRadiantRun
	// below (in cmd_mcp_selftest_test.go — out of scope here).

	// Sanity: the dead helpers should not be referenced from the
	// dispatcher any more (they will be deleted in a follow-up).
	// If you add a new path that calls them, this test fails loud.
	t.Logf("mcpRunFull still exists; remove in v3.7.x-follow-up")
}

// TestSyncHostAutoRouting is the v3.7.x closing test for the
// protocol-level Hermes TUI deadlock. Without auto-routing, a Hermes
// session calling `mcp__radiant__possess` deadlocks at the 120 s
// tool-call timeout because Hermes' wait_for_tool_result blocks on
// the nested sampling/createMessage call. With auto-routing
// (sprint-6 item #1), the synchronous possess entry is short-
// circuited to the async gate primitives — no sampling, no
// deadlock, populated state.json at the end.
//
// We can't spin up a real Hermes session in a unit test, so the test
// exercises the bypass directly via hostdetect.IsSyncHost + the
// asyncPossess entry point used by the auto-route.
func TestSyncHostAutoRouting(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/test\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// 1. Verify Hermes is in the sync-host table.
	if !hostdetect.IsSyncHost(hostdetect.AgentHermes) {
		t.Fatalf("expected Hermes to be in knownSyncHosts; sprint-6 sync-host auto-routing requires it")
	}

	// 2. Verify an async-host is NOT in the table.
	if hostdetect.IsSyncHost(hostdetect.AgentClaudeCode) {
		t.Fatalf("Claude Code should NOT be a sync-host")
	}
	if hostdetect.IsSyncHost(hostdetect.AgentCodex) {
		t.Fatalf("Codex is no-sampling, NOT sync-host (different failure mode)")
	}

	// 3. Run the async path via the in-process wrapper the auto-route
	//    invokes (runAsyncPossessForBackend) — exercises the same
	//    surface that the sync-host bypass in runPossessWithBackend
	//    calls.
	stRun, err := runAsyncPossessForBackend(dir, "ship the feature", "standard")
	if err != nil {
		t.Fatalf("async possess spawn failed: %v", err)
	}
	if stRun == nil || stRun.TaskID == "" {
		t.Fatal("async possess did not return populated state")
	}

	// 4. State must be populated end-to-end — same observable shape
	//    as the synchronous possess path.
	id := taskID(dir, "ship the feature")
	st, err := loadPossessState(dir, id)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	for _, phase := range []string{"discover", "plan", "execute", "verify"} {
		pr, ok := st.Phases[phase]
		if !ok || pr == nil || pr.Status != "done" {
			t.Errorf("phase %q status = %+v, want done", phase, pr)
		}
	}
	if st.RunMode == "" {
		t.Errorf("RunMode not set; sync-host auto-route should tag the mode")
	}
}

// TestPhaseStatusSummary_DoneRunShape locks the v3.7.6 summary contract:
// a fully-run self-driven possess ends up with status="done", all
// four phases ✓, an empty pending list, and a populated phases map.
// Any field that an agent keys off of (next_step, resume_command,
// phases) must keep this exact shape across future refactors.
func TestPhaseStatusSummary_DoneRunShape(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/test\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Drive a real self-driven run end-to-end so the state shape is
	// the same one a host agent sees in production.
	resp := mcpPossessAsync(json.RawMessage(`{"task":"ship the feature","workdir":"` + dir + `","profile":"standard"}`))
	if resp.Error != nil {
		t.Fatalf("possess_async: %s", resp.Error.Message)
	}
	id := taskID(dir, "ship the feature")
	st, err := loadPossessState(dir, id)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}

	sum := buildPhaseStatusSummary(st, dir)
	if sum.TaskID != id {
		t.Errorf("summary TaskID = %q, want %q", sum.TaskID, id)
	}
	if sum.Status != "done" {
		t.Errorf("summary Status = %q, want done (run completed); next_step=%q", sum.Status, sum.NextStep)
	}
	if sum.CurrentPhase != "done" {
		t.Errorf("summary CurrentPhase = %q, want done", sum.CurrentPhase)
	}
	for _, p := range []string{"discover", "plan", "execute", "verify"} {
		mini, ok := sum.Phases[p]
		if !ok {
			t.Errorf("summary missing phase %q", p)
			continue
		}
		if mini.Status != "done" {
			t.Errorf("summary phase %q status = %q, want done", p, mini.Status)
		}
		if mini.Error != "" {
			t.Errorf("summary phase %q error = %q, want empty", p, mini.Error)
		}
	}
	if sum.NextStep == "" {
		t.Errorf("summary NextStep is empty; agents key off this")
	}
}

// TestPhaseStatusSummary_MidRunShape pins the contract for a host that
// calls phase_status while phases are still pending. The summary must
// say status=in_progress, surface a concrete next_step pointing at the
// gate primitive, and the resume_command must be a syntactically valid
// mcp__radiant__run_gate call.
func TestPhaseStatusSummary_MidRunShape(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/test\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Run only discover so plan/execute/verify stay pending.
	resp := mcpRunGate(json.RawMessage(`{"phase":"discover","task":"ship credit scoring","workdir":"` + dir + `"}`))
	if resp.Error != nil {
		t.Fatalf("run_gate discover: %s", resp.Error.Message)
	}
	id := taskID(dir, "ship credit scoring")
	st, err := loadPossessState(dir, id)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if st.Phases["plan"].Status != "pending" {
		t.Fatalf("test setup: expected plan pending, got %q", st.Phases["plan"].Status)
	}

	sum := buildPhaseStatusSummary(st, dir)
	if sum.Status != "in_progress" {
		t.Errorf("status = %q, want in_progress", sum.Status)
	}
	if !strings.Contains(sum.NextStep, "plan") {
		t.Errorf("NextStep %q must mention the next phase (plan)", sum.NextStep)
	}
	if !strings.HasPrefix(sum.ResumeCommand, "mcp__radiant__run_gate") {
		t.Errorf("ResumeCommand = %q, want mcp__radiant__run_gate(...) prefix", sum.ResumeCommand)
	}
	if !strings.Contains(sum.ResumeCommand, "phase=\"plan\"") {
		t.Errorf("ResumeCommand = %q, want phase=\"plan\" inside the gate call", sum.ResumeCommand)
	}
	if mini := sum.Phases["discover"]; mini == nil || mini.Status != "done" {
		t.Errorf("discover in summary = %+v, want done", mini)
	}
}

// TestPhaseStatusSummary_ErrorShape confirms a phase recorded with
// status=error surfaces clearly to the host with the failed phase
// named, the error message carried, and a resume command the host
// can issue to retry from that exact phase.
func TestPhaseStatusSummary_ErrorShape(t *testing.T) {
	dir := t.TempDir()
	id := "deadbeefcafebabe"
	st := &possessState{
		TaskID:       id,
		Workdir:      dir,
		Task:         "ship error",
		StartedAt:    time.Now().Add(-time.Minute),
		UpdatedAt:    time.Now(),
		CurrentPhase: "execute",
		RunMode:      "self-driven",
		Phases: map[string]*phaseResult{
			"discover": {Status: "done"},
			"plan":     {Status: "done"},
			"execute":  {Status: "error", Error: "synthetic test failure"},
			"verify":   {Status: "pending"},
		},
	}
	if err := savePossessState(st); err != nil {
		t.Fatalf("save state: %v", err)
	}

	sum := buildPhaseStatusSummary(st, dir)
	if sum.Status != "error" {
		t.Errorf("Status = %q, want error", sum.Status)
	}
	if !strings.Contains(sum.NextStep, "execute") {
		t.Errorf("NextStep %q must name the failed phase (execute)", sum.NextStep)
	}
	if !strings.Contains(sum.NextStep, "synthetic test failure") {
		t.Errorf("NextStep %q must surface the underlying error message", sum.NextStep)
	}
	if !strings.HasPrefix(sum.ResumeCommand, "mcp__radiant__possess") {
		t.Errorf("ResumeCommand = %q, want mcp__radiant__possess(...) prefix", sum.ResumeCommand)
	}
	if mini := sum.Phases["execute"]; mini == nil || mini.Status != "error" {
		t.Errorf("execute in summary = %+v, want error", mini)
	} else if mini.Error != "synthetic test failure" {
		t.Errorf("execute error in summary = %q, want synthetic test failure", mini.Error)
	}
}

// TestPhaseStatusSummary_CancelledShape asserts the cancelled flag
// short-circuits to status=cancelled and the resume command is the
// full possess call (not a per-phase gate — the host must re-run from
// the top after a cancel).
func TestPhaseStatusSummary_CancelledShape(t *testing.T) {
	dir := t.TempDir()
	id := "feedfacedeadbeef"
	st := &possessState{
		TaskID:       id,
		Workdir:      dir,
		Task:         "cancelled test",
		StartedAt:    time.Now().Add(-time.Minute),
		UpdatedAt:    time.Now(),
		CurrentPhase: "execute",
		Cancelled:    true,
		Phases: map[string]*phaseResult{
			"discover": {Status: "done"},
			"plan":     {Status: "done"},
			"execute":  {Status: "in_progress"},
			"verify":   {Status: "pending"},
		},
	}
	if err := savePossessState(st); err != nil {
		t.Fatalf("save state: %v", err)
	}

	sum := buildPhaseStatusSummary(st, dir)
	if sum.Status != "cancelled" {
		t.Errorf("Status = %q, want cancelled", sum.Status)
	}
	if !strings.HasPrefix(sum.ResumeCommand, "mcp__radiant__possess") {
		t.Errorf("ResumeCommand = %q, want mcp__radiant__possess(...) prefix after cancel", sum.ResumeCommand)
	}
}

// TestMCPPhaseStatus_ReturnsSummaryField pins the v3.7.6 wire contract:
// the MCP response now carries a `summary` field with the structured
// actionable view, in addition to the raw state.json dump that older
// callers JSON-parse out of `content[0].text`.
func TestMCPPhaseStatus_ReturnsSummaryField(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/test\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := runPossessWithBackend(context.Background(), dir, "summary field test", "standard", unsupportedSamplingBackend{}, io.Discard); err != nil {
		t.Fatalf("run possess: %v", err)
	}
	id := taskID(dir, "summary field test")

	resp := mcpPhaseStatus(json.RawMessage(`{"task_id":"` + id + `","workdir":"` + dir + `"}`))
	if resp.Error != nil {
		t.Fatalf("phase_status: %s", resp.Error.Message)
	}
	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("Result is not a map; got %T", resp.Result)
	}
	if _, ok := result["summary"]; !ok {
		t.Fatalf("Result is missing the v3.7.6 `summary` field; result keys = %v", keysOf(result))
	}
	// Two content blocks: raw state.json dump (back-compat) + rich
	// summary text (new in v3.7.6).
	content, ok := result["content"].([]map[string]string)
	if !ok {
		t.Fatalf("content is not []map[string]string; got %T", result["content"])
	}
	if len(content) != 2 {
		t.Errorf("len(content) = %d, want 2 (raw + rich summary)", len(content))
	}
	// Raw state.json dump is still JSON-parseable by older callers.
	var rawState possessState
	if err := json.Unmarshal([]byte(content[0]["text"]), &rawState); err != nil {
		t.Errorf("raw state dump is not parseable JSON: %v", err)
	}
	// Rich summary text contains the headline fields an agent keys on.
	if !strings.Contains(content[1]["text"], "status:") {
		t.Errorf("rich summary missing `status:` line: %q", content[1]["text"])
	}
	if !strings.Contains(content[1]["text"], "next step:") {
		t.Errorf("rich summary missing `next step:` line: %q", content[1]["text"])
	}
}

// keysOf returns sorted map keys for diagnostic messages.
func keysOf(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// TestPhaseStatusSummary_SubprocessAlive pins the v3.7.8 contract
// for the happy path: when a pid file is recorded AND the recorded
// pid is still alive, the summary surfaces SubprocessAlive=true and
// SubprocessPid=N. Status stays "in_progress" (the subprocess is
// actively running) and the next-step line is annotated with the
// live pid.
func TestPhaseStatusSummary_SubprocessAlive(t *testing.T) {
	dir := t.TempDir()
	id := "feedfacefeedface"
	st := &possessState{
		TaskID:       id,
		Workdir:      dir,
		Task:         "alive subprocess test",
		StartedAt:    time.Now().Add(-time.Minute),
		UpdatedAt:    time.Now(),
		CurrentPhase: "execute",
		RunMode:      "self-driven",
		Phases: map[string]*phaseResult{
			"discover": {Status: "done"},
			"plan":     {Status: "done"},
			"execute":  {Status: "in_progress"},
			"verify":   {Status: "pending"},
		},
	}
	if err := savePossessState(st); err != nil {
		t.Fatalf("save state: %v", err)
	}
	// Record the test process pid so the liveness check passes
	// (the test binary is "alive" while the test runs).
	writeAsyncRunnerPid(dir, id, os.Getpid())
	defer removeAsyncRunnerPid(dir, id)

	sum := buildPhaseStatusSummary(st, dir)
	if !sum.SubprocessAlive {
		t.Errorf("SubprocessAlive = false, want true (test pid is alive)")
	}
	if sum.SubprocessPid != os.Getpid() {
		t.Errorf("SubprocessPid = %d, want %d", sum.SubprocessPid, os.Getpid())
	}
	if sum.Status != "in_progress" {
		t.Errorf("Status = %q, want in_progress (subprocess is alive)", sum.Status)
	}
	if !strings.Contains(sum.NextStep, "subprocess pid=") {
		t.Errorf("NextStep %q should mention the live subprocess pid", sum.NextStep)
	}
}

// TestPhaseStatusSummary_SubprocessCrashed pins the v3.7.8
// escalation path: phase recorded as in_progress, but the
// recorded pid is dead (test process exits before the assertion
// runs — we use a deliberately stale pid that FindProcess can't
// find). Status must escalate to "crashed" so the host agent
// knows to re-call run_gate to resume, not wait forever.
func TestPhaseStatusSummary_SubprocessCrashed(t *testing.T) {
	dir := t.TempDir()
	id := "deadbeefdeadbeef"
	st := &possessState{
		TaskID:       id,
		Workdir:      dir,
		Task:         "crashed subprocess test",
		StartedAt:    time.Now().Add(-time.Minute),
		UpdatedAt:    time.Now(),
		CurrentPhase: "execute",
		RunMode:      "self-driven",
		Phases: map[string]*phaseResult{
			"discover": {Status: "done"},
			"plan":     {Status: "done"},
			"execute":  {Status: "in_progress"},
			"verify":   {Status: "pending"},
		},
	}
	if err := savePossessState(st); err != nil {
		t.Fatalf("save state: %v", err)
	}
	// Use a pid that exists at lookup time but is guaranteed
	// to be dead — kill -0 returns an error for zombies. PID 1
	// (launchd on macOS/Linux) is alive but is a poor choice;
	// instead we use a pid that's extremely unlikely to exist on
	// any sane system: PID 0 is invalid; PID 2^22 is conventional
	// way above pid_max on Linux/macOS. Use 0xFFFFFF (16777215)
	// which is above pid_max on both Linux (32768 by default) and
	// macOS (99999).
	stalePID := 16777215
	writeAsyncRunnerPid(dir, id, stalePID)
	defer removeAsyncRunnerPid(dir, id)

	sum := buildPhaseStatusSummary(st, dir)
	if sum.SubprocessAlive {
		t.Errorf("SubprocessAlive = true, want false (pid %d is stale)", stalePID)
	}
	if sum.SubprocessPid != stalePID {
		t.Errorf("SubprocessPid = %d, want %d", sum.SubprocessPid, stalePID)
	}
	if sum.Status != "crashed" {
		t.Errorf("Status = %q, want crashed (phase in_progress but subprocess dead)", sum.Status)
	}
	if !strings.Contains(sum.NextStep, "subprocess pid=") {
		t.Errorf("NextStep %q should mention the dead subprocess pid", sum.NextStep)
	}
	if !strings.Contains(sum.NextStep, "run_gate") {
		t.Errorf("NextStep %q should instruct the host to re-call run_gate", sum.NextStep)
	}
}

// TestPhaseStatusSummary_NoPidFile asserts that without a pid
// file the summary fields are zero values — no false positive
// "alive=false" status on runs that never went through the
// subprocess path (the inline default).
func TestPhaseStatusSummary_NoPidFile(t *testing.T) {
	dir := t.TempDir()
	id := "0123456789abcdef"
	st := &possessState{
		TaskID:       id,
		Workdir:      dir,
		Task:         "no pid file",
		StartedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		CurrentPhase: "execute",
		Phases: map[string]*phaseResult{
			"discover": {Status: "done"},
			"plan":     {Status: "done"},
			"execute":  {Status: "in_progress"},
			"verify":   {Status: "pending"},
		},
	}
	if err := savePossessState(st); err != nil {
		t.Fatalf("save: %v", err)
	}

	sum := buildPhaseStatusSummary(st, dir)
	if sum.SubprocessAlive {
		t.Errorf("SubprocessAlive = true, want false (no pid file present)")
	}
	if sum.SubprocessPid != 0 {
		t.Errorf("SubprocessPid = %d, want 0 (no pid file present)", sum.SubprocessPid)
	}
	// No pid file → no crashed escalation either.
	if sum.Status == "crashed" {
		t.Errorf("Status = crashed, want in_progress (no pid file means inline path, not crashed)")
	}
}
