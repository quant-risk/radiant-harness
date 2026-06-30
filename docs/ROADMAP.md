# Roadmap — radiant-harness

## Roadmap objective

Make radiant-harness a reliable drop-in governance layer for host agents:
installable from GitHub, usable through MCP, auditable through persisted
state, and clear enough for another agent to complete real project work.

## Shipped in v3.7.11 (2026-06-30)

- **`radiant phase watch --on-change-exit`** — exit 0 immediately
  after the FIRST change observed AFTER the initial snapshot.
  Useful for "wait until anything changes" notifications without
  a full watch. Combine with `--max-poll` to bound the wait.
- **`radiant phase watch --follow=<anchor-ticket-id>`** +
  **`radiant phase redirect <old> <new>`** — `--follow` tracks
  the anchor ticket's state initially; mid-watch, the operator
  can write a `redirect.json` via `phase redirect` and the
  watcher switches to the new ticket transparently. Use case:
  resume re-dispatches with a new ticket id without losing
  the watcher's stream.
- **`docs/HOSTS.md`** — offline-readable per-host opt-in matrix
  for all 13 Light-mode hosts + the "no agent detected" case.
  Documents when to opt in, how to verify, when NOT to opt in,
  and governance rules for flipping verdicts. Companion to
  `radiant doctor --async-host` (which already shipped in v3.7.10).
- **9 new tests pin the contract** — on-change-exit
  transition/max-poll semantics, follow redirect file format
  (write/read round-trip, missing/corrupt handling, RFC3339
  payload shape).

## Shipped in v3.7.10 (2026-06-30)

- **`radiant phase watch <task-id>` CLI** — polls the persisted
  phase state and re-emits the summary on change. Companion to
  the MCP `radiant_phase_status` for hosts that want streaming
  without round-tripping through the MCP transport. Exits 0 on
  terminal state, exits 1 after `--max-poll`, exits 130 on
  SIGINT. NDJSON mode (`--json`) is `jq -c` line-by-line
  parseable.
- **Per-task nested pid tree** — `TaskLive.tree` exposes
  `parent_alive` + `children_pids` + `children_alive` +
  `child_count` so a host can distinguish "parent died
  cleanly" from "parent died; N helpers orphaned".
  `Dispatcher.spawnAgent` now spawns a `refreshChildPidsLoop`
  goroutine that `pgrep -P`'s the agent every 5s and writes
  the children sidecar at
  `.radiant-harness/fleet/pids/agent-<...>.pid.children`.
- **Async-subprocess opt-in matrix + diagnostic.** Two new CLI
  flags (`--async-subprocess`, `--fleet-async-subprocess`) on
  `radiant mcp serve` join the env-var path with CLI-flag >
  env-var > default-off precedence. `radiant doctor
  --async-host` (v3.7.10+) scores all 13 known hosts — only
  Hermes is currently flagged RecommendAsync=true (TUI gates
  tool-call completion on subprocess exit); the rest default
  to inline.
- **22 new tests pin the contract** — pid sidecar roundtrip,
  watch terminal/max-poll/JSON/no-reemit semantics, status CLI
  shape, doctor exit-code contract, envBool parsing.

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
| Real CI host reproducing fleet cross-process need (gates default-flip of `RADIANT_FLEET_ASYNC_SUBPROCESS=1`) | Concrete reproduction (CI host with hard MCP tool-call deadline against a large fleet) | M | Maintainers | v3.7.11 docs/HOSTS.md governance rules | Document the host, opt in by default, validate end-to-end |
| Real sync host reproducing loop async need (gates default-flip of `RADIANT_ASYNC_SUBPROCESS=1`) | Concrete reproduction (Hermes TUI aside, no other known sync host yet) | M | Maintainers | v3.7.11 docs/HOSTS.md governance rules | Document the host, opt in by default, validate end-to-end |

## Next

| Item | Value | Effort | Owner | Dependencies | Done when |
|------|-------|--------|-------|--------------|-----------|
| `radiant phase redirect --list` | Show all current redirects in the workdir | S | Maintainers | v3.7.11 redirect.json protocol | Operator can clean up after a long fleet run without manually finding each redirect.json |
| `radiant phase follow` alias for `radiant phase watch --follow=<ticket>` | Shorter CLI surface for the follow use case | S | Maintainers | v3.7.11 --follow | Operator doesn't need to remember two flag spellings |
| Recursive fleet pid tree (v3.7.10 nested pid tracking extended to grandchildren) | Distinguish grandchild death from child death | M | Maintainers | v3.7.10 pgrep -P refresh | Status surfaces which grandchild process died, not just that one did |

## Later

| Item | Value | Effort | Owner | Dependencies | Done when |
|------|-------|--------|-------|--------------|-----------|
| Richer ontology tooling | Better scope discovery and skill routing | M | Maintainers | Glossary/ontology adoption | Ontology can be validated against specs and skills |
| Per-host skill bundles | Smaller drop-in for host-specific stacks | M | Maintainers | Skill catalog | Each host has a default skill bundle surfaced on first run |
