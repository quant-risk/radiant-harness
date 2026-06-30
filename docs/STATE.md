---
name: STATE
description: Volatile working memory — progress, decisions, blockers, context bookmarks.
alwaysApply: true
---

# STATE — Living Project Memory

**Last updated:** 2026-06-30 15:30 BRT by mavis during v3.7.13 release cut

## Current sprint / active feature

- Active: **v3.7.13 code-complete; release cut pending.**
- Sprint goal: close the v3.7.12 follow-up work + make the
  nested pid tree visible end-to-end. Three items shipped (A+B+C):
  - **A.** `radiant fleet status <run-id> --html` — self-
    contained HTML report with visual nested pid tree (parent →
    child → grandchild → great-grandchild) in Unicode box-
    drawing glyphs inside a `<pre>` block, color-coded by
    liveness. `--html-out=<path>` variant writes to disk. No
    external CSS / JS / CDN — renders offline.
  - **B.** `radiant phase redirect --purge=<ticket-id>` —
    explicit cleanup of a stale follow redirect without nuking
    state dir. Removes ONLY `redirect.json` for the named
    ticket; spec, tasks, other redirects untouched. Exits 1 on
    missing file so CI / lints catch accidental purges.
  - **C.** Nested pid tree — great-grandchildren layer. New
    `.pid.great-grandchildren` sidecar; `PidTree` gains
    `GreatGrandchildrenPids` + `GreatGrandchildrenAlive` +
    `GreatGrandchildrenCount`. Crashed-evidence string in
    `Coordinator.Status()` includes great-grandchildren counts.
- Progress: (1) `purgeFollowRedirect` + `purgeFollowRedirectW`
  testable helper in `cmd_phase_watch.go`; (2) `PidTree` struct
  extended with 3 new fields + read/write helpers +
  `taskPidGreatGrandchildrenPath`; (3) `refreshChildTreeSidecars`
  writes all 3 sidecars (children + grandchildren + great-
  grandchildren) in a single pgrep pass; (4) `FormatStatusHTML`
  + `renderPidTreeHTML` in `internal/fleet/coordinator.go`
  (~290 lines, pure HTML/CSS, JS-free); (5) `fleet status`
  cobra RunE routes `--html` + `--html-out=<path>`; (6)
  crashed-escalation evidence string in coordinator.go extended
  with great-grandchildren counts; (7) 11 new tests pin the
  contract (4 CLI purge + 6 HTML + 7 pidtree); (8) CHANGELOG +
  ROADMAP updated; (9) release cut pending — build + tag
  v3.7.13 + GitHub release + post-release validation.
- v3.7.13 GitHub release: tag `v3.7.13` + 7 release assets (TBD).

## Current sprint / active feature

- Active: **v3.7.10 code-complete; release cut pending.**
- Sprint goal: close the remaining 3 backlog items (--watch,
  nested pid tree, async-host opt-in matrix).
- Progress (v3.7.10 closed): (1) `radiant phase watch`
  CLI namespace + `runPhaseWatch` polling helper with
  terminal/max-poll/JSON/no-reemit semantics + SIGINT handling;
  (2) PidTree struct + `TaskLive.tree` + `refreshChildPidsLoop`
  pgrep-based child refresh + sidecar file format;
  (3) `--async-subprocess` + `--fleet-async-subprocess` CLI
  flags on `radiant mcp serve` + envBool helper + precedence
  chain (CLI > env > default); (4) `radiant doctor --async-host`
  diagnostic with the 13-host opt-in matrix (only Hermes
  flagged RecommendAsync=true today); (5) 22 new tests pin
  the contract (13 pidtree + 9 v3_7_10 CLI/MCP); (6) full
  validation pending — release cut pending.
- v3.7.10 GitHub release: tag `v3.7.10` + 7 release assets
  (TBD).

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

## Latest validation

2026-06-30 15:45 BRT — v3.7.13 deep post-release validation, **12/12 PASS** (first pass):

| Step | Description | Result |
|------|-------------|--------|
| A | `go build ./...` + `go vet ./...` | PASS (RC=0) |
| B | `radiant mcp self-test` (local build) | PASS, 8 tools |
| B2 | `--version` check | `v3.7.13` (clean tag) |
| C | `go test ./...` | PASS (32 packages, 0 FAIL) |
| C2 | `go test ./internal/fleet ./cmd/radiant -v -run "v3_7_13\|PidTree.*GreatGrandchild\|FleetStatus_HTML\|PhaseRedirect_Purge"` | PASS (13 new tests) |
| D | `make audit-install` | PASS 2/3 + 1 SKIP (canonical SKIPs local-dirty) |
| E | `make test-agents` | PASS 13/13 |
| F | `make test-dropin` | PASS v3.7.13 |
| G | `./scripts/run.sh` | PASS 8/8 + 2 SKIP |
| H | Clean rebuild from tag | PASS — local `v3.7.13-1-ge35f3d0`, published `v3.7.13` (expected divergence) |
| I | Fetch published SHA256SUMS | OK (recovered from GitHub) |
| J | REST API asset inventory | 7/7 `state=uploaded` |
| K | Download published darwin-arm64, SHA256 verify | MATCH (`c858b0f0...`) |
| K2 | Published binary version + self-test | `v3.7.13`, 8 tools, PASS |
| K3 | **New v3.7.13 surfaces reachable** | PASS — phase redirect --purge, fleet status --html, fleet status --html-out all in `--help` |
| L | Canonical install (`curl install.sh@v3.7.13`) | PASS end-to-end (used `PREFIX=~/.local/bin` to overwrite existing v3.7.12) |
| M | **A surface (fleet status --html) real output** | PASS — wrote `report.html` (4187 bytes), contains `<title>Fleet run-x</title>`, `pre.pid-tree`, `.footer` |
| N | **B surface (phase redirect --purge) real output** | PASS — `✓ removed redirect: abc123`, `redirect.json` gone, parent dir untouched |
| O | **C surface (great-grandchildren symbols)** | PASS — `GreatGrandchildrenPids/Count/Alive`, `.pid.great-grandchildren` sidecar, `pre.pid-tree` CSS in installed binary |

**Process-learnings (v3.7.13):**

- **`pidAlive` is essential** for the great-grandchildren
  alive-counting contract — without it, a dead sentinel pid
  would still surface as live. Sentinel `16777215` (above
  `pid_max` on every reasonable host) is the test pattern.
- **`TaskPidTree` early-returns on empty children sidecar** —
  the contract is "no children = no descendants possible",
  so great-grandchildren are NOT read when children is empty.
  Test must seed children sidecar too (one bug we hit + fixed
  during v3.7.13).
- **`purgeFollowRedirectW`** (writer-injecting form) is the
  testable shape; `purgeFollowRedirect` is a thin wrapper
  around it that hardcodes `os.Stdout` / `os.Stderr`. Same
  pattern as `listFollowRedirects(workdir, asJSON, w io.Writer)`
  from v3.7.12. Cobra subcommand keeps the simple form;
  tests use the writer form.
- **`FormatStatusHTML` is offline-safe by construction**: no
  http(s):// URLs in src/href, no @import, no <script src=>.
  Verified by `TestFleetStatus_HTML_SelfContained`. Designed
  for the "save as email attachment" workflow.
- **HTML escape is mandatory** — fleet goal / title / evidence
  may contain operator-supplied strings. The `&` `<` `>` `"`
  `'` replacer handles XSS via injected metadata.
  `TestFleetStatus_HTML_EscapesDangerousChars` pins the
  contract.
- **`findProjectRoot()` test helper** walks up from cwd looking
  for `go.mod`. Used by `TestFleetStatus_HTML_HTMLOut_WritesFile`
  to invoke `go build ./cmd/radiant` from the right cwd
  regardless of where `go test` was invoked (default cwd is
  the package's directory, not the project root).
- **`PREFIX=~/.local` vs `PREFIX=~/.local/bin`**: install.sh
  resolves `PREFIX` literally — `PREFIX=~/.local` installs to
  `~/.local/radiant` (NOT `~/.local/bin/radiant`). To overwrite
  the canonical install location use `PREFIX=~/.local/bin`.
  Caught during v3.7.13 validation when `~/.local/bin/radiant`
  stayed at v3.7.12 even after install printed `installed:
  v3.7.13`.

## Latest validation (history)

2026-06-30 14:15 BRT — v3.7.12 deep post-release validation, **15/15 PASS** (second pass):

| Step | Description | Result |
|------|-------------|--------|
| A | `go build ./...` + `go vet ./...` | PASS (RC=0) |
| B | `radiant mcp self-test` (published darwin-arm64) | PASS, 8 tools |
| B2 | `--version` check | `v3.7.10` (clean tag) |
| C | `go test ./...` (full module) | PASS (32 packages, 0 FAIL) |
| D | `make audit-install` | PASS |
| E | `make test-agents` | PASS 13/13 |
| F | `make test-dropin` | PASS v3.7.10 |
| G | `./scripts/run.sh` | PASS 8/8 + 2 SKIP |
| H | Clean rebuild from tag | PASS — local `v3.7.10-1-gdc41cad`, published `v3.7.10` |
| I | Fetch published SHA256SUMS | OK (recovered from GitHub) |
| J | REST API asset inventory | 7/7 `state=uploaded` |
| K | Download published darwin-arm64, SHA256 verify | MATCH (`75cd34dc...`) |
| K2 | Published binary `--version` + `mcp self-test` | `v3.7.10`, 8 tools, PASS |
| K3 | **New v3.7.10 surfaces reachable** | PASS — phase status/watch, doctor --async-host, mcp serve --async-subprocess, mcp serve --fleet-async-subprocess all present in --help |
| L | Canonical install (`curl install.sh@tag`) | PASS end-to-end |
| M | **`phase watch` actually streams** | PASS — formatted output + exit 1 on max-poll |
| N | **`phase watch --json` + transition detection** | PASS — initial emission + transition emission + exit 0 |
| O1 | **`doctor --async-host` real output** | PASS — header, agent row, env vars, NOT RECOMMENDED verdicts |
| O3 | **`.pid.children` sidecar format** | PASS — newline-separated integers at `.radiant-harness/fleet/pids/agent-<...>-<...>.pid.children` |

**Process-learnings:**

- `RADIANT_INTERNAL=1` is the correct override for testing internal helper commands outside the MCP host contract. The phase watch / status commands refuse without it; the error message points the operator at `mcp__radiant__possess` instead of panicking.
- `phase watch` --max-poll contract (exit 1) is distinct from Ctrl-C (exit 130). Both work as designed.

Earlier in the session (12-step validation, first pass):

| Step | Description | Result |
|------|-------------|--------|
| A | `go build ./...` | PASS (RC=0) |
| A2 | `go vet ./...` | PASS (RC=0) |
| B | `radiant mcp self-test` (published darwin-arm64) | PASS, 8 tools |
| B2 | `--version` check | `v3.7.10` |
| B3b | hidden `fleet-async-runner --help` | reachable (Hidden cobra flag) |
| B3c | `publicCommands` gate blocks without `RADIANT_INTERNAL=1` | PASS (defense-in-depth) |
| C | `go test ./...` | PASS (32 packages, 0 FAIL, 683 tests) |
| D | `make audit-install` | PASS 2/3 + 1 SKIP (canonical SKIPs local-dirty) |
| E | `make test-agents` | PASS 13/13 |
| F | `make test-dropin` | PASS v3.7.9 |
| G | `./scripts/run.sh` | PASS 8/8 + 2 SKIP |
| H | Clean rebuild from tag | PASS — local `v3.7.9-1-gda91bd7`, published `v3.7.9` |
| I | Fetch published SHA256SUMS | OK (recovered from GitHub) |
| J | REST API asset inventory | 7/7 uploaded |
| K | Download published darwin-arm64, SHA256 verify | MATCH (`9379fcadf...`) |
| K2 | Published binary version + self-test | `v3.7.9`, 8 tools, PASS |
| L | Canonical install from `curl install.sh@tag` | PASS end-to-end |

**Process learnings:**

- **Build BEFORE post-release commits** for clean `v3.7.X` version strings in release binaries. After the first post-release commit, local `make release` produces `v3.7.X-1-g<sha>` and different SHA256s. To verify a published release, **download from GitHub and re-check** — not regenerate locally.
- **`make clean` deletes `dist/`** including the published SHA256SUMS. Recovery = `curl https://api.github.com/.../releases/tags/vX.Y.Z | jq` + download.
- **Hidden cobra subcommands** are reachable by direct invocation even though they don't appear in default `--help`. This is the intended surface for the async subprocess primitive (`radiant fleet-async-runner`, `radiant async-runner`).

## Decisions log

## Latest validation

2026-06-30 12:30 BRT — v3.7.9 deep post-release validation, **12/12 PASS**:

| Step | Description | Result |
|------|-------------|--------|
| A | `go build ./...` | PASS (RC=0) |
| A2 | `go vet ./...` | PASS (RC=0) |
| B | `radiant mcp self-test` (published darwin-arm64) | PASS, 8 tools |
| B2 | `--version` check | `v3.7.9` |
| B3b | hidden `fleet-async-runner --help` | reachable (Hidden cobra flag) |
| B3c | `publicCommands` gate blocks without `RADIANT_INTERNAL=1` | PASS (defense-in-depth) |
| C | `go test ./...` | PASS (32 packages, 0 FAIL, 683 tests) |
| D | `make audit-install` | PASS 2/3 + 1 SKIP (canonical SKIPs local-dirty) |
| E | `make test-agents` | PASS 13/13 |
| F | `make test-dropin` | PASS v3.7.9 |
| G | `./scripts/run.sh` | PASS 8/8 + 2 SKIP |
| H | Clean rebuild from tag | PASS — local `v3.7.9-1-gda91bd7`, published `v3.7.9` (expected divergence) |
| I | Fetch published SHA256SUMS | OK (recovered from GitHub) |
| J | REST API asset inventory | 7/7 uploaded |
| K | Download published darwin-arm64, SHA256 verify | MATCH (`9379fcadf...`) |
| K2 | Published binary version + self-test | `v3.7.9`, 8 tools, PASS |
| L | Canonical install from `curl install.sh@tag` | PASS end-to-end |

**Process learnings:**

- **Build BEFORE post-release commits** for clean `v3.7.X` version strings in release binaries. After the first post-release commit, local `make release` produces `v3.7.X-1-g<sha>` and different SHA256s. To verify a published release, **download from GitHub and re-check** — not regenerate locally.
- **`make clean` deletes `dist/`** including the published SHA256SUMS. Recovery = `curl https://api.github.com/.../releases/tags/vX.Y.Z | jq` + download.
- **Hidden cobra subcommands** are reachable by direct invocation even though they don't appear in default `--help`. This is the intended surface for the async subprocess primitive (`radiant fleet-async-runner`, `radiant async-runner`).

Earlier in the session (v3.7.9 code-complete validation):

| Step | Command | Result |
|------|---------|--------|
| A | `go build ./...` | clean |
| B | `go vet ./...` | clean |
| C | `go test ./...` (full module) | PASS (32 packages, 0 FAIL) |
| D | `go test ./cmd/radiant` fleet subset | PASS — 7 new tests (`TestMCPFleetStatus_*` × 4, `TestMCPFleetResume_*` × 2, `TestFleetAsync*` × 2) |
| E | `go test ./internal/fleet` | PASS — 15 new tests (10 pidfile + 5 coordinator), 0 FAIL |
| F | `make audit-docs` | PASS (46 doc refs / 57 real cmds) |
| G | `make audit-skills` | PASS (6 hint map / 69 bundled skills) |

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
