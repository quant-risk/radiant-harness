package fleet

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SharedContext is the inter-agent context store.
// Agents read from it and write to it; writes are atomic and conflict-safe.
type SharedContext struct {
	RunID     string            `json:"run_id"`
	Goal      string            `json:"goal"`
	Tasks     []Task            `json:"tasks"`
	Meta      map[string]string `json:"meta"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// Task is a unit of work assigned to an Implementer agent.
type Task struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Files       []string   `json:"files"`
	DoneWhen    string     `json:"done_when"`
	Status      TaskStatus `json:"status"`
	AgentID     string     `json:"agent_id,omitempty"`
	WorktreeDir string     `json:"worktree_dir,omitempty"`
	Evidence    string     `json:"evidence,omitempty"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// TaskStatus is the lifecycle of a task.
type TaskStatus string

const (
	TaskPending  TaskStatus = "pending"
	TaskAssigned TaskStatus = "assigned"
	TaskDone     TaskStatus = "done"
	TaskFailed   TaskStatus = "failed"
	TaskConflict TaskStatus = "conflict"
	// TaskCrashed (v3.7.9+) — the task's agent subprocess
	// died without writing a terminal status (TaskDone or
	// TaskFailed). Detected by reading the task pid file and
	// running `kill -0` on it; if the pid is dead but the
	// task is still TaskAssigned, the status is escalated to
	// TaskCrashed so the operator can distinguish "agent is
	// still running" from "agent died mid-execution".
	//
	// Set automatically by `Coordinator.Status()` when a
	// livenessDir is configured. Persisted by callers via
	// `Store.CrashTask` if they want the crashed state to
	// survive across process restarts.
	TaskCrashed TaskStatus = "crashed"
)

// Store is the persistent, mutex-protected shared context store.
type Store struct {
	mu   sync.Mutex
	path string
	ctx  SharedContext
}

// NewStore initializes a shared context store at projectDir/.radiant-harness/fleet/<runID>.json.
func NewStore(projectDir, runID, goal string) (*Store, error) {
	dir := filepath.Join(projectDir, ".radiant-harness", "fleet")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir fleet: %w", err)
	}
	path := filepath.Join(dir, runID+".json")
	s := &Store{
		path: path,
		ctx: SharedContext{
			RunID:     runID,
			Goal:      goal,
			Tasks:     []Task{},
			Meta:      map[string]string{},
			UpdatedAt: time.Now().UTC(),
		},
	}
	return s, s.persist()
}

// LoadStore reads an existing store from disk.
func LoadStore(projectDir, runID string) (*Store, error) {
	path := filepath.Join(projectDir, ".radiant-harness", "fleet", runID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read store: %w", err)
	}
	var ctx SharedContext
	if err := json.Unmarshal(data, &ctx); err != nil {
		return nil, fmt.Errorf("parse store: %w", err)
	}
	return &Store{path: path, ctx: ctx}, nil
}

// SetTasks replaces the task list (Planner output).
func (s *Store) SetTasks(tasks []Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ctx.Tasks = tasks
	s.ctx.UpdatedAt = time.Now().UTC()
	return s.persist()
}

// ClaimTask atomically assigns the first pending task to agentID.
// Returns nil if no tasks are available.
func (s *Store) ClaimTask(agentID, worktreeDir string) (*Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.ctx.Tasks {
		if s.ctx.Tasks[i].Status == TaskPending {
			s.ctx.Tasks[i].Status = TaskAssigned
			s.ctx.Tasks[i].AgentID = agentID
			s.ctx.Tasks[i].WorktreeDir = worktreeDir
			s.ctx.Tasks[i].UpdatedAt = time.Now().UTC()
			s.ctx.UpdatedAt = time.Now().UTC()
			task := s.ctx.Tasks[i]
			return &task, s.persist()
		}
	}
	return nil, nil // no tasks available
}

// CompleteTask marks a task done with evidence.
func (s *Store) CompleteTask(taskID, evidence string, success bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.ctx.Tasks {
		if s.ctx.Tasks[i].ID == taskID {
			if success {
				s.ctx.Tasks[i].Status = TaskDone
			} else {
				s.ctx.Tasks[i].Status = TaskFailed
			}
			s.ctx.Tasks[i].Evidence = evidence
			s.ctx.Tasks[i].UpdatedAt = time.Now().UTC()
			s.ctx.UpdatedAt = time.Now().UTC()
			return s.persist()
		}
	}
	return fmt.Errorf("task %q not found", taskID)
}

// ResetTask resets a failed task back to pending so it can be re-dispatched.
func (s *Store) ResetTask(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.ctx.Tasks {
		if s.ctx.Tasks[i].ID == taskID {
			s.ctx.Tasks[i].Status = TaskPending
			s.ctx.Tasks[i].Evidence = ""
			s.ctx.Tasks[i].AgentID = ""
			s.ctx.Tasks[i].WorktreeDir = ""
			s.ctx.Tasks[i].UpdatedAt = time.Now().UTC()
			s.ctx.UpdatedAt = time.Now().UTC()
			return s.persist()
		}
	}
	return fmt.Errorf("task %q not found", taskID)
}

// CrashTask (v3.7.9+) marks a task as TaskCrashed — the agent
// subprocess died without writing a terminal status. Persists
// the supplied evidence (typically the liveness probe result,
// e.g. "pid 12345 not alive"). Idempotent: re-calling on an
// already-crashed task updates the evidence + timestamp.
func (s *Store) CrashTask(taskID, evidence string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.ctx.Tasks {
		if s.ctx.Tasks[i].ID == taskID {
			s.ctx.Tasks[i].Status = TaskCrashed
			s.ctx.Tasks[i].Evidence = evidence
			s.ctx.Tasks[i].UpdatedAt = time.Now().UTC()
			s.ctx.UpdatedAt = time.Now().UTC()
			return s.persist()
		}
	}
	return fmt.Errorf("task %q not found", taskID)
}

// SetMeta sets a metadata key in the shared context.
func (s *Store) SetMeta(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ctx.Meta[key] = value
	s.ctx.UpdatedAt = time.Now().UTC()
	return s.persist()
}

// Snapshot returns a copy of the current context.
func (s *Store) Snapshot() SharedContext {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Deep copy tasks
	tasks := make([]Task, len(s.ctx.Tasks))
	copy(tasks, s.ctx.Tasks)
	meta := make(map[string]string, len(s.ctx.Meta))
	for k, v := range s.ctx.Meta {
		meta[k] = v
	}
	return SharedContext{
		RunID:     s.ctx.RunID,
		Goal:      s.ctx.Goal,
		Tasks:     tasks,
		Meta:      meta,
		UpdatedAt: s.ctx.UpdatedAt,
	}
}

// persist writes the store atomically. Caller must hold s.mu.
// FleetSummary is a lightweight view of a persisted fleet for history listings.
type FleetSummary struct {
	RunID     string    `json:"run_id"`
	Goal      string    `json:"goal"`
	Total     int       `json:"total"`
	Done      int       `json:"done"`
	Failed    int       `json:"failed"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ListFleets returns a summary of all persisted fleets, newest-first.
func ListFleets(projectDir string) ([]FleetSummary, error) {
	dir := filepath.Join(projectDir, ".radiant-harness", "fleet")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []FleetSummary
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		runID := e.Name()[:len(e.Name())-5]
		store, err := LoadStore(projectDir, runID)
		if err != nil {
			continue
		}
		snap := store.Snapshot()
		s := FleetSummary{RunID: runID, Goal: snap.Goal, Total: len(snap.Tasks), UpdatedAt: snap.UpdatedAt}
		for _, t := range snap.Tasks {
			switch t.Status {
			case TaskDone:
				s.Done++
			case TaskFailed:
				s.Failed++
			}
		}
		out = append(out, s)
	}
	// Sort newest-first by UpdatedAt (insertion sort — typically small N).
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].UpdatedAt.After(out[j-1].UpdatedAt); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out, nil
}

func (s *Store) persist() error {
	data, err := json.MarshalIndent(s.ctx, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.path)
	tmp, err := os.CreateTemp(dir, ".fleet-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	tmp.Close()
	return os.Rename(name, s.path)
}
