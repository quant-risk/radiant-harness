package fleet_test

import (
	"strings"
	"testing"

	"github.com/quant-risk/radiant-harness/v3/internal/fleet"
)

// ── FormatStatus improvements ──────────────────────────────────────────────

func newStatusWithTasks(t *testing.T, tasks []fleet.Task) fleet.FleetStatus {
	t.Helper()
	runID := uniqueRunID("s62")
	cleanBranch(t, runID)
	store, err := fleet.NewStore(".", runID, "build search feature")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetTasks(tasks); err != nil {
		t.Fatal(err)
	}
	coord := fleet.NewCoordinator(store, 2)
	return coord.Status()
}

func TestFormatStatus_ShowsTaskCounts(t *testing.T) {
	if !isGitRepo(t) {
		t.Skip("not in a git repo")
	}
	tasks := []fleet.Task{
		{ID: "t1", Title: "Research", DoneWhen: "done", Status: fleet.TaskPending},
		{ID: "t2", Title: "Implement", DoneWhen: "done", Status: fleet.TaskDone},
		{ID: "t3", Title: "Verify", DoneWhen: "done", Status: fleet.TaskFailed},
	}
	out := fleet.FormatStatus(newStatusWithTasks(t, tasks))

	if !strings.Contains(out, "3 total") {
		t.Errorf("expected '3 total' in output:\n%s", out)
	}
	if !strings.Contains(out, "1 pending") {
		t.Errorf("expected '1 pending' in output:\n%s", out)
	}
	if !strings.Contains(out, "1 done") {
		t.Errorf("expected '1 done' in output:\n%s", out)
	}
	if !strings.Contains(out, "1 failed") {
		t.Errorf("expected '1 failed' in output:\n%s", out)
	}
}

func TestFormatStatus_NoTasks_ShowsHint(t *testing.T) {
	if !isGitRepo(t) {
		t.Skip("not in a git repo")
	}
	out := fleet.FormatStatus(newStatusWithTasks(t, nil))
	if !strings.Contains(out, "fleet plan") {
		t.Errorf("expected 'fleet plan' hint when no tasks:\n%s", out)
	}
}

func TestFormatStatus_DoneTask_ShowsEvidencePreview(t *testing.T) {
	if !isGitRepo(t) {
		t.Skip("not in a git repo")
	}
	tasks := []fleet.Task{
		{ID: "t1", Title: "Implement", DoneWhen: "done", Status: fleet.TaskDone,
			Evidence: "All 42 tests pass and coverage is 88%"},
	}
	out := fleet.FormatStatus(newStatusWithTasks(t, tasks))
	if !strings.Contains(out, "All 42 tests pass") {
		t.Errorf("expected evidence preview in output:\n%s", out)
	}
}

func TestFormatStatus_AssignedTask_ShowsWorktree(t *testing.T) {
	if !isGitRepo(t) {
		t.Skip("not in a git repo")
	}
	tasks := []fleet.Task{
		{ID: "t1", Title: "Implement", DoneWhen: "done", Status: fleet.TaskAssigned,
			AgentID: "agent-01", WorktreeDir: "/tmp/wt-1"},
	}
	out := fleet.FormatStatus(newStatusWithTasks(t, tasks))
	if !strings.Contains(out, "/tmp/wt-1") {
		t.Errorf("expected worktree dir in output:\n%s", out)
	}
}

// ── FormatSummary ──────────────────────────────────────────────────────────

func TestFormatSummary_NoDoneTasks(t *testing.T) {
	if !isGitRepo(t) {
		t.Skip("not in a git repo")
	}
	tasks := []fleet.Task{
		{ID: "t1", Title: "Pending task", DoneWhen: "done", Status: fleet.TaskPending},
	}
	out := fleet.FormatSummary(newStatusWithTasks(t, tasks))
	if !strings.Contains(out, "No completed tasks") {
		t.Errorf("expected 'No completed tasks' when none done:\n%s", out)
	}
}

func TestFormatSummary_ShowsCompletedCount(t *testing.T) {
	if !isGitRepo(t) {
		t.Skip("not in a git repo")
	}
	tasks := []fleet.Task{
		{ID: "t1", Title: "Task 1", DoneWhen: "done", Status: fleet.TaskDone, Evidence: "evidence A"},
		{ID: "t2", Title: "Task 2", DoneWhen: "done", Status: fleet.TaskDone, Evidence: "evidence B"},
		{ID: "t3", Title: "Task 3", DoneWhen: "done", Status: fleet.TaskPending},
	}
	out := fleet.FormatSummary(newStatusWithTasks(t, tasks))
	if !strings.Contains(out, "2/3") {
		t.Errorf("expected '2/3' completion count:\n%s", out)
	}
}

func TestFormatSummary_ShowsEvidence(t *testing.T) {
	if !isGitRepo(t) {
		t.Skip("not in a git repo")
	}
	tasks := []fleet.Task{
		{ID: "t1", Title: "Build API", DoneWhen: "done", Status: fleet.TaskDone,
			Evidence: "REST API implemented with 95% coverage"},
	}
	out := fleet.FormatSummary(newStatusWithTasks(t, tasks))
	if !strings.Contains(out, "REST API implemented") {
		t.Errorf("expected evidence in summary:\n%s", out)
	}
}

func TestFormatSummary_ShowsFailedTasks(t *testing.T) {
	if !isGitRepo(t) {
		t.Skip("not in a git repo")
	}
	tasks := []fleet.Task{
		{ID: "t1", Title: "Done task", DoneWhen: "done", Status: fleet.TaskDone, Evidence: "ok"},
		{ID: "t2", Title: "Failed task", DoneWhen: "done", Status: fleet.TaskFailed},
	}
	out := fleet.FormatSummary(newStatusWithTasks(t, tasks))
	if !strings.Contains(out, "failed") {
		t.Errorf("expected failed task in summary:\n%s", out)
	}
	if !strings.Contains(out, "Failed task") {
		t.Errorf("expected failed task title in summary:\n%s", out)
	}
}

func TestFormatSummary_ShowsGoal(t *testing.T) {
	if !isGitRepo(t) {
		t.Skip("not in a git repo")
	}
	out := fleet.FormatSummary(newStatusWithTasks(t, nil))
	if !strings.Contains(out, "build search feature") {
		t.Errorf("expected goal in summary:\n%s", out)
	}
}
