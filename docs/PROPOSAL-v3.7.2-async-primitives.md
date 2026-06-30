# PROPOSAL: v3.7.2 — async primitives for synchronous hosts (Hermes TUI)

**Status:** Draft, opens v3.7.2 workstream.
**Author:** mavis (post-mortem of v3.7.1 release scope review, 2026-06-29 23:55).
**Trigger:** User (Henrique) identified that v3.7.1 fixes the *Codex* hollow-stub case
but does NOT unblock Hermes TUI, because Hermes is a synchronous tool-call host.

## The problem v3.7.2 must solve

`mcp__radiant__possess` is a single MCP tool that round-trips via
`sampling/createMessage` N times (4 phases × ~30s cold-start per sampling call).
It works on hosts where the TUI / shell can process tool calls asynchronously
(Claude Code, Codex CLI when env propagates). It deadlocks on hosts where the
TUI blocks on `wait_for_tool_result` before processing anything else.

Hermes TUI is the canonical example: synchronous, no progress indicator,
120s tool-call timeout, then the harness's sampling callback never lands
because the TUI is still waiting on the original tool call to return.

```
Hermes TUI                          radiant mcp serve
  │                                       │
  │── tool_call: radiant_possess(task) ───►
  │   (TUI blocked waiting here)            │
  │                                       │── discover (offline) ✓
  │                                       │── plan (offline) ✓
  │                                       │── execute needs sampling
  │                                       │── sampling/createMessage ──► ??? 
  │                                       │     (callback is dropped
  │                                       │      or queued behind
  │                                       │      tool-call wait)
  │                                       │
  ▼                                       ▼
120s timeout on the tool call → fail
```

## What v3.7.1 actually fixed (and what it didn't)

| Host | Failure mode | v3.7.1 fix |
|---|---|---|
| Claude Code | (works) | — |
| Codex CLI | MCP subprocess without env → driver -32601 → empty state.json | sentinel + self-driven fallback ✓ |
| **Hermes TUI** | synchronous tool call → sampling callback never lands → 120s timeout | **not fixed** — deadlock is at the protocol layer, before the driver |
| Cline/OpenCode/Kimi/OpenClaw/MiniMax | depends on subprocess env propagation | partial (probe cache helps) |

## The v3.7.2 design: decompose possess into async primitives

The pre-v3.3.0 design wrapped the loop in a single `radiant_run(goal=…)`
MCP tool. That hit timeouts on real hosts (Hermes 20–40s × 4 phases = 160s;
Codex didn't see the tool at all). v3.3.0 refactored to bounded primitives
(`skill_list`, `skill_load`, `possess`, `phase_status`). v3.7.2 goes one step
further: **decompose possess itself into async primitives** so even the
possess loop doesn't trap synchronous hosts.

### Primitive 1: `radiant_run_gate(phase, task, workdir)`

Spawns ONE phase (discover / plan / execute / verify) as a subprocess,
returns immediately with a `ticket` + `state_path`. The phase runs to
completion (or failure) in the background; the host agent polls
`radiant_phase_status(ticket=…)` until done.

**No `sampling/createMessage` round-trip** — the gate phase is offline:
discover scans the filesystem, plan reads skill content + writes spec,
execute runs scripts/run.sh and the host agent's own tools (the gate
orchestrator does NOT call sampling back to the host).

This is what already exists in `internal/loop/Loop.RunOnePhase` — we just
expose it via MCP.

### Primitive 2: `radiant_possess_async(task, workdir)`

Returns immediately with a `ticket` and `workdir/state.json` path.
Internally launches a background `radiant run` subprocess that drives
discover → plan → execute → verify asynchronously. Host agent polls
`radiant_phase_status(ticket=…)` for progress; on completion, reads
artifacts from the workdir.

The MCP call itself takes <500ms (just process spawn + ticket return).

### Refactor: `radiant_possess` becomes a polling wrapper

When called from a host, `radiant_possess` now detects host capabilities
(probe cache) and:

- **Async-capable host** (Claude Code, etc.): existing behaviour — calls
  `sampling/createMessage` per phase, drives interactively.
- **Synchronous host** (Hermes TUI): internally fires
  `radiant_possess_async` as a subprocess, polls `phase_status` until
  done, returns the final state. **Still blocks the tool call** but
  each phase round-trip is bounded because the subprocess drives itself
  without bouncing sampling back to the host.

The synchronous `radiant_possess` on Hermes still costs the host agent
the full wall-clock time, but no sampling-callback deadlock.

### Trade-offs

| Aspect | Old (v3.7.1) | New (v3.7.2) |
|---|---|---|
| Hermes deadlock | yes | no (sample callbacks replaced by local subprocess polling) |
| Tool-call latency | 120s timeout = fail | 500ms return + polling |
| Agent-side complexity | simple — 1 tool call | medium — must poll `phase_status` |
| Progress visibility | none until timeout | `phase_status` per poll |
| Code complexity | low | medium — async subprocess management |
| Backward compat | n/a | full — old code keeps working |

## Implementation plan

### Step 1 — design + types (completed)

Files:
- `cmd/radiant/cmd_mcp_run_gate.go` — MCP tool handler for one offline phase
- `cmd/radiant/cmd_mcp_possess_async.go` — MCP tool handler for the full offline loop
- `internal/possess/async.go` — `AsyncGate` interface, ticket generation, state-path helper
- `cmd/radiant/cmd_mcp_runtime.go` — register the 2 new tools
- `cmd/radiant/cmd_mcp_possess.go` — refactor: `radiant_possess` for sync hosts fires `possess_async` internally

### Step 2 — real implementation (completed for self-driven offline mode)

- MCP handlers persist state to `.radiant-harness/state/<ticket>/`.
- `radiant_run_gate` runs one self-driven phase.
- `radiant_possess_async` runs all four self-driven phases.
- `radiant_phase_status` reads the persisted state.
- Tests:
  - `TestRunGate_DiscoverOffline`
  - `TestRunGate_PlanThenExecute`
  - `TestRunGate_RejectsInvalidPhase`
  - `TestPossessAsync_AllPhasesOffline`

### Step 3 — AGENTS-FOR-TASKS.md update

Replace the "use bounded primitives + Python/bash directly" workaround with:
- "On async hosts (Claude Code): use `radiant_possess` as before."
- "On synchronous hosts (Hermes TUI): use `radiant_possess_async` + poll
  `radiant_phase_status`, or call `radiant_run_gate` per phase."

### Step 4 — release notes

CHANGELOG.md `[3.7.2]` with:
- Async primitives added
- Synchronous host deadlock closed
- `radiant_possess` now self-detects host and picks sync/async path
- Migration: existing code keeps working; new code can adopt async primitives
  to get progress visibility on synchronous hosts

## What's NOT in v3.7.2

- Real-time streaming of progress (websocket to host) — TBD future version
- Full background subprocess detachment for long-running async execution — TBD future version
- Multi-host async orchestration (Fleet mode async) — TBD future version

## Decision needed

Approve this proposal so the v3.7.2 workstream can land in 3-4 PRs:

1. PR-A: this proposal + skeleton stubs (compiles, returns "in development")
2. PR-B: real `AsyncGate` + subprocess plumbing
3. PR-C: `radiant_possess` refactor (sync host auto-async)
4. PR-D: docs (CHANGELOG + AGENTS-FOR-TASKS) + release tag v3.7.2

Or: skip v3.7.2 if the bounded-primitive hybrid pattern (v3.7.2-prep
docs in CHANGELOG) is enough for Hermes users. That's a judgment call.

The hybrid pattern works today; v3.7.2 just makes the synchronous-host
experience match the async-host experience (single tool call, real
execution, no 120s hang).
