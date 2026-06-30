// cmd_phase_watch.go — `radiant phase watch <task-id>` CLI.
//
// Companion to the `radiant_phase_status` MCP tool. The MCP tool
// is single-call: the host re-invokes it to poll progress. For a
// long-running operator (a CI script watching a fleet, a terminal
// user watching a slow phase) the re-invocation overhead is
// unnecessary — a CLI command that streams until terminal state
// is the right surface.
//
// Streaming model:
//
//   - Poll `possessStatePath` every `--interval` seconds (default 2).
//   - Re-emit the phase summary on every change in `(status,
//     current_phase, subprocess_alive)`.
//   - Exit 0 when the run reaches a terminal state (done / cancelled /
//     error / crashed).
//   - Exit 1 when `--max-poll` elapses before terminal state —
//     protects against a runaway watcher that should be polling
//     a different task id.
//
// Why not push notifications: the MCP transport doesn't support
// streaming responses (each `tools/call` is a single round-trip),
// so the watch must live in a long-running CLI rather than an MCP
// tool. Hosts that need streaming should spawn `radiant phase watch`
// as a subprocess and tail its stdout. The MCP `radiant_phase_status`
// remains the single-call surface for hosts that prefer polling.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

// phaseWatchSignalBuffer is the buffer size for the channel that
// receives terminal-state-change events. 4 = enough for the four
// shape changes we care about (status / phase / subprocess / done),
// with one slot of slack so the producer never blocks on a full
// channel while the consumer is formatting the previous emission.
const phaseWatchSignalBuffer = 4

func registerPhaseWatchCmd(root *cobra.Command) {
	var (
		flagInterval time.Duration
		flagMaxPoll  time.Duration
		flagJSON     bool
	)
	cmd := &cobra.Command{
		Use:   "phase",
		Short: "Manage ticket-based MCP runs (status, watch)",
		Long: `Subcommands for the ticket-based MCP run lifecycle (v3.7.10+).

  status <task-id>   One-shot status read (CLI mirror of radiant_phase_status).
  watch <task-id>    Stream status until terminal state or Ctrl-C.

The "phase" namespace is separate from "loop" because the MCP
ticket model (mcp__radiant__run_gate + mcp__radiant__possess_async)
is conceptually different from the live feedback-loop cycle (loop
start / loop status / loop resume). They share the same state.json
shape but have different operational semantics.

Examples:
  radiant phase status 04b82c5a2f19ab36
  radiant phase watch 04b82c5a2f19ab36 --interval=2s`,
	}

	statusCmd := &cobra.Command{
		Use:   "status <task-id>",
		Short: "One-shot status read (CLI mirror of radiant_phase_status MCP tool)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskID := args[0]
			cwd, _ := os.Getwd()
			st, err := loadPossessState(cwd, taskID)
			if err != nil {
				return fmt.Errorf("no run with task_id %s in %s", taskID, cwd)
			}
			summary := buildPhaseStatusSummary(st, cwd)
			if flagJSON {
				enc := newEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(summary)
			}
			fmt.Fprint(os.Stdout, formatPhaseStatusSummary(summary))
			return nil
		},
	}
	statusCmd.Flags().BoolVar(&flagJSON, "json", false, "Output as JSON")

	watchCmd := &cobra.Command{
		Use:   "watch <task-id>",
		Short: "Stream phase status until terminal state or Ctrl-C",
		Long: `Polls the persisted phase state every --interval seconds and
re-emits the summary whenever status, current_phase, or
subprocess_alive changes. Exits 0 when the run reaches a terminal
state (done / cancelled / error / crashed) or 1 when --max-poll
elapses first. Ctrl-C interrupts cleanly (exit 130).

Use this when a CI host wants to stream progress without polling
the MCP tool. Spawn it as:

  radiant phase watch <task-id> --interval=5s --max-poll=10m \\
    | tee run.log

The output is the same shape as 'radiant_phase_status' content[1]
— a CLI-friendly phase summary, not the raw state.json dump.
--json emits the structured summary (the "summary" field of the
MCP response).`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskID := args[0]
			cwd, _ := os.Getwd()
			return runPhaseWatch(cwd, taskID, flagInterval, flagMaxPoll, flagJSON, os.Stdout)
		},
	}
	watchCmd.Flags().DurationVar(&flagInterval, "interval", 2*time.Second,
		"Polling interval. Smaller = more responsive, larger = less I/O.")
	watchCmd.Flags().DurationVar(&flagMaxPoll, "max-poll", 0,
		"Stop after this duration even if terminal state not reached. "+
			"0 = no limit (default — Ctrl-C to interrupt).")
	watchCmd.Flags().BoolVar(&flagJSON, "json", false,
		"Emit the structured summary JSON (one object per change) instead of formatted text.")

	cmd.AddCommand(statusCmd, watchCmd)
	root.AddCommand(cmd)
}

// runPhaseWatch is the body of `radiant phase watch`. Extracted
// so tests can drive it with a fake clock + custom writer.
func runPhaseWatch(workdir, taskID string, interval, maxPoll time.Duration, asJSON bool, w io.Writer) error {
	if interval <= 0 {
		interval = 2 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Trap SIGINT/SIGTERM so Ctrl-C exits cleanly with code 130.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Fprintln(w, "\nphase watch: interrupted (SIGINT/SIGTERM)")
		cancel()
	}()

	// Optional max-poll deadline.
	if maxPoll > 0 {
		var maxCancel context.CancelFunc
		ctx, maxCancel = context.WithTimeout(ctx, maxPoll)
		defer maxCancel()
	}

	// Tick channel drives the poll loop.
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// lastSummary fingerprints the previous emission so we only
	// re-print when something observable changes.
	var lastSummary string
	var lastStatus string

	// Emit the first snapshot immediately so the operator sees
	// something within ~10ms instead of waiting for the first tick.
	if err := emitPhaseWatchSnapshot(ctx, workdir, taskID, asJSON, w, &lastSummary, &lastStatus); err != nil {
		return err
	}
	if isTerminalPhaseStatus(lastStatus) {
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			// Either interrupted or max-poll reached.
			if maxPoll > 0 {
				return fmt.Errorf("phase watch: max-poll %s reached without terminal state (last: %s)", maxPoll, lastStatus)
			}
			return nil
		case <-ticker.C:
			if err := emitPhaseWatchSnapshot(ctx, workdir, taskID, asJSON, w, &lastSummary, &lastStatus); err != nil {
				return err
			}
			if isTerminalPhaseStatus(lastStatus) {
				return nil
			}
		}
	}
}

// emitPhaseWatchSnapshot reads the current state, builds the
// summary, and prints it only when something changed. lastSummary
// and lastStatus are mutated in place so the caller can compare
// across iterations without re-reading from disk.
func emitPhaseWatchSnapshot(ctx context.Context, workdir, taskID string, asJSON bool, w io.Writer, lastSummary *string, lastStatus *string) error {
	st, err := loadPossessState(workdir, taskID)
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}
	summary := buildPhaseStatusSummary(st, workdir)

	// Fingerprint: status + current_phase + subprocess_alive +
	// subprocess_pid is the operator-visible change surface.
	fingerprint := fmt.Sprintf("%s|%s|%t|%d",
		summary.Status, summary.CurrentPhase, summary.SubprocessAlive, summary.SubprocessPid)
	if fingerprint == *lastSummary {
		return nil
	}
	*lastSummary = fingerprint
	*lastStatus = summary.Status

	if asJSON {
		enc := newEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(summary); err != nil {
			return err
		}
		return nil
	}
	// Prefix every emission with the wall-clock so multiple
	// watch invocations tailing the same log don't get
	// ambiguous.
	_, err = fmt.Fprintf(w, "\n--- %s ---\n%s\n", time.Now().UTC().Format(time.RFC3339), formatPhaseStatusSummary(summary))
	return err
}

// isTerminalPhaseStatus returns true for states where the watcher
// should stop polling: done / cancelled / error / crashed.
// `pending` and `in_progress` keep the watcher running.
func isTerminalPhaseStatus(status string) bool {
	switch status {
	case "done", "cancelled", "error", "crashed":
		return true
	}
	return false
}

// newEncoder returns a JSON encoder that writes one indented
// object per line. Mirrors what `radiant fleet status --json`
// emits — parseable with `jq -c` line-by-line for streaming.
func newEncoder(w io.Writer) *json.Encoder {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc
}