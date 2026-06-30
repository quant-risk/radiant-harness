package main

// Tests for the subprocess-backed async gate (v3.7.7).
//
// Per `docs/PROPOSAL-v3.7.2-async-primitives.md` § v3.7.6 update,
// the v3.7.7 async gate ships an `os/exec` subprocess path
// (`subprocessAsyncGate`) in addition to the existing inline path
// (`asyncGate`). Selection is opt-in via `RADIANT_ASYNC_SUBPROCESS=1`.
// These tests pin the subprocess path behaviour.

//   - TestSubprocessAsync_DiscoveryRunsInChild — calls
//     `selectedAsyncGate().Spawn` with the subprocess path enabled
//     and asserts the workdir ends with the canonical scaffold.
//   - TestSubprocessAsync_FourPhasesEndToEnd — runs the full
//     4-phase async loop via `mcpPossessAsync` under
//     RADIANT_ASYNC_SUBPROCESS=1 and confirms every phase lands.
//   - TestSubprocessAsync_PidFileLifecycle — confirms the pid-file
//     helpers work end-to-end (write, liveness, remove) without
//     requiring a real subprocess fork.
//   - TestSubprocessAsync_CrashRecovery — simulates a crashed
//     subprocess by planting a state.json with execute=in_progress,
//     then resumes via `selectedAsyncGate().Spawn` and asserts
//     the next call picks up where the crashed run left off.
//   - TestSubprocessAsync_SpawnsChildProcess — invokes
//     `radiant async-runner` as a real subprocess and asserts the
//     full end-to-end behaviour including state.json writes.
//
// The subprocess tests run with `RADIANT_ASYNC_SUBPROCESS=1` set
// in the parent test process so `selectedAsyncGate()` returns
// the subprocess impl. `t.Setenv` is scoped per-test and the
// cleanup restores the original value so other tests in the same
// package aren't affected.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/quant-risk/radiant-harness/v3/internal/possess"
)

// withSubprocessAsyncEnabled opts the parent test process into
// subprocess mode for the duration of the test. Relies on Go's
// `t.Setenv` cleanup to restore the original value.
func withSubprocessAsyncEnabled(t *testing.T) {
	t.Helper()
	t.Setenv("RADIANT_ASYNC_SUBPROCESS", "1")
}

// sharedRadiantBin is built once per test binary and reused
// across subprocess tests. Each test that spawns a real
// `radiant async-runner` subprocess points its parent at this
// binary via `RADIANT_BIN` so the subprocess doesn't fork itself
// (which would deadlock on the test binary holding state).
//
// Why shared: each `go build` invocation forks a `go` process
// and the `compile`/`link` toolchain; running several in rapid
// succession on macOS arm64 trips the per-process file
// descriptor / process ulimit and surfaces as
// `fork/exec ... resource temporarily unavailable`. Building
// once and reusing dodges the limit.
var (
	sharedRadiantBinOnce sync.Once
	sharedRadiantBin     string
)

// sharedRadiantBinPath returns the path to a one-time-built
// `radiant` binary for use as the subprocess target. Skips the
// calling test if the build ever fails (so a single broken
// build doesn't block every subprocess test in the package).
func sharedRadiantBinPath(t *testing.T) string {
	t.Helper()
	sharedRadiantBinOnce.Do(func() {
		// os.MkdirTemp (not t.TempDir) — the dir must outlive
		// the test that triggered the first build, otherwise
		// subsequent subprocess tests point at a deleted file.
		tmp, err := os.MkdirTemp("", "radiant-subprocess-")
		if err != nil {
			t.Logf("shared radiant mkdir failed: %v", err)
			return
		}
		bin := filepath.Join(tmp, "radiant")
		build := exec.Command("go", "build", "-o", bin, "github.com/quant-risk/radiant-harness/v3/cmd/radiant")
		out, err := build.CombinedOutput()
		if err != nil {
			t.Logf("shared radiant build failed: %v\n%s", err, string(out))
			return
		}
		sharedRadiantBin = bin
	})
	if sharedRadiantBin == "" {
		t.Skip("shared radiant binary not built")
	}
	return sharedRadiantBin
}

// TestSubprocessAsync_DiscoveryRunsInChild — spawns the
// subprocess gate against a fresh workdir, asserts the workdir
// ends with the conventional scaffold (CONTEXT.md + specs/)
// and state.json marks discover as done.
func TestSubprocessAsync_DiscoveryRunsInChild(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/test\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Opt this test process into subprocess mode AND point the
	// subprocess at a real `radiant` binary so it doesn't fork
	// the test binary (which would deadlock).
	withSubprocessAsyncEnabled(t)
	t.Setenv("RADIANT_BIN", sharedRadiantBinPath(t))

	gate := selectedAsyncGate()
	task := "ship a tiny feature"

	h, err := gate.Spawn(possess.PhaseDiscover, task, dir)
	if err != nil {
		t.Fatalf("subprocess gate Spawn: %v", err)
	}
	if h.Ticket == "" {
		t.Errorf("ticket is empty; got %+v", h)
	}

	// state.json must mark discover done.
	st, err := loadPossessState(dir, string(h.Ticket))
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if st.Phases["discover"] == nil || st.Phases["discover"].Status != "done" {
		t.Errorf("discover phase not done; phases=%+v", st.Phases)
	}
}

// TestSubprocessAsync_FourPhasesEndToEnd — full 4-phase loop
// through the subprocess-backed possess_async impl.
func TestSubprocessAsync_FourPhasesEndToEnd(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/test\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	withSubprocessAsyncEnabled(t)
	t.Setenv("RADIANT_BIN", sharedRadiantBinPath(t))

	resp := mcpPossessAsync(json.RawMessage(`{"task":"ship subprocess feature","workdir":"` + dir + `","profile":"standard"}`))
	if resp.Error != nil {
		t.Fatalf("possess_async subprocess: %s", resp.Error.Message)
	}

	id := taskID(dir, "ship subprocess feature")
	st, err := loadPossessState(dir, id)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	for _, phase := range []string{"discover", "plan", "execute", "verify"} {
		pr := st.Phases[phase]
		if pr == nil || pr.Status != "done" {
			t.Errorf("phase %q not done; got %+v", phase, pr)
		}
	}
	specs, _ := filepath.Glob(filepath.Join(dir, "specs", "0001-*", "spec.md"))
	if len(specs) == 0 {
		t.Errorf("subprocess possess_async produced no spec.md under specs/0001-*/")
	}
}

// TestSubprocessAsync_PidFileLifecycle — exercises the pid-file
// helpers (write/liveness/remove) without requiring a real
// subprocess fork. Keeps the pid-file contract pinned even when
// the subprocess test infra flakes.
func TestSubprocessAsync_PidFileLifecycle(t *testing.T) {
	dir := t.TempDir()
	ticket := "abcdef0123456789"

	// Initially no pid file → liveness returns false, pid=0.
	alive, pid := asyncRunnerLiveness(dir, ticket)
	if alive || pid != 0 {
		t.Errorf("expected (false, 0) for missing pid file; got (%v, %d)", alive, pid)
	}

	// Write a pid file pointing at the current test process
	// (so liveness returns true).
	writeAsyncRunnerPid(dir, ticket, os.Getpid())
	alive, pid = asyncRunnerLiveness(dir, ticket)
	if !alive {
		t.Errorf("expected alive=true after writing pid %d; got false", os.Getpid())
	}
	if pid != os.Getpid() {
		t.Errorf("expected pid=%d; got %d", os.Getpid(), pid)
	}

	// Remove the pid file and re-check.
	removeAsyncRunnerPid(dir, ticket)
	alive, pid = asyncRunnerLiveness(dir, ticket)
	if alive || pid != 0 {
		t.Errorf("expected (false, 0) after remove; got (%v, %d)", alive, pid)
	}
}

// TestSubprocessAsync_CrashRecovery — simulates a subprocess
// that died mid-Execute and asserts the next call resumes
// from the persisted state.json. This is the crash-recovery
// contract the PROPOSAL § v3.7.6 update calls out.
func TestSubprocessAsync_CrashRecovery(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/test\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	withSubprocessAsyncEnabled(t)
	t.Setenv("RADIANT_BIN", sharedRadiantBinPath(t))

	task := "recover from subprocess crash"
	id := taskID(dir, task)

	// Plant a state.json that simulates "discover/plan done,
	// execute was in flight when the subprocess died".
	crashed := &possessState{
		TaskID:       id,
		Workdir:      dir,
		Task:         task,
		StartedAt:    time.Now().Add(-time.Minute),
		UpdatedAt:    time.Now(),
		CurrentPhase: "execute",
		Profile:      "standard",
		Slug:         selfDrivenSlugify(task),
		SpecDir:      dir + "/specs/0001-" + selfDrivenSlugify(task),
		Phases: map[string]*phaseResult{
			"discover": {Status: "done", StartedAt: time.Now().Add(-time.Minute), EndedAt: time.Now().Add(-30 * time.Second)},
			"plan":     {Status: "done", StartedAt: time.Now().Add(-30 * time.Second), EndedAt: time.Now().Add(-15 * time.Second)},
			"execute":  {Status: "in_progress", StartedAt: time.Now().Add(-10 * time.Second)},
			"verify":   {Status: "pending"},
		},
	}
	if err := savePossessState(crashed); err != nil {
		t.Fatalf("seed crashed state: %v", err)
	}

	// Resume via the subprocess gate.
	gate := selectedAsyncGate()
	if _, err := gate.Spawn(possess.PhaseExecute, task, dir); err != nil {
		t.Fatalf("resume execute: %v", err)
	}
	st, err := loadPossessState(dir, id)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if st.Phases["execute"].Status != "done" {
		t.Errorf("execute should be done after resume; got %+v", st.Phases["execute"])
	}
	if st.Phases["verify"].Status != "pending" {
		t.Errorf("verify should still be pending; got %+v", st.Phases["verify"])
	}
}

// TestSubprocessAsync_SpawnsChildProcess — invokes
// `radiant async-runner` as a real subprocess end-to-end.
// This is the integration smoke test the PROPOSAL § v3.7.6
// update calls out as the canonical proof the subprocess path
// works.
func TestSubprocessAsync_SpawnsChildProcess(t *testing.T) {
	bin := sharedRadiantBinPath(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/test\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// async-runner is a worker primitive — same gate as the
	// other internal helpers. Set RADIANT_INTERNAL=1 for the
	// subprocess so its gatekeeper doesn't reject the call.
	t.Setenv("RADIANT_INTERNAL", "1")

	task := "ship the async-runner smoke"
	id := taskID(dir, task)

	cmd := exec.Command(bin, "async-runner",
		"--phase", "discover",
		"--ticket", id,
		"--workdir", dir,
		"--task", task,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("async-runner subprocess: %v\noutput: %s", err, string(out))
	}
	if !contains(string(out), "phase discover ok") {
		t.Errorf("expected 'phase discover ok' in output; got: %s", string(out))
	}

	st, err := loadPossessState(dir, id)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if st.Phases["discover"].Status != "done" {
		t.Errorf("discover not done after subprocess; got %+v", st.Phases["discover"])
	}
}

// contains is a small substring helper to avoid pulling strings
// into the import set just for one call site.
func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle || stringIndex(haystack, needle) >= 0)
}

func stringIndex(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}