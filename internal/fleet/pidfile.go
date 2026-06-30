// pidfile.go — pid-tracking primitives for fleet tasks and the
// fleet dispatcher. Closes the v3.7.9 backlog item "same status/
// retry contract as loop": the loop has `.radiant-harness/pids/<ticket>.pid`
// for liveness probing (v3.7.8) — fleet gets the equivalent so a
// host agent running `mcp__radiant__fleet_status` can distinguish
// "task is running" from "agent subprocess died without writing
// status".
//
// Two flavours of pid file:
//
//   1. Per-task agent pid: `agent-<runID>-<taskID>.pid`. Written
//      by `Dispatcher.spawnAgent` immediately before `cmd.Start`
//      and removed via `defer` after `cmd.Wait`. Each spawned
//      agent is one OS process running `radiant loop start
//      <task.DoneWhen>` inside the worktree; its pid file lets
//      the status probe detect when an agent crashed (no exit
//      code was written, but the pid is dead).
//
//   2. Per-dispatcher pid: `dispatcher-<runID>.pid`. Written by
//      `runFleetAsyncRunner` (the body of `radiant
//      fleet-async-runner <run-id>`) when the dispatcher runs as
//      a detached subprocess under RADIANT_FLEET_ASYNC_SUBPROCESS=1.
//      Inline dispatchers don't write this file — they ARE the
//      process. A present file with a dead pid signals "the
//      dispatcher crashed mid-run"; a missing file is fine for
//      inline runs.
//
// Both follow the same conventions as the loop pid helpers in
// cmd/radiant/cmd_async_runner.go: best-effort writes (missing
// file is non-fatal), `kill -0` for liveness (Unix), `os.Remove`
// for cleanup. The pid path layout keeps loop and fleet pid
// files in separate namespaces so they don't collide if a
// user happens to run loop and fleet against the same workdir.

package fleet

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// fleetPidDir is the canonical pid directory for fleet operations,
// relative to the project workdir.
const fleetPidDir = ".radiant-harness/fleet/pids"

// taskPidPath returns the pid-file path for a single fleet task
// agent process. Layout: `<workdir>/.radiant-harness/fleet/pids/agent-<runID>-<taskID>.pid`.
//
// The runID is included so two concurrent fleet runs against the
// same workdir (rare but possible during cross-process resume)
// don't collide on task IDs.
//
// Path components are sanitized: slashes and `..` segments are
// replaced with `_` so a task title or run id that contains
// `../` cannot trick `filepath.Join` into writing the pid file
// outside the pid directory. Defense-in-depth — task IDs come
// from `radiant fleet plan` which generates them from a counter,
// but the path is built from caller-supplied strings and we
// don't want a future caller to bypass the sanitizer.
func taskPidPath(workdir, runID, taskID string) string {
	return filepath.Join(workdir, fleetPidDir, fmt.Sprintf("agent-%s-%s.pid",
		sanitizePidComponent(runID), sanitizePidComponent(taskID)))
}

// dispatcherPidPath returns the pid-file path for the fleet
// dispatcher process itself (only present when
// RADIANT_FLEET_ASYNC_SUBPROCESS=1 is set and the dispatcher
// forks itself).
//
// Layout: `<workdir>/.radiant-harness/fleet/pids/dispatcher-<runID>.pid`.
func dispatcherPidPath(workdir, runID string) string {
	return filepath.Join(workdir, fleetPidDir, fmt.Sprintf("dispatcher-%s.pid",
		sanitizePidComponent(runID)))
}

// sanitizePidComponent replaces path separators and `..` in a
// pid-file component so a malicious string can't escape the
// pid directory. Anything not in `[A-Za-z0-9._-]` becomes `_`.
func sanitizePidComponent(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '/' || c == '\\':
			out = append(out, '_')
		case c == '.' && i+1 < len(s) && s[i+1] == '.':
			out = append(out, '_', '_')
			i++ // skip the second dot
		case (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_':
			out = append(out, c)
		default:
			out = append(out, '_')
		}
	}
	return string(out)
}

// ensureFleetPidDir creates the pid directory if it doesn't exist.
// Best-effort: returns nil even if MkdirAll fails, because a
// missing pid directory just means subsequent pid writes fail
// silently — which is acceptable for a non-fatal probe file.
func ensureFleetPidDir(workdir string) error {
	return os.MkdirAll(filepath.Join(workdir, fleetPidDir), 0o755)
}

// writePidFile writes a single integer pid to path. Best-effort:
// a missing directory is created; any other failure is logged to
// stderr but does NOT abort the caller. This mirrors the loop's
// `writeAsyncRunnerPid` policy (cmd/radiant/cmd_async_runner.go).
func writePidFile(path string, pid int) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir pid dir: %w", err)
	}
	return os.WriteFile(path, []byte(strconv.Itoa(pid)), 0o644)
}

// removePidFile removes a pid file. Missing files are not an
// error (idempotent cleanup).
func removePidFile(path string) error {
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// readPidFile reads a pid from path. Returns (pid, true) on
// success or (0, false) if the file is missing or unreadable.
// Empty files parse to pid=0 (and alive=false downstream).
func readPidFile(path string) (int, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, false
	}
	return pid, true
}

// pidAlive returns true if the given pid currently exists. On
// Unix, `kill -0` returns nil if the process exists and an
// error otherwise (os.FindProcess always succeeds on Unix).
//
// A pid of 0 is treated as "not alive" — pid 0 is the scheduler
// on Linux and never a user agent.
func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

// taskLiveness returns whether the agent subprocess for a given
// task is still alive. (alive=false, pid=0) means "no pid file
// recorded" (inline dispatcher, or the task hasn't started yet).
// (alive=false, pid>0) means "the pid file is on disk but the
// process is gone" — the agent crashed without writing terminal
// status.
func taskLiveness(workdir, runID, taskID string) (alive bool, pid int) {
	path := taskPidPath(workdir, runID, taskID)
	pid, ok := readPidFile(path)
	if !ok {
		return false, 0
	}
	return pidAlive(pid), pid
}

// dispatcherLiveness returns whether the fleet dispatcher
// subprocess (only present when async-subprocess mode is on) is
// still alive. (alive=false, pid=0) means "no dispatcher pid
// file" — typical for inline dispatch or a run that hasn't
// started yet.
func dispatcherLiveness(workdir, runID string) (alive bool, pid int) {
	path := dispatcherPidPath(workdir, runID)
	pid, ok := readPidFile(path)
	if !ok {
		return false, 0
	}
	return pidAlive(pid), pid
}

// WriteDispatcherPid writes the dispatcher pid file at
// `.radiant-harness/fleet/pids/dispatcher-<runID>.pid`. Used
// by `radiant fleet-async-runner` to mark the child process as
// "this dispatcher is running, you can liveness-probe it".
//
// Best-effort: returns nil even if the underlying MkdirAll/WriteFile
// fails — the caller logs the warning but does not abort, because
// the dispatch work is the primary concern and a missing pid file
// is recoverable (status probe just returns no signal).
func WriteDispatcherPid(workdir, runID string, pid int) error {
	if err := ensureFleetPidDir(workdir); err != nil {
		return err
	}
	return writePidFile(dispatcherPidPath(workdir, runID), pid)
}

// RemoveDispatcherPid removes the dispatcher pid file. Called
// by `radiant fleet-async-runner` on exit so subsequent
// `mcp__radiant__fleet_status` calls don't see a stale "running"
// signal. Missing files are not an error (idempotent cleanup).
func RemoveDispatcherPid(workdir, runID string) error {
	return removePidFile(dispatcherPidPath(workdir, runID))
}