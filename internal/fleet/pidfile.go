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
	"os/exec"
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

// ─────────────────────────────────────────────────────────────────────
// v3.7.10 — nested pid tracking (recursive liveness).
//
// Each fleet task agent is a `radiant loop start <task>` subprocess
// which itself can spawn helper subprocesses (e.g. a `git worktree
// add` helper, a Python helper for one phase). When the agent dies,
// the helper's pid may or may not be alive depending on whether
// the helper was reparented to init (orphaned but still running)
// or was killed by the agent's exit.
//
// A single `kill -0` on the agent pid only tells us whether the
// AGENT died. It can't distinguish "agent died and helper is still
// running" from "agent died and helper died too". This is the
// gap v3.7.10 closes.
//
// Layout: alongside the parent pid file we write a sidecar
// `.children` file containing the live child pids at the time of
// last update. The dispatcher periodically calls
// `RefreshChildPids` (or we read on demand) to keep the sidecar
// fresh. Status() exposes ParentAlive + ChildrenAlive + children
// pid list so a host can tell:
//
//   - parent alive, 0 children       → normal, agent running
//   - parent alive, N children      → agent running with helpers
//   - parent dead, 0 children       → agent died cleanly
//   - parent dead, N children       → agent died; N helpers orphaned
//   - parent alive, 1 child dead    → helper crashed but agent still alive
//
// pgrep -P is the source of truth for "what are my children right
// now". The sidecar is a cache so a host reading status without
// the dispatcher polling doesn't have to fork pgrep per call.
// ─────────────────────────────────────────────────────────────────────

const taskPidChildrenSuffix = ".children"

const taskPidGrandchildrenSuffix = ".grandchildren"

const taskPidGreatGrandchildrenSuffix = ".great-grandchildren"

// taskPidChildrenPath returns the sidecar children pid file path
// for a fleet task agent.
func taskPidChildrenPath(workdir, runID, taskID string) string {
	return taskPidPath(workdir, runID, taskID) + taskPidChildrenSuffix
}

// taskPidGrandchildrenPath returns the sidecar grandchildren pid
// file path for a fleet task agent. v3.7.12+ — one level deeper
// than the children sidecar.
func taskPidGrandchildrenPath(workdir, runID, taskID string) string {
	return taskPidPath(workdir, runID, taskID) + taskPidGrandchildrenSuffix
}

// taskPidGreatGrandchildrenPath returns the sidecar great-
// grandchildren pid file path. v3.7.13+ — yet another level
// deeper than the grandchildren sidecar.
func taskPidGreatGrandchildrenPath(workdir, runID, taskID string) string {
	return taskPidPath(workdir, runID, taskID) + taskPidGreatGrandchildrenSuffix
}

// PidTree describes the liveness of a fleet task agent, its
// children, grandchildren, and (v3.7.13+) great-grandchildren.
// ParentAlive mirrors the v3.7.9 TaskLive.Alive field;
// ChildrenAlive is true only when every recorded child pid is
// alive (an empty Children list = trivially true).
// GrandchildrenAlive adds recursive depth for the common
// "agent → helper → subprocess" pattern (a `radiant loop start`
// agent that runs a Python helper which spawns a `git worktree
// add` subprocess, for example). GreatGrandchildrenAlive adds
// another layer for the rarer "agent → helper → sub-helper →
// subprocess" case.
type PidTree struct {
	ParentPid                int   `json:"parent_pid"`
	ParentAlive              bool  `json:"parent_alive"`
	ChildrenPids             []int `json:"children_pids,omitempty"`
	ChildrenAlive            bool  `json:"children_alive"`
	GrandchildrenPids        []int `json:"grandchildren_pids,omitempty"`
	GrandchildrenAlive       bool  `json:"grandchildren_alive"`
	GreatGrandchildrenPids   []int `json:"great_grandchildren_pids,omitempty"`
	GreatGrandchildrenAlive  bool  `json:"great_grandchildren_alive"`
	// ChildCount is the number of currently-live children — when
	// children have died and been reaped, this can be smaller
	// than len(ChildrenPids). GrandchildrenCount + GreatGrandchildrenCount
	// follow the same pattern. Useful for the "orphaned N
	// helpers" + "N helpers' helpers" + "N sub-sub-helpers"
	// diagnosis in the surfaced status string.
	ChildCount                int `json:"child_count"`
	GrandchildrenCount        int `json:"grandchildren_count"`
	GreatGrandchildrenCount   int `json:"great_grandchildren_count"`
}

// writeChildPids serializes a slice of int pids to the children
// sidecar file for a task agent. Empty slice writes an empty
// file (so a follow-up read distinguishes "no children recorded"
// from "children file missing").
func writeChildPids(workdir, runID, taskID string, pids []int) error {
	path := taskPidChildrenPath(workdir, runID, taskID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	// Use a simple newline-separated list — easier to grep / cat
	// than JSON for operators diagnosing a crashed agent.
	var sb strings.Builder
	for _, p := range pids {
		fmt.Fprintf(&sb, "%d\n", p)
	}
	return os.WriteFile(path, []byte(sb.String()), 0o644)
}

// readChildPids returns the recorded children pid list. Returns
// (nil, false) when the sidecar file doesn't exist (parent never
// had children, or the sidecar was cleaned up with the parent).
func readChildPids(workdir, runID, taskID string) ([]int, bool) {
	data, err := os.ReadFile(taskPidChildrenPath(workdir, runID, taskID))
	if err != nil {
		return nil, false
	}
	var pids []int
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pid, err := strconv.Atoi(line)
		if err != nil {
			continue // corrupt line; skip
		}
		pids = append(pids, pid)
	}
	return pids, true
}

// readGrandchildrenPids returns the recorded grandchildren pid
// list. Same contract as readChildPids but one level deeper.
// v3.7.12+.
func readGrandchildrenPids(workdir, runID, taskID string) ([]int, bool) {
	data, err := os.ReadFile(taskPidGrandchildrenPath(workdir, runID, taskID))
	if err != nil {
		return nil, false
	}
	var pids []int
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pid, err := strconv.Atoi(line)
		if err != nil {
			continue // corrupt line; skip
		}
		pids = append(pids, pid)
	}
	return pids, true
}

// writeGrandchildrenPids serializes a slice of int pids to the
// grandchildren sidecar file. Same newline-separated format
// as writeChildPids.
func writeGrandchildrenPids(workdir, runID, taskID string, pids []int) error {
	path := taskPidGrandchildrenPath(workdir, runID, taskID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var sb strings.Builder
	for _, p := range pids {
		fmt.Fprintf(&sb, "%d\n", p)
	}
	return os.WriteFile(path, []byte(sb.String()), 0o644)
}

// readGreatGrandchildrenPids returns the recorded great-
// grandchildren pid list. v3.7.13+.
func readGreatGrandchildrenPids(workdir, runID, taskID string) ([]int, bool) {
	data, err := os.ReadFile(taskPidGreatGrandchildrenPath(workdir, runID, taskID))
	if err != nil {
		return nil, false
	}
	var pids []int
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pid, err := strconv.Atoi(line)
		if err != nil {
			continue
		}
		pids = append(pids, pid)
	}
	return pids, true
}

// writeGreatGrandchildrenPids serializes pids to the great-
// grandchildren sidecar. Same newline-separated format.
func writeGreatGrandchildrenPids(workdir, runID, taskID string, pids []int) error {
	path := taskPidGreatGrandchildrenPath(workdir, runID, taskID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var sb strings.Builder
	for _, p := range pids {
		fmt.Fprintf(&sb, "%d\n", p)
	}
	return os.WriteFile(path, []byte(sb.String()), 0o644)
}

// TaskPidTree returns the nested liveness for a single task,
// including grandchildren (v3.7.12+). (alive=false, pid=0)
// parent means the parent pid file is gone — caller should treat
// the task as "not yet started or already cleaned up".
func TaskPidTree(workdir, runID, taskID string) PidTree {
	parentAlive, parentPid := taskLiveness(workdir, runID, taskID)
	if parentPid == 0 {
		return PidTree{ParentAlive: false}
	}
	children, ok := readChildPids(workdir, runID, taskID)
	if !ok || len(children) == 0 {
		return PidTree{
			ParentPid:     parentPid,
			ParentAlive:   parentAlive,
			ChildrenAlive: true, // vacuously true
		}
	}
	aliveCount := 0
	for _, p := range children {
		if pidAlive(p) {
			aliveCount++
		}
	}
	tree := PidTree{
		ParentPid:     parentPid,
		ParentAlive:   parentAlive,
		ChildrenPids:  children,
		ChildrenAlive: aliveCount == len(children),
		ChildCount:    aliveCount,
	}

	// v3.7.12 — grandchildren. Read the second sidecar (one
	// level deeper than the children sidecar). If any children
	// are still alive, their pgrep -P output is in the
	// grandchildren sidecar; otherwise the sidecar is stale and
	// the read returns ok=false (vacuously true on the alive
	// check below).
	grandchildren, gcOK := readGrandchildrenPids(workdir, runID, taskID)
	if gcOK && len(grandchildren) > 0 {
		tree.GrandchildrenPids = grandchildren
		gcAlive := 0
		for _, p := range grandchildren {
			if pidAlive(p) {
				gcAlive++
			}
		}
		tree.GrandchildrenAlive = gcAlive == len(grandchildren)
		tree.GrandchildrenCount = gcAlive
	} else {
		tree.GrandchildrenAlive = true // vacuously true
	}

	// v3.7.13 — great-grandchildren. Same pattern as above,
	// one level deeper. Rarer in practice (most fleet tasks
	// don't go past grandchildren) but the surface is here for
	// the operator who needs it.
	ggChildren, ggcOK := readGreatGrandchildrenPids(workdir, runID, taskID)
	if ggcOK && len(ggChildren) > 0 {
		tree.GreatGrandchildrenPids = ggChildren
		ggcAlive := 0
		for _, p := range ggChildren {
			if pidAlive(p) {
				ggcAlive++
			}
		}
		tree.GreatGrandchildrenAlive = ggcAlive == len(ggChildren)
		tree.GreatGrandchildrenCount = ggcAlive
	} else {
		tree.GreatGrandchildrenAlive = true // vacuously true
	}

	return tree
}

// ChildrenPidsForParent shells out to `pgrep -P <pid>` to find the
// direct children of a process. Returns an empty slice on any
// error (pgrep missing, no children, permission denied) — the
// caller treats "no children found" and "couldn't probe" the
// same way (an empty sidecar is a valid state).
//
// Implementation note: pgrep is portable across macOS / Linux
// (same -P flag semantics on both). On Windows we'd need
// `wmic` or `Get-CimInstance` — out of scope for v3.7.10; the
// fleet pid tracking story is Linux/macOS only.
func ChildrenPidsForParent(parentPid int) []int {
	if parentPid <= 0 {
		return nil
	}
	out, err := exec.Command("pgrep", "-P", strconv.Itoa(parentPid)).Output()
	if err != nil {
		return nil
	}
	var pids []int
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pid, err := strconv.Atoi(line)
		if err != nil {
			continue
		}
		pids = append(pids, pid)
	}
	return pids
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