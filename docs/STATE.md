---
name: STATE
description: Volatile working memory — progress, decisions, blockers, context bookmarks.
alwaysApply: true
---

# STATE — Living Project Memory

**Last updated:** 2026-06-30 09:40 BRT by mavis during v3.7.x backlog burndown

## Current sprint / active feature

- Active: **v3.7.x backlog burndown** — v3.7.6 shipped, post-release
  validation recorded, and the two open follow-ups from the v3.7.6
  release notes (CHANGELOG backfill + run.sh flake) are now closed.
- Sprint goal: clear the v3.7.x release-history debt before starting
  v3.7.7 work. Two risks identified in the v3.7.6 release summary
  were actionable in scope and are now done.
- Progress: (1) CHANGELOG backfill for v3.7.3 / v3.7.4 / v3.7.5
  landed in commit `82b1245` — every v3.7.x tag now has a dated
  section, zero `[Unreleased]` placeholders remain; (2) the
  `./scripts/run.sh` flake (root cause: `TestRunAllContextCanceled`
  in `internal/fleet/dispatch_test.go` was asserting `ExitCode != 0`
  on a context-cancelled subprocess, which fails ~5% of the time
  on macOS arm64 because Go's `exec.CommandContext` can deliver
  SIGTERM before escalating to SIGKILL — letting the shell exit
  cleanly with code 0) is fixed in commit `435f107` — the test now
  asserts the semantically correct invariant (either non-zero
  exit OR fast elapsed time), `internal/fleet` 50/50 PASS in
  isolation, `./scripts/run.sh` 10/10 PASS in a row.

## Next concrete action

- v3.7.7 work. Order: (1) implement async subprocess per
  `docs/PROPOSAL-v3.7.2-async-primitives.md` § v3.7.6 update — gate
  on a real host need first (sampling-backed sync-host possess or
  fleet cross-process worktree); (2) async gate pid/liveness probe
  so `radiant_phase_status` distinguishes alive from crashed without
  re-running the gate; (3) cross-host restart hint audit (the
  install.sh restart-hint table covers the 12 Light-mode hosts,
  Gemini should be added per v3.7.6 wiring).

## Latest validation

2026-06-30 09:20 BRT — v3.7.6 post-release validation, full matrix:

| Step | Command | Result |
|------|---------|--------|
| A | `go build ./...` | clean |
| B | `radiant mcp self-test` | PASS, 6 tools (`radiant_possess`, `radiant_run_gate`, `radiant_possess_async`, `radiant_phase_status`, `radiant_skill_list`, `radiant_skill_load`) |
| C | `go test ./cmd/radiant ./internal/...` | PASS |
| D | `go test ./...` (full module) | PASS |
| E | `make audit-docs` | PASS (46 doc refs / 57 real cmds) |
| F | `make audit-skills` | PASS (6 hint map / 69 bundled skills) |
| G | `make audit-install` | **PASS, 3/3, 0 SKIP** — canonical `curl \| bash` resolves v3.7.6 + verifies SHA256 |
| H | `make test-agents` | PASS, 13/13 (incl. `gemini`) |
| I | `make test-dropin` | PASS, against v3.7.6 |
| J | `./scripts/run.sh` | PASS, 8/8 + 2 SKIP doctor (4 runs in a row after warmup) |
| K | canonical install end-to-end (`RADIANT_VERSION=3.7.6 bash install.sh --no-verify`) | PASS — installed binary reports `v3.7.6`, `mcp self-test` PASS |

Earlier in the session (v3.7.6 prep pass):

- `./scripts/run.sh` — PASS (8 PASS, 2 SKIP — doctor + doctor --mcp
  informational in a host-less shell).
- `radiant mcp self-test` — PASS, 6 MCP tools listed.
- `make audit-docs` — PASS, 46 doc references / 57 real cmds.
- `make audit-skills` — PASS, 6 hint map entries / 69 bundled skills.
- `make audit-install` — PASS for reachable paths; canonical
  `curl | bash` SKIP only because local tree is 3 commits ahead of
  v3.7.5 (no matching tag yet). After v3.7.6 tag is published, this
  path will land on PASS.
- `make test-agents` — PASS; matrix regenerated (13 agents incl.
  Gemini).
- `make test-dropin` — PASS against local tree.
- `go test ./cmd/radiant ./internal/...` — PASS.
- `go test ./...` — PASS (full module).

## Decisions log

- 2026-06-30: keep `radiant_possess` as the primary path for hosts with
  sampling and use self-driven scaffolds for hosts without sampling.
- 2026-06-30: keep `radiant_run_gate` and `radiant_possess_async` as real
  offline MCP primitives for synchronous hosts.
- 2026-06-30: defer true background subprocess for `radiant_possess_async`
  to v3.7.7 — the inline offline path completes in <500 ms and a real
  subprocess adds pid + lock + crash-recovery machinery for negligible
  latency win. Spec lives in `docs/PROPOSAL-v3.7.2-async-primitives.md`
  § v3.7.6 update.
- 2026-06-30: surface doctor steps as SKIP (not FAIL) in `scripts/run.sh`
  so the validation matrix is reliable from CI and from inside a host
  session. Real failures (audit-install, test-agents, test-dropin,
  `go test ./...`) still exit non-zero.
- 2026-06-30: backfill v3.7.3-v3.7.5 CHANGELOG entries deferred — those
  four `[Unreleased]` sections will be picked up by the next v3.7.x
  release that ships the relevant feature. v3.7.6 documents the
  consolidation + new work only.
- 2026-06-30: Google Gemini CLI added as the 13th Light-mode host.
  Detection via `GEMINI_CLI` / `GEMINI_PROJECT_ROOT` / `GEMINI_API_KEY`;
  config at `~/.gemini/settings.json` with standard `mcpServers` JSON
  shape (same helper as Claude/Cursor).

## Blockers

- None for the v3.7.x burndown.

## Context bookmarks

- `README.md` — public install and usage entrypoint.
- `AGENTS-FOR-TASKS.md` — instructions for third-party host agents
  (now lists all 6 MCP tools and the sync-host alternative workflow).
- `INSTALL.md` — install flow + 13-agent host table.
- `cmd/radiant/cmd_mcp_runtime.go` — MCP tool registration +
  `mcpPhaseStatus` summary builder.
- `cmd/radiant/cmd_mcp_possess_self_driven.go` — self-driven fallback.
- `internal/hostdetect/hostdetect.go` — host fingerprints (13 agents).
- `internal/possess/async.go` — async gate primitives (interfaces;
  current impl is in-process, subprocess deferred).
- `scripts/e2e/dropin_self_driven_e2e.py` — public install E2E.
- `scripts/run.sh` — canonical validation entrypoint.
- `scripts/test-agents.sh` — 13-agent cross-install matrix.
- `scripts/audit-install.sh` — install-path audit (canonical
  `curl | bash` will PASS once v3.7.6 is tagged).
- `docs/ROADMAP.md` — remaining backlog (now organised around the
  deferred async-subprocess work).
- `docs/PROPOSAL-v3.7.2-async-primitives.md` — async design + v3.7.6
  deferral note.
- `docs/STATE.md` — this file.

## Deferred ideas / backlog

- Real background subprocess for `radiant_possess_async` (spec in
  `docs/PROPOSAL-v3.7.2-async-primitives.md` § v3.7.6 update).
- Async gate pid/liveness probe (`radiant_phase_status` should
  distinguish alive from crashed without re-running the gate).
- Fleet-mode async primitives (same status/retry guarantees as loop).
- Add the Gemini restart hint to `install.sh` (the
  `--agent=<name>` restart-hint table at the bottom of install.sh
  has 12 entries; v3.7.6 wired the gemini host matrix but did not
  add a restart-hint case).
