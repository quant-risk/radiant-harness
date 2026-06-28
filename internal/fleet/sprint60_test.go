package fleet_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/quant-risk/radiant-harness/internal/fleet"
)

// ── extraArgs forwarding ───────────────────────────────────────────────────
// These tests verify that Dispatcher passes extraArgs verbatim to each
// spawned subprocess. We use a fake binary that prints its argv to stdout
// and captures that output.

func captureArgsBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "print-args")
	content := "#!/bin/sh\necho \"$@\"\n"
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	return script
}

func TestDispatchExtraArgs_ModelForwarded(t *testing.T) {
	if !isGitRepo(t) {
		t.Skip("not in a git repo")
	}
	runID := uniqueRunID("d60-model")
	cleanBranch(t, runID)

	store, err := fleet.NewStore(".", runID, "goal")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetTasks([]fleet.Task{{
		ID: "t1", Title: "implement X", DoneWhen: "tests pass", Status: fleet.TaskPending,
	}}); err != nil {
		t.Fatal(err)
	}

	iso, err := fleet.NewIsolator(store, ".")
	if err != nil {
		t.Fatal(err)
	}

	var buf captureWriter
	bin := captureArgsBinary(t)
	d, err := fleet.NewDispatcher(iso, fleet.DispatchConfig{
		Binary:  bin,
		Timeout: 5 * time.Second,
		Stdout:  &buf,
	})
	if err != nil {
		t.Fatal(err)
	}

	extraArgs := []string{"--model", "claude-sonnet-4-6", "--auto-route"}
	results, err := d.RunAll(context.Background(), extraArgs)
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	// The script echoes its args; check they contain our extraArgs.
	out := buf.String()
	for _, arg := range extraArgs {
		if !contains(out, arg) {
			t.Errorf("expected arg %q in subprocess output %q", arg, out)
		}
	}
	// Also check the task's DoneWhen was passed as the goal arg.
	if !contains(out, "tests pass") {
		t.Errorf("expected DoneWhen %q in subprocess output %q", "tests pass", out)
	}
}

func TestDispatchExtraArgs_NoExtraArgs(t *testing.T) {
	if !isGitRepo(t) {
		t.Skip("not in a git repo")
	}
	runID := uniqueRunID("d60-noargs")
	cleanBranch(t, runID)

	store, err := fleet.NewStore(".", runID, "goal")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetTasks([]fleet.Task{{
		ID: "t2", Title: "task", DoneWhen: "done", Status: fleet.TaskPending,
	}}); err != nil {
		t.Fatal(err)
	}

	iso, err := fleet.NewIsolator(store, ".")
	if err != nil {
		t.Fatal(err)
	}

	bin := captureArgsBinary(t)
	d, err := fleet.NewDispatcher(iso, fleet.DispatchConfig{Binary: bin, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}

	// No extraArgs — should not error.
	results, err := d.RunAll(context.Background(), nil)
	if err != nil {
		t.Fatalf("RunAll with nil extraArgs: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestDispatchExtraArgs_AutoRouteFlag(t *testing.T) {
	if !isGitRepo(t) {
		t.Skip("not in a git repo")
	}
	runID := uniqueRunID("d60-ar")
	cleanBranch(t, runID)

	store, err := fleet.NewStore(".", runID, "goal")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetTasks([]fleet.Task{{
		ID: "t3", Title: "task", DoneWhen: "all tests green", Status: fleet.TaskPending,
	}}); err != nil {
		t.Fatal(err)
	}

	iso, err := fleet.NewIsolator(store, ".")
	if err != nil {
		t.Fatal(err)
	}

	var buf captureWriter
	bin := captureArgsBinary(t)
	d, err := fleet.NewDispatcher(iso, fleet.DispatchConfig{
		Binary:  bin,
		Timeout: 5 * time.Second,
		Stdout:  &buf,
	})
	if err != nil {
		t.Fatal(err)
	}

	results, err := d.RunAll(context.Background(), []string{"--auto-route"})
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	out := buf.String()
	if !contains(out, "--auto-route") {
		t.Errorf("expected --auto-route forwarded, got %q", out)
	}
}

// ── multi-task dispatch ────────────────────────────────────────────────────

func TestDispatchMultiTask_ExtraArgsOnAll(t *testing.T) {
	if !isGitRepo(t) {
		t.Skip("not in a git repo")
	}
	runID := uniqueRunID("d60-multi")
	cleanBranch(t, runID)

	store, err := fleet.NewStore(".", runID, "goal")
	if err != nil {
		t.Fatal(err)
	}
	tasks := []fleet.Task{
		{ID: "m1", Title: "task 1", DoneWhen: "done-1", Status: fleet.TaskPending},
		{ID: "m2", Title: "task 2", DoneWhen: "done-2", Status: fleet.TaskPending},
	}
	if err := store.SetTasks(tasks); err != nil {
		t.Fatal(err)
	}

	iso, err := fleet.NewIsolator(store, ".")
	if err != nil {
		t.Fatal(err)
	}

	var buf captureWriter
	bin := captureArgsBinary(t)
	d, err := fleet.NewDispatcher(iso, fleet.DispatchConfig{
		Binary:  bin,
		Timeout: 5 * time.Second,
		Stdout:  &buf,
	})
	if err != nil {
		t.Fatal(err)
	}

	results, err := d.RunAll(context.Background(), []string{"--model", "claude-opus-4-8"})
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	out := buf.String()
	// Both processes should have received --model flag.
	count := countOccurrences(out, "--model")
	if count < 2 {
		t.Errorf("expected --model to appear at least twice (once per agent), got %d times in %q", count, out)
	}
}

// ── helpers ────────────────────────────────────────────────────────────────

type captureWriter struct {
	mu   sync.Mutex
	data []byte
}

func (w *captureWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.data = append(w.data, p...)
	return len(p), nil
}

func (w *captureWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return string(w.data)
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexString(s, sub) >= 0)
}

func indexString(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func countOccurrences(s, sub string) int {
	count := 0
	for i := 0; i <= len(s)-len(sub); {
		if s[i:i+len(sub)] == sub {
			count++
			i += len(sub)
		} else {
			i++
		}
	}
	return count
}

var _ = fmt.Sprintf // keep fmt imported
