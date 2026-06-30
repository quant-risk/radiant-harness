// pidfile_test.go — coverage for the v3.7.9 pid-tracking
// primitives (task + dispatcher pid files). Mirrors the
// cmd_async_runner.go pid tests but in the fleet package.

package fleet

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestTaskPidPath_Layout(t *testing.T) {
	got := taskPidPath("/tmp/work", "fleet-abc123", "task-1")
	want := filepath.Join("/tmp/work", ".radiant-harness", "fleet", "pids", "agent-fleet-abc123-task-1.pid")
	if got != want {
		t.Fatalf("taskPidPath = %q, want %q", got, want)
	}
}

func TestDispatcherPidPath_Layout(t *testing.T) {
	got := dispatcherPidPath("/tmp/work", "fleet-abc123")
	want := filepath.Join("/tmp/work", ".radiant-harness", "fleet", "pids", "dispatcher-fleet-abc123.pid")
	if got != want {
		t.Fatalf("dispatcherPidPath = %q, want %q", got, want)
	}
}

func TestSanitizePidComponent(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"run-1", "run-1"},
		// "../" becomes "___" — the `..` becomes `__`, the
		// `/` becomes `_`. Then "etc/passwd" → "etc_passwd".
		{"../etc/passwd", "___etc_passwd"},
		{"fleet/abc", "fleet_abc"},
		{"foo\\bar", "foo_bar"},
		{"foo bar", "foo_bar"},
		{"", ""},
	}
	for _, c := range cases {
		got := sanitizePidComponent(c.in)
		if got != c.want {
			t.Errorf("sanitizePidComponent(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestTaskLiveness_NoFile(t *testing.T) {
	dir := t.TempDir()
	alive, pid := taskLiveness(dir, "run-1", "task-1")
	if alive {
		t.Errorf("alive=true with no pid file")
	}
	if pid != 0 {
		t.Errorf("pid=%d, want 0 for missing file", pid)
	}
}

func TestTaskLiveness_Alive(t *testing.T) {
	dir := t.TempDir()
	// Write our own pid — we KNOW it's alive.
	our := os.Getpid()
	if err := writePidFile(taskPidPath(dir, "run-1", "task-1"), our); err != nil {
		t.Fatalf("writePidFile: %v", err)
	}
	alive, pid := taskLiveness(dir, "run-1", "task-1")
	if !alive {
		t.Errorf("alive=false for our own pid")
	}
	if pid != our {
		t.Errorf("pid=%d, want %d", pid, our)
	}
}

func TestTaskLiveness_Crashed(t *testing.T) {
	dir := t.TempDir()
	// Pid 16777215 is above pid_max on Linux (32768) and macOS
	// (99999), so `kill -0` reliably returns "no such process"
	// without depending on host state. The pid file present +
	// pid unkillable = "agent crashed without writing
	// terminal status".
	stale := 16777215
	if err := writePidFile(taskPidPath(dir, "run-1", "task-1"), stale); err != nil {
		t.Fatalf("writePidFile: %v", err)
	}
	alive, pid := taskLiveness(dir, "run-1", "task-1")
	if alive {
		t.Errorf("alive=true for stale pid %d (expected crash signal)", stale)
	}
	if pid != stale {
		t.Errorf("pid=%d, want %d (the file value should survive)", pid, stale)
	}
}

func TestDispatcherLiveness_NoFile(t *testing.T) {
	dir := t.TempDir()
	alive, pid := dispatcherLiveness(dir, "run-1")
	if alive {
		t.Errorf("alive=true with no pid file")
	}
	if pid != 0 {
		t.Errorf("pid=%d, want 0 for missing file", pid)
	}
}

func TestDispatcherLiveness_Alive(t *testing.T) {
	dir := t.TempDir()
	our := os.Getpid()
	if err := WriteDispatcherPid(dir, "run-1", our); err != nil {
		t.Fatalf("WriteDispatcherPid: %v", err)
	}
	alive, pid := dispatcherLiveness(dir, "run-1")
	if !alive {
		t.Errorf("alive=false for our own pid")
	}
	if pid != our {
		t.Errorf("pid=%d, want %d", pid, our)
	}
}

func TestDispatcherPidRoundtrip(t *testing.T) {
	dir := t.TempDir()
	// WriteDispatcherPid + RemoveDispatcherPid should be
	// idempotent on missing files (Remove returns nil).
	if err := RemoveDispatcherPid(dir, "never-existed"); err != nil {
		t.Fatalf("RemoveDispatcherPid on missing file: %v", err)
	}
	// Write + verify.
	if err := WriteDispatcherPid(dir, "run-1", 99999); err != nil {
		t.Fatalf("WriteDispatcherPid: %v", err)
	}
	// File should exist.
	if _, err := os.Stat(dispatcherPidPath(dir, "run-1")); err != nil {
		t.Fatalf("stat: %v", err)
	}
	// Content should be the pid.
	data, err := os.ReadFile(dispatcherPidPath(dir, "run-1"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	got, err := strconv.Atoi(string(data))
	if err != nil {
		t.Fatalf("parse %q: %v", data, err)
	}
	if got != 99999 {
		t.Errorf("pid file content = %d, want 99999", got)
	}
	// Remove + verify.
	if err := RemoveDispatcherPid(dir, "run-1"); err != nil {
		t.Fatalf("RemoveDispatcherPid: %v", err)
	}
	if _, err := os.Stat(dispatcherPidPath(dir, "run-1")); !os.IsNotExist(err) {
		t.Errorf("pid file should be removed, got err=%v", err)
	}
}

func TestPidAlive_ZeroIsDead(t *testing.T) {
	// Pid 0 must always report dead (it's the scheduler,
	// not a user agent).
	if pidAlive(0) {
		t.Errorf("pidAlive(0) = true, want false")
	}
	if pidAlive(-1) {
		t.Errorf("pidAlive(-1) = true, want false")
	}
}

func TestEnsureFleetPidDir_CreatesPath(t *testing.T) {
	dir := t.TempDir()
	if err := ensureFleetPidDir(dir); err != nil {
		t.Fatalf("ensureFleetPidDir: %v", err)
	}
	want := filepath.Join(dir, fleetPidDir)
	info, err := os.Stat(want)
	if err != nil {
		t.Fatalf("stat %s: %v", want, err)
	}
	if !info.IsDir() {
		t.Errorf("%s is not a directory", want)
	}
}