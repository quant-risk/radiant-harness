package fleet

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@radiant.local")
	run("config", "user.name", "radiant test")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# t\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "init")
	return dir
}

func gitAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

func TestClaimIsolated_ProvisionsRealWorktree(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not installed")
	}
	repo := initRepo(t)
	store, err := NewStore(repo, "run-iso", "test goal")
	if err != nil {
		t.Fatal(err)
	}
	store.SetTasks([]Task{
		{ID: "task-1", Title: "first", Status: TaskPending},
		{ID: "task-2", Title: "second", Status: TaskPending},
	})

	iso, err := NewIsolator(store, repo)
	if err != nil {
		t.Fatal(err)
	}

	task, wt, err := iso.ClaimIsolated("agent-01")
	if err != nil {
		t.Fatalf("ClaimIsolated: %v", err)
	}
	if task == nil {
		t.Fatal("expected a task, got nil")
	}
	// The recorded WorktreeDir must be the real, on-disk worktree path.
	if task.WorktreeDir != wt.Path {
		t.Errorf("task.WorktreeDir = %q, want %q", task.WorktreeDir, wt.Path)
	}
	if _, err := os.Stat(wt.Path); err != nil {
		t.Errorf("worktree not created on disk: %v", err)
	}
	if !strings.Contains(wt.Branch, "run-iso/task-1") {
		t.Errorf("branch = %q, want it to contain run-iso/task-1", wt.Branch)
	}
}

func TestClaimIsolated_TwoAgentsGetDifferentWorktrees(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not installed")
	}
	repo := initRepo(t)
	store, _ := NewStore(repo, "run-iso2", "goal")
	store.SetTasks([]Task{
		{ID: "a", Status: TaskPending},
		{ID: "b", Status: TaskPending},
	})
	iso, _ := NewIsolator(store, repo)

	_, wtA, err := iso.ClaimIsolated("agent-A")
	if err != nil {
		t.Fatal(err)
	}
	_, wtB, err := iso.ClaimIsolated("agent-B")
	if err != nil {
		t.Fatal(err)
	}
	if wtA.Path == wtB.Path {
		t.Fatal("two agents got the same worktree — no isolation")
	}
}

func TestClaimIsolated_NoPendingTasks(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not installed")
	}
	repo := initRepo(t)
	store, _ := NewStore(repo, "run-empty", "goal")
	iso, _ := NewIsolator(store, repo)

	task, _, err := iso.ClaimIsolated("agent-01")
	if err != nil {
		t.Fatalf("ClaimIsolated on empty store: %v", err)
	}
	if task != nil {
		t.Errorf("expected nil task when nothing pending, got %+v", task)
	}
}

func TestClaimIsolated_Release(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not installed")
	}
	repo := initRepo(t)
	store, _ := NewStore(repo, "run-rel", "goal")
	store.SetTasks([]Task{{ID: "t", Status: TaskPending}})
	iso, _ := NewIsolator(store, repo)

	_, wt, err := iso.ClaimIsolated("agent-01")
	if err != nil {
		t.Fatal(err)
	}
	if err := iso.Release(wt, true); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if _, err := os.Stat(wt.Path); !os.IsNotExist(err) {
		t.Errorf("worktree should be removed after Release")
	}
}

func TestNewIsolator_RejectsNonRepo(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	store, _ := NewStore(dir, "run-x", "goal")
	if _, err := NewIsolator(store, dir); err == nil {
		t.Error("NewIsolator should reject a non-git directory")
	}
}
