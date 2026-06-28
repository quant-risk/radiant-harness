package fleet_test

import (
	"strings"
	"testing"

	"github.com/quant-risk/radiant-harness/internal/fleet"
)

// ── FormatStatus terminal-state detection (used by fleet watch) ───────────
// We test the condition that watch uses: no pending/assigned tasks → done.

func allTerminal(tasks []fleet.Task) bool {
	for _, t := range tasks {
		if t.Status == fleet.TaskPending || t.Status == fleet.TaskAssigned {
			return false
		}
	}
	return len(tasks) > 0
}

func TestWatch_AllDone_IsTerminal(t *testing.T) {
	tasks := []fleet.Task{
		{ID: "t1", Status: fleet.TaskDone},
		{ID: "t2", Status: fleet.TaskDone},
	}
	if !allTerminal(tasks) {
		t.Error("expected terminal when all tasks done")
	}
}

func TestWatch_AllFailed_IsTerminal(t *testing.T) {
	tasks := []fleet.Task{
		{ID: "t1", Status: fleet.TaskFailed},
	}
	if !allTerminal(tasks) {
		t.Error("expected terminal when all tasks failed")
	}
}

func TestWatch_MixedDoneAndFailed_IsTerminal(t *testing.T) {
	tasks := []fleet.Task{
		{ID: "t1", Status: fleet.TaskDone},
		{ID: "t2", Status: fleet.TaskFailed},
	}
	if !allTerminal(tasks) {
		t.Error("expected terminal when done+failed")
	}
}

func TestWatch_OnePending_NotTerminal(t *testing.T) {
	tasks := []fleet.Task{
		{ID: "t1", Status: fleet.TaskDone},
		{ID: "t2", Status: fleet.TaskPending},
	}
	if allTerminal(tasks) {
		t.Error("should not be terminal while one task is pending")
	}
}

func TestWatch_OneAssigned_NotTerminal(t *testing.T) {
	tasks := []fleet.Task{
		{ID: "t1", Status: fleet.TaskDone},
		{ID: "t2", Status: fleet.TaskAssigned},
	}
	if allTerminal(tasks) {
		t.Error("should not be terminal while one task is assigned")
	}
}

func TestWatch_EmptyTasks_NotTerminal(t *testing.T) {
	if allTerminal(nil) {
		t.Error("empty task list should not be terminal (nothing finished)")
	}
}

// ── FormatStatus used by watch shows live state ────────────────────────────

func TestWatch_FormatStatus_UpdatesOnNewState(t *testing.T) {
	if !isGitRepo(t) {
		t.Skip("not in a git repo")
	}
	runID := uniqueRunID("w66")
	cleanBranch(t, runID)

	store, err := fleet.NewStore(".", runID, "goal")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetTasks([]fleet.Task{
		{ID: "t1", Title: "task one", DoneWhen: "done", Status: fleet.TaskPending},
	}); err != nil {
		t.Fatal(err)
	}

	coord := fleet.NewCoordinator(store, 1)
	out1 := fleet.FormatStatus(coord.Status())
	if !strings.Contains(out1, "pending") {
		t.Errorf("expected pending in initial status: %q", out1)
	}

	// Simulate task completing.
	if err := store.CompleteTask("t1", "all tests pass", true); err != nil {
		t.Fatalf("CompleteTask: %v", err)
	}

	coord2 := fleet.NewCoordinator(store, 1)
	out2 := fleet.FormatStatus(coord2.Status())
	if !strings.Contains(out2, "done") {
		t.Errorf("expected done in updated status: %q", out2)
	}
	if !strings.Contains(out2, "all tests pass") {
		t.Errorf("expected evidence in updated status: %q", out2)
	}
}
