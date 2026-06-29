//go:build !light_only

package loop

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// ── CancelRun PID file round-trip ─────────────────────────────────────────

func TestCancelRun_NoPIDFile_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	err := CancelRun(dir, "nonexistent-run")
	if err == nil {
		t.Fatal("expected error when PID file missing")
	}
}

func TestCancelRun_InvalidPID_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	pidDir := filepath.Join(dir, ".radiant-harness", "pids")
	_ = os.MkdirAll(pidDir, 0o755)
	_ = os.WriteFile(filepath.Join(pidDir, "run-bad.pid"), []byte("not-a-pid"), 0o644)
	err := CancelRun(dir, "run-bad")
	if err == nil {
		t.Fatal("expected error for invalid PID content")
	}
}

func TestWritePID_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	if err := writePID(dir, "my-run"); err != nil {
		t.Fatalf("writePID: %v", err)
	}
	data, err := os.ReadFile(pidPath(dir, "my-run"))
	if err != nil {
		t.Fatalf("read PID file: %v", err)
	}
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		t.Fatalf("invalid PID in file: %q", data)
	}
	if pid != os.Getpid() {
		t.Errorf("expected PID %d, got %d", os.Getpid(), pid)
	}
}

func TestRemovePID_CleansUp(t *testing.T) {
	dir := t.TempDir()
	_ = writePID(dir, "run-x")
	removePID(dir, "run-x")
	if _, err := os.Stat(pidPath(dir, "run-x")); !os.IsNotExist(err) {
		t.Error("expected PID file to be removed")
	}
}

func TestCancelRun_RealProcess_SendsSIGTERM(t *testing.T) {
	// Spawn /bin/sleep so we have a real PID to signal without killing the test.
	proc, err := os.StartProcess("/bin/sleep", []string{"sleep", "60"}, &os.ProcAttr{})
	if err != nil {
		t.Skip("cannot start /bin/sleep:", err)
	}
	defer proc.Wait()

	dir := t.TempDir()
	pidDir := filepath.Join(dir, ".radiant-harness", "pids")
	_ = os.MkdirAll(pidDir, 0o755)
	_ = os.WriteFile(filepath.Join(pidDir, "sleep-run.pid"),
		[]byte(strconv.Itoa(proc.Pid)), 0o644)

	if err := CancelRun(dir, "sleep-run"); err != nil {
		t.Errorf("CancelRun: %v", err)
	}
}
