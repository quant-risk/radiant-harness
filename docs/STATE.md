---
name: STATE
description: Volatile working memory — progress, decisions, blockers, context bookmarks.
alwaysApply: true
---

# STATE — Living Project Memory

**Last updated:** 2026-06-30 12:25 BRT by mavis during v3.7.9 code-complete

## Current sprint / active feature

- Active: **v3.7.9 code-complete; release cut pending.**
- Sprint goal: fleet gets the same status/retry/liveness
  contract as the loop. Three layers shipped (A+B+C):
  - **A.** `mcp__radiant__fleet_status` +
    `mcp__radiant__fleet_resume` MCP tools (host can drive
    fleet from the wire).
  - **B.** Liveness probe via `Coordinator.WithLivenessDir`:
    `DispatcherAlive` + `DispatcherPid` + per-task
    `TaskLiveness` map; `TaskAssigned` with dead pid escalates
    to `TaskCrashed`.
  - **C.** Subprocess gate on dispatcher via
    `DispatchConfig.AsyncSubprocess` +
    `radiant fleet-async-runner <run-id>` (Hidden subcommand
    gated by `RADIANT_FLEET_ASYNC_RUNNER=1`).
- Progress (v3.7.9 closed): (1) `internal/fleet/pidfile.go`
  with per-task + per-dispatcher pid paths, sanitize helper,
  WriteDispatcherPid / RemoveDispatcherPid exports; (2)
  DispatchConfig gains AsyncSubprocess + Workdir, RunAll forks
  subprocess when enabled, spawnAgent writes per-task pid
  file before Start and removes it via defer; (3)
  Coordinator.WithLivenessDir + Status() liveness fields +
  crashed escalation; (4) TaskCrashed lifecycle + Store.
  CrashTask; (5) cmd_mcp_fleet_async.go with mcpFleetStatus +
  mcpFleetResume tools + fleetAsyncSubprocessEnabled helper;
  (6) cmd_fleet_async_runner.go subcommand; (7) 22 new tests
  pinning the contract (10 pidfile, 5 coordinator, 7 MCP);
  (8) full validation 7/7 PASS — see below.
- v3.7.9 GitHub release: tag `v3.7.9` + 7 release assets (TBD).

## Next concrete action

- v3.7.10 backlog. Order: (1) Real host opt-in for
  `RADIANT_FLEET_ASYNC_SUBPROCESS=1` — needs a reproduction of
  a sampling-backed fleet cross-process need (CI host with
  hard MCP tool-call deadline against a large fleet) before
  turning the subprocess path on by default; (2) `--watch`
  flag for `radiant_phase_status` (poll pid file + emit MCP
  notifications on alive→dead transitions); (3) Per-task
  nested pid tracking (recursive liveness) for fleet — which
  child process died, not just that one did.

## Latest validation

2026-06-30 12:20 BRT — v3.7.9 code-complete validation, full matrix:

| Step | Command | Result |
|------|---------|--------|
| A | `go build ./...` | clean |
| B | `go vet ./...` | clean |
| C | `go test ./...` (full module) | PASS (32 packages, 0 FAIL) |
| D | `go test ./cmd/radiant` fleet subset | PASS — 7 new tests (`TestMCPFleetStatus_*` × 4, `TestMCPFleetResume_*` × 2, `TestFleetAsync*` × 2) |
| E | `go test ./internal/fleet` | PASS — 15 new tests (10 pidfile + 5 coordinator), 0 FAIL |
| F | `make audit-docs` | PASS (46 doc refs / 57 real cmds) |
| G | `make audit-skills` | PASS (6 hint map / 69 bundled skills) |

Earlier in the session (v3.7.8 post-release validation):

| Step | Command | Result |
|------|---------|--------|
| A | `go build ./...` | clean |
| B | `radiant mcp self-test` | PASS, 6 tools (`radiant_possess`, `radiant_run_gate`, `radiant_possess_async`, `radiant_phase_status`, `radiant_skill_list`, `radiant_skill_load`) |
| C | `go test ./cmd/radiant ./internal/...` | PASS (32 packages, 0 FAIL) |
| D | `go test ./...` (full module) | PASS |
| E | `make audit-docs` | PASS (46 doc refs / 57 real cmds) |
| F | `make audit-skills` | PASS (6 hint map / 69 bundled skills) |

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
  (now lists 8 MCP tools after v3.7.9: the original 6 +
  `radiant_fleet_status` + `radiant_fleet_resume`).
- `INSTALL.md` — install flow + 13-agent host table.
- `cmd/radiant/cmd_mcp_runtime.go` — MCP tool registration +
  `mcpPhaseStatus` summary builder.
- `cmd/radiant/cmd_mcp_fleet_async.go` — fleet MCP wrappers
  (v3.7.9): `mcpFleetStatus` + `mcpFleetResume`.
- `cmd/radiant/cmd_fleet_async_runner.go` — Hidden subcommand
  for the dispatcher subprocess path (v3.7.9).
- `cmd/radiant/cmd_mcp_possess_self_driven.go` — self-driven fallback.
- `internal/hostdetect/hostdetect.go` — host fingerprints (13 agents).
- `internal/fleet/pidfile.go` — pid file primitives for fleet
  tasks + dispatcher (v3.7.9). Mirrors cmd_async_runner.go for loop.
- `internal/fleet/coordinator.go` — `WithLivenessDir` +
  `TaskCrashed` escalation (v3.7.9).
- `internal/possess/async.go` — async gate primitives (interfaces;
  current impl is in-process, subprocess deferred).
- `scripts/e2e/dropin_self_driven_e2e.py` — public install E2E.
- `scripts/run.sh` — canonical validation entrypoint.
- `scripts/test-agents.sh` — 13-agent cross-install matrix.
- `scripts/audit-install.sh` — install-path audit (canonical
  `curl | bash` will PASS once v3.7.9 is tagged).
- `docs/ROADMAP.md` — remaining backlog (v3.7.10 = real-host
  opt-in + `--watch` flag + recursive liveness).
- `docs/PROPOSAL-v3.7.2-async-primitives.md` — async design + v3.7.6
  deferral note (now resolved for fleet via v3.7.9).
- `docs/STATE.md` — this file.

## Deferred ideas / backlog

- Real host opt-in for `RADIANT_FLEET_ASYNC_SUBPROCESS=1` —
  needs a reproduction of a sampling-backed fleet cross-process
  need (CI host with hard MCP tool-call deadline against a
  large fleet) before turning the subprocess path on by default.
- Real host opt-in for `RADIANT_ASYNC_SUBPROCESS=1` — same
  gating as fleet, but for the loop's own subprocess path.
- `--watch` flag for `radiant_phase_status` — poll the pid file
  every N seconds and emit an MCP notification when liveness
  transitions alive → dead. Not strictly necessary (the host
  can poll), but useful for CI hosts that want to stream
  progress.
- Per-task nested pid tracking (recursive liveness) for fleet
  — distinguish "agent parent died" from "child helper died".
  v3.7.9 only tracks the top-level per-task pid.
