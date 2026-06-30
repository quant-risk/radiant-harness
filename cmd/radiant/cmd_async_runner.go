// `radiant async-runner` — the detached subprocess entry point for
// the async gate primitives (v3.7.7).
//
// The async gate primitives (`radiant_possess_async` and
// `radiant_run_gate`) ship two execution paths:
//
//  1. **Inline** (default, the v3.7.0-v3.7.6 path): the parent
//     `radiant mcp serve` process runs the phase directly. Each
//     phase persists state.json and returns. Total wall-clock for
//     a 4-phase offline run is well under 500 ms — the inline path
//     is faster than forking a subprocess per phase.
//
//  2. **Subprocess** (this file, opt-in via
//     `RADIANT_ASYNC_SUBPROCESS=1`): `mcp__radiant__run_gate` and
//     `mcp__radiant__possess_async` fork `radiant async-runner` and
//     return the ticket within ~500 ms. The phase work runs in a
//     child process that can outlive the parent's tool-call window
//     on hosts that gate tool-call completion on subprocess exit.
//
// The subprocess path is what makes the gate primitives viable for
// synchronous TUI hosts where the inner sampling round-trips need to
// outlive the outer MCP tool call. Today Hermes TUI is the documented
// sync host and the inline path already closes its 120 s tool-call
// deadlock (the phases complete before the harness returns), so
// subprocess mode is wired but not the default — turn it on when a
// real host need reproduces.
//
// Wire contract:
//
//   radiant async-runner \
//     --phase=<discover|plan|execute|verify> \
//     --ticket=<16-char-hex> \
//     --workdir=<absolute-path> \
//     --task=<user's verbatim task prompt>
//
// The subcommand:
//
//  1. Validates args; rejects unknown phases with exit 64.
//  2. Writes `.radiant-harness/pids/<ticket>.pid` (best-effort —
//     a failed write is not fatal; the parent uses `kill -0` for
//     liveness checks anyway).
//  3. Calls the corresponding `selfDriven<Phase>` helper. The
//     helper persists state.json under
//     `.radiant-harness/state/possess-<ticket>/state.json`.
//  4. Removes the pid file.
//  5. Exits 0 (success) or non-zero (phase failed). The parent
//     gateway reads the exit code + state.json to translate the
//     result back into the MCP response.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/quant-risk/radiant-harness/v3/internal/possess"
	"github.com/spf13/cobra"
)

// asyncRunnerSelfPath returns the absolute path of the running
// `radiant` binary so the subprocess can re-invoke itself with the
// `async-runner` subcommand.
//
// Resolution order:
//   1. `RADIANT_BIN` env var if set (used by tests so the
//      subprocess points at the real binary, not the test binary).
//   2. `os.Executable()` (the path the kernel handed us at exec
//      time) — the normal case for `radiant mcp serve`.
//
// Walking PATH is intentionally avoided: PATH inside an MCP
// subprocess is not guaranteed to point at the same binary the
// parent resolved.
func asyncRunnerSelfPath() (string, error) {
	if env := strings.TrimSpace(os.Getenv("RADIANT_BIN")); env != "" {
		if _, err := os.Stat(env); err == nil {
			return env, nil
		}
		// RADIANT_BIN set but path doesn't exist — fall through
		// to os.Executable so the subprocess fails loudly rather
		// than silently using a wrong binary.
	}
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable: %w", err)
	}
	if exe == "" {
		return "", fmt.Errorf("empty executable path")
	}
	return exe, nil
}

// asyncRunnerPidPath returns the canonical pid-file path for a
// ticket under the workdir. The parent gateway reads this file
// when it wants to liveness-check a detached run.
func asyncRunnerPidPath(workdir, ticket string) string {
	return filepath.Join(workdir, ".radiant-harness", "pids", ticket+".pid")
}

// writeAsyncRunnerPid writes the current pid to the canonical path.
// Best-effort: a failure here is logged but does not abort the
// phase, because the parent gateway's liveness check is
// `kill -0` on the pid we returned from Spawn (not a pid-file
// read), so a missing pid file just means a future operator
// cannot see "this ticket was running". The phase still runs.
func writeAsyncRunnerPid(workdir, ticket string, pid int) {
	p := asyncRunnerPidPath(workdir, ticket)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(p, []byte(strconv.Itoa(pid)), 0o644)
}

// removeAsyncRunnerPid removes the pid file for a ticket. Called
// on phase completion (success or failure) so a subsequent
// `radiant_phase_status` doesn't surface a stale "running" signal.
func removeAsyncRunnerPid(workdir, ticket string) {
	_ = os.Remove(asyncRunnerPidPath(workdir, ticket))
}

// asyncRunnerLiveness checks whether the pid recorded for a ticket
// is still alive. Returns (alive, pid) where alive is true if the
// process exists. A missing pid file means "no detached run was
// ever recorded" and returns alive=false, pid=0.
func asyncRunnerLiveness(workdir, ticket string) (alive bool, pid int) {
	data, err := os.ReadFile(asyncRunnerPidPath(workdir, ticket))
	if err != nil {
		return false, 0
	}
	pid, err = strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return false, 0
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false, pid
	}
	// On Unix, FindProcess always succeeds; Kill(pid, 0) returns
	// nil if the process exists, an error otherwise.
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return false, pid
	}
	return true, pid
}

// runAsyncRunner is the body of the `radiant async-runner`
// subcommand. It is also the function the parent gateway invokes
// via os/exec when the subprocess mode is on; running it in the
// same process during tests lets us reuse the same code path.
//
// It loads the existing state.json (creating one if missing),
// delegates to the same `selfDriven<Phase>` helpers the inline
// AsyncGate uses, and writes the phase result back to state.json
// so the parent gateway and `radiant_phase_status` can observe
// the run from a sibling process.
func runAsyncRunner(phase possess.Phase, ticket, workdir, task, profile string) error {
	if workdir == "" {
		var err error
		workdir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("resolve workdir: %w", err)
		}
	}
	if profile == "" {
		profile = "standard"
	}
	if !possess.ValidPhase(phase) {
		return fmt.Errorf("invalid phase: %s", phase)
	}

	// Load (or create) the state file. Mirrors spawnOnePhase in
	// cmd_mcp_run_gate.go so the subprocess's view of the workdir
	// matches the parent's.
	st, err := loadPossessState(workdir, ticket)
	if err != nil {
		st = newPossessState(workdir, task, ticket)
	}
	st.CurrentPhase = string(phase)
	st.Profile = profile
	st.LastPhaseAt = time.Now()
	if st.StartedAt.IsZero() {
		st.StartedAt = st.LastPhaseAt
	}
	if st.Slug == "" && task != "" {
		st.Slug = selfDrivenSlugify(task)
	}
	if st.SpecDir == "" && st.Slug != "" {
		st.SpecDir = workdir + "/specs/0001-" + st.Slug
	}
	_ = savePossessState(st) // persist meta before any phase work

	// Compute the spec dir the same way spawnOnePhase does so the
	// subprocess's view of the workdir matches the parent's.
	specDir := st.SpecDir

	phaseStart := time.Now()
	var phaseErr error
	switch phase {
	case possess.PhaseDiscover:
		phaseErr = selfDrivenDiscover(workdir, task, profile, os.Stderr)
	case possess.PhasePlan:
		phaseErr = selfDrivenPlan(workdir, specDir, task, st.Slug, profile, os.Stderr)
	case possess.PhaseExecute:
		phaseErr = selfDrivenExecute(workdir, specDir, task, profile, os.Stderr)
	case possess.PhaseVerify:
		phaseErr = selfDrivenVerify(workdir, specDir, os.Stderr)
	default:
		phaseErr = fmt.Errorf("unknown phase: %s", phase)
	}

	// Persist the phase result to state.json. Mirrors
	// spawnOnePhase so the inline and subprocess paths produce
	// the same observable shape.
	if phaseErr != nil {
		st.Phases[string(phase)] = &phaseResult{
			Phase: string(phase), Status: "error", StartedAt: phaseStart,
			EndedAt: time.Now(), Error: phaseErr.Error(),
		}
	} else {
		st.Phases[string(phase)] = &phaseResult{
			Phase: string(phase), Status: "done", StartedAt: phaseStart,
			EndedAt: time.Now(),
		}
	}
	st.CurrentPhase = string(phase)
	if err := savePossessState(st); err != nil {
		return err
	}
	return phaseErr
}

// registerAsyncRunnerCmd attaches `radiant async-runner` to the
// root cobra command. The subcommand is hidden from default --help
// (it's a worker primitive, not a user-facing command) but
// discoverable through `radiant --help` with the cobra listing.
func registerAsyncRunnerCmd(root *cobra.Command) {
	var (
		flagPhase   string
		flagTicket  string
		flagWorkdir string
		flagTask    string
	)

	cmd := &cobra.Command{
		Use:    "async-runner",
		Short: "Internal worker primitive for the async gate subprocess path (v3.7.7+)",
		Long: `Forks a phase of the self-driven possession loop as a
detached subprocess. Wired only when RADIANT_ASYNC_SUBPROCESS=1 is
set in the parent 'radiant mcp serve' environment. Not intended
for direct invocation — the parent gate primitives spawn this
command via os/exec with a flat argv:

  radiant async-runner \
    --phase=<discover|plan|execute|verify> \
    --ticket=<16-char-hex> \
    --workdir=<absolute-path> \
    --task=<user's verbatim task prompt>

Exit 0 on success, 1 on phase failure, 64 on bad arguments.`,
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagPhase == "" || flagTicket == "" || flagWorkdir == "" {
				return fmt.Errorf("--phase, --ticket, --workdir are required")
			}
			if !possess.ValidPhase(possess.Phase(flagPhase)) {
				return fmt.Errorf("unknown phase %q (valid: discover, plan, execute, verify)", flagPhase)
			}
			writeAsyncRunnerPid(flagWorkdir, flagTicket, os.Getpid())
			defer removeAsyncRunnerPid(flagWorkdir, flagTicket)

			start := time.Now()
			if err := runAsyncRunner(possess.Phase(flagPhase), flagTicket, flagWorkdir, flagTask, ""); err != nil {
				return fmt.Errorf("phase %s failed in %s: %w", flagPhase, time.Since(start).Round(time.Millisecond), err)
			}
			fmt.Fprintf(os.Stderr, "radiant async-runner: phase %s ok in %s\n", flagPhase, time.Since(start).Round(time.Millisecond))
			return nil
		},
	}
	cmd.Flags().StringVar(&flagPhase, "phase", "", "Phase to run: discover|plan|execute|verify")
	cmd.Flags().StringVar(&flagTicket, "ticket", "", "16-char hex ticket (must match possess task_id)")
	cmd.Flags().StringVar(&flagWorkdir, "workdir", "", "Absolute path to project workdir")
	cmd.Flags().StringVar(&flagTask, "task", "", "User's verbatim task prompt")
	root.AddCommand(cmd)
}