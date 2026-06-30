// cmd_fleet_async_runner.go — the detached subprocess entry
// point for the fleet async gate (v3.7.9). Mirrors
// `radiant async-runner` for the loop.
//
// Wire contract:
//
//   radiant fleet-async-runner <run-id> [-- <extra-args-joined-by-\x1f>]
//
// When RADIANT_FLEET_ASYNC_SUBPROCESS=1 is set in the parent
// environment, `Dispatcher.RunAll` forks this subcommand and
// returns immediately. The child process:
//
//  1. Validates that RADIANT_FLEET_ASYNC_RUNNER=1 is set in
//     its environment (defense-in-depth against accidentally
//     invoking the runner inline).
//  2. Writes `.radiant-harness/fleet/pids/dispatcher-<runID>.pid`
//     so the parent can liveness-probe it via
//     `mcp__radiant__fleet_status`.
//  3. Re-invokes `Dispatcher.RunAll` with AsyncSubprocess=false
//     (inline). The dispatcher writes per-task pid files via
//     `spawnAgent`.
//  4. Removes the dispatcher pid file on exit.
//
// Exit 0 on success, non-zero on dispatch failure. The parent
// dispatcher does not wait for the child (cmd.Wait runs in a
// goroutine for cleanup); the pid file is the source of truth.

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/quant-risk/radiant-harness/v3/internal/fleet"
	"github.com/spf13/cobra"
)

// runFleetAsyncRunner is the body of `radiant fleet-async-runner`.
// Extracted so tests can call it directly without re-forking the
// binary — the dispatcher writes pid files exactly the same way
// regardless of who calls it.
func runFleetAsyncRunner(runID string) error {
	workdir, _ := os.Getwd()

	store, err := fleet.LoadStore(workdir, runID)
	if err != nil {
		return fmt.Errorf("load fleet %q: %w", runID, err)
	}

	// Re-invoke the dispatcher inline. The `AsyncSubprocess: false`
	// field is critical — without it, the dispatcher would try to
	// fork itself again, leading to infinite recursion.
	cfg := fleet.DispatchConfig{
		Workdir:         workdir,
		AsyncSubprocess: false,
	}

	iso, err := fleet.NewIsolator(store, workdir)
	if err != nil {
		return fmt.Errorf("isolator: %w", err)
	}
	disp, err := fleet.NewDispatcher(iso, cfg)
	if err != nil {
		return fmt.Errorf("dispatcher: %w", err)
	}

	start := time.Now()
	results, err := disp.RunAll(context.Background(), nil)
	elapsed := time.Since(start)
	if err != nil {
		fmt.Fprintf(os.Stderr, "radiant fleet-async-runner: dispatch failed in %s: %v\n", elapsed.Round(time.Millisecond), err)
		return err
	}
	done, failed := 0, 0
	for _, r := range results {
		if r.Err == nil && r.ExitCode == 0 {
			done++
		} else {
			failed++
		}
	}
	fmt.Fprintf(os.Stderr, "radiant fleet-async-runner: %s — %d done, %d failed in %s\n",
		runID, done, failed, elapsed.Round(time.Millisecond))
	return nil
}

func registerFleetAsyncRunnerCmd(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:    "fleet-async-runner",
		Short: "Internal worker primitive for the fleet async subprocess path (v3.7.9+)",
		Long: `Forks a fleet dispatch as a detached subprocess. Wired
only when RADIANT_FLEET_ASYNC_SUBPROCESS=1 is set in the parent
'radiant mcp serve' environment. Not intended for direct
invocation — the parent dispatcher spawns this command via
os/exec with a flat argv:

  radiant fleet-async-runner <run-id>

Exit 0 on success, non-zero on dispatch failure.`,
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runID := args[0]
			workdir, _ := os.Getwd()

			// Defense-in-depth: refuse to run if the parent
			// didn't tag the env. This prevents an operator who
			// manually invokes `radiant fleet-async-runner ...`
			// from accidentally creating a pid file that
			// `mcp__radiant__fleet_status` would then probe.
			if os.Getenv("RADIANT_FLEET_ASYNC_RUNNER") != "1" {
				return fmt.Errorf("fleet-async-runner must be invoked via the parent dispatcher (RADIANT_FLEET_ASYNC_RUNNER=1 missing in env)")
			}

			// Write the dispatcher pid file before doing real
			// work so a follow-up `mcp__radiant__fleet_status`
			// can liveness-probe even if dispatch is slow.
			if err := fleet.WriteDispatcherPid(workdir, runID, os.Getpid()); err != nil {
				fmt.Fprintf(os.Stderr, "warning: write dispatcher pid: %v\n", err)
			}
			defer fleet.RemoveDispatcherPid(workdir, runID)

			return runFleetAsyncRunner(runID)
		},
	}
	root.AddCommand(cmd)
}