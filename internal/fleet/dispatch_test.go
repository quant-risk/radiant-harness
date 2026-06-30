package fleet_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/quant-risk/radiant-harness/v3/internal/fleet"
)

var dispatchTestCounter int64

// uniqueRunID returns a run ID unique across test binary invocations.
// Uses UnixNano as a base so runs started within the same second still diverge
// via the atomic counter.
var dispatchTestNano = time.Now().UnixNano()

func uniqueRunID(prefix string) string {
	n := atomic.AddInt64(&dispatchTestCounter, 1)
	return fmt.Sprintf("%s-%x-%d", prefix, dispatchTestNano&0xffff, n)
}

// cleanBranch removes a git branch created by a dispatch test.
// Called via t.Cleanup so orphan branches don't accumulate between runs.
func cleanBranch(t *testing.T, branchPrefix string) {
	t.Helper()
	t.Cleanup(func() {
		out, err := exec.Command("git", "branch", "--list", "radiant/wt/"+branchPrefix+"*").Output()
		if err != nil || len(out) == 0 {
			return
		}
		branches := []string{}
		for _, b := range []byte(out) {
			_ = b // iterate below
		}
		lines := string(out)
		for _, line := range splitLines(lines) {
			if line != "" {
				branches = append(branches, line)
			}
		}
		for _, b := range branches {
			_ = exec.Command("git", "branch", "-D", b).Run()
		}
	})
}

func splitLines(s string) []string {
	var out []string
	cur := []byte{}
	for _, b := range []byte(s) {
		if b == '\n' {
			line := string(cur)
			for len(line) > 0 && (line[0] == ' ' || line[0] == '*') {
				line = line[1:]
			}
			if line != "" {
				out = append(out, line)
			}
			cur = cur[:0]
		} else {
			cur = append(cur, b)
		}
	}
	return out
}

// helperBinary returns the path to a small shell script that stands in for
// the radiant binary in tests. We can't use the real binary (not built yet in
// unit test context) so we use /bin/true (always exits 0) or a temp script.
func helperBinary(t *testing.T, exitCode int) string {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "fake-radiant")
	content := "#!/bin/sh\n"
	if exitCode != 0 {
		content += "exit 1\n"
	}
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	return script
}

func isGitRepo(t *testing.T) bool {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = "."
	return cmd.Run() == nil
}

// ── DispatchConfig defaults ────────────────────────────────────────────────

func TestDispatchConfigDefaults(t *testing.T) {
	cfg := fleet.DispatchConfig{}
	if cfg.Binary != "" {
		t.Error("Binary should be empty by default (resolved to os.Executable in NewDispatcher)")
	}
	if cfg.Timeout != 0 {
		t.Error("Timeout should be 0 by default (no timeout)")
	}
	if cfg.Stdout != nil {
		t.Error("Stdout should be nil by default (discard)")
	}
}

// ── AgentResult fields ─────────────────────────────────────────────────────

func TestAgentResultZeroValue(t *testing.T) {
	var r fleet.AgentResult
	if r.ExitCode != 0 {
		t.Error("zero AgentResult should have ExitCode 0")
	}
	if r.Err != nil {
		t.Error("zero AgentResult should have nil Err")
	}
}

// ── NewDispatcher ──────────────────────────────────────────────────────────

func TestNewDispatcherResolvesExecutable(t *testing.T) {
	if !isGitRepo(t) {
		t.Skip("not in a git repo")
	}
	store, err := fleet.NewStore(".", uniqueRunID("disp-newdisp"), "test goal")
	if err != nil {
		t.Fatal(err)
	}
	iso, err := fleet.NewIsolator(store, ".")
	if err != nil {
		t.Fatal(err)
	}

	cfg := fleet.DispatchConfig{Binary: "/bin/true"}
	d, err := fleet.NewDispatcher(iso, cfg)
	if err != nil {
		t.Fatalf("NewDispatcher should not error: %v", err)
	}
	if d == nil {
		t.Fatal("expected non-nil Dispatcher")
	}
}

// ── RunAll with no pending tasks ───────────────────────────────────────────

func TestRunAllNoPendingTasks(t *testing.T) {
	if !isGitRepo(t) {
		t.Skip("not in a git repo")
	}
	store, err := fleet.NewStore(".", uniqueRunID("disp-empty"), "goal")
	if err != nil {
		t.Fatal(err)
	}
	iso, err := fleet.NewIsolator(store, ".")
	if err != nil {
		t.Fatal(err)
	}
	d, err := fleet.NewDispatcher(iso, fleet.DispatchConfig{Binary: "/bin/true"})
	if err != nil {
		t.Fatal(err)
	}

	results, err := d.RunAll(context.Background(), nil)
	if err != nil {
		t.Fatalf("RunAll with no tasks should not error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty task list, got %d", len(results))
	}
}

// ── RunAll with one task — successful process ──────────────────────────────

func TestRunAllOneTaskSuccess(t *testing.T) {
	if !isGitRepo(t) {
		t.Skip("not in a git repo")
	}
	runID := uniqueRunID("disp-ok")
	cleanBranch(t, runID)
	store, err := fleet.NewStore(".", runID, "goal")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetTasks([]fleet.Task{{ID: "t1", Title: "implement rate limiting", DoneWhen: "tests pass", Status: fleet.TaskPending}}); err != nil {
		t.Fatal(err)
	}

	iso, err := fleet.NewIsolator(store, ".")
	if err != nil {
		t.Fatal(err)
	}

	bin := helperBinary(t, 0) // exits 0
	d, err := fleet.NewDispatcher(iso, fleet.DispatchConfig{
		Binary:  bin,
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	results, err := d.RunAll(context.Background(), nil)
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ExitCode != 0 {
		t.Errorf("expected exit 0, got %d", results[0].ExitCode)
	}
	if results[0].Err != nil {
		t.Errorf("expected nil Err, got %v", results[0].Err)
	}
	if results[0].TaskID != "t1" {
		t.Errorf("expected task ID t1, got %q", results[0].TaskID)
	}

	// Verify task marked done in store.
	snap := store.Snapshot()
	for _, task := range snap.Tasks {
		if task.ID == "t1" && task.Status != fleet.TaskDone {
			t.Errorf("task t1 should be TaskDone, got %q", task.Status)
		}
	}
}

// ── RunAll with one task — failing process ─────────────────────────────────

func TestRunAllOneTaskFailure(t *testing.T) {
	if !isGitRepo(t) {
		t.Skip("not in a git repo")
	}
	runID := uniqueRunID("disp-fail")
	cleanBranch(t, runID)
	store, err := fleet.NewStore(".", runID, "goal")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetTasks([]fleet.Task{{ID: "t2", Title: "failing task", DoneWhen: "tests pass", Status: fleet.TaskPending}}); err != nil {
		t.Fatal(err)
	}

	iso, err := fleet.NewIsolator(store, ".")
	if err != nil {
		t.Fatal(err)
	}

	bin := helperBinary(t, 1) // exits 1
	d, err := fleet.NewDispatcher(iso, fleet.DispatchConfig{
		Binary:  bin,
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	results, err := d.RunAll(context.Background(), nil)
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ExitCode == 0 {
		t.Error("expected non-zero exit for failing process")
	}

	// Verify task marked failed in store.
	snap := store.Snapshot()
	for _, task := range snap.Tasks {
		if task.ID == "t2" && task.Status != fleet.TaskFailed {
			t.Errorf("task t2 should be TaskFailed, got %q", task.Status)
		}
	}
}

// ── Context cancellation ───────────────────────────────────────────────────

func TestRunAllContextCanceled(t *testing.T) {
	if !isGitRepo(t) {
		t.Skip("not in a git repo")
	}
	runID := uniqueRunID("disp-cancel")
	cleanBranch(t, runID)
	store, err := fleet.NewStore(".", runID, "goal")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetTasks([]fleet.Task{{ID: "t3", Title: "slow task", DoneWhen: "done", Status: fleet.TaskPending}}); err != nil {
		t.Fatal(err)
	}

	iso, err := fleet.NewIsolator(store, ".")
	if err != nil {
		t.Fatal(err)
	}

	// Use a script that sleeps — context will cancel it.
	dir := t.TempDir()
	script := filepath.Join(dir, "slow")
	_ = os.WriteFile(script, []byte("#!/bin/sh\nsleep 10\n"), 0o755)

	d, err := fleet.NewDispatcher(iso, fleet.DispatchConfig{Binary: script})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	results, err := d.RunAll(ctx, nil)
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	// Process killed by context — should still return a result.
	if len(results) != 1 {
		t.Fatalf("expected 1 result even on cancel, got %d", len(results))
	}
	// Process killed, exit code should be non-zero.
	if results[0].ExitCode == 0 {
		t.Error("expected non-zero exit for killed process")
	}
}
