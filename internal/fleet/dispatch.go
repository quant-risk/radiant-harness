package fleet

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/quant-risk/radiant-harness/internal/worktree"
)

// DispatchConfig controls how a fleet of agent processes is launched.
type DispatchConfig struct {
	// Binary is the path to the radiant executable. Defaults to os.Executable().
	Binary string

	// Env is the environment passed to each agent process.
	// If nil, the current process environment is inherited.
	Env []string

	// Stdout / Stderr receive merged output from all agents.
	// If nil, output is discarded.
	Stdout io.Writer
	Stderr io.Writer

	// Timeout per agent process. 0 = no timeout.
	Timeout time.Duration
}

// AgentResult is the outcome of a single spawned agent process.
type AgentResult struct {
	AgentID  string
	TaskID   string
	ExitCode int
	Err      error
	Elapsed  time.Duration
}

// Dispatcher spawns real OS processes — one per fleet task — inside
// dedicated git worktrees. It coordinates with the Isolator so each
// process gets an isolated checkout and updates task state on completion.
type Dispatcher struct {
	iso *Isolator
	cfg DispatchConfig
}

// NewDispatcher creates a Dispatcher.
func NewDispatcher(iso *Isolator, cfg DispatchConfig) (*Dispatcher, error) {
	if cfg.Binary == "" {
		self, err := os.Executable()
		if err != nil {
			return nil, fmt.Errorf("resolve binary: %w", err)
		}
		cfg.Binary = self
	}
	return &Dispatcher{iso: iso, cfg: cfg}, nil
}

// RunAll claims all pending tasks from the store, spawns one process per
// task in its own worktree, and waits for all to complete. Results are
// collected and returned in completion order (not submission order).
//
// Each agent process runs:
//
//	<binary> loop start <task.DoneWhen> [extra args...]
//
// with RADIANT_WORKTREE_DIR=<worktree.Path> injected into the environment.
// The agent writes its own loop state into the worktree's
// .radiant-harness/ directory, keeping the main repo untouched.
func (d *Dispatcher) RunAll(ctx context.Context, extraArgs []string) ([]AgentResult, error) {
	type claimed struct {
		task *Task
		wt   worktree.Worktree
	}

	// Claim all pending tasks upfront so we know the total count.
	var tasks []claimed
	for {
		task, wt, err := d.iso.ClaimIsolated("fleet-dispatcher")
		if err != nil {
			return nil, fmt.Errorf("claim task: %w", err)
		}
		if task == nil {
			break // no more pending tasks
		}
		tasks = append(tasks, claimed{task, wt})
	}

	if len(tasks) == 0 {
		return nil, nil
	}

	results := make([]AgentResult, len(tasks))
	var wg sync.WaitGroup

	for i, c := range tasks {
		wg.Add(1)
		go func(idx int, task *Task, wt worktree.Worktree) {
			defer wg.Done()
			result := d.spawnAgent(ctx, task, wt, extraArgs)
			results[idx] = result

			// Update task state in the store based on process exit.
			success := result.Err == nil && result.ExitCode == 0
			evidence := fmt.Sprintf("process exited %d in %s", result.ExitCode, result.Elapsed.Round(time.Millisecond))
			if result.Err != nil {
				evidence = result.Err.Error()
			}
			_ = d.iso.store.CompleteTask(task.ID, evidence, success)

			// Release the worktree after the agent finishes.
			_ = d.iso.Release(wt, true)
		}(i, c.task, c.wt)
	}

	wg.Wait()
	return results, nil
}

// spawnAgent runs a single agent process for the given task in its worktree.
func (d *Dispatcher) spawnAgent(ctx context.Context, task *Task, wt worktree.Worktree, extraArgs []string) AgentResult {
	started := time.Now()

	agentCtx := ctx
	if d.cfg.Timeout > 0 {
		var cancel context.CancelFunc
		agentCtx, cancel = context.WithTimeout(ctx, d.cfg.Timeout)
		defer cancel()
	}

	// Build command: <binary> loop start "<task.DoneWhen>" [extraArgs...]
	cmdArgs := append([]string{"loop", "start", task.DoneWhen}, extraArgs...)
	cmd := exec.CommandContext(agentCtx, d.cfg.Binary, cmdArgs...)

	// Run in the isolated worktree.
	cmd.Dir = wt.Path

	// Environment: inherit + agent-specific vars.
	env := d.cfg.Env
	if env == nil {
		env = os.Environ()
	}
	env = append(env,
		fmt.Sprintf("RADIANT_WORKTREE_DIR=%s", wt.Path),
		fmt.Sprintf("RADIANT_AGENT_ID=agent-%s", task.ID),
		fmt.Sprintf("RADIANT_TASK_ID=%s", task.ID),
	)
	cmd.Env = env

	if d.cfg.Stdout != nil {
		cmd.Stdout = d.cfg.Stdout
	}
	if d.cfg.Stderr != nil {
		cmd.Stderr = d.cfg.Stderr
	}

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
			err = nil // non-zero exit is not a dispatch error
		}
	}

	return AgentResult{
		AgentID:  fmt.Sprintf("agent-%s", task.ID),
		TaskID:   task.ID,
		ExitCode: exitCode,
		Err:      err,
		Elapsed:  time.Since(started),
	}
}
