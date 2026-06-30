package fleet

import (
	"fmt"

	"github.com/quant-risk/radiant-harness/v3/internal/worktree"
)

// Isolator bridges the Fleet to real git worktrees so parallel agents claim
// tasks into isolated checkouts instead of sharing one working tree. Before
// this, Store.ClaimTask recorded a WorktreeDir string that nothing created;
// now ClaimIsolated actually provisions the worktree.
type Isolator struct {
	store *Store
	mgr   *worktree.Manager
}

// NewIsolator wires a Store to a worktree Manager rooted at repoDir. It errors
// if repoDir is not a git work tree.
func NewIsolator(store *Store, repoDir string) (*Isolator, error) {
	mgr, err := worktree.NewManager(repoDir)
	if err != nil {
		return nil, err
	}
	return &Isolator{store: store, mgr: mgr}, nil
}

// ClaimIsolated provisions a dedicated git worktree for the next pending task,
// then atomically claims that task for agentID with the worktree path recorded.
// The worktree is named "<runID>/<taskID>". Returns nil if no task is pending.
//
// On claim failure the freshly created worktree is removed so we don't leak
// checkouts for tasks that were never assigned.
func (iso *Isolator) ClaimIsolated(agentID string) (*Task, worktree.Worktree, error) {
	snap := iso.store.Snapshot()
	var pending *Task
	for i := range snap.Tasks {
		if snap.Tasks[i].Status == TaskPending {
			pending = &snap.Tasks[i]
			break
		}
	}
	if pending == nil {
		return nil, worktree.Worktree{}, nil
	}

	name := snap.RunID + "/" + pending.ID
	wt, err := iso.mgr.Add(name)
	if err != nil {
		return nil, worktree.Worktree{}, fmt.Errorf("provision worktree: %w", err)
	}

	task, err := iso.store.ClaimTask(agentID, wt.Path)
	if err != nil || task == nil {
		// Roll back the worktree we just made.
		_ = iso.mgr.Remove(wt, true)
		if err != nil {
			return nil, worktree.Worktree{}, fmt.Errorf("claim after provision: %w", err)
		}
		return nil, worktree.Worktree{}, nil // race: task taken between snapshot and claim
	}
	return task, wt, nil
}

// Release removes the worktree for a completed task.
func (iso *Isolator) Release(wt worktree.Worktree, force bool) error {
	return iso.mgr.Remove(wt, force)
}

// Manager exposes the underlying worktree manager for list/prune operations.
func (iso *Isolator) Manager() *worktree.Manager { return iso.mgr }
