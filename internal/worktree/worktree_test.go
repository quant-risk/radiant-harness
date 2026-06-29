package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initRepo creates a real git repository with one commit in a temp dir, so
// `git worktree add` has a HEAD to branch from.
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
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test\n"), 0o644); err != nil {
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

func TestNewManager_RejectsNonRepo(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not installed")
	}
	if _, err := NewManager(t.TempDir()); err == nil {
		t.Error("NewManager should reject a non-git directory")
	}
}

func TestNewManager_AcceptsRepo(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not installed")
	}
	if _, err := NewManager(initRepo(t)); err != nil {
		t.Errorf("NewManager rejected a valid repo: %v", err)
	}
}

func TestAddAndList(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not installed")
	}
	repo := initRepo(t)
	m, err := NewManager(repo)
	if err != nil {
		t.Fatal(err)
	}

	wt, err := m.Add("run-1/task-1")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if wt.Branch != "radiant/wt/run-1/task-1" {
		t.Errorf("branch = %q, want radiant/wt/run-1/task-1", wt.Branch)
	}
	// The worktree directory must exist on disk.
	if _, err := os.Stat(wt.Path); err != nil {
		t.Errorf("worktree path not created: %v", err)
	}

	list, err := m.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	// main worktree + the one we added
	if len(list) < 2 {
		t.Errorf("expected ≥2 worktrees, got %d: %+v", len(list), list)
	}
	var found bool
	for _, w := range list {
		if w.Branch == "radiant/wt/run-1/task-1" {
			found = true
		}
	}
	if !found {
		t.Errorf("added worktree not in list: %+v", list)
	}
}

func TestAddIsolation(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not installed")
	}
	repo := initRepo(t)
	m, _ := NewManager(repo)

	wtA, err := m.Add("run-1/task-a")
	if err != nil {
		t.Fatal(err)
	}
	wtB, err := m.Add("run-1/task-b")
	if err != nil {
		t.Fatal(err)
	}
	// Two agents writing the same filename must not collide: each worktree
	// is a separate directory.
	if wtA.Path == wtB.Path {
		t.Fatal("two worktrees share a path — no isolation")
	}
	fileA := filepath.Join(wtA.Path, "work.txt")
	fileB := filepath.Join(wtB.Path, "work.txt")
	os.WriteFile(fileA, []byte("from A"), 0o644)
	os.WriteFile(fileB, []byte("from B"), 0o644)

	a, _ := os.ReadFile(fileA)
	b, _ := os.ReadFile(fileB)
	if string(a) != "from A" || string(b) != "from B" {
		t.Errorf("worktrees not isolated: A=%q B=%q", a, b)
	}
}

func TestRemove(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not installed")
	}
	repo := initRepo(t)
	m, _ := NewManager(repo)

	wt, err := m.Add("run-1/task-x")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Remove(wt, true); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Stat(wt.Path); !os.IsNotExist(err) {
		t.Errorf("worktree path should be gone after Remove, stat err = %v", err)
	}
}

func TestAddEmptyName(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not installed")
	}
	m, _ := NewManager(initRepo(t))
	if _, err := m.Add(""); err == nil {
		t.Error("Add with empty name should error")
	}
}

func TestPrune(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not installed")
	}
	repo := initRepo(t)
	m, _ := NewManager(repo)
	wt, _ := m.Add("run-1/task-prune")
	// Delete the directory out from under git, then prune.
	os.RemoveAll(wt.Path)
	if err := m.Prune(); err != nil {
		t.Errorf("Prune: %v", err)
	}
}
