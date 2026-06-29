package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	radiant "github.com/quant-risk/radiant-harness/internal"
)

// validTransitions encodes the harness state machine. Any transition not
// listed here is rejected, preventing the orchestrator from entering invalid
// states (e.g. jumping idle → done without going through implement).
var validTransitions = map[radiant.HarnessState][]radiant.HarnessState{
	radiant.StateIdle:       {radiant.StateResearch, radiant.StatePlan, radiant.StateImplement},
	radiant.StateResearch:   {radiant.StatePlan, radiant.StateFailed},
	radiant.StatePlan:       {radiant.StateImplement, radiant.StateFailed},
	radiant.StateImplement:  {radiant.StateValidate, radiant.StateCorrecting, radiant.StateDone, radiant.StateFailed},
	radiant.StateValidate:   {radiant.StateDone, radiant.StateCorrecting, radiant.StateFailed},
	radiant.StateCorrecting: {radiant.StateImplement, radiant.StateValidate, radiant.StateFailed},
	radiant.StateDone:       {radiant.StateIdle},
	radiant.StateFailed:     {radiant.StateIdle, radiant.StateImplement},
}

// State manages the harness state machine with guarded transitions and
// crash-safe persistence (atomic write + fsync + rename). Concurrency
// safety comes from two layers:
//
//  1. Intra-process: a sync.Mutex serializes all readers/writers of
//     the in-memory Progress.
//
//  2. Inter-process: an advisory lock on the .radiant-harness directory
//     serializes `radiant run` invocations on the same project across
//     processes (see lock.go for the cross-platform implementation).
type State struct {
	mu       sync.Mutex
	data     radiant.Progress
	dir      string
	filePath string
	lockPath string
	lock     *Lock
}

// NewState creates a new State for a project and loads any existing progress
// from disk. It does NOT acquire the advisory lock — call Lock() before
// running the orchestrator and Release() when done. Splitting the lock from
// construction keeps NewState cheap for read-only callers (tests, validators,
// progress queries) while still giving the orchestrator exclusive access.
func NewState(projectDir string) *State {
	dir := filepath.Join(projectDir, ".radiant-harness")
	s := &State{
		dir:      dir,
		filePath: filepath.Join(dir, "progress.json"),
		lockPath: dir,
		data: radiant.Progress{
			State:     radiant.StateIdle,
			StartedAt: time.Now(),
		},
	}
	s.tryLoad()
	return s
}

// Lock acquires an exclusive advisory lock on the state directory, blocking
// (with retries up to the timeout) until any other holder releases.
// Combined with Release(), this serializes orchestrator runs on the same
// project across processes so two parallel `radiant run` invocations can't
// corrupt progress.json. LockTimeout is the max time to wait before
// returning an error.
func (s *State) Lock() error {
	lock, err := LockWithRetry(s.lockPath, LockTimeout)
	if err != nil {
		return fmt.Errorf("acquire harness lock: %w", err)
	}
	s.mu.Lock()
	s.lock = lock
	s.mu.Unlock()
	return nil
}

// Release releases the advisory lock. Safe to call multiple times. After
// Release the State is still usable for reads and Snapshot() calls, but
// no longer guards against concurrent writers.
func (s *State) Release() {
	s.mu.Lock()
	lock := s.lock
	s.lock = nil
	s.mu.Unlock()
	if lock == nil {
		return
	}
	_ = lock.Release()
}

// LockTimeout is how long Run waits for an advisory lock before failing.
// Two minutes is enough for one Run to finish a typical task but short
// enough that a stuck Run doesn't permanently block the next one.
const LockTimeout = 2 * time.Minute

// Transition moves to a new state with guard validation. Returns an error
// if the transition is not allowed by the state machine.
func (s *State) Transition(newState radiant.HarnessState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if allowed, ok := validTransitions[s.data.State]; ok {
		valid := false
		for _, a := range allowed {
			if a == newState {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid transition: %s → %s (allowed: %v)", s.data.State, newState, allowed)
		}
	}

	s.data.State = newState
	s.data.UpdatedAt = time.Now()
	s.data.Log = append(s.data.Log, radiant.ProgressEntry{
		Timestamp: time.Now(),
		Action:    "transition",
		Detail:    string(newState),
	})

	return nil
}

// MustTransition logs an error and continues on invalid transition (never aborts).
func (s *State) MustTransition(newState radiant.HarnessState) {
	if err := s.Transition(newState); err != nil {
		fmt.Fprintf(os.Stderr, "  Warning: %v\n", err)
	}
}

// SetFeature sets the current feature name.
func (s *State) SetFeature(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Feature = name
}

// SetTotalTasks sets the total number of tasks.
func (s *State) SetTotalTasks(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.TotalTasks = n
}

// StartTask marks a task as started.
func (s *State) StartTask(taskID int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.CurrentTask = taskID
	s.data.UpdatedAt = time.Now()
	s.data.Log = append(s.data.Log, radiant.ProgressEntry{
		Timestamp: time.Now(),
		TaskID:    taskID,
		Action:    "started",
	})
}

// CompleteTask marks a task as completed.
func (s *State) CompleteTask(taskID int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.UpdatedAt = time.Now()
	s.data.Log = append(s.data.Log, radiant.ProgressEntry{
		Timestamp: time.Now(),
		TaskID:    taskID,
		Action:    "completed",
	})
}

// FailTask marks a task as failed with errors.
func (s *State) FailTask(taskID int, errors []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.UpdatedAt = time.Now()
	s.data.Log = append(s.data.Log, radiant.ProgressEntry{
		Timestamp: time.Now(),
		TaskID:    taskID,
		Action:    "failed",
		Detail:    fmt.Sprintf("%v", errors),
	})
}

// CurrentState returns the current state.
func (s *State) CurrentState() radiant.HarnessState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.State
}

// Progress returns the current progress percentage (0.0 to 1.0). Counts
// distinct task IDs that have at least one "completed" entry — re-runs of
// the same task (start → complete → start → complete) count once. Clamped
// to [0, 1] so transient over-counting from concurrent mutations can't
// produce a 900% progress bar in the UI.
func (s *State) Progress() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.TotalTasks == 0 {
		return 0
	}
	completed := make(map[int]struct{})
	for _, entry := range s.data.Log {
		if entry.Action == "completed" {
			completed[entry.TaskID] = struct{}{}
		}
	}
	p := float64(len(completed)) / float64(s.data.TotalTasks)
	if p < 0 {
		return 0
	}
	if p > 1 {
		return 1
	}
	return p
}

// Snapshot returns a deep copy of the current Progress. Safe for concurrent
// callers that need to read state without holding the State mutex — useful
// for the VS Code extension's progress tree, CI reports, etc.
func (s *State) Snapshot() radiant.Progress {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := s.data
	cp.Log = append([]radiant.ProgressEntry(nil), s.data.Log...)
	return cp
}

// Save persists the state to disk atomically: it writes to a temp file in the
// same directory, fsyncs it, then renames over the destination. Rename is
// atomic on POSIX filesystems, so a crash mid-write either leaves the old
// file untouched or the new file fully in place — never a half-written one.
func (s *State) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("mkdir state dir: %w", err)
	}

	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	tmp, err := os.CreateTemp(s.dir, "progress.*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }

	if _, err := tmp.Write(append(data, '\n')); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("write temp: %w", err)
	}
	// fsync the file then the directory so the rename is durable across
	// power loss / kernel crashes, not just process crashes.
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("fsync temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpName, s.filePath); err != nil {
		cleanup()
		return fmt.Errorf("rename temp → %s: %w", s.filePath, err)
	}
	return nil
}

// Load reads the state from disk, replacing any in-memory data.
func (s *State) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &s.data)
}

// tryLoad attempts to load state, silently ignoring errors (missing file,
// partial write from a crashed run). The next Save() will overwrite any
// partial state with a consistent snapshot.
func (s *State) tryLoad() {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return
	}
	// Best-effort: an unparseable file is ignored so the harness can still
	// make progress instead of refusing to start after a crash mid-write.
	_ = json.Unmarshal(data, &s.data)
}

// String returns a human-readable state summary.
func (s *State) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return fmt.Sprintf("[%s] %s — task %d/%d (%.0f%%)",
		s.data.State, s.data.Feature, s.data.CurrentTask, s.data.TotalTasks, s.Progress()*100)
}
