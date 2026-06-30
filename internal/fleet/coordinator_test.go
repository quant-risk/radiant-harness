// coordinator_test.go — coverage for v3.7.9 FleetStatus liveness
// fields + crashed-status escalation. Mirrors the loop's
// phaseStatusSummary crashed branch but for fleet tasks.

package fleet

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFleetStatus_LivenessDisabledByDefault pins the
// backwards-compat contract: a Coordinator without WithLivenessDir
// returns FleetStatus WITHOUT the v3.7.9 fields populated. Old
// callers (CLI `radiant fleet status`, history dumps) parse the
// `Tasks` array and don't read liveness — omitempty keeps them
// working.
func TestFleetStatus_LivenessDisabledByDefault(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir, "run-1", "test goal")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if err := store.SetTasks([]Task{
		{ID: "t1", Title: "task one", Status: TaskAssigned},
	}); err != nil {
		t.Fatalf("SetTasks: %v", err)
	}
	coord := NewCoordinator(store, 1)
	status := coord.Status()

	if status.DispatcherAlive {
		t.Errorf("DispatcherAlive=true with no livenessDir")
	}
	if status.DispatcherPid != 0 {
		t.Errorf("DispatcherPid=%d, want 0 with no livenessDir", status.DispatcherPid)
	}
	if status.TaskLiveness != nil {
		t.Errorf("TaskLiveness=%v, want nil with no livenessDir", status.TaskLiveness)
	}
	// Tasks array is unmodified.
	if len(status.Tasks) != 1 || status.Tasks[0].Status != TaskAssigned {
		t.Errorf("tasks mutated by Status(): %+v", status.Tasks)
	}
}

// TestFleetStatus_CrashedEscalation fires the crashed escalation:
// a task the store considers TaskAssigned but whose pid file
// points at a dead process must surface as TaskCrashed so the
// operator sees "agent died" not "still running". Pid 16777215
// is above pid_max on Linux/macOS so kill -0 returns "no such
// process".
func TestFleetStatus_CrashedEscalation(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir, "run-2", "test goal")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if err := store.SetTasks([]Task{
		{ID: "t1", Title: "task one", Status: TaskAssigned},
		{ID: "t2", Title: "task two", Status: TaskAssigned},
	}); err != nil {
		t.Fatalf("SetTasks: %v", err)
	}
	// Simulate a crashed agent: write a stale pid for t1.
	stale := 16777215
	if err := writePidFile(taskPidPath(dir, "run-2", "t1"), stale); err != nil {
		t.Fatalf("writePidFile: %v", err)
	}
	// t2 has no pid file — "not yet started".

	coord := NewCoordinator(store, 2).WithLivenessDir(dir)
	status := coord.Status()

	// t1: crashed (assigned + dead pid).
	if status.Tasks[0].Status != TaskCrashed {
		t.Errorf("t1 status = %q, want %q", status.Tasks[0].Status, TaskCrashed)
	}
	if !contains(status.Tasks[0].Evidence, "16777215") {
		t.Errorf("t1 evidence should reference the dead pid, got %q", status.Tasks[0].Evidence)
	}
	// t2: still assigned (no pid file → not crashed).
	if status.Tasks[1].Status != TaskAssigned {
		t.Errorf("t2 status = %q, want %q (no pid file = not crashed)", status.Tasks[1].Status, TaskAssigned)
	}
	// Liveness map reflects both tasks.
	if len(status.TaskLiveness) != 2 {
		t.Errorf("TaskLiveness len = %d, want 2", len(status.TaskLiveness))
	}
	if l := status.TaskLiveness["t1"]; l.Alive || l.Pid != stale {
		t.Errorf("t1 liveness = %+v, want alive=false pid=%d", l, stale)
	}
	if l := status.TaskLiveness["t2"]; l.Alive || l.Pid != 0 {
		t.Errorf("t2 liveness = %+v, want alive=false pid=0 (no file)", l)
	}
}

// TestFleetStatus_DispatcherLiveness verifies the dispatcher pid
// file is read correctly when present.
func TestFleetStatus_DispatcherLiveness(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir, "run-3", "test goal")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	coord := NewCoordinator(store, 0).WithLivenessDir(dir)

	// No file → no dispatcher signal.
	status := coord.Status()
	if status.DispatcherAlive || status.DispatcherPid != 0 {
		t.Errorf("expected no dispatcher signal, got alive=%v pid=%d",
			status.DispatcherAlive, status.DispatcherPid)
	}

	// Write our own pid → alive=true.
	our := os.Getpid()
	if err := WriteDispatcherPid(dir, "run-3", our); err != nil {
		t.Fatalf("WriteDispatcherPid: %v", err)
	}
	status = coord.Status()
	if !status.DispatcherAlive || status.DispatcherPid != our {
		t.Errorf("dispatcher signal after write: alive=%v pid=%d, want alive=true pid=%d",
			status.DispatcherAlive, status.DispatcherPid, our)
	}

	// Stale pid → alive=false pid=stale (crash signal).
	stale := 16777215
	if err := WriteDispatcherPid(dir, "run-3", stale); err != nil {
		t.Fatalf("WriteDispatcherPid(stale): %v", err)
	}
	status = coord.Status()
	if status.DispatcherAlive {
		t.Errorf("DispatcherAlive=true for stale pid %d", stale)
	}
	if status.DispatcherPid != stale {
		t.Errorf("DispatcherPid=%d, want %d", status.DispatcherPid, stale)
	}
}

// TestCrashTask_PersistsStatus verifies the Store helper for
// promoting a task to crashed. Round-trips through the store
// so a crashed state survives across process restarts.
func TestCrashTask_PersistsStatus(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir, "run-4", "test goal")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if err := store.SetTasks([]Task{
		{ID: "t1", Title: "task one", Status: TaskAssigned},
	}); err != nil {
		t.Fatalf("SetTasks: %v", err)
	}
	if err := store.CrashTask("t1", "pid 12345 not alive"); err != nil {
		t.Fatalf("CrashTask: %v", err)
	}
	// Reload from disk to confirm persistence.
	store2, err := LoadStore(dir, "run-4")
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	if store2.Snapshot().Tasks[0].Status != TaskCrashed {
		t.Errorf("reloaded status = %q, want %q",
			store2.Snapshot().Tasks[0].Status, TaskCrashed)
	}
	if store2.Snapshot().Tasks[0].Evidence != "pid 12345 not alive" {
		t.Errorf("reloaded evidence = %q, want %q",
			store2.Snapshot().Tasks[0].Evidence, "pid 12345 not alive")
	}
}

// contains is a tiny test helper — keeps the assertions above
// readable without pulling in strings.Contains.
func contains(haystack, needle string) bool {
	if len(needle) > len(haystack) {
		return false
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

// TestFleetPidPath_DoesNotEscapeWorkdir verifies the pid paths
// stay inside the workdir even when run/task IDs contain
// slashes or `..`. Defense against a malicious task title
// smuggling a path out of the pid directory.
func TestFleetPidPath_DoesNotEscapeWorkdir(t *testing.T) {
	workdir := "/tmp/radiant-test-workdir"
	got := taskPidPath(workdir, "run-1", "../../../etc/passwd")
	if !filepath.IsAbs(got) {
		t.Errorf("path should be absolute, got %q", got)
	}
	pidDir := filepath.Join(workdir, ".radiant-harness", "fleet", "pids")
	// Filename has the malicious component sanitized — but the
	// parent directory must still be the pid directory (NOT
	// /tmp/radiant-test-workdir/.radiant-harness/fleet/etc).
	if filepath.Dir(got) != pidDir {
		t.Errorf("path escaped pid dir: got %q, want parent %q", got, pidDir)
	}
}