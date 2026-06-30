---
name: STATE
description: Volatile working memory — progress, decisions, blockers, context bookmarks.
alwaysApply: true
---

# STATE — Living Project Memory

**Last updated:** 2026-06-30 10:35 BRT by mavis during v3.7.7 release prep

## Current sprint / active feature

- Active: **v3.7.7 release prep** — subprocess-backed async gate
  primitives are implemented and tested; the gemini (and 4 other
  hosts') restart hint landed in `install.sh`; CHANGELOG and
  ROADMAP are updated. Ready for tag + GitHub release.
- Sprint goal: ship the v3.7.7 work per `docs/PROPOSAL-v3.7.2-async-
  primitives.md` § v3.7.6 update — give the gate primitives a real
  subprocess path (opt-in via `RADIANT_ASYNC_SUBPROCESS=1`) so
  future sampling-backed sync-host and fleet cross-process needs
  can be served without redesigning the runtime.
- Progress: (1) `radiant async-runner` subcommand
  (`cmd_async_runner.go`); (2) `subprocessAsyncGate` +
  `subprocessPossessAsync` impls of `possess.AsyncGate` and
  `possess.PossessAsync` (`cmd_mcp_async_subprocess.go`); (3)
  `selectedPossessAsync()` / `selectedAsyncGate()` selection
  helpers wired into `mcpPossessAsync` + `mcpRunGate`; (4) pid
  file management under `.radiant-harness/pids/<ticket>.pid`;
  (5) `RADIANT_BIN` env var honored so tests point at a
  one-time-built binary instead of forking themselves; (6) 5 new
  tests pin the subprocess path behaviour.

## Next concrete action

- Tag v3.7.6 + build cross-platform binaries + GitHub release with
  the standard 6 assets + SHA256SUMS. Then re-run `make
  audit-install` to confirm canonical `curl | bash` resolves v3.7.7.
- After tag: move to v3.7.8 backlog (async gate pid/liveness probe,
  fleet async primitives, future host opt-in for
  `RADIANT_ASYNC_SUBPROCESS=1`).

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

- Async gate pid/liveness probe (`radiant_phase_status` should
  distinguish alive from crashed without re-running the gate).
- Fleet-mode async primitives (same status/retry guarantees as loop).
- Real host opt-in for `RADIANT_ASYNC_SUBPROCESS=1` — needs a
  reproduction of a sampling-backed sync-host possess or fleet
  cross-process worktree need. Without a real host need, the
  inline path is correct.
