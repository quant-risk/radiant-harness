package loop

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Phase represents a stage of the autonomous loop.
type Phase string

const (
	PhaseIdle     Phase = "idle"
	PhaseDiscover Phase = "discover"
	PhasePlan     Phase = "plan"
	PhaseExecute  Phase = "execute"
	PhaseVerify   Phase = "verify"
	PhasePersist  Phase = "persist"
	PhaseDone     Phase = "done"
	PhaseFailed   Phase = "failed"
)

// validTransitions is the state machine — only listed transitions are allowed.
var validTransitions = map[Phase][]Phase{
	PhaseIdle:     {PhaseDiscover},
	PhaseDiscover: {PhasePlan, PhaseFailed},
	PhasePlan:     {PhaseExecute, PhaseFailed},
	PhaseExecute:  {PhaseVerify, PhaseFailed},
	PhaseVerify:   {PhasePersist, PhaseExecute, PhaseFailed}, // PhaseExecute = retry
	PhasePersist:  {PhaseDone, PhaseDiscover},                // PhaseDiscover = next iter
	PhaseDone:     {PhaseIdle},
	PhaseFailed:   {PhaseIdle, PhaseDiscover},
}

// ExitReason describes why a loop terminated.
type ExitReason string

const (
	ExitSuccess  ExitReason = "success"
	ExitBudget   ExitReason = "budget_exhausted"
	ExitMaxIter  ExitReason = "max_iterations"
	ExitCritical ExitReason = "critical_failure"
	ExitCanceled ExitReason = "canceled"
)

// LoopState is the persisted state of a running or completed loop.
// Stored atomically in .radiant-harness/loop.json.
type LoopState struct {
	RunID          string          `json:"run_id"`
	Goal           string          `json:"goal"`
	Phase          Phase           `json:"phase"`
	Iteration      int             `json:"iteration"`
	MaxIter        int             `json:"max_iter"`
	ConsecFailures int             `json:"consec_failures"`
	ExitReason     ExitReason      `json:"exit_reason,omitempty"`
	Budget         Snapshot        `json:"budget"`
	StartedAt      time.Time       `json:"started_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
	Log            []PhaseLogEntry `json:"log"`
}

// PhaseLogEntry records a single phase transition.
type PhaseLogEntry struct {
	Timestamp time.Time `json:"ts"`
	From      Phase     `json:"from"`
	To        Phase     `json:"to"`
	Note      string    `json:"note,omitempty"`
}

// maxConsecFailures is the number of consecutive critical failures before
// the loop gives up and exits with ExitCritical.
const maxConsecFailures = 3

// Cycle manages the loop state machine for a single run.
// All exported methods are safe for concurrent use.
type Cycle struct {
	mu         sync.Mutex
	state      LoopState
	projectDir string
	statePath  string
}

// NewCycle creates a new Cycle for the given goal and project directory.
// It does NOT start the loop — call Run() for that.
func NewCycle(projectDir, runID, goal string, budget *Budget) *Cycle {
	return &Cycle{
		projectDir: projectDir,
		statePath:  filepath.Join(projectDir, ".radiant-harness", "loop.json"),
		state: LoopState{
			RunID:     runID,
			Goal:      goal,
			Phase:     PhaseIdle,
			Iteration: 0,
			MaxIter:   budget.MaxIter(),
			Budget:    budget.Snapshot(),
			StartedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
	}
}

// LoadCycle resumes a cycle from a persisted loop.json file.
func LoadCycle(projectDir string) (*Cycle, error) {
	path := filepath.Join(projectDir, ".radiant-harness", "loop.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read loop.json: %w", err)
	}
	var state LoopState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse loop.json: %w", err)
	}
	return &Cycle{
		projectDir: projectDir,
		statePath:  path,
		state:      state,
	}, nil
}

// Transition moves the cycle to the next phase, enforcing valid transitions.
// It persists state atomically after every successful transition.
func (c *Cycle) Transition(to Phase, note string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	from := c.state.Phase
	if !isValidTransition(from, to) {
		return fmt.Errorf("invalid transition %s → %s", from, to)
	}

	// Track consecutive failures
	if to == PhaseFailed {
		c.state.ConsecFailures++
	} else if to == PhasePersist || to == PhaseDone {
		c.state.ConsecFailures = 0
	}

	c.state.Phase = to
	c.state.UpdatedAt = time.Now().UTC()
	c.state.Log = append(c.state.Log, PhaseLogEntry{
		Timestamp: time.Now().UTC(),
		From:      from,
		To:        to,
		Note:      note,
	})

	return c.persistLocked()
}

// IncrIteration increments the iteration counter and persists.
func (c *Cycle) IncrIteration() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state.Iteration++
	c.state.UpdatedAt = time.Now().UTC()
	return c.persistLocked()
}

// SetExit records the exit reason and final state.
func (c *Cycle) SetExit(reason ExitReason, note string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.state.ExitReason = reason
	targetPhase := PhaseDone
	if reason != ExitSuccess {
		targetPhase = PhaseFailed
	}
	c.state.Phase = targetPhase
	c.state.UpdatedAt = time.Now().UTC()
	c.state.Log = append(c.state.Log, PhaseLogEntry{
		Timestamp: time.Now().UTC(),
		From:      c.state.Phase,
		To:        targetPhase,
		Note:      fmt.Sprintf("exit: %s — %s", reason, note),
	})
	return c.persistLocked()
}

// UpdateBudget refreshes the budget snapshot in the persisted state.
func (c *Cycle) UpdateBudget(b *Budget) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state.Budget = b.Snapshot()
	c.state.UpdatedAt = time.Now().UTC()
	return c.persistLocked()
}

// State returns a copy of the current loop state.
func (c *Cycle) State() LoopState {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state
}

// ShouldContinue returns true if the loop may proceed to the next iteration.
// It checks budget, max iterations, and consecutive failure limits.
func (c *Cycle) ShouldContinue(b *Budget) (bool, ExitReason) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.state.ConsecFailures >= maxConsecFailures {
		return false, ExitCritical
	}
	if b.Status() == BudgetExceeded {
		return false, ExitBudget
	}
	if c.state.MaxIter > 0 && c.state.Iteration >= c.state.MaxIter {
		return false, ExitMaxIter
	}
	return true, ""
}

// persistLocked writes state to disk atomically. Caller must hold c.mu.
func (c *Cycle) persistLocked() error {
	data, err := json.MarshalIndent(c.state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal loop state: %w", err)
	}
	dir := filepath.Dir(c.statePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".loop-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	tmp.Close()
	return os.Rename(tmpName, c.statePath)
}

// isValidTransition returns true if the transition from → to is allowed.
func isValidTransition(from, to Phase) bool {
	allowed, ok := validTransitions[from]
	if !ok {
		return false
	}
	for _, a := range allowed {
		if a == to {
			return true
		}
	}
	return false
}

// FormatStatus renders the current loop state as a human-readable string.
func FormatStatus(state LoopState) string {
	if state.RunID == "" {
		return "No active loop. Start one with: radiant loop start \"<goal>\"\n"
	}

	status := fmt.Sprintf("Run:   %s\n", state.RunID)
	status += fmt.Sprintf("Goal:  %s\n", state.Goal)
	status += fmt.Sprintf("Phase: %s\n", state.Phase)
	status += fmt.Sprintf("Iter:  %d / %d\n", state.Iteration, state.MaxIter)

	if state.ExitReason != "" {
		status += fmt.Sprintf("Exit:  %s\n", state.ExitReason)
	}
	if state.ConsecFailures > 0 {
		status += fmt.Sprintf("Fails: %d consecutive\n", state.ConsecFailures)
	}

	status += fmt.Sprintf("Since: %s\n", state.StartedAt.Format("2006-01-02 15:04 UTC"))

	if len(state.Log) > 0 {
		status += "\nRecent transitions:\n"
		start := len(state.Log) - 5
		if start < 0 {
			start = 0
		}
		for _, entry := range state.Log[start:] {
			note := ""
			if entry.Note != "" {
				note = " — " + entry.Note
			}
			status += fmt.Sprintf("  %s  %s → %s%s\n",
				entry.Timestamp.Format("15:04:05"),
				entry.From, entry.To, note)
		}
	}

	return status
}
