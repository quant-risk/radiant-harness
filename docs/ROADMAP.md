# Roadmap — radiant-harness

## Roadmap objective

Make radiant-harness a reliable drop-in governance layer for host agents:
installable from GitHub, usable through MCP, auditable through persisted
state, and clear enough for another agent to complete real project work.

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

## Next

| Item | Value | Effort | Owner | Dependencies | Done when |
|------|-------|--------|-------|--------------|-----------|
| Async gate pid/liveness probe | Cross-process cancel/inspect | M | Maintainers | v3.7.7 subprocess path | `radiant_phase_status` distinguishes "alive" from "crashed" without re-running the gate |
| Fleet async primitives | More predictable parallel orchestration | L | Maintainers | Stable loop async | Fleet has the same status/retry guarantees as loop |
| Backfill v3.7.3-v3.7.5 CHANGELOG entries (Done in commit 82b1245, but Worth tracking for future sprints where v3.7.0-v3.7.x history has gaps) | Honest release history | S | Maintainers | Git log for the period | Each tag has a CHANGELOG subsection with date + feature summary |

## Later

| Item | Value | Effort | Owner | Dependencies | Done when |
|------|-------|--------|-------|--------------|-----------|
| Richer ontology tooling | Better scope discovery and skill routing | M | Maintainers | Glossary/ontology adoption | Ontology can be validated against specs and skills |
| Per-host skill bundles | Smaller drop-in for host-specific stacks | M | Maintainers | Skill catalog | Each host has a default skill bundle surfaced on first run |
