// Package worktree manages isolated git worktrees so parallel Fleet agents
// never collide on the working tree. This implements the "each finding gets
// its own isolated git worktree" pattern from the loop-engineering literature:
// before this package, internal/fleet only had a WorktreeDir string field with
// no actual worktree creation — two Implementer agents in the same repo would
// step on each other's edits.
//
// Each worktree is a real `git worktree` checked out on its own branch under
// .radiant-harness/worktrees/<runID>/<taskID>, so agents commit independently
// and the coordinator merges/resolves afterward (see internal/fleet/resolver).
package worktree

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Worktree is a single isolated checkout.
type Worktree struct {
	Path   string `json:"path"`
	Branch string `json:"branch"`
}

// Manager creates and removes worktrees rooted at a git repository.
type Manager struct {
	repoDir string
	baseDir string // where worktrees are created (under the repo)
}

// NewManager returns a Manager for repoDir. It returns an error if repoDir is
// not the top level of a git working tree.
func NewManager(repoDir string) (*Manager, error) {
	out, err := runGit(repoDir, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		return nil, fmt.Errorf("not a git repo (%s): %w", repoDir, err)
	}
	if strings.TrimSpace(out) != "true" {
		return nil, fmt.Errorf("not inside a git work tree: %s", repoDir)
	}
	return &Manager{
		repoDir: repoDir,
		baseDir: filepath.Join(repoDir, ".radiant-harness", "worktrees"),
	}, nil
}

// Add creates an isolated worktree named `name` (typically "<runID>/<taskID>")
// on a fresh branch "radiant/wt/<name>". The branch is created from the current
// HEAD. Returns the worktree handle.
func (m *Manager) Add(name string) (Worktree, error) {
	if name == "" {
		return Worktree{}, fmt.Errorf("worktree name is empty")
	}
	path := filepath.Join(m.baseDir, filepath.FromSlash(name))
	branch := "radiant/wt/" + name

	// `git worktree add -b <branch> <path>` creates the branch + checkout.
	if _, err := runGit(m.repoDir, "worktree", "add", "-b", branch, path); err != nil {
		return Worktree{}, fmt.Errorf("worktree add %q: %w", name, err)
	}
	return Worktree{Path: path, Branch: branch}, nil
}

// Remove deletes a worktree's checkout. With force=true it discards
// uncommitted changes; otherwise git refuses to remove a dirty worktree.
func (m *Manager) Remove(wt Worktree, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, wt.Path)
	if _, err := runGit(m.repoDir, args...); err != nil {
		return fmt.Errorf("worktree remove %q: %w", wt.Path, err)
	}
	return nil
}

// List returns all worktrees registered with the repo (including the main
// working tree). Parses `git worktree list --porcelain`.
func (m *Manager) List() ([]Worktree, error) {
	out, err := runGit(m.repoDir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("worktree list: %w", err)
	}
	var result []Worktree
	var cur Worktree
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			cur = Worktree{Path: strings.TrimPrefix(line, "worktree ")}
		case strings.HasPrefix(line, "branch "):
			// refs/heads/<branch>
			ref := strings.TrimPrefix(line, "branch ")
			cur.Branch = strings.TrimPrefix(ref, "refs/heads/")
		case line == "":
			if cur.Path != "" {
				result = append(result, cur)
				cur = Worktree{}
			}
		}
	}
	if cur.Path != "" {
		result = append(result, cur)
	}
	return result, nil
}

// Prune removes worktree administrative entries whose directories have already
// been deleted from disk (e.g. after a crash).
func (m *Manager) Prune() error {
	if _, err := runGit(m.repoDir, "worktree", "prune"); err != nil {
		return fmt.Errorf("worktree prune: %w", err)
	}
	return nil
}

// runGit runs a git subcommand in dir and returns combined output.
func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
	}
	return string(out), nil
}
