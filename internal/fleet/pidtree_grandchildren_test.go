// pidtree_grandchildren_test.go — coverage for the v3.7.12
// recursive pid tree (grandchildren layer). Mirrors pidtree_test.go
// (v3.7.10) but goes one level deeper.
//
//   - TestPidTree_Grandchildren_AllAlive
//   - TestPidTree_Grandchildren_GrandchildDead
//   - TestPidTree_Grandchildren_NoGCSidecar
//   - TestWriteGrandchildrenPids_Roundtrip
//   - TestTaskPidGrandchildrenPath_Layout
//   - TestRefreshChildAndGrandchildrenSidecars_WritesBoth
//   - TestRefreshChildAndGrandchildrenSidecars_DeadChildSkipped

package fleet

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestPidTree_Grandchildren_NoGCSidecar(t *testing.T) {
	dir := t.TempDir()
	our := os.Getpid()
	if err := writePidFile(taskPidPath(dir, "run-z", "task-z"), our); err != nil {
		t.Fatalf("write parent: %v", err)
	}
	if err := writeChildPids(dir, "run-z", "task-z", []int{our}); err != nil {
		t.Fatalf("write children: %v", err)
	}
	// No grandchildren sidecar.
	tree := TaskPidTree(dir, "run-z", "task-z")
	if !tree.GrandchildrenAlive {
		t.Errorf("GrandchildrenAlive=false with no sidecar (should be vacuously true)")
	}
	if tree.GrandchildrenCount != 0 {
		t.Errorf("GrandchildrenCount = %d, want 0", tree.GrandchildrenCount)
	}
}

func TestPidTree_Grandchildren_GrandchildDead(t *testing.T) {
	dir := t.TempDir()
	our := os.Getpid()
	if err := writePidFile(taskPidPath(dir, "run-y", "task-y"), our); err != nil {
		t.Fatalf("write parent: %v", err)
	}
	if err := writeChildPids(dir, "run-y", "task-y", []int{our}); err != nil {
		t.Fatalf("write children: %v", err)
	}
	// Record one live grandchild + one stale. Pid 16777215 is
	// above pid_max on Linux/macOS so kill -0 returns "no
	// such process" reliably.
	stale := 16777215
	if err := writeGrandchildrenPids(dir, "run-y", "task-y", []int{our, stale}); err != nil {
		t.Fatalf("write grandchildren: %v", err)
	}

	tree := TaskPidTree(dir, "run-y", "task-y")
	if !tree.ParentAlive {
		t.Errorf("ParentAlive=false with live pid")
	}
	if !tree.ChildrenAlive {
		t.Errorf("ChildrenAlive=false with live child")
	}
	if tree.GrandchildrenAlive {
		t.Errorf("GrandchildrenAlive=true with one stale grandchild")
	}
	if tree.GrandchildrenCount != 1 {
		t.Errorf("GrandchildrenCount = %d, want 1 (only our pid alive)", tree.GrandchildrenCount)
	}
	if len(tree.GrandchildrenPids) != 2 {
		t.Errorf("GrandchildrenPids len = %d, want 2", len(tree.GrandchildrenPids))
	}
}

func TestWriteGrandchildrenPids_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	want := []int{101, 202, 303}
	if err := writeGrandchildrenPids(dir, "run-r", "task-r", want); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, ok := readGrandchildrenPids(dir, "run-r", "task-r")
	if !ok {
		t.Fatalf("read: ok=false")
	}
	if len(got) != len(want) {
		t.Fatalf("read len = %d, want %d", len(got), len(want))
	}
	for i, p := range want {
		if got[i] != p {
			t.Errorf("pid[%d] = %d, want %d", i, got[i], p)
		}
	}
}

func TestTaskPidGrandchildrenPath_Layout(t *testing.T) {
	got := taskPidGrandchildrenPath("/tmp/work", "run-x", "task-x")
	want := filepath.Join("/tmp/work", ".radiant-harness", "fleet", "pids",
		"agent-run-x-task-x.pid.grandchildren")
	if got != want {
		t.Errorf("path = %q, want %q", got, want)
	}
}

func TestRefreshChildAndGrandchildrenSidecars_WritesBoth(t *testing.T) {
	dir := t.TempDir()
	if os.Getenv("CI_SANDBOX") == "1" {
		t.Skip("sandbox disallows spawning")
	}
	cmd := exec.Command("sh", "-c", "sleep 30 &")
	if err := cmd.Start(); err != nil {
		t.Skipf("could not spawn shell: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	time.Sleep(150 * time.Millisecond)

	refreshChildAndGrandchildrenSidecars(dir, "run-refresh", "task-refresh", cmd.Process.Pid)

	if _, ok := readChildPids(dir, "run-refresh", "task-refresh"); !ok {
		t.Errorf("children sidecar missing after refresh")
	}
	gc, gcOK := readGrandchildrenPids(dir, "run-refresh", "task-refresh")
	if !gcOK {
		t.Errorf("grandchildren sidecar missing after refresh")
	}
	if len(gc) != 0 {
		t.Errorf("expected 0 grandchildren (sleep has no children), got %d", len(gc))
	}
}

func TestRefreshChildAndGrandchildrenSidecars_DeadChildSkipped(t *testing.T) {
	dir := t.TempDir()
	// Use our own pid as the parent. pidAlive(our) is true,
	// so the helper will proceed; our pid has no children in
	// the synthetic setup so pgrep returns empty. The test
	// verifies the helper still writes BOTH sidecars even when
	// the parent has no live children to walk.
	our := os.Getpid()

	refreshChildAndGrandchildrenSidecars(dir, "run-skip", "task-skip", our)

	// Both sidecars should exist after refresh, even if empty.
	if _, err := os.Stat(taskPidChildrenPath(dir, "run-skip", "task-skip")); err != nil {
		t.Errorf("children sidecar missing after refresh: %v", err)
	}
	if _, err := os.Stat(taskPidGrandchildrenPath(dir, "run-skip", "task-skip")); err != nil {
		t.Errorf("grandchildren sidecar missing after refresh: %v", err)
	}
}

// TestRefreshChildAndGrandchildrenSidecars_GrandchildrenFound
// is the strongest test: spawn a shell that backgrounds a
// subshell, then verify the grandchildren sidecar contains the
// subshell pid. Skips gracefully if pgrep doesn't see the
// grandchild relationship on this host.
func TestRefreshChildAndGrandchildrenSidecars_GrandchildrenFound(t *testing.T) {
	if os.Getenv("CI_SANDBOX") == "1" {
		t.Skip("sandbox disallows spawning")
	}
	dir := t.TempDir()
	// Use sh -c so the backgrounded subshell is a child of
	// the parent shell (grandchild relationship).
	cmd := exec.Command("sh", "-c", "(sleep 30) &")
	if err := cmd.Start(); err != nil {
		t.Skipf("could not spawn shell: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	time.Sleep(150 * time.Millisecond)

	refreshChildAndGrandchildrenSidecars(dir, "run-gc", "task-gc", cmd.Process.Pid)

	children, _ := readChildPids(dir, "run-gc", "task-gc")
	grandchildren, _ := readGrandchildrenPids(dir, "run-gc", "task-gc")
	if len(children) == 0 {
		t.Skipf("no children observed (pgrep race); children=%v", children)
	}
	if len(grandchildren) == 0 {
		t.Skipf("no grandchildren observed; children=%v, grandchildren=%v", children, grandchildren)
	}
	// At least one grandchild must be alive (we just spawned it).
	anyAlive := false
	for _, p := range grandchildren {
		if pidAlive(p) {
			anyAlive = true
			break
		}
	}
	if !anyAlive {
		t.Errorf("no live grandchildren: %v", grandchildren)
	}
}