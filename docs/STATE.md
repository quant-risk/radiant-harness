---
name: STATE
description: Volatile working memory — progress, decisions, blockers, context bookmarks.
alwaysApply: true
---

# STATE — Living Project Memory

**Last updated:** 2026-06-30 11:35 BRT by mavis during v3.7.8 post-release validation

## Current sprint / active feature

- Active: **v3.7.8 post-release validation done; v3.7.9 kickoff.**
- Sprint goal: surface subprocess alive-vs-crashed from
  `radiant_phase_status` so a host agent can tell the difference
  between "phase still running" and "subprocess crashed without
  writing an error".
- Progress (v3.7.8 closed): (1) `phaseStatusSummary` extended
  with `subprocess_alive` (bool) + `subprocess_pid` (int) fields
  populated from `.radiant-harness/pids/<ticket>.pid`; (2) status
  escalation from `in_progress` to `crashed` when pid is dead;
  (3) next-step line annotated with pid + liveness; (4) format
  helper (`content[1].text`) gains a `subprocess:` line; (5) 3
  new tests pin the contract (SubprocessAlive / SubprocessCrashed
  / NoPidFile); (6) full validation 7/7 PASS (see below).
- v3.7.8 GitHub release: tag `v3.7.8` + 7 release assets.

## Next concrete action

- v3.7.9 work. Order: (1) Fleet async primitives — same
  status/retry guarantees as loop but for fleet ops; (2) real
  host opt-in for `RADIANT_ASYNC_SUBPROCESS=1` — needs a
  reproduction of a sampling-backed sync-host possess need
  before turning the subprocess path on by default for any
  host; (3) `--watch` flag for `radiant_phase_status` (poll
  pid file + emit MCP notifications on alive→dead transitions).

## Latest validation

2026-06-30 11:00 BRT — v3.7.7 post-release validation, full matrix:

| Step | Command | Result |
|------|---------|--------|
| A | `go build ./...` | clean |
| B | `radiant mcp self-test` | PASS, 6 tools (`radiant_possess`, `radiant_run_gate`, `radiant_possess_async`, `radiant_phase_status`, `radiant_skill_list`, `radiant_skill_load`) |
| C | `go test ./cmd/radiant ./internal/...` | PASS (32 packages, 0 FAIL) |
| D | `go test ./...` (full module) | PASS |
| E | `make audit-docs` | PASS (46 doc refs / 57 real cmds) |
| F | `make audit-skills` | PASS (6 hint map / 69 bundled skills) |
| G | `make audit-install` | **PASS, 3/3, 0 SKIP** — canonical `curl \| bash` resolves v3.7.7, SHA256 verified, installed binary reports `v3.7.7` |
| H | `make test-agents` | PASS, 13/13 (incl. `gemini`) |
| I | `make test-dropin` | PASS, against v3.7.7 |
| J | canonical install end-to-end (curl published asset, chmod, `--version`, `mcp self-test`) | PASS — `v3.7.7`, 6 tools, total 9 ms |
| K | `./scripts/run.sh` | PASS, 8/8 + 2 SKIP doctor |

Earlier in the session (v3.7.6 post-release validation):

| Step | Command | Result |
|------|---------|--------|
| A | `go build ./...` | clean |
| B | `radiant mcp self-test` | PASS, 6 tools |
| C | `go test ./cmd/radiant ./internal/...` | PASS |
| D | `go test ./...` (full module) | PASS |
| E | `make audit-docs` | PASS (46/57) |
| F | `make audit-skills` | PASS (6/69) |
| G | `make audit-install` | **PASS, 3/3, 0 SKIP** — canonical `curl \| bash` resolves v3.7.6, SHA256 verified, installed binary reports `v3.7.6` |
| H | `make test-agents` | PASS, 13/13 (incl. `gemini`) |
| I | `make test-dropin` | PASS, against v3.7.6 |
| J | `./scripts/run.sh` | PASS, 8/8 + 2 SKIP doctor (4 runs in a row after warmup) |
| K | `RADIANT_VERSION=3.7.6 bash install.sh --no-verify` end-to-end | PASS — `v3.7.6`, `mcp self-test` PASS |

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

- Fleet-mode async primitives (same status/retry guarantees as loop).
- Real host opt-in for `RADIANT_ASYNC_SUBPROCESS=1` — needs a
  reproduction of a sampling-backed sync-host possess or fleet
  cross-process worktree need. Without a real host need, the
  inline path is correct.
- `--watch` flag for `radiant_phase_status` — poll the pid file
  every N seconds and emit an MCP notification when liveness
  transitions alive → dead. Not strictly necessary (the host
  can poll), but useful for CI hosts that want to stream
  progress.
