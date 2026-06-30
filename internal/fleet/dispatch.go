package fleet

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/quant-risk/radiant-harness/v3/internal/worktree"
)

// retryBackoff returns the wait duration for attempt n (0-based) using
// exponential backoff: 2^n seconds, capped at 60s.
func retryBackoff(n int) time.Duration {
	d := time.Duration(1<<uint(n)) * time.Second
	if d > 60*time.Second {
		d = 60 * time.Second
	}
	return d
}

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

	// MaxConcurrency caps the number of agent processes running simultaneously.
	// 0 = unlimited (all tasks start in parallel).
	MaxConcurrency int

	// MaxRetries is the number of automatic retries per task on transient failure
	// (non-zero exit when the process started successfully). 0 = no retries.
	MaxRetries int

	// AsyncSubprocess makes RunAll fork a detached subprocess
	// (`radiant fleet-async-runner <run-id>`) instead of running the
	// dispatch inline. The subprocess writes
	// `.radiant-harness/fleet/pids/dispatcher-<runID>.pid` and runs
	// to completion (or until MaxConcurrency/MaxRetries/Timeout
	// limits are reached); the caller returns immediately with a
	// (runID, dispatcherPid) pair and can poll status via
	// `mcp__radiant__fleet_status`.
	//
	// Mirrors v3.7.7's loop subprocess gate for `radiant_run_gate`.
	// Opt-in only — inline is the default and faster for small
	// fleets. Turn this on when a real cross-process need
	// reproduces (CI host with a hard MCP tool-call deadline, or
	// a fleet that takes longer than the caller's wait window).
	AsyncSubprocess bool

	// Workdir is the project root the dispatcher should write pid
	// files into. Defaults to os.Getwd() if empty. Required for
	// pid-file tracking to function.
	Workdir string
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
	if cfg.Workdir == "" {
		cfg.Workdir, _ = os.Getwd()
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
//
// If cfg.AsyncSubprocess is true, RunAll instead forks a single detached
// subprocess (`<binary> fleet-async-runner <run-id>`), writes a pid
// file at `.radiant-harness/fleet/pids/dispatcher-<runID>.pid`, and
// returns immediately with an empty results slice. The subprocess
// re-invokes RunAll inline. Status can be polled via the
// `mcp__radiant__fleet_status` MCP tool.
func (d *Dispatcher) RunAll(ctx context.Context, extraArgs []string) ([]AgentResult, error) {
	if d.cfg.AsyncSubprocess {
		return d.runAsyncSubprocess(ctx, extraArgs)
	}

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

	// Semaphore limits concurrent agent processes when MaxConcurrency > 0.
	var sem chan struct{}
	if d.cfg.MaxConcurrency > 0 {
		sem = make(chan struct{}, d.cfg.MaxConcurrency)
	}

	for i, c := range tasks {
		wg.Add(1)
		go func(idx int, task *Task, wt worktree.Worktree) {
			defer wg.Done()
			if sem != nil {
				sem <- struct{}{}
				defer func() { <-sem }()
			}
			result := d.spawnAgent(ctx, task, wt, extraArgs)

			// Auto-retry on transient failure (process started but exited non-zero).
			for attempt := 0; attempt < d.cfg.MaxRetries && result.Err == nil && result.ExitCode != 0; attempt++ {
				select {
				case <-ctx.Done():
					break
				case <-time.After(retryBackoff(attempt)):
				}
				result = d.spawnAgent(ctx, task, wt, extraArgs)
			}

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

// runAsyncSubprocess forks a detached `radiant fleet-async-runner`
// subprocess that re-invokes RunAll inline. The parent (this
// process) returns immediately. Pid tracking lives in the child —
// it writes `dispatcher-<runID>.pid` at startup and removes it
// on exit.
//
// Returns (nil, nil) on successful fork — the caller has no
// results to surface yet. To observe progress, call
// `mcp__radiant__fleet_status`.
func (d *Dispatcher) runAsyncSubprocess(ctx context.Context, extraArgs []string) ([]AgentResult, error) {
	runID := d.iso.store.Snapshot().RunID
	if runID == "" {
		return nil, fmt.Errorf("async subprocess: empty run id")
	}

	// Build the args the child will receive. The child re-runs
	// the dispatch path but with AsyncSubprocess=false to avoid
	// infinite recursion.
	args := []string{"fleet-async-runner", runID}
	if len(extraArgs) > 0 {
		args = append(args, "--", strings.Join(extraArgs, "\x1f"))
	}

	cmd := exec.CommandContext(ctx, d.cfg.Binary, args...)
	cmd.Dir = d.cfg.Workdir

	env := d.cfg.Env
	if env == nil {
		env = os.Environ()
	}
	env = append(env,
		"RADIANT_FLEET_ASYNC_RUNNER=1",
		fmt.Sprintf("RADIANT_RUN_ID=%s", runID),
	)
	cmd.Env = env

	// Detach stdio so the child doesn't inherit our TTY.
	if d.cfg.Stdout != nil {
		cmd.Stdout = d.cfg.Stdout
	}
	if d.cfg.Stderr != nil {
		cmd.Stderr = d.cfg.Stderr
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("fork fleet-async-runner: %w", err)
	}

	// Best-effort dispatcher pid write so a follow-up
	// `mcp__radiant__fleet_status` can liveness-probe the child.
	// The child ALSO writes its own pid file when it boots; if
	// the parent races ahead, the child's write wins (same
	// value). If the child crashes before booting, the parent's
	// write gives the operator a real pid to inspect.
	if err := ensureFleetPidDir(d.cfg.Workdir); err == nil {
		_ = writePidFile(dispatcherPidPath(d.cfg.Workdir, runID), cmd.Process.Pid)
	}

	// Release the child — we don't wait. The child cleans up its
	// own pid file on exit.
	go func() {
		_ = cmd.Wait()
	}()

	return nil, nil
}

// ResumeAll re-dispatches all failed tasks, leaving done/pending tasks untouched.
// It resets each failed task back to pending, then calls RunAll so the normal
// claim-isolate-spawn path handles them.
func (d *Dispatcher) ResumeAll(ctx context.Context, extraArgs []string) ([]AgentResult, error) {
	// Reset failed tasks to pending so RunAll can claim them.
	snap := d.iso.store.Snapshot()
	for _, t := range snap.Tasks {
		if t.Status == TaskFailed {
			if err := d.iso.store.ResetTask(t.ID); err != nil {
				return nil, fmt.Errorf("reset task %q: %w", t.ID, err)
			}
		}
	}
	return d.RunAll(ctx, extraArgs)
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
	runID := d.iso.store.Snapshot().RunID
	env = append(env,
		fmt.Sprintf("RADIANT_WORKTREE_DIR=%s", wt.Path),
		fmt.Sprintf("RADIANT_AGENT_ID=agent-%s", task.ID),
		fmt.Sprintf("RADIANT_TASK_ID=%s", task.ID),
		fmt.Sprintf("RADIANT_RUN_ID=%s", runID),
	)
	cmd.Env = env

	if d.cfg.Stdout != nil {
		cmd.Stdout = d.cfg.Stdout
	}
	if d.cfg.Stderr != nil {
		cmd.Stderr = d.cfg.Stderr
	}

	// Write the agent's pid file before Start so a follow-up
	// `mcp__radiant__fleet_status` can liveness-probe the child
	// while it's running. The file is removed via defer once
	// cmd.Run returns (success, timeout, or non-zero exit).
	//
	// Best-effort: a failed write is logged but does NOT abort
	// the agent — the spawn is the primary work, the pid file
	// is just a probe handle. Same policy as the loop's
	// `writeAsyncRunnerPid`.
	pidPath := taskPidPath(d.cfg.Workdir, runID, task.ID)
	if err := writePidFile(pidPath, 0); err == nil {
		// pre-create empty so the path exists; we'll overwrite
		// with the real pid after Start. Some status probes
		// distinguish "no file" (task not started) from "empty
		// file" (task in transition) — keep the path consistent.
		_ = removePidFile(pidPath)
	}

	if err := cmd.Start(); err != nil {
		return AgentResult{
			AgentID:  fmt.Sprintf("agent-%s", task.ID),
			TaskID:   task.ID,
			ExitCode: -1,
			Err:      fmt.Errorf("start: %w", err),
			Elapsed:  time.Since(started),
		}
	}

	// Now that we have a real pid, write it.
	_ = writePidFile(pidPath, cmd.Process.Pid)
	defer removePidFile(pidPath)

	err := cmd.Wait()
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
