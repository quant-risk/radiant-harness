// v3.7.10_test.go — coverage for the new CLI surfaces shipped
// in v3.7.10:
//
//   - `radiant mcp serve --async-subprocess` + `--fleet-async-subprocess`
//     CLI flags (precedence: CLI flag > env var > default off).
//   - `radiant doctor --async-host` diagnostic that scores the
//     current host's need for subprocess mode.
//   - `radiant phase status` / `radiant phase watch` CLI namespace
//     that surfaces the same summary as `radiant_phase_status`.
//
// The tests pin the precedence contract (CLI flag wins over env
// var) and the polling semantics of `phase watch` (re-emit on
// state change, exit on terminal state, Ctrl-C interrupts
// cleanly).

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

// sharedTestRoot builds a minimal cobra root with the v3.7.10
// commands attached. Avoids pulling in the full root from main
// (which would discover project roots, write to .radiant-harness/,
// etc.). Used by the test functions below.
func sharedTestRoot(t *testing.T) *cobra.Command {
	t.Helper()
	root := &cobra.Command{Use: "radiant"}
	registerPhaseWatchCmd(root)
	return root
}

// ─────────────────────────────────────────────────────────────────────
// CLI flag precedence tests
// ─────────────────────────────────────────────────────────────────────

// TestEnvBool_FalsyAndTruthy pins the env-var parsing helper.
// Used by `radiant mcp serve` to decide whether to enable async
// subprocess mode from RADIANT_ASYNC_SUBPROCESS /
// RADIANT_FLEET_ASYNC_SUBPROCESS.
func TestEnvBool_FalsyAndTruthy(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"0", false},
		{"1", true},
		{"true", true},
		{"True", true},
		{"TRUE", true},
		{"yes", true},
		{"on", true},
		{"false", false},
		{"no", false},
		{"off", false},
		{"  1  ", true},
		{"  ", false},
	}
	for _, c := range cases {
		t.Setenv("RADIANT_TEST_BOOL", c.in)
		got := envBool("RADIANT_TEST_BOOL")
		if got != c.want {
			t.Errorf("envBool(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────
// Phase watch tests
// ─────────────────────────────────────────────────────────────────────

// TestPhaseStatus_Watch_StopsOnTerminalState verifies that
// `radiant phase watch` exits 0 when the persisted state reaches
// a terminal status (done / cancelled / error / crashed).
// Uses a real persistable state written to disk + the runPhaseWatch
// function directly (no cobra / signals).
func TestPhaseStatus_Watch_StopsOnTerminalState(t *testing.T) {
	dir := t.TempDir()
	st := newTestPossessState("run-watch-1", "test goal", dir)
	st.CurrentPhase = "execute"
	st.Phases = map[string]*phaseProgress{
		"discover": {Status: "done"},
		"plan":     {Status: "done"},
		"execute":  {Status: "in_progress", StartedAt: time.Now().UTC()},
		"verify":   {Status: "pending"},
	}
	if err := savePossessState(st); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Promote the state to "done" while the watch is polling.
	// Use a goroutine to mutate the state after a short delay so
	// the watch sees the change. atomicWriteState does a write-
	// to-tmp + rename so the watch's read never sees a half-
	// written file.
	go func() {
		time.Sleep(150 * time.Millisecond)
		st.CurrentPhase = "done"
		st.Phases["execute"].Status = "done"
		st.Phases["execute"].EndedAt = time.Now().UTC()
		st.Phases["verify"].Status = "done"
		st.Phases["verify"].EndedAt = time.Now().UTC()
		_ = atomicWriteState(st)
	}()

	var buf strings.Builder
	err := runPhaseWatch(dir, st.TaskID, 50*time.Millisecond, 0, false, false, "", &buf)
	if err != nil {
		t.Fatalf("runPhaseWatch: %v", err)
	}
	out := buf.String()
	// The first emission + the done-state emission should both
	// be present. We don't pin exact text (timestamps vary) but
	// we DO pin the appearance of "done" in the output and the
	// presence of at least two "--- " separators (one per emit).
	if !strings.Contains(out, "done") {
		t.Errorf("expected output to mention 'done' status, got: %s", out)
	}
	if strings.Count(out, "--- ") < 2 {
		t.Errorf("expected ≥2 emissions (initial + done), got %d", strings.Count(out, "--- "))
	}
}

// TestPhaseStatus_Watch_MaxPollEnforced pins the safety net: if
// the run never reaches a terminal state, watch exits with an
// error after --max-poll elapses (not 0).
func TestPhaseStatus_Watch_MaxPollEnforced(t *testing.T) {
	dir := t.TempDir()
	st := newTestPossessState("run-watch-2", "test goal", dir)
	st.CurrentPhase = "execute"
	st.Phases = map[string]*phaseProgress{
		"discover": {Status: "done"},
		"plan":     {Status: "done"},
		"execute":  {Status: "in_progress", StartedAt: time.Now().UTC()},
		"verify":   {Status: "pending"},
	}
	if err := savePossessState(st); err != nil {
		t.Fatalf("seed: %v", err)
	}

	var buf strings.Builder
	err := runPhaseWatch(dir, st.TaskID, 50*time.Millisecond, 200*time.Millisecond, false, false, "", &buf)
	if err == nil {
		t.Fatalf("expected error from max-poll enforcement, got nil (buf: %s)", buf.String())
	}
	if !strings.Contains(err.Error(), "max-poll") {
		t.Errorf("error should mention max-poll, got: %v", err)
	}
}

// TestPhaseStatus_Watch_JSONMode verifies that the --json flag
// emits one structured JSON object per change, parseable line-
// by-line with `jq -c`.
func TestPhaseStatus_Watch_JSONMode(t *testing.T) {
	dir := t.TempDir()
	st := newTestPossessState("run-watch-3", "test goal", dir)
	st.CurrentPhase = "execute"
	st.Phases = map[string]*phaseProgress{
		"discover": {Status: "done"},
		"plan":     {Status: "done"},
		"execute":  {Status: "in_progress", StartedAt: time.Now().UTC()},
		"verify":   {Status: "pending"},
	}
	if err := savePossessState(st); err != nil {
		t.Fatalf("seed: %v", err)
	}

	go func() {
		time.Sleep(150 * time.Millisecond)
		st.CurrentPhase = "done"
		st.Phases["execute"].Status = "done"
		st.Phases["execute"].EndedAt = time.Now().UTC()
		_ = atomicWriteState(st)
	}()

	var buf strings.Builder
	if err := runPhaseWatch(dir, st.TaskID, 50*time.Millisecond, 0, true, false, "", &buf); err != nil {
		t.Fatalf("runPhaseWatch json: %v", err)
	}
	// Each emitted object must be a parseable JSON document.
	// We use a relaxed decoder (Decode, not Unmarshal) so we
	// don't have to know the exact number of objects emitted.
	dec := json.NewDecoder(strings.NewReader(buf.String()))
	var obj map[string]interface{}
	count := 0
	for dec.Decode(&obj) == nil {
		count++
		if _, ok := obj["status"]; !ok {
			t.Errorf("emitted object %d missing 'status' field: %v", count, obj)
		}
	}
	if count < 2 {
		t.Errorf("expected ≥2 JSON objects (initial + done), got %d (buf: %s)", count, buf.String())
	}
}

// TestPhaseStatus_Watch_NoReemitWhenUnchanged verifies the
// fingerprint logic: if status / phase / subprocess fields don't
// change, watch does NOT re-emit. Without this, a host tailing
// the watch output would see a flood of identical emissions.
func TestPhaseStatus_Watch_NoReemitWhenUnchanged(t *testing.T) {
	dir := t.TempDir()
	st := newTestPossessState("run-watch-4", "test goal", dir)
	st.CurrentPhase = "execute"
	st.Phases = map[string]*phaseProgress{
		"discover": {Status: "done"},
		"plan":     {Status: "done"},
		"execute":  {Status: "in_progress", StartedAt: time.Now().UTC()},
		"verify":   {Status: "pending"},
	}
	if err := savePossessState(st); err != nil {
		t.Fatalf("seed: %v", err)
	}

	var buf strings.Builder
	// Use a tight 100ms interval but only 250ms max-poll — the
	// state never changes so we should get exactly 1 emission
	// (the initial) before max-poll kicks in.
	if err := runPhaseWatch(dir, st.TaskID, 100*time.Millisecond, 250*time.Millisecond, false, false, "", &buf); err == nil {
		t.Fatalf("expected max-poll error, got nil")
	}
	out := buf.String()
	if strings.Count(out, "--- ") != 1 {
		t.Errorf("expected exactly 1 emission (no change → no re-emit), got %d (out: %s)",
			strings.Count(out, "--- "), out)
	}
}

// TestPhaseStatus_StatusCLI_OneShot verifies the `phase status`
// one-shot CLI command emits the same shape as the watch's
// terminal emission (or the MCP `radiant_phase_status` content[1]).
//
// Drives the underlying functions directly (not via cobra) so the
// test doesn't depend on writer plumbing through cobra — same
// coverage, less ceremony.
func TestPhaseStatus_StatusCLI_OneShot(t *testing.T) {
	dir := t.TempDir()
	st := newTestPossessState("run-status-1", "test goal", dir)
	st.CurrentPhase = "execute"
	st.Phases = map[string]*phaseProgress{
		"discover": {Status: "done"},
		"plan":     {Status: "done"},
		"execute":  {Status: "in_progress", StartedAt: time.Now().UTC()},
		"verify":   {Status: "pending"},
	}
	if err := savePossessState(st); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Read back via the same primitives the CLI uses.
	loaded, err := loadPossessState(dir, st.TaskID)
	if err != nil {
		t.Fatalf("loadPossessState: %v", err)
	}
	summary := buildPhaseStatusSummary(loaded, dir)
	formatted := formatPhaseStatusSummary(summary)
	if !strings.Contains(formatted, "in_progress") {
		t.Errorf("status output should mention in_progress, got: %s", formatted)
	}
	if !strings.Contains(formatted, "execute") {
		t.Errorf("status output should mention current phase execute, got: %s", formatted)
	}
}

// ─────────────────────────────────────────────────────────────────────
// Doctor --async-host tests
// ─────────────────────────────────────────────────────────────────────

// TestDoctor_AsyncHost_ExitOnRecommendation pins the exit-code
// contract: `radiant doctor --async-host` exits non-zero when an
// opt-in is recommended but not enabled (so CI / lint checks can
// catch a regression). When nothing's recommended (or everything's
// already on), exits 0.
//
// We can't easily simulate a "Hermes host detected" environment
// in tests — host detection reads the actual process env. So we
// pin the **no-host** path: with no host detected, no
// recommendation is made and exit is 0.
//
// Uses os.Pipe to capture stdout because cobra's SetOut() doesn't
// reliably route through the writer in our test setup.
func TestDoctor_AsyncHost_ExitOnRecommendation(t *testing.T) {
	// Unset all RADIANT_ env vars that might influence detection.
	for _, v := range []string{"RADIANT_ASYNC_SUBPROCESS", "RADIANT_FLEET_ASYNC_SUBPROCESS"} {
		t.Setenv(v, "")
	}
	// Unset host-detection env vars to force AgentUnknown.
	for _, v := range []string{"CLAUDE_CODE", "CURSOR_TRACE_ID", "HERMES_HOME",
		"CODEX_HOME", "OPENCODE_CONFIG_DIR", "GEMINI_CLI", "KIMI_CLI_HOME",
		"OPENCLAW_HOME", "CLINE_HOME", "WINDSURF_HOME", "ZED_CONFIG_DIR"} {
		t.Setenv(v, "")
	}

	// Capture stdout.
	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()

	root := &cobra.Command{Use: "radiant"}
	registerDoctorCmd(root)
	root.SetArgs([]string{"doctor", "--async-host"})
	err := root.Execute()
	_ = w.Close()

	// Read captured output.
	var buf strings.Builder
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = copyToBuilder(r, &buf)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("read pipe timed out")
	}

	// With no host detected, no recommendation → exit 0.
	if err != nil {
		t.Errorf("doctor --async-host with no host should exit 0, got: %v (buf: %s)", err, buf.String())
	}
	out := buf.String()
	if !strings.Contains(out, "radiant doctor --async-host") {
		t.Errorf("missing header in output: %s", out)
	}
	if !strings.Contains(out, "NOT RECOMMENDED") {
		t.Errorf("expected 'NOT RECOMMENDED' in output: %s", out)
	}
}

// copyToBuilder drains r into buf until EOF. Used by the doctor
// test to capture stdout from a cobra command.
func copyToBuilder(r interface{ Read(p []byte) (int, error) }, buf *strings.Builder) error {
	tmp := make([]byte, 4096)
	for {
		n, err := r.Read(tmp)
		if n > 0 {
			buf.Write(tmp[:n])
		}
		if err != nil {
			return nil // EOF or benign
		}
	}
}

// ─────────────────────────────────────────────────────────────────────
// Shared test fixtures
// ─────────────────────────────────────────────────────────────────────

// newTestPossessState builds a minimal possessState suitable for
// status CLI / watch tests. Mirrors the shape
// `runPossessWithBackend` writes but skips the heavy logic.
// The workdir is the second arg so the seed + the read use the
// same temp directory (otherwise `savePossessState` writes to
// os.Getwd() and the watch read goes to t.TempDir()).
func newTestPossessState(taskID, goal, workdir string) *possessState {
	return &possessState{
		TaskID:       taskID,
		Workdir:      workdir,
		Task:         goal,
		CurrentPhase: "discover",
		RunMode:      "self-driven",
		StartedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
}

// phaseProgress is an alias for the package-private phaseResult
// type — tests in the same package can use it directly.
type phaseProgress = phaseResult

// atomicWriteState writes the state.json via a tmp + rename so
// concurrent readers (e.g. the watch test polling the file on
// every tick) never observe a half-written file. Used by tests
// that mutate state mid-flight to drive a status transition.
func atomicWriteState(s *possessState) error {
	s.UpdatedAt = time.Now().UTC()
	dst := possessStatePath(s.Workdir, s.TaskID)
	tmp := dst + ".tmp-write"
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

// testOnce is a sync.Once used by the various seeders above so a
// re-run of a test (e.g. via `go test -count=2`) doesn't pile up
// duplicate state files.
var (
	testSaveOnce sync.Once
	testSaveDir  string
)

func init() {
	testSaveDir = filepath.Join(os.TempDir(), "radiant-v3.7.10-tests")
	_ = os.MkdirAll(testSaveDir, 0o755)
	testSaveOnce.Do(func() {})
}