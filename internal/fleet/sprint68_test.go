package fleet_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/quant-risk/radiant-harness/v3/internal/fleet"
)

// ── FleetStatus JSON tags ──────────────────────────────────────────────────

func TestFleetStatus_JSONRoundtrip(t *testing.T) {
	if !isGitRepo(t) {
		t.Skip("not in a git repo")
	}
	runID := uniqueRunID("s68")
	cleanBranch(t, runID)

	store, err := fleet.NewStore(".", runID, "sprint68 goal")
	if err != nil {
		t.Fatal(err)
	}
	_ = store.SetTasks([]fleet.Task{
		{ID: "t1", Title: "do something", DoneWhen: "tests pass", Status: fleet.TaskPending},
	})
	coord := fleet.NewCoordinator(store, 1)
	status := coord.Status()

	b, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	raw := string(b)

	// snake_case keys must be present.
	for _, key := range []string{`"run_id"`, `"goal"`, `"agent_count"`, `"tasks"`, `"started_at"`, `"updated_at"`} {
		if !strings.Contains(raw, key) {
			t.Errorf("expected JSON key %q in output: %s", key, raw)
		}
	}

	// Tasks array must contain the task.
	if !strings.Contains(raw, "t1") {
		t.Errorf("expected task ID 't1' in JSON: %s", raw)
	}
}

func TestFleetStatus_JSON_GoalPreserved(t *testing.T) {
	if !isGitRepo(t) {
		t.Skip("not in a git repo")
	}
	runID := uniqueRunID("s68g")
	cleanBranch(t, runID)

	store, err := fleet.NewStore(".", runID, "build autonomous CI")
	if err != nil {
		t.Fatal(err)
	}
	coord := fleet.NewCoordinator(store, 0)
	b, _ := json.Marshal(coord.Status())
	if !strings.Contains(string(b), "build autonomous CI") {
		t.Errorf("goal not in JSON: %s", string(b))
	}
}

func TestFleetStatus_JSON_RunIDPresent(t *testing.T) {
	if !isGitRepo(t) {
		t.Skip("not in a git repo")
	}
	runID := uniqueRunID("s68r")
	cleanBranch(t, runID)

	store, err := fleet.NewStore(".", runID, "goal")
	if err != nil {
		t.Fatal(err)
	}
	coord := fleet.NewCoordinator(store, 0)
	b, _ := json.Marshal(coord.Status())
	if !strings.Contains(string(b), runID) {
		t.Errorf("run_id not in JSON: %s", string(b))
	}
}

func TestFleetStatus_JSON_TasksArray(t *testing.T) {
	if !isGitRepo(t) {
		t.Skip("not in a git repo")
	}
	runID := uniqueRunID("s68t")
	cleanBranch(t, runID)

	store, err := fleet.NewStore(".", runID, "g")
	if err != nil {
		t.Fatal(err)
	}
	_ = store.SetTasks([]fleet.Task{
		{ID: "a", Title: "alpha", DoneWhen: "done", Status: fleet.TaskPending},
		{ID: "b", Title: "beta", DoneWhen: "done", Status: fleet.TaskPending},
	})
	coord := fleet.NewCoordinator(store, 2)
	var out map[string]interface{}
	b, _ := json.Marshal(coord.Status())
	_ = json.Unmarshal(b, &out)
	tasks, ok := out["tasks"].([]interface{})
	if !ok {
		t.Fatalf("expected 'tasks' array, got: %T", out["tasks"])
	}
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(tasks))
	}
}
