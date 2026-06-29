package fleet

import (
	"strings"
	"testing"
	"time"
)

// ── Roles tests ────────────────────────────────────────────────────────────────

func TestDefaultRoleConfigs_AllFourRoles(t *testing.T) {
	roles := DefaultRoleConfigs()
	for _, role := range []AgentRole{RolePlanner, RoleImplementer, RoleVerifier, RoleSummarizer} {
		if _, ok := roles[role]; !ok {
			t.Errorf("missing role config for %q", role)
		}
	}
}

func TestDefaultRoleConfigs_BudgetsSet(t *testing.T) {
	roles := DefaultRoleConfigs()
	for role, cfg := range roles {
		if cfg.TokenBudget <= 0 {
			t.Errorf("role %q has zero token budget", role)
		}
		if cfg.MaxIterations <= 0 {
			t.Errorf("role %q has zero max iterations", role)
		}
	}
}

func TestDefaultRoleConfigs_ImplementerLargestBudget(t *testing.T) {
	roles := DefaultRoleConfigs()
	implBudget := roles[RoleImplementer].TokenBudget
	for role, cfg := range roles {
		if role == RoleImplementer {
			continue
		}
		if cfg.TokenBudget >= implBudget {
			t.Errorf("role %q has budget %d >= implementer budget %d",
				role, cfg.TokenBudget, implBudget)
		}
	}
}

func TestDefaultRoleConfigs_VerifierPromptIsAdversarial(t *testing.T) {
	roles := DefaultRoleConfigs()
	prompt := roles[RoleVerifier].SystemPrompt
	if !strings.Contains(prompt, "BROKEN") {
		t.Errorf("verifier prompt should contain adversarial 'BROKEN' instruction, got:\n%s", prompt)
	}
	if !strings.Contains(strings.ToUpper(prompt), "REJECTED") {
		t.Errorf("verifier prompt should contain REJECTED verdict option")
	}
}

func TestFormatRoleConfig(t *testing.T) {
	cfg := DefaultRoleConfigs()[RolePlanner]
	out := FormatRoleConfig(cfg)
	if !strings.Contains(out, "planner") {
		t.Errorf("expected role name in output, got: %s", out)
	}
}

// ── Store tests ────────────────────────────────────────────────────────────────

func TestStore_NewAndSnapshot(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir, "fleet-001", "refactor auth module")
	if err != nil {
		t.Fatal(err)
	}
	snap := s.Snapshot()
	if snap.RunID != "fleet-001" {
		t.Errorf("RunID = %q, want fleet-001", snap.RunID)
	}
	if snap.Goal != "refactor auth module" {
		t.Errorf("Goal = %q, want refactor auth module", snap.Goal)
	}
}

func TestStore_SetAndClaimTask(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewStore(dir, "fleet-002", "goal")

	tasks := []Task{
		{ID: "t1", Title: "Implement login", Files: []string{"auth/login.go"}, Status: TaskPending},
		{ID: "t2", Title: "Write tests", Files: []string{"auth/login_test.go"}, Status: TaskPending},
	}
	if err := s.SetTasks(tasks); err != nil {
		t.Fatal(err)
	}

	// Agent 1 claims first task
	task, err := s.ClaimTask("agent-1", "/worktree/agent-1")
	if err != nil {
		t.Fatal(err)
	}
	if task == nil {
		t.Fatal("expected task, got nil")
	}
	if task.ID != "t1" {
		t.Errorf("claimed task ID = %q, want t1", task.ID)
	}
	if task.AgentID != "agent-1" {
		t.Errorf("AgentID = %q, want agent-1", task.AgentID)
	}

	// Agent 2 claims second task
	task2, err := s.ClaimTask("agent-2", "/worktree/agent-2")
	if err != nil || task2 == nil {
		t.Fatal("second agent should claim second task")
	}
	if task2.ID != "t2" {
		t.Errorf("second task ID = %q, want t2", task2.ID)
	}

	// No more tasks
	task3, err := s.ClaimTask("agent-3", "/worktree/agent-3")
	if err != nil {
		t.Fatal(err)
	}
	if task3 != nil {
		t.Errorf("expected nil (no tasks left), got task %q", task3.ID)
	}
}

func TestStore_CompleteTask(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewStore(dir, "fleet-003", "goal")
	s.SetTasks([]Task{{ID: "t1", Status: TaskPending}})
	s.ClaimTask("agent-1", "/wt")

	if err := s.CompleteTask("t1", "tests pass", true); err != nil {
		t.Fatal(err)
	}
	snap := s.Snapshot()
	if snap.Tasks[0].Status != TaskDone {
		t.Errorf("task status = %q, want done", snap.Tasks[0].Status)
	}
	if snap.Tasks[0].Evidence != "tests pass" {
		t.Errorf("evidence = %q, want 'tests pass'", snap.Tasks[0].Evidence)
	}
}

func TestStore_CompleteTask_Failed(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewStore(dir, "fleet-004", "goal")
	s.SetTasks([]Task{{ID: "t1", Status: TaskPending}})
	s.ClaimTask("agent-1", "/wt")
	s.CompleteTask("t1", "compilation error", false)

	snap := s.Snapshot()
	if snap.Tasks[0].Status != TaskFailed {
		t.Errorf("task status = %q, want failed", snap.Tasks[0].Status)
	}
}

func TestStore_CompleteTask_NotFound(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewStore(dir, "fleet-005", "goal")
	err := s.CompleteTask("nonexistent", "evidence", true)
	if err == nil {
		t.Error("expected error for unknown task ID")
	}
}

func TestStore_SetMeta(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewStore(dir, "fleet-006", "goal")
	if err := s.SetMeta("phase", "planning"); err != nil {
		t.Fatal(err)
	}
	snap := s.Snapshot()
	if snap.Meta["phase"] != "planning" {
		t.Errorf("meta phase = %q, want planning", snap.Meta["phase"])
	}
}

func TestStore_LoadStore(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewStore(dir, "fleet-007", "load test goal")
	s.SetTasks([]Task{{ID: "t1", Title: "task one", Status: TaskPending}})
	s.ClaimTask("ag1", "/wt")

	// Load from disk
	s2, err := LoadStore(dir, "fleet-007")
	if err != nil {
		t.Fatal(err)
	}
	snap := s2.Snapshot()
	if snap.Goal != "load test goal" {
		t.Errorf("loaded goal = %q", snap.Goal)
	}
	if snap.Tasks[0].Status != TaskAssigned {
		t.Errorf("loaded task status = %q, want assigned", snap.Tasks[0].Status)
	}
}

// ── Resolver tests ────────────────────────────────────────────────────────────

func TestDetectConflicts_NoConflict(t *testing.T) {
	tasks := []Task{
		{ID: "t1", Files: []string{"a.go"}, Status: TaskDone, AgentID: "ag1"},
		{ID: "t2", Files: []string{"b.go"}, Status: TaskDone, AgentID: "ag2"},
	}
	conflicts := DetectConflicts(tasks)
	if len(conflicts) != 0 {
		t.Errorf("expected no conflicts, got %d", len(conflicts))
	}
}

func TestDetectConflicts_DetectsOverlap(t *testing.T) {
	tasks := []Task{
		{ID: "t1", Files: []string{"shared.go", "a.go"}, Status: TaskDone, AgentID: "ag1"},
		{ID: "t2", Files: []string{"shared.go", "b.go"}, Status: TaskDone, AgentID: "ag2"},
	}
	conflicts := DetectConflicts(tasks)
	if len(conflicts) != 1 {
		t.Errorf("expected 1 conflict, got %d", len(conflicts))
	}
	if conflicts[0].File != "shared.go" {
		t.Errorf("conflict file = %q, want shared.go", conflicts[0].File)
	}
}

func TestDetectConflicts_SkipsPendingTasks(t *testing.T) {
	tasks := []Task{
		{ID: "t1", Files: []string{"a.go"}, Status: TaskDone, AgentID: "ag1"},
		{ID: "t2", Files: []string{"a.go"}, Status: TaskPending}, // pending — not done yet
	}
	conflicts := DetectConflicts(tasks)
	if len(conflicts) != 0 {
		t.Errorf("pending tasks should not produce conflicts, got %d", len(conflicts))
	}
}

func TestResolveConflict_PrefersMoreEvidence(t *testing.T) {
	tasks := []Task{
		{ID: "t1", AgentID: "ag1", Evidence: "short", Status: TaskDone},
		{ID: "t2", AgentID: "ag2", Evidence: "longer evidence with test results and coverage", Status: TaskDone},
	}
	conflict := Conflict{File: "x.go", TaskA: "t1", TaskB: "t2", AgentA: "ag1", AgentB: "ag2"}
	res := ResolveConflict(conflict, tasks)
	if res.Winner != "ag2" {
		t.Errorf("winner = %q, want ag2 (more evidence)", res.Winner)
	}
}

func TestResolveConflict_PrefersSuccessOverFailed(t *testing.T) {
	tasks := []Task{
		{ID: "t1", AgentID: "ag1", Evidence: "long evidence", Status: TaskFailed},
		{ID: "t2", AgentID: "ag2", Evidence: "short", Status: TaskDone},
	}
	conflict := Conflict{File: "x.go", TaskA: "t1", TaskB: "t2", AgentA: "ag1", AgentB: "ag2"}
	res := ResolveConflict(conflict, tasks)
	if res.Winner != "ag2" {
		t.Errorf("winner = %q, want ag2 (task not failed)", res.Winner)
	}
}

func TestFormatConflicts_NoConflict(t *testing.T) {
	out := FormatConflicts(nil, nil)
	if !strings.Contains(out, "No conflicts") {
		t.Errorf("expected 'No conflicts', got: %s", out)
	}
}

func TestFormatConflicts_WithResolution(t *testing.T) {
	conflicts := []Conflict{
		{File: "main.go", AgentA: "ag1", AgentB: "ag2", TaskA: "t1", TaskB: "t2",
			DetectedAt: time.Now()},
	}
	resolutions := []Resolution{
		{Conflict: conflicts[0], Winner: "ag1", Reason: "more evidence"},
	}
	out := FormatConflicts(conflicts, resolutions)
	if !strings.Contains(out, "main.go") {
		t.Errorf("expected file name in output, got: %s", out)
	}
	if !strings.Contains(out, "ag1") {
		t.Errorf("expected winner in output, got: %s", out)
	}
}

// ── Coordinator tests ─────────────────────────────────────────────────────────

func TestCoordinator_RegisterAndStatus(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewStore(dir, "fleet-c01", "coordinate goal")
	c := NewCoordinator(s, 2)

	c.RegisterAgent("ag1", RolePlanner)
	c.RegisterAgent("ag2", RoleImplementer)

	status := c.Status()
	if status.AgentCount != 2 {
		t.Errorf("AgentCount = %d, want 2", status.AgentCount)
	}
	if status.Goal != "coordinate goal" {
		t.Errorf("Goal = %q, want coordinate goal", status.Goal)
	}
}

func TestCoordinator_RolePromptContainsGoal(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewStore(dir, "fleet-c02", "implement payment flow")
	c := NewCoordinator(s, 1)

	prompt := c.RolePrompt(RoleImplementer, nil)
	if !strings.Contains(prompt, "implement payment flow") {
		t.Errorf("role prompt should contain goal, got:\n%s", prompt)
	}
}

func TestCoordinator_RolePromptWithTask(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewStore(dir, "fleet-c03", "goal")
	c := NewCoordinator(s, 1)

	task := &Task{
		ID:       "t1",
		Title:    "Add JWT login",
		Files:    []string{"auth/login.go"},
		DoneWhen: "tests pass",
	}
	prompt := c.RolePrompt(RoleImplementer, task)
	if !strings.Contains(prompt, "Add JWT login") {
		t.Errorf("prompt should contain task title, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "auth/login.go") {
		t.Errorf("prompt should contain task files, got:\n%s", prompt)
	}
}

func TestFormatFleetStatus(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewStore(dir, "fleet-c04", "build feature")
	s.SetTasks([]Task{
		{ID: "t1", Title: "Implement login", Status: TaskDone, AgentID: "ag1"},
		{ID: "t2", Title: "Write tests", Status: TaskPending},
	})
	c := NewCoordinator(s, 2)
	c.RegisterAgent("ag1", RoleImplementer)

	status := c.Status()
	out := FormatStatus(status)

	if !strings.Contains(out, "fleet-c04") {
		t.Errorf("expected run ID in output, got: %s", out)
	}
	if !strings.Contains(out, "done") {
		t.Errorf("expected done status in output, got: %s", out)
	}
	if !strings.Contains(out, "pending") {
		t.Errorf("expected pending status in output, got: %s", out)
	}
}
