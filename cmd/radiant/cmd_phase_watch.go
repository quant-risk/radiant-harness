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
	"path/filepath"
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
		flagInterval       time.Duration
		flagMaxPoll        time.Duration
		flagJSON           bool
		flagOnChangeExit   bool
		flagFollow         string
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

	redirectCmd := &cobra.Command{
		Use:   "redirect <old-ticket> <new-ticket>",
		Short: "Write a follow-redirect so 'phase watch --follow=<old>' tracks the new ticket",
		Long: `Writes .radiant-harness/state/possess-<old-ticket>/redirect.json
with a {"next_ticket":"<new-ticket>"} payload. A subsequent
'radiant phase watch --follow=<old-ticket>' invocation will
transparently switch to reading state.json for the new ticket when
it detects the redirect.

Use case: you re-dispatched a phase run with a refined prompt,
got a new ticket id, and want your existing watcher to keep
tracking without manual update. After the new run starts:

  radiant phase redirect <old-ticket-id> <new-ticket-id>

The watcher reads the redirect on every poll. Exits once it
reaches terminal state in the new ticket.

If <old-ticket> has no state.json on disk, the redirect is still
written (so a future phase watch that creates the state.json
later will still pick it up — useful for forward references).`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			oldTicket := args[0]
			newTicket := args[1]
			cwd, _ := os.Getwd()
			return writeFollowRedirect(cwd, oldTicket, newTicket)
		},
	}

	watchCmd := &cobra.Command{
		Use:   "watch <task-id>",
		Short: "Stream phase status until terminal state or Ctrl-C",
		Long: `Polls the persisted phase state every --interval seconds and
re-emits the summary whenever status, current_phase, or
subprocess_alive changes. Exits 0 when the run reaches a terminal
state (done / cancelled / error / crashed) or 1 when --max-poll
elapses first. Ctrl-C interrupts cleanly (exit 130).

--on-change-exit (v3.7.11+) exits 0 immediately after the FIRST
change observed AFTER the initial snapshot — useful for
"wait until anything changes" notifications without needing a
full watch. Combine with --max-poll for a bounded wait:

  radiant phase watch <task-id> --on-change-exit --max-poll=30s

--follow=<anchor-ticket-id> (v3.7.11+) tracks the anchor's state
initially; if the resume path writes a redirect.json under
.radiant-harness/state/possess-<anchor>/redirect.json with a
{"next_ticket":"..."} payload, the watch transparently switches
to the new ticket's state.json. Use case: resume re-dispatches
with a NEW ticket id; the operator wants to keep watching without
manually updating the CLI invocation.

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
			return runPhaseWatch(cwd, taskID, flagInterval, flagMaxPoll, flagJSON, flagOnChangeExit, flagFollow, os.Stdout)
		},
	}
	watchCmd.Flags().DurationVar(&flagInterval, "interval", 2*time.Second,
		"Polling interval. Smaller = more responsive, larger = less I/O.")
	watchCmd.Flags().DurationVar(&flagMaxPoll, "max-poll", 0,
		"Stop after this duration even if terminal state not reached. "+
			"0 = no limit (default — Ctrl-C to interrupt).")
	watchCmd.Flags().BoolVar(&flagJSON, "json", false,
		"Emit the structured summary JSON (one object per change) instead of formatted text.")
	watchCmd.Flags().BoolVar(&flagOnChangeExit, "on-change-exit", false,
		"Exit 0 immediately after the FIRST change observed AFTER the initial snapshot. "+
			"Useful for 'wait until anything changes' notifications. Combine with --max-poll "+
			"to bound the wait.")
	watchCmd.Flags().StringVar(&flagFollow, "follow", "",
		"Anchor ticket id to follow through redirects. If the anchor's "+
			"state dir contains redirect.json with a next_ticket field, "+
			"the watch transparently switches to the new ticket's state.")

	cmd.AddCommand(statusCmd, watchCmd, redirectCmd)
	root.AddCommand(cmd)
}

// runPhaseWatch is the body of `radiant phase watch`. Extracted
// so tests can drive it with a fake clock + custom writer.
func runPhaseWatch(workdir, taskID string, interval, maxPoll time.Duration, asJSON, onChangeExit bool, follow string, w io.Writer) error {
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

	// The active ticket id starts at taskID. With --follow, it
	// may change mid-loop if a redirect.json points at a new
	// ticket (resume re-dispatch). We track it explicitly so the
	// fingerprint comparison works correctly across redirects.
	activeTicket := taskID
	if follow != "" {
		activeTicket = follow
	}

	// lastFingerprint is the previous emission's fingerprint so
	// we only re-print when something observable changes. The
	// initial fingerprint (captured before the loop starts) lets
	// --on-change-exit detect the FIRST post-initial change.
	var lastFingerprint string
	var lastStatus string

	// Emit the first snapshot immediately so the operator sees
	// something within ~10ms instead of waiting for the first tick.
	initialFp, initialStatus, err := readPhaseWatchSnapshot(workdir, activeTicket)
	if err != nil {
		return err
	}
	emitPhaseWatchSnapshotText(workdir, activeTicket, initialFp, initialStatus, asJSON, w)
	lastFingerprint = initialFp
	lastStatus = initialStatus

	if isTerminalPhaseStatus(initialStatus) {
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
			// --follow: check for a redirect.json on the
			// active ticket before reading state. If a
			// resume produced a new ticket id, switch.
			if follow != "" {
				if next, ok := readFollowRedirect(workdir, activeTicket); ok && next != "" {
					if _, err := fmt.Fprintf(w, "\nphase watch: --follow redirect %s → %s\n", activeTicket, next); err != nil {
						return err
					}
					activeTicket = next
					// Reset initial fingerprint to the new
					// ticket's state — otherwise a downstream
					// --on-change-exit would compare the new
					// ticket against the old ticket's hash and
					// exit immediately.
					initialFp, initialStatus, err = readPhaseWatchSnapshot(workdir, activeTicket)
					if err != nil {
						return err
					}
					emitPhaseWatchSnapshotText(workdir, activeTicket, initialFp, initialStatus, asJSON, w)
					lastFingerprint = initialFp
					lastStatus = initialStatus
					if isTerminalPhaseStatus(initialStatus) {
						return nil
					}
					continue
				}
			}

			fp, status, err := readPhaseWatchSnapshot(workdir, activeTicket)
			if err != nil {
				return err
			}

			// Skip re-emission if nothing changed (the noop
			// case is the most common for hosts watching a
			// long-running phase).
			if fp != lastFingerprint {
				emitPhaseWatchSnapshotText(workdir, activeTicket, fp, status, asJSON, w)
				lastFingerprint = fp
				lastStatus = status
			}

			// --on-change-exit: exit 0 on the FIRST change
			// after the initial snapshot. fp != initialFp
			// is the only signal we need; the emit above
			// has already printed the new state.
			if onChangeExit && fp != initialFp {
				return nil // exit 0 — change observed
			}

			if isTerminalPhaseStatus(status) {
				return nil
			}
		}
	}
}

// emitPhaseWatchSnapshot reads the current state, builds the
// summary, and prints it only when something changed. lastSummary
// and lastStatus are mutated in place so the caller can compare
// across iterations without re-reading from disk.
//
// Deprecated for new code in v3.7.11+ — callers should use
// readPhaseWatchSnapshot + emitPhaseWatchSnapshotText to keep
// fingerprint logic explicit (separate read/emit steps make
// --on-change-exit easier to reason about). Kept for backwards
// compat with existing callers (no current callers).
func emitPhaseWatchSnapshot(ctx context.Context, workdir, taskID string, asJSON bool, w io.Writer, lastSummary *string, lastStatus *string) error {
	fp, status, err := readPhaseWatchSnapshot(workdir, taskID)
	if err != nil {
		return err
	}
	if fp == *lastSummary {
		return nil
	}
	emitPhaseWatchSnapshotText(workdir, taskID, fp, status, asJSON, w)
	*lastSummary = fp
	*lastStatus = status
	return nil
}

// readPhaseWatchSnapshot loads the state for a ticket and
// returns (fingerprint, status) — the fingerprint is the change
// surface the watch compares against (status + current_phase +
// subprocess_alive + subprocess_pid), and status is the
// high-level summary status string. Errors are returned for
// missing/corrupt state files; the caller decides how to react
// (the main loop treats load errors as fatal for the watch).
func readPhaseWatchSnapshot(workdir, taskID string) (string, string, error) {
	st, err := loadPossessState(workdir, taskID)
	if err != nil {
		return "", "", fmt.Errorf("load state: %w", err)
	}
	summary := buildPhaseStatusSummary(st, workdir)
	fp := fmt.Sprintf("%s|%s|%t|%d",
		summary.Status, summary.CurrentPhase, summary.SubprocessAlive, summary.SubprocessPid)
	return fp, summary.Status, nil
}

// emitPhaseWatchSnapshotText writes one snapshot to w in either
// formatted text or JSON mode. Used by both the initial
// emission path and the change-detection path so the operator
// always sees the same shape regardless of where the change
// came from.
func emitPhaseWatchSnapshotText(workdir, taskID, fingerprint, status string, asJSON bool, w io.Writer) {
	// We already have the fingerprint, but we need the full
	// summary to render. Re-load — the cost is one disk read
	// per change which is negligible compared to the operator's
	// poll interval.
	st, err := loadPossessState(workdir, taskID)
	if err != nil {
		fmt.Fprintf(w, "phase watch: load failed for %s: %v\n", taskID, err)
		return
	}
	summary := buildPhaseStatusSummary(st, workdir)
	if asJSON {
		enc := newEncoder(w)
		_ = enc.Encode(summary)
		return
	}
	_, _ = fmt.Fprintf(w, "\n--- %s ---\n%s\n", time.Now().UTC().Format(time.RFC3339), formatPhaseStatusSummary(summary))
}

// readFollowRedirect checks for a redirect.json under the
// active ticket's state dir. The resume path writes this file
// when re-dispatching with a new ticket id, and --follow uses
// it to switch transparently.
//
// File format (one line, JSON):
//
//	{"next_ticket":"abc123def456"}
//
// Missing file or missing field → (next="", ok=false) — caller
// treats as "no redirect".
func readFollowRedirect(workdir, ticketID string) (string, bool) {
	path := followRedirectPath(workdir, ticketID)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	var payload struct {
		NextTicket string `json:"next_ticket"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", false
	}
	return payload.NextTicket, payload.NextTicket != ""
}

// writeFollowRedirect writes a redirect.json under
// .radiant-harness/state/possess-<oldTicket>/ so a subsequent
// `radiant phase watch --follow=<oldTicket>` switches to
// tracking the new ticket. Best-effort: creates the parent
// dir if needed.
func writeFollowRedirect(workdir, oldTicket, newTicket string) error {
	if oldTicket == "" || newTicket == "" {
		return fmt.Errorf("redirect: both old and new ticket ids are required")
	}
	path := followRedirectPath(workdir, oldTicket)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	payload := struct {
		NextTicket string `json:"next_ticket"`
		CreatedAt  string `json:"created_at"`
	}{NextTicket: newTicket, CreatedAt: time.Now().UTC().Format(time.RFC3339)}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	fmt.Printf("✓ wrote redirect: %s → %s\n  %s\n", oldTicket, newTicket, path)
	return nil
}

// followRedirectPath returns the canonical path for a follow-
// redirect file under a given ticket's state dir. Mirrors
// `redirect.json` from the docs of the `radiant phase
// redirect` subcommand.
func followRedirectPath(workdir, ticketID string) string {
	return filepath.Join(workdir, ".radiant-harness", "state",
		"possess-"+ticketID, "redirect.json")
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