// cmd_mcp_fleet_async_test.go — coverage for the v3.7.9 fleet
// MCP wrappers (radiant_fleet_status, radiant_fleet_resume) and
// the fleet-async-runner subprocess entry point.
//
//   - TestMCPFleetStatus_EmptyRun — verifies status returns the
//     expected shape when the fleet store has no tasks yet.
//   - TestMCPFleetStatus_CrashedEscalation — plants a stale task
//     pid file and asserts the status surfaces TaskCrashed.
//   - TestMCPFleetStatus_MissingRun — verifies the error path
//     when run_id doesn't exist.
//   - TestMCPFleetResume_NoFailedTasks — verifies resume is a
//     no-op when there are no failed tasks to re-dispatch.
//   - TestFleetAsyncRunner_EnvGuard — verifies the
//     `radiant fleet-async-runner` subcommand refuses to run
//     when RADIANT_FLEET_ASYNC_RUNNER is not set.

package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/quant-risk/radiant-harness/v3/internal/fleet"
)

// withFleetSubprocessAsyncEnabled opts into fleet async subprocess
// for the duration of the test. Same scoping rules as the loop's
// `withSubprocessAsyncEnabled` helper.
func withFleetSubprocessAsyncEnabled(t *testing.T) {
	t.Helper()
	t.Setenv("RADIANT_FLEET_ASYNC_SUBPROCESS", "1")
}

// TestMCPFleetStatus_EmptyRun — a freshly-created fleet run has
// no tasks yet, so the status returns the expected "no tasks"
// hint and the liveness fields are absent.
func TestMCPFleetStatus_EmptyRun(t *testing.T) {
	dir := t.TempDir()
	store, err := fleet.NewStore(dir, "run-empty", "ship the thing")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	_ = store

	resp := mcpFleetStatus(json.RawMessage(`{"run_id":"run-empty","workdir":"` + dir + `"}`))
	if resp.Error != nil {
		t.Fatalf("status error: %+v", resp.Error)
	}
	content := resp.Result.(map[string]interface{})["content"].([]map[string]string)
	text := content[0]["text"]
	if !strings.Contains(text, "run-empty") {
		t.Errorf("status text missing run id: %s", text)
	}
	if !strings.Contains(text, "(no tasks") && !strings.Contains(text, "radiant fleet plan") {
		t.Errorf("status should hint at plan step: %s", text)
	}
}

// TestMCPFleetStatus_CrashedEscalation — plant a stale pid for
// an assigned task and verify the status surfaces TaskCrashed
// (not TaskAssigned).
func TestMCPFleetStatus_CrashedEscalation(t *testing.T) {
	dir := t.TempDir()
	store, err := fleet.NewStore(dir, "run-crash", "ship it")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if err := store.SetTasks([]fleet.Task{
		{ID: "t1", Title: "task one", Status: fleet.TaskAssigned},
	}); err != nil {
		t.Fatalf("SetTasks: %v", err)
	}
	// Plant a stale pid (pid 16777215 is above pid_max on
	// Linux/macOS so kill -0 returns "no such process"). The
	// pid directory must exist first; in production the
	// dispatcher writes it, but in tests we set it up
	// ourselves.
	pidDir := filepath.Join(dir, ".radiant-harness", "fleet", "pids")
	if err := os.MkdirAll(pidDir, 0o755); err != nil {
		t.Fatalf("mkdir pid dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pidDir, "agent-run-crash-t1.pid"),
		[]byte("16777215"), 0o644); err != nil {
		t.Fatalf("plant stale pid: %v", err)
	}

	resp := mcpFleetStatus(json.RawMessage(`{"run_id":"run-crash","workdir":"` + dir + `"}`))
	if resp.Error != nil {
		t.Fatalf("status error: %+v", resp.Error)
	}
	content := resp.Result.(map[string]interface{})["content"].([]map[string]string)
	text := content[0]["text"]
	if !strings.Contains(text, "crashed") {
		t.Errorf("status text should mention 'crashed' for dead pid, got: %s", text)
	}
	if !strings.Contains(text, "16777215") {
		t.Errorf("status text should surface the dead pid for diagnosis, got: %s", text)
	}
	if !strings.Contains(text, "radiant_fleet_resume") {
		t.Errorf("status text should hint at next step (resume), got: %s", text)
	}
}

// TestMCPFleetStatus_MissingRun — run_id that doesn't exist on
// disk should produce an MCP error response, not a panic.
func TestMCPFleetStatus_MissingRun(t *testing.T) {
	dir := t.TempDir()
	resp := mcpFleetStatus(json.RawMessage(`{"run_id":"never-existed","workdir":"` + dir + `"}`))
	if resp.Error == nil {
		t.Fatalf("expected error for missing run, got result")
	}
	if !strings.Contains(resp.Error.Message, "load fleet") {
		t.Errorf("error message should mention load failure: %s", resp.Error.Message)
	}
}

// TestMCPFleetStatus_RequiresRunID — the tool rejects empty
// run_id with a clear error so a host agent doesn't accidentally
// probe the wrong store.
func TestMCPFleetStatus_RequiresRunID(t *testing.T) {
	resp := mcpFleetStatus(json.RawMessage(`{"workdir":"/tmp"}`))
	if resp.Error == nil {
		t.Fatalf("expected error for missing run_id")
	}
	if !strings.Contains(resp.Error.Message, "run_id") {
		t.Errorf("error message should mention run_id: %s", resp.Error.Message)
	}
}

// TestMCPFleetResume_NoFailedTasks — resume on a fleet run with
// no failed tasks is a successful no-op (returns the same
// status it would have without the call). Verifies the tool
// doesn't error spuriously.
//
// Note: the resume path needs a git repo (the Isolator calls
// `git rev-parse --is-inside-work-tree` to set up worktrees).
// We init a fresh repo in the temp dir.
func TestMCPFleetResume_NoFailedTasks(t *testing.T) {
	dir := t.TempDir()
	initGitRepoForTest(t, dir)

	store, err := fleet.NewStore(dir, "run-clean", "ship it")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if err := store.SetTasks([]fleet.Task{
		{ID: "t1", Title: "task one", Status: fleet.TaskDone},
	}); err != nil {
		t.Fatalf("SetTasks: %v", err)
	}

	resp := mcpFleetResume(json.RawMessage(`{"run_id":"run-clean","workdir":"` + dir + `"}`))
	if resp.Error != nil {
		t.Fatalf("resume error: %+v", resp.Error)
	}
	content := resp.Result.(map[string]interface{})["content"].([]map[string]string)
	text := content[0]["text"]
	if !strings.Contains(text, "run-clean") {
		t.Errorf("resume text should reference the run id: %s", text)
	}
}

// TestMCPFleetResume_MissingRun — same error path as status.
func TestMCPFleetResume_MissingRun(t *testing.T) {
	dir := t.TempDir()
	resp := mcpFleetResume(json.RawMessage(`{"run_id":"never-existed","workdir":"` + dir + `"}`))
	if resp.Error == nil {
		t.Fatalf("expected error for missing run")
	}
}

// TestFleetAsyncRunner_EnvGuard — the `radiant fleet-async-runner`
// subcommand MUST refuse to run when RADIANT_FLEET_ASYNC_RUNNER
// is not set. Otherwise an operator manually invoking the
// subcommand would create a dispatcher pid file that
// `mcp__radiant__fleet_status` would then probe — the file
// would dangle forever.
func TestFleetAsyncRunner_EnvGuard(t *testing.T) {
	// Run the cobra command directly (no env). Should exit
	// with an error mentioning the env guard.
	root := newTestRootForFleetRunner()
	root.SetArgs([]string{"fleet-async-runner", "run-x"})
	// Make sure the env var is unset.
	t.Setenv("RADIANT_FLEET_ASYNC_RUNNER", "")
	err := root.Execute()
	if err == nil {
		t.Fatalf("expected env-guard error, got success")
	}
	if !strings.Contains(err.Error(), "RADIANT_FLEET_ASYNC_RUNNER") {
		t.Errorf("error should mention the missing env var: %v", err)
	}
}

// TestFleetAsyncSubprocessEnabled_DefaultsFalse — pin the
// default (no env) so a future change doesn't accidentally
// enable subprocess mode for everyone.
func TestFleetAsyncSubprocessEnabled_DefaultsFalse(t *testing.T) {
	t.Setenv("RADIANT_FLEET_ASYNC_SUBPROCESS", "")
	if fleetAsyncSubprocessEnabled() {
		t.Errorf("fleetAsyncSubprocessEnabled() = true with empty env")
	}
	t.Setenv("RADIANT_FLEET_ASYNC_SUBPROCESS", "1")
	if !fleetAsyncSubprocessEnabled() {
		t.Errorf("fleetAsyncSubprocessEnabled() = false with RADIANT_FLEET_ASYNC_SUBPROCESS=1")
	}
	t.Setenv("RADIANT_FLEET_ASYNC_SUBPROCESS", "true")
	if !fleetAsyncSubprocessEnabled() {
		t.Errorf("fleetAsyncSubprocessEnabled() = false with RADIANT_FLEET_ASYNC_SUBPROCESS=true")
	}
}

// newTestRootForFleetRunner builds a minimal cobra root with
// only the fleet-async-runner command attached. Avoids pulling
// in the full root from main (which would try to discover
// project roots, run side effects, etc.).
func newTestRootForFleetRunner() *cobra.Command {
	root := &cobra.Command{Use: "radiant"}
	registerFleetAsyncRunnerCmd(root)
	return root
}

// initGitRepoForTest initializes a fresh git repo in dir so the
// Isolator (which calls `git rev-parse --is-inside-work-tree`)
// can proceed. Skips the test on systems without git.
func initGitRepoForTest(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test"},
		{"commit", "--allow-empty", "-q", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Skipf("git %v failed: %v\n%s", args, err, out)
		}
	}
}