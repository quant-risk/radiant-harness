package fleet_test

// Sprint 76 — end-to-end pipeline tests.
// These tests exercise the full start→plan→dispatch(mock)→status→summary
// pipeline without spawning real LLM processes.

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/quant-risk/radiant-harness/internal/fleet"
)

// ── E2E: store → coordinator → status ────────────────────────────────────

func TestE2E_StartToStatus(t *testing.T) {
	if !isGitRepo(t) {
		t.Skip("not in a git repo")
	}
	runID := uniqueRunID("e2e")
	cleanBranch(t, runID)

	store, err := fleet.NewStore(".", runID, "build a REST API")
	if err != nil {
		t.Fatal(err)
	}

	tasks := []fleet.Task{
		{ID: "t1", Title: "Research API patterns", DoneWhen: "patterns documented", Status: fleet.TaskPending},
		{ID: "t2", Title: "Implement endpoints", DoneWhen: "endpoints working", Status: fleet.TaskPending},
		{ID: "t3", Title: "Write tests", DoneWhen: "tests green", Status: fleet.TaskPending},
	}
	if err := store.SetTasks(tasks); err != nil {
		t.Fatal(err)
	}

	coord := fleet.NewCoordinator(store, 3)
	status := coord.Status()

	if status.RunID != runID {
		t.Errorf("expected run_id %q, got %q", runID, status.RunID)
	}
	if len(status.Tasks) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(status.Tasks))
	}

	out := fleet.FormatStatus(status)
	if !strings.Contains(out, "pending") {
		t.Errorf("expected 'pending' in status output: %q", out)
	}
}

// ── E2E: complete tasks, verify summary ──────────────────────────────────

func TestE2E_DispatchToSummary(t *testing.T) {
	if !isGitRepo(t) {
		t.Skip("not in a git repo")
	}
	runID := uniqueRunID("e2es")
	cleanBranch(t, runID)

	store, err := fleet.NewStore(".", runID, "migrate DB schema")
	if err != nil {
		t.Fatal(err)
	}
	_ = store.SetTasks([]fleet.Task{
		{ID: "a", Title: "Schema diff", DoneWhen: "diff done", Status: fleet.TaskPending},
		{ID: "b", Title: "Migration script", DoneWhen: "script ready", Status: fleet.TaskPending},
	})

	// Simulate dispatch completing both tasks.
	_ = store.CompleteTask("a", "diff shows 3 added columns", true)
	_ = store.CompleteTask("b", "migration_v2.sql created", true)

	coord := fleet.NewCoordinator(store, 2)
	status := coord.Status()
	summary := fleet.FormatSummary(status)

	if !strings.Contains(summary, "diff shows 3 added columns") {
		t.Errorf("summary missing task-a evidence: %q", summary)
	}
	if !strings.Contains(summary, "migration_v2.sql created") {
		t.Errorf("summary missing task-b evidence: %q", summary)
	}
}

// ── E2E: ResetTask allows re-dispatch ─────────────────────────────────────

func TestE2E_ResetTask_AllowsRedispatch(t *testing.T) {
	if !isGitRepo(t) {
		t.Skip("not in a git repo")
	}
	runID := uniqueRunID("e2er")
	cleanBranch(t, runID)

	store, err := fleet.NewStore(".", runID, "fix flaky tests")
	if err != nil {
		t.Fatal(err)
	}
	_ = store.SetTasks([]fleet.Task{
		{ID: "f1", Title: "Fix test A", DoneWhen: "test green", Status: fleet.TaskPending},
	})
	_ = store.CompleteTask("f1", "still failing", false)

	// Verify it's failed.
	snap := store.Snapshot()
	if snap.Tasks[0].Status != fleet.TaskFailed {
		t.Fatalf("expected TaskFailed, got %v", snap.Tasks[0].Status)
	}

	// Reset.
	if err := store.ResetTask("f1"); err != nil {
		t.Fatalf("ResetTask: %v", err)
	}

	// Should be pending again, no evidence.
	snap2 := store.Snapshot()
	if snap2.Tasks[0].Status != fleet.TaskPending {
		t.Errorf("expected TaskPending after reset, got %v", snap2.Tasks[0].Status)
	}
	if snap2.Tasks[0].Evidence != "" {
		t.Errorf("expected empty Evidence after reset, got %q", snap2.Tasks[0].Evidence)
	}
}

// ── E2E: JSON round-trip of full FleetStatus ──────────────────────────────

func TestE2E_FleetStatus_JSONRoundtrip_Full(t *testing.T) {
	if !isGitRepo(t) {
		t.Skip("not in a git repo")
	}
	runID := uniqueRunID("e2ej")
	cleanBranch(t, runID)

	store, err := fleet.NewStore(".", runID, "comprehensive goal")
	if err != nil {
		t.Fatal(err)
	}
	_ = store.SetTasks([]fleet.Task{
		{ID: "x1", Title: "Task one", DoneWhen: "x done", Status: fleet.TaskPending},
	})
	_ = store.CompleteTask("x1", "x completed", true)

	coord := fleet.NewCoordinator(store, 1)
	status := coord.Status()

	b, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var decoded fleet.FleetStatus
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if decoded.RunID != runID {
		t.Errorf("run_id mismatch: got %q", decoded.RunID)
	}
	if len(decoded.Tasks) != 1 || decoded.Tasks[0].Status != fleet.TaskDone {
		t.Errorf("tasks mismatch after round-trip: %+v", decoded.Tasks)
	}
}

// ── E2E: watch termination condition ─────────────────────────────────────

func TestE2E_Watch_TerminatesWhenAllDone(t *testing.T) {
	if !isGitRepo(t) {
		t.Skip("not in a git repo")
	}
	runID := uniqueRunID("e2ew")
	cleanBranch(t, runID)

	store, err := fleet.NewStore(".", runID, "watch goal")
	if err != nil {
		t.Fatal(err)
	}
	_ = store.SetTasks([]fleet.Task{
		{ID: "w1", Status: fleet.TaskPending, Title: "t", DoneWhen: "done"},
		{ID: "w2", Status: fleet.TaskPending, Title: "t", DoneWhen: "done"},
	})

	isTerminal := func(tasks []fleet.Task) bool {
		for _, t := range tasks {
			if t.Status == fleet.TaskPending || t.Status == fleet.TaskAssigned {
				return false
			}
		}
		return len(tasks) > 0
	}

	coord := fleet.NewCoordinator(store, 2)
	if isTerminal(coord.Status().Tasks) {
		t.Fatal("should not be terminal before any completions")
	}

	_ = store.CompleteTask("w1", "ok", true)
	coord2 := fleet.NewCoordinator(store, 2)
	if isTerminal(coord2.Status().Tasks) {
		t.Fatal("should not be terminal with one pending")
	}

	_ = store.CompleteTask("w2", "ok", true)
	coord3 := fleet.NewCoordinator(store, 2)
	if !isTerminal(coord3.Status().Tasks) {
		t.Error("should be terminal when all done")
	}
}

// ── E2E: plan → dispatch (mock) timeline ─────────────────────────────────

func TestE2E_PlanToDispatch_MockBinary(t *testing.T) {
	if !isGitRepo(t) {
		t.Skip("not in a git repo")
	}
	runID := uniqueRunID("e2ep")
	cleanBranch(t, runID)

	store, err := fleet.NewStore(".", runID, "plan then dispatch")
	if err != nil {
		t.Fatal(err)
	}

	// Simulate planner output.
	planned := []fleet.Task{
		{ID: "task-01", Title: "Research", DoneWhen: "research done", Status: fleet.TaskPending},
		{ID: "task-02", Title: "Implement", DoneWhen: "impl done", Status: fleet.TaskPending},
		{ID: "task-03", Title: "Verify", DoneWhen: "tests pass", Status: fleet.TaskPending},
	}
	if err := store.SetTasks(planned); err != nil {
		t.Fatal(err)
	}

	// Simulate dispatcher completing all tasks.
	for _, task := range planned {
		_, _ = store.ClaimTask("mock-agent", "/tmp/wt-"+task.ID)
		_ = store.CompleteTask(task.ID, "done by mock agent", true)
	}

	coord := fleet.NewCoordinator(store, 3)
	status := coord.Status()

	for _, task := range status.Tasks {
		if task.Status != fleet.TaskDone {
			t.Errorf("task %q should be done, got %v", task.ID, task.Status)
		}
	}

	summary := fleet.FormatSummary(status)
	if !strings.Contains(summary, "done by mock agent") {
		t.Errorf("summary missing evidence: %q", summary)
	}
}

// ── E2E: store persists across load ───────────────────────────────────────

func TestE2E_StorePersistsAcrossLoad(t *testing.T) {
	if !isGitRepo(t) {
		t.Skip("not in a git repo")
	}
	runID := uniqueRunID("e2el")
	cleanBranch(t, runID)

	store, err := fleet.NewStore(".", runID, "persistence test")
	if err != nil {
		t.Fatal(err)
	}
	_ = store.SetTasks([]fleet.Task{
		{ID: "p1", Title: "Task", DoneWhen: "done", Status: fleet.TaskPending},
	})
	_ = store.CompleteTask("p1", "persisted evidence", true)

	// Re-load from disk.
	store2, err := fleet.LoadStore(".", runID)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	snap := store2.Snapshot()
	if len(snap.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(snap.Tasks))
	}
	if snap.Tasks[0].Status != fleet.TaskDone {
		t.Errorf("expected TaskDone after reload, got %v", snap.Tasks[0].Status)
	}
	if snap.Tasks[0].Evidence != "persisted evidence" {
		t.Errorf("evidence not persisted: %q", snap.Tasks[0].Evidence)
	}
}

// ── E2E: timing — UpdatedAt advances on each operation ────────────────────

func TestE2E_UpdatedAt_AdvancesOnMutation(t *testing.T) {
	if !isGitRepo(t) {
		t.Skip("not in a git repo")
	}
	runID := uniqueRunID("e2et")
	cleanBranch(t, runID)

	store, err := fleet.NewStore(".", runID, "timing test")
	if err != nil {
		t.Fatal(err)
	}
	snap1 := store.Snapshot()
	time.Sleep(2 * time.Millisecond)

	_ = store.SetTasks([]fleet.Task{
		{ID: "q1", Title: "t", DoneWhen: "done", Status: fleet.TaskPending},
	})
	snap2 := store.Snapshot()
	if !snap2.UpdatedAt.After(snap1.UpdatedAt) {
		t.Errorf("UpdatedAt should advance: %v → %v", snap1.UpdatedAt, snap2.UpdatedAt)
	}
}
