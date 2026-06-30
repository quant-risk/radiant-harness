# Roadmap — radiant-harness

## Roadmap objective

Make radiant-harness a reliable drop-in governance layer for host agents:
installable from GitHub, usable through MCP, auditable through persisted
state, and clear enough for another agent to complete real project work.

## Shipped in v3.7.9 (2026-06-30)

- **Fleet async primitives (A+B+C combined).** Closes the
  "same status/retry contract as loop" backlog item with all
  three layers:
  - **A.** `mcp__radiant__fleet_status` + `mcp__radiant__fleet_resume`
    — host agents can now drive the fleet lifecycle from the
    MCP wire, mirroring how `radiant_phase_status` +
    `radiant_possess_async` work for the loop.
  - **B.** Liveness probe via `Coordinator.WithLivenessDir`:
    `FleetStatus` gains `DispatcherAlive`, `DispatcherPid`,
    and `TaskLiveness` map; assigned tasks with dead pids
    escalate to `TaskCrashed`. Parity with v3.7.8's loop
    `phaseStatusSummary` crashed branch.
  - **C.** Subprocess gate: `DispatchConfig.AsyncSubprocess`
    forks `radiant fleet-async-runner <run-id>` and returns
    immediately. Parity with v3.7.7's loop async-runner.
    Inline remains the default.
- **Per-task + per-dispatcher pid files.**
  `.radiant-harness/fleet/pids/{agent,dispatcher}-<...>.pid`
  with `kill -0` liveness probes. Always-on for per-task
  (works with inline dispatchers); per-dispatcher only when
  async subprocess mode is on. `sanitizePidComponent`
  defends against path traversal in run / task IDs.
- **22 new tests pin the contract:** 10 in
  `internal/fleet/pidfile_test.go`, 5 in
  `internal/fleet/coordinator_test.go`, 7 in
  `cmd/radiant/cmd_mcp_fleet_async_test.go`.

## Shipped in v3.7.8 (2026-06-30)

- **Async gate pid/liveness probe.** `radiant_phase_status` now
  cross-references the persisted subprocess pid against the
  running OS process list. New `subprocess_alive` +
  `subprocess_pid` fields in the summary; status escalates from
  `in_progress` to `crashed` when the recorded pid is dead,
  with the next-step line pointing at `mcp__radiant__run_gate`
  so the host can resume. 3 new tests pin the contract
  (SubprocessAlive / SubprocessCrashed / NoPidFile).

## Shipped in v3.7.7 (2026-06-30)

- **Subprocess-backed async gate primitives.** `radiant
  async-runner` subcommand + `subprocessAsyncGate` /
  `subprocessPossessAsync` impls + pid file management. Opt-in
  via `RADIANT_ASYNC_SUBPROCESS=1`. Inline path stays the
  default; subprocess path is for future sampling-backed sync-
  host or fleet cross-process needs. 5 new tests pin the
  subprocess path behaviour.

## Shipped in v3.7.6 (2026-06-30)

- **Host matrix broadened.** Google Gemini CLI added as the 13th
  Light-mode host; `setup-mcp --agent=gemini` writes
  `~/.gemini/settings.json` with the standard `mcpServers` JSON
  shape. Detection fingerprint is documented.
- **Status UX improved.** `radiant_phase_status` returns a
  structured `summary` (next step, resume command, pending files,
  marker count, last gate, clear error/cancel state). Five new
  contract tests pin the four phases × {done, in_progress, error,
  cancelled} matrix.
- **Validation entrypoint extended.** `scripts/run.sh` now covers
  the full install/test/audit matrix (was 4 commands); doctor
  steps surfaced as SKIP, not FAIL, in a host-less shell.
- **Doc/backlog consolidated.** Spec placeholders closed,
  `docs/ROADMAP.md` and `docs/STATE.md` re-organised as living
  memory, `radiant_run_gate` and `radiant_possess_async` finally
  documented in `AGENTS-FOR-TASKS.md` § MCP tools.
- **External user case removed.** MenuFlex spec purged from the
  harness repo (did not belong here).

## Shipped in v3.7.6 follow-ups (2026-06-30)

- **Per-agent restart hints added for 5 hosts** in `install.sh`:
  gemini, kimi, openclaw, cline, MiniMax. The post-install
  table now covers all 13 Light-mode hosts with vendor-specific
  restart commands.

## Now

| Item | Value | Effort | Owner | Dependencies | Done when |
|------|-------|--------|-------|--------------|-----------|
| Turn on `RADIANT_ASYNC_SUBPROCESS=1` for a real host that needs it (sampling-backed sync-host possess OR fleet cross-process worktree) | Concrete reproduction gates the work | M | Maintainers | Reproduce the need | Document the host, opt in by default, validate end-to-end |
| Turn on `RADIANT_FLEET_ASYNC_SUBPROCESS=1` for the same real host needs | Concrete reproduction (e.g. CI host with hard MCP tool-call deadline against a large fleet) | M | Maintainers | v3.7.9 fleet async primitives | Document the host, opt in by default, validate end-to-end |

## Next

| Item | Value | Effort | Owner | Dependencies | Done when |
|------|-------|--------|-------|--------------|-----------|
| `--watch` flag for `radiant_phase_status` | Poll pid file + emit MCP notifications on alive→dead transitions | S | Maintainers | v3.7.8 pid probe | `radiant_phase_status --watch <ticket>` streams until terminal state or Ctrl-C |
| Per-task nested pid tracking (recursive liveness) | Distinguish crashed parent from crashed child agent | M | Maintainers | v3.7.9 fleet pid files | Status surfaces which child process died, not just that one did |
| Backfill v3.7.3-v3.7.5 CHANGELOG entries (Done in commit 82b1245, but Worth tracking for future sprints where v3.7.0-v3.7.x history has gaps) | Honest release history | S | Maintainers | Git log for the period | Each tag has a CHANGELOG subsection with date + feature summary |

## Later

| Item | Value | Effort | Owner | Dependencies | Done when |
|------|-------|--------|-------|--------------|-----------|
| Richer ontology tooling | Better scope discovery and skill routing | M | Maintainers | Glossary/ontology adoption | Ontology can be validated against specs and skills |
| Per-host skill bundles | Smaller drop-in for host-specific stacks | M | Maintainers | Skill catalog | Each host has a default skill bundle surfaced on first run |
