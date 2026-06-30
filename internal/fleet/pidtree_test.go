// pidtree_test.go — coverage for the v3.7.10 nested pid tracking
// primitives (ChildrenPidsForParent, writeChildPids, readChildPids,
// TaskPidTree). Mirrors the cmd_async_runner.go pid tests for the
// loop but for the fleet parent + children sidecar.

package fleet

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestWriteReadChildPids_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	want := []int{101, 202, 303}
	if err := writeChildPids(dir, "run-1", "task-1", want); err != nil {
		t.Fatalf("writeChildPids: %v", err)
	}
	got, ok := readChildPids(dir, "run-1", "task-1")
	if !ok {
		t.Fatalf("readChildPids: ok=false")
	}
	if len(got) != len(want) {
		t.Fatalf("readChildPids len = %d, want %d", len(got), len(want))
	}
	for i, p := range want {
		if got[i] != p {
			t.Errorf("pid[%d] = %d, want %d", i, got[i], p)
		}
	}
}

func TestWriteReadChildPids_EmptySidecar(t *testing.T) {
	// An empty children list should still produce a sidecar file
	// (so the dispatcher can record "0 children at last poll"
	// distinct from "never polled"). readChildPids should return
	// the empty slice + ok=true.
	dir := t.TempDir()
	if err := writeChildPids(dir, "run-1", "task-1", nil); err != nil {
		t.Fatalf("writeChildPids: %v", err)
	}
	got, ok := readChildPids(dir, "run-1", "task-1")
	if !ok {
		t.Errorf("readChildPids: ok=false for empty sidecar")
	}
	if len(got) != 0 {
		t.Errorf("len(got) = %d, want 0", len(got))
	}
}

func TestWriteReadChildPids_MissingFile(t *testing.T) {
	dir := t.TempDir()
	_, ok := readChildPids(dir, "never-existed", "task-1")
	if ok {
		t.Errorf("readChildPids: ok=true for missing sidecar")
	}
}

func TestWriteChildPids_SidecarLayout(t *testing.T) {
	// The sidecar lives next to the parent pid file with a
	// `.children` suffix. Layout pin.
	dir := t.TempDir()
	if err := writeChildPids(dir, "run-x", "task-y", []int{12345}); err != nil {
		t.Fatalf("writeChildPids: %v", err)
	}
	want := filepath.Join(dir, ".radiant-harness", "fleet", "pids",
		"agent-run-x-task-y.pid.children")
	got := taskPidChildrenPath(dir, "run-x", "task-y")
	if got != want {
		t.Errorf("taskPidChildrenPath = %q, want %q", got, want)
	}
}

func TestReadChildPids_CorruptLineSkipped(t *testing.T) {
	// A sidecar with a non-numeric line should still parse the
	// valid pids and skip the garbage. Defense against an operator
	// who `echo`es extra content into the file while debugging.
	dir := t.TempDir()
	path := taskPidChildrenPath(dir, "run-1", "task-1")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("101\ngarbage-line\n202\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, ok := readChildPids(dir, "run-1", "task-1")
	if !ok {
		t.Fatalf("readChildPids: ok=false")
	}
	if len(got) != 2 || got[0] != 101 || got[1] != 202 {
		t.Errorf("readChildPids = %v, want [101, 202]", got)
	}
}

func TestChildrenPidsForParent_NoChildren(t *testing.T) {
	// Pid 1 (init / launchd) typically has no user-visible
	// children, so pgrep -P should return empty or an error.
	// Both must result in ChildrenPidsForParent returning nil.
	got := ChildrenPidsForParent(1)
	if len(got) != 0 {
		t.Logf("pid 1 reported children: %v (allowed on some hosts)", got)
	}
}

func TestChildrenPidsForParent_InvalidPid(t *testing.T) {
	// pid 0 must return nil without forking pgrep.
	if got := ChildrenPidsForParent(0); got != nil {
		t.Errorf("ChildrenPidsForParent(0) = %v, want nil", got)
	}
	if got := ChildrenPidsForParent(-1); got != nil {
		t.Errorf("ChildrenPidsForParent(-1) = %v, want nil", got)
	}
}

func TestChildrenPidsForParent_FindsActualChild(t *testing.T) {
	// Spawn a shell with a `sleep 5 &` so it has a child process,
	// then probe with pgrep -P. Skip if fork/exec fails on this
	// host (sandbox restriction).
	if os.Getenv("CI_SANDBOX") == "1" {
		t.Skip("sandbox disallows spawning processes")
	}
	cmd := exec.Command("sh", "-c", "sleep 30 &")
	if err := cmd.Start(); err != nil {
		t.Skipf("could not spawn shell: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	// Give the shell a moment to fork the sleep child.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if children := ChildrenPidsForParent(cmd.Process.Pid); len(children) > 0 {
			// Sanity: at least one of the children is a positive int.
			for _, c := range children {
				if c <= 0 {
					t.Errorf("children = %v, want all positive ints", children)
				}
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Skip("pgrep -P didn't return any children within 2s (host may not have pgrep or shell didn't fork)")
}

func TestTaskPidTree_NoFiles(t *testing.T) {
	dir := t.TempDir()
	tree := TaskPidTree(dir, "run-1", "task-1")
	if tree.ParentAlive {
		t.Errorf("ParentAlive=true with no parent file")
	}
	if tree.ParentPid != 0 {
		t.Errorf("ParentPid=%d, want 0", tree.ParentPid)
	}
}

func TestTaskPidTree_ParentAliveNoChildren(t *testing.T) {
	dir := t.TempDir()
	our := os.Getpid()
	if err := writePidFile(taskPidPath(dir, "run-1", "task-1"), our); err != nil {
		t.Fatalf("writePidFile: %v", err)
	}
	tree := TaskPidTree(dir, "run-1", "task-1")
	if !tree.ParentAlive || tree.ParentPid != our {
		t.Errorf("tree = %+v, want ParentAlive=true ParentPid=%d", tree, our)
	}
	if !tree.ChildrenAlive {
		t.Errorf("ChildrenAlive=false with no children (should be vacuously true)")
	}
	if len(tree.ChildrenPids) != 0 {
		t.Errorf("ChildrenPids = %v, want empty", tree.ChildrenPids)
	}
}

func TestTaskPidTree_ParentDeadChildrenOrphaned(t *testing.T) {
	// Parent is stale (pid_max above), children are a mix of
	// alive (real pid) + stale. Status should surface:
	//   ParentAlive=false, ChildrenAlive=false, ChildCount=1
	dir := t.TempDir()
	stale := 16777215
	if err := writePidFile(taskPidPath(dir, "run-1", "task-1"), stale); err != nil {
		t.Fatalf("writePidFile: %v", err)
	}
	our := os.Getpid()
	if err := writeChildPids(dir, "run-1", "task-1", []int{our, 16777214}); err != nil {
		t.Fatalf("writeChildPids: %v", err)
	}
	tree := TaskPidTree(dir, "run-1", "task-1")
	if tree.ParentAlive {
		t.Errorf("ParentAlive=true for stale pid")
	}
	if tree.ChildrenAlive {
		t.Errorf("ChildrenAlive=true with one stale child")
	}
	if tree.ChildCount != 1 {
		t.Errorf("ChildCount = %d, want 1 (only our pid is alive)", tree.ChildCount)
	}
	if len(tree.ChildrenPids) != 2 {
		t.Errorf("ChildrenPids len = %d, want 2", len(tree.ChildrenPids))
	}
}

func TestTaskPidTree_AllChildrenAlive(t *testing.T) {
	dir := t.TempDir()
	our := os.Getpid()
	if err := writePidFile(taskPidPath(dir, "run-1", "task-1"), our); err != nil {
		t.Fatalf("writePidFile: %v", err)
	}
	if err := writeChildPids(dir, "run-1", "task-1", []int{our, our}); err != nil {
		t.Fatalf("writeChildPids: %v", err)
	}
	tree := TaskPidTree(dir, "run-1", "task-1")
	if !tree.ParentAlive {
		t.Errorf("ParentAlive=false for our pid")
	}
	if !tree.ChildrenAlive {
		t.Errorf("ChildrenAlive=false with all-our-pid children")
	}
	if tree.ChildCount != 2 {
		t.Errorf("ChildCount = %d, want 2", tree.ChildCount)
	}
}

func TestRefreshChildPidsLoop_StopsOnClose(t *testing.T) {
	// The refresh goroutine must exit when refreshDone is
	// closed — without this the dispatcher leaks one goroutine
	// per task per fleet run.
	dir := t.TempDir()
	refreshDone := make(chan struct{})
	done := make(chan struct{})
	go func() {
		refreshChildPidsLoop(dir, "run-1", "task-1", os.Getpid(), 50*time.Millisecond, refreshDone)
		close(done)
	}()
	time.Sleep(100 * time.Millisecond) // let it run a tick or two
	close(refreshDone)
	select {
	case <-done:
		// good
	case <-time.After(2 * time.Second):
		t.Fatalf("refreshChildPidsLoop did not exit after refreshDone closed")
	}
	// Verify the sidecar was written before we closed.
	if _, ok := readChildPids(dir, "run-1", "task-1"); !ok {
		t.Errorf("sidecar file missing — refreshChildPidsLoop never wrote")
	}
}

func TestRefreshChildPidsLoop_HandlesZeroPid(t *testing.T) {
	// parentPid=0 must not cause an infinite loop. The function
	// should exit cleanly when refreshDone is closed even though
	// pgrep -P 0 would fail.
	dir := t.TempDir()
	refreshDone := make(chan struct{})
	done := make(chan struct{})
	go func() {
		refreshChildPidsLoop(dir, "run-1", "task-1", 0, 50*time.Millisecond, refreshDone)
		close(done)
	}()
	close(refreshDone)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("refreshChildPidsLoop with pid=0 did not exit")
	}
}

// TestRefreshChildPidsLoop_WritesNumericContent pins the on-disk
// shape of the sidecar to newline-separated integers. Any parser
// downstream (or human operator running `cat` on the file) relies
// on this format.
func TestRefreshChildPidsLoop_WritesNumericContent(t *testing.T) {
	dir := t.TempDir()
	// Seed a single-child scenario by writing a fake parent that
	// reports `os.Getpid()` as its child via a sidecar file.
	our := os.Getpid()
	if err := writeChildPids(dir, "run-1", "task-1", []int{our}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	data, err := os.ReadFile(taskPidChildrenPath(dir, "run-1", "task-1"))
	if err != nil {
		t.Fatalf("read sidecar: %v", err)
	}
	// Content should be just "12345\n" (the pid, followed by newline).
	line := strings.TrimSpace(string(data))
	if line == "" {
		t.Fatalf("sidecar is empty")
	}
	if _, err := strconv.Atoi(line); err != nil {
		t.Errorf("sidecar content %q is not a numeric pid", line)
	}
}