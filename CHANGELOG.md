# Changelog

All notable changes to `radiant-harness` (Light) are documented here. The
format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
the project adheres to [Semantic Versioning](https://semver.org/).

## [3.7.2-prep] — 2026-06-29 — Skill name drift fix + Hermes TUI workstream

Two fixes ship together because both surfaced in the same Hermes TUI review
session (2026-06-29 23:55).

### Skill name drift in selfDrivenSkillHints (FIXED)

`cmd/radiant/cmd_mcp_possess_self_driven.go::selfDrivenSkillHints` listed
4 skills that don't exist as bundled directories. When a user task
contained a generic keyword like "credit" (without the `-risk-modeling`
suffix), the first pass picked the ghost skill, the host tried to load
it, got nothing, and the scaffold was orphaned.

| hint keyword | hint value (ghost) | real bundle |
|---|---|---|
| `credit` | `credit-risk-modeling` | `credit-risk` |
| `risk` | `risk-management` | `credit-risk` (default; specific risks hit second-pass verbatim) |
| `model`, `ml`, `forecast` | `ml-modeling` | `ml` |
| `compliance`, `regulatory`, `basel`, `ifrs` | `regulatory-compliance` | `regulatory` |

**Fixed:** hints renamed to real bundled skill names. Updated
`cmd_mcp_runtime.go` tool descriptions (lines 58, 67, 355) and
`internal/casetest/canned.go` test data (lines 31, 33, 36) that
mentioned the ghost names.

### Drift detector (ADDED)

**`scripts/audit-skills.sh` + `make audit-skills`** — extracts every
`{"keyword", "skill-name"}` pair from the hint map and asserts each
skill name has a corresponding `internal/skill/skills/<name>/SKILL.md`.
Closes the regression class — same pattern as `make audit-docs` from
the v3.0.0 doc-drift fix.

```
$ make audit-skills
audit-skills: 6 hint(s), 69 bundled skill(s)
  ✓ present: camada-agentica credit-risk fraud-detection ml nova-feature regulatory

✓ all hint map entries reference real bundled skills
```

### Hermes TUI synchronous workstream (DOCUMENTED)

v3.7.1 release notes framed the fix as "closes Codex hollow-stub trap".
In practice the user expectation was that Hermes TUI also worked. It
doesn't — Hermes TUI implements synchronous tool calls (sync
`wait_for_tool_result`), so the harness's `sampling/createMessage`
callback never gets processed: Hermes is blocked waiting for
`radiant_possess` to return while the harness waits for sampling to
complete. Deadlock guaranteed at the execute phase, regardless of
v3.7.1.

**Recommended workstream for Hermes TUI hosts:**

```
1. radiant_skill_list    ← enumerates bundled skills (no round-trip)
2. radiant_skill_load    ← reads SKILL.md content (no round-trip)
3. radiant_init / radiant_create_spec  ← scaffolds the spec/tasks (no round-trip)
4. Python / bash direto  ← fills the [host-agent: ...] markers
```

Each tool is small, returns fast, doesn't trap the TUI. Hermes
becomes the research/spec-writer/skill-loader; the actual execution
lives in the same chat where Python/bash run. This is the same
hybrid pattern that solved the iFood Pago MenuFlex case (2026-06-29).

**Operational rule (carried forward):** `radiant_possess` via MCP
is a dead end on Hermes TUI. The supported path on synchronous
hosts is the bounded primitive pattern above. Workstream v3.7.2 will
add async primitives (`radiant_run_gate`, `radiant_possess_async`)
so even `radiant_possess` doesn't trap the TUI — but until then,
treat Hermes as a hybrid host.

### Verified

- `go test ./...` — 31 packages PASS, 0 FAIL.
- `make smoke` — 17/17 OK.
- `make audit-skills` — 6/6 hints reference real bundled skills.
- `go vet ./...` — clean.

### Files changed (commit c830e3b)

- `cmd/radiant/cmd_mcp_possess_self_driven.go` — hint map renamed
- `cmd/radiant/cmd_mcp_runtime.go` — tool descriptions updated
- `internal/casetest/canned.go` — test data updated
- `Makefile` — added `audit-skills` + `audit-docs` PHONY targets
- `scripts/audit-skills.sh` — new drift detector

## [3.7.1] — 2026-06-29 — Agentic driver -32601 fallback closes Codex hollow-stub

A Codex CLI run at 2026-06-29 23:44 produced

```json
"phases": { "discover": { "status": "error",
  "error": "sampling at iter 1: sampling unsupported on host
   (json-rpc -32601) (method=sampling/createMessage)" } }
```

in `~/Downloads/gpt-5-codex/.radiant-harness/state/possess-0371d5f41b4ecd67/state.json`.
The harness bootstrapped the project (AGENTS.md / docs / specs / scripts /
.radiant-harness/ + 70+ bundled skills), then attempted the agentic driver,
which the Codex MCP server rejected with `-32601`. v3.7.0 surfaced the error
as fatal and exited. Result: empty docs/specs/scripts on disk, agent forced
to fill by hand.

### Root cause

Codex CLI spawned `radiant mcp serve` as an MCP subprocess without
propagating `CODEX_HOME` (the agent host runs in its own env namespace).
`hostdetect.Detect()` inside the harness returned `AgentUnknown`, so the
v3.7.0 pre-flight short-circuit (which required `detected.Agent !=
AgentUnknown` to trigger self-driven) didn't fire. The agentic driver ran,
made one sampling call, and got `-32601`.

### Fixed

- **`internal/possess/driver.go`** — new sentinel
  `ErrHostSamplingUnsupported`. When the first `ChatWithTools` call
  surfaces `ErrSamplingUnsupported`, the driver returns this sentinel
  wrapped instead of letting the error propagate as generic.

- **`cmd/radiant/cmd_mcp_possess.go::routeAgenticErr`** — new helper
  inspects the agentic driver's error. Sentinel match → route to
  `runSelfDrivenPossess` with reason `"sampling unsupported mid-run
  (driver fallback v3.7.1)"`, persist the probe evidence, return success.
  Any other error propagates unchanged.

- **`runPossessWithBackend`** — driver call site now wraps the helper.
  Pre-flight short-circuit path is unchanged (still requires positive
  `detected.Agent` to skip sampling; an open follow-up described below
  relaxes this).

### Tests added

In `cmd/radiant/cmd_mcp_possess_test.go`:

- `TestRouteAgenticErr_FallsBackOnSamplingUnsupported` — sentinel
  triggers the self-driven scaffold; asserts `specs/*/spec.md` exists.
- `TestRouteAgenticErr_PropagatesUnrelatedErrors` — non-sentinel
  errors do NOT silently downgrade.

Hand-rolled integration test (in `internal/possess/driver_test.go`)
that runs `runPossessWithBackend` with a backend stub returning
`ErrSamplingUnsupported` on every `ChatWithTools` call. Pre-fix:
state.json shows `discover: status=error`. Post-fix: state.json shows
all 4 phases done via self-driven fallback; workdir lands with 196
files including `spec.md`, `tasks.md`, `CONTEXT.md`, `handoff.md`,
`verify.md` with `[host-agent: fill in …]` markers.

### Verified

- `go test ./...` — 31 packages PASS, 0 FAIL.
- `go test ./internal/possess/ ./cmd/radiant/` — 5 regression tests PASS:
    - `TestDriverRunsToolsAndStopsOnVerdict` (v3.7.0 happy path)
    - `TestDriverFallsBackWhenModelNeverCallsTools` (v3.7.0)
    - `TestRunPossessWithBackendFallsBackToSelfDriven` (v3.6.x)
    - `TestRouteAgenticErr_FallsBackOnSamplingUnsupported` (v3.7.1)
    - `TestRouteAgenticErr_PropagatesUnrelatedErrors` (v3.7.1)
- `make smoke` — 17/17 OK (whitelist accepts `v3.7.1`).
- Hand-traced the exact failing scenario from the 23:44 Codex run:
  `--workdir` on `/Users/henrique/Downloads/gpt-5-codex`-style
  layout → 196 files populated, no `discover: error` phase.

### Diagnostic pattern (carried forward)

Whenever a fallback bug ships, write a hand-rolled integration test
that reproduces the *exact* failing scenario from production —
the bug class from the prior round (`v3.5.1` `-32601`) was
documented in `internal/llm/sampling.go` and the memory but no
test exercised the agentic path end-to-end. v3.7.1 closes that
test gap. Future "host-doesn't-support-X" failures get caught by
the same pattern.

### Known follow-up (open, NOT in 3.7.1)

- **Pre-flight over-reliance on positive detection.** Today
  `runPossessWithBackend` only routes to self-driven when
  `detected.Agent != AgentUnknown && ResolveSupport(detected).
  Supports == false && probed`. When Detect returns AgentUnknown
  (e.g. MCP subprocess with truncated env), the pre-flight skips
  and the driver runs. v3.7.1 catches the failure at the
  sentinel level — works in practice. The cleaner fix is a
  pre-flight that doesn't require positive detection:
  "if ANY probe cache row says supports=false, OR ANY agent in
  knownSamplingUnsupported would land here, route to
  self-driven regardless of detected.Agent". Tracked for v3.7.2.

- **`setup-mcp --pass-env=` flag.** Lets the host agent enumerate
  env vars that should propagate to the `radiant mcp serve`
  subprocess. Codex / Claude Code / OpenCode would each pass
  their respective agent identifier env. Tracked for v3.7.2.

### Operational rule (carried forward)

Multi-stage fallbacks need explicit sentinels AND `errors.Is`
matches in the caller. The driver returning
`ErrHostSamplingUnsupported` instead of a generic error, AND
the caller refusing to silently downgrade on anything else —
that's what closes the gap. Comments like "see § Why this is
the only path" aren't enough; the type system has to enforce
the contract.

## [3.7.0] — 2026-06-29 — Agentic tool-calling driver

v3.6.x closed the **hollow stub** problem (a host that doesn't speak
sampling/createMessage gets a templated scaffold instead of empty
dirs) and the **probe lying** problem (SupportsSampling is now a
runtime-verified value, not a constant). What v3.6.x didn't do is
**drive** real work via sampling: even on hosts that DO implement
sampling, the harness only emitted Markdown the agent then had to
apply by hand.

v3.7.0 ships the agentic driver that makes `mcp__radiant__possess`
emit real tool calls when the host supports them.

### Added

- **Optional `llm.ToolCapable` interface** (`internal/llm/tools.go`).
  Backends that implement `ChatWithTools(messages, tools, choice)`
  opt into the agentic surface. Existing backends don't break; the
  driver asserts at construction time so silent fallback is
  impossible — better to fail loud than to lose the capability and
  have the operator discover it from a fall-through to text-only.

- **Wire-format tool support in `SamplingBackend`**
  (`internal/llm/sampling.go`). `samplingParams.Tools` and
  `ToolChoice` serialise to `sampling/createMessage` with the
  Anthropic shape (most host MCP proxies accept this transparently).
  `parseSamplingContent` lazily decodes the response: pure-text
  → legacy `text` channel; array with `tool_use` blocks → new
  `rendered` channel that the agentic driver reads.

- **Named `ChatResponseChoice` + `ChatResponseMessage`** in
  `internal/llm/types.go` with `ToolCalls []ToolCall`. Wire format
  mirrors Anthropic's `content: [{type, ...}]` blocks. The previous
  inline anonymous struct kept backward compatibility for callers
  reading `resp.Choices[0].Message.Content` — the new shape has the
  same field access but adds `ToolCalls` next to it.

- **`internal/possess/driver.go`** — the agentic loop:
    1. Build the wire tool manifest once (`Registry.Names()` →
       `llm.Tool{Name, Description, JSON schema from Params}`).
    2. Per iteration: `ChatWithTools(messages, tools, choice)`
       → if response text contains `VERDICT: APPROVED|REJECTED`
       or `REVIEW: PASS|FAIL`, capture verdict and stop.
       → else dispatch every `ToolCall` via
       `Registry.Call(ctx, name, input)` against the built-in
       tools (`read_file`, `write_file`, `search_code`,
       `run_gate`).
       → append a `tool_result` echo as the next user message,
       loop until VERDICT, `MaxIter`, `MaxWall`, or
       `context.Cancel`.
    3. Returns `Trace` with `Iterations`,
       `ToolInvocations []ToolRecord`, `TextSoFar`,
       `Wall`, `Verdict`, so the caller can surface it.

  `MaxIter` defaults to 25 (lean/standard/thorough profiles
  override). `MaxWall` defaults to 10 min. Emits a loud
  `ErrBackendToolsUnsupported` after 2 consecutive text-only
  turns so callers know to fall back.

- **`runPossessWithDriver`** in
  `cmd/radiant/cmd_mcp_possess.go::runPossessWithBackend` —
  agentic system + task prompts that establish role, the strict
  VERDICT/REVIEW format, and the explicit tool-use rules ("don't
  end with VERDICT until you've written spec.md, tasks.md, and at
  least one runnable artefact AND run a gate that passed"). The
  agentic driver runs once and consumes the whole 4-phase loop;
  the prior "split by phase" shape is gone because tool_use /
  tool_result is inherently interleaved.

- **3 regression tests** in
  `internal/possess/driver_test.go`:
    - `TestDriverRunsToolsAndStopsOnVerdict` — scripted 4-turn
      happy path (list_dir → read_file → write_file →
      verdict text); asserts tool-invocation order, IDs, and
      re-sampling count.
    - `TestDriverFallsBackWhenModelNeverCallsTools` —
      `ErrBackendToolsUnsupported` after 2 text-only turns.
    - `TestNewDriverRejectsNonToolableBackend` —
      `NewDriver` refuses a non-`ToolCapable` Backend with a
      loud error.

### Changed

- **`runPossessWithBackend`** — pre-flight + dispatch. If
  `ResolveSupport(detected).SupportsSampling == false`, skip
  the driver and fall through to `runSelfDrivenPossess` (v3.6.x
  behaviour). Otherwise, if the backend implements
  `llm.ToolCapable`, dispatch to the agentic driver. Otherwise
  fall through to the v3.6 per-phase text sampling path.

### Fallback matrix (consolidated reference)

| Host sampling capability | Path taken |
|---|---|
| sampling + tools (Claude Code / Anthropic, future native hosts) | Agentic driver — model calls tools, harness dispatches via Registry, real files written + gates run |
| sampling, model never calls tools despite manifest | Driver bails with `ErrBackendToolsUnsupported` after 2 text-only iterations |
| sampling -32601 mid-run (Codex GPT-5, Hermes mimo) | Self-driven scaffold fallback (v3.6.x contract) |
| No `tools` propagation at all | Per-phase text sampling (legacy) |
| `ResolveSupport` reports `SupportsSampling=false` (probe-verified or well-attested) | Self-driven scaffold (skip sampling) |

### Verified

- `go test ./...` — 31 packages, **0 FAIL**.
- `go test ./internal/possess` — **3 new regression tests PASS**.
- `make smoke` — `17/17 OK` (whitelist accepts `v3.7.0`).
- `make release` — 6 binaries built cleanly (`radiant-{linux,darwin,windows}-{amd64,arm64}`).

### Operational rules (carried forward)

1. **Silent fallback is the enemy of progress.** `NewDriver`
   refuses a non-`ToolCapable` Backend with a loud error because
   silent downgrade to text-only would hide the capability loss
   from the operator. Same rule applies to any future "the host
   MAY do X but doesn't" detection: surface the gap, let the
   operator decide.

2. **Wire + driver + integrator land together.** A "tool-calling
   agentic loop" requires three primitives: (a) wire-format
   support on the sampling layer, (b) driver loop with
   VERDICT/REVIEW short-circuit + tool_result echo,
   (c) integrator that constructs the Registry per-project and
   weaves the fallback path. Missing any one makes the loop a
   no-op. The v3.7.0 PR lands all three at once because the
   tool scaffolding (`internal/tools/{fs,gate}` from Sprints
   69–72) was already in place — the gap was wire + driver +
   integrator.

3. **No new tool with broader surface was introduced.** The
   four built-in tools (`read_file`, `write_file`,
   `search_code`, `run_gate`) already enforced project-boundary
   via `fsutil.PathIsSafe` and a policy allowlist for shell gates.
   The agentic driver plugs into existing infrastructure; future
   "add tool X" work doesn't need another architectural shift.

## [3.6.0] — 2026-06-29 — Self-driven possession for hosts without sampling

v3.5.1 fixed the crash but left a **hollow stub** behind: when a host
agent didn't implement `sampling/createMessage` (Codex GPT-5 first;
Cline/OpenCode/Kimi/OpenClaw/VSCode/`mavis-code` next in line on
faith-only defaults), `mcp__radiant__possess` exited 0 with empty
`docs/`, `specs/`, `scripts/` directories. The host agent then had
to fill them in by hand — exactly the work the harness is meant to
absorb. v3.6.0 fixes the **promise gap**, not just the crash.

### Added

- **Persistent capability state** (`internal/hostdetect/probe.go`).
  Empirical probe results now persist to
  `~/.radiant-harness/agent-capabilities.json` (atomic rename,
  schema-versioned). `hostdetect.ResolveSupport(agent)` returns the
  probed value first, falls back to the declared constant, and
  finally to the new `knownSamplingUnsupported` map. `Detect()`
  reports a new `SamplingSource` field (`"probe" | "declared"`)
  so the user can see whether the answer is empirical or aspirational.
  Codex is pinned in `knownSamplingUnsupported` so the very first
  `radiant_possess` call on a fresh Codex box dispatches to the
  self-driven path with zero probe latency.
- **Self-driven possession pipeline**
  (`cmd/radiant/cmd_mcp_possess_self_driven.go`). When
  `ResolveSupport` reports `false` (probe-verified or well-attested),
  **or** when a live sampling call returns JSON-RPC `-32601`
  mid-run, possession routes to a deterministic 4-phase pipeline
  that emits **usable artifacts** instead of placeholders:
  - `.radiant-harness/CONTEXT.md` — project fingerprint + suggested
    bundled skills (matched against task keywords)
  - `specs/0001-<slug>/spec.md` + `tasks.md` — templated with
    `[host-agent: fill in ...]` markers so the next agent knows
    exactly which sections need real content
  - `scripts/run.sh` — templated entrypoint the host agent replaces
  - `docs/README.md` — overview + layout map of every templated file
  - `.radiant-harness/handoff.md` — instructions for the next agent
  - `.radiant-harness/verify.md` — templated-vs-filled report so the
    state file tells the truth about what still needs the host's
    understanding

  The state machine and `radiant_phase_status` shape are identical to
  the LLM-driven path; downstream tools see the same `done` values.

- **Safe CLI commands became public**
  (`cmd/radiant/main.go::publicCommands`). `radiant spec`, `radiant
  audit`, `radiant skills`, and `radiant context` no longer require
  `RADIANT_INTERNAL=1`. Hosts with shell access (Hermes, Codex's
  bash, any agent that runs shell) can now drive the SDD pipeline by
  itself when MCP sampling is unavailable: scaffold `spec.md`, audit
  the layout, list skills, assemble context — none of these touch
  the LLM loop or bypass the harness contract. Dangerous surfaces
  (`loop`, `run`, `fleet`, `eval`, `models`, `train`, `predict`,
  `drift`, `profile`, `autodata`, `evaluate`, `incident`, `stats`,
  `causal-estimate`, `integrate`, `improve`, `bench`, `budget`,
  `worktree`, `state`, `handoff`, `tools`, `pricing`, `telemetry`)
  remain gated.

- **`setup-mcp` pre-flight warning**
  (`cmd/radiant/cmd_setup_mcp.go`). Before writing the MCP config, if
  `hostdetect.ResolveSupport` returns `probed=true && supports=false`
  for the detected agent, the command now prints a 5-line warning
  explaining that the harness will route to self-driven scaffolding
  and pointing the user at `AGENTS-FOR-TASKS.md`.

### Changed

- **`runPossessWithBackend`** — pre-flight check against
  `ResolveSupport` before any sampling call, plus graceful handoff
  to `runSelfDrivenPossess` mid-run when a phase fails with
  `-32601`. Previously it returned an error and exited; now it
  records the probe evidence and switches to the deterministic
  pipeline so the rest of the phases still produce real artifacts.
- **`runPossessForCLI`** — the hidden debug mirror
  (`radiant mcp possess --task=...`) now also honours
  `hostdetect.ResolveSupport`. Override with
  `RADIANT_FORCE_SAMPLING=1` to exercise the legacy stub path on
  purpose.

### Fixed

- **`cmd_mcp_possess_test.go`** — rewrote the regression test
  (`TestRunPossessWithBackendFallsBackToSelfDriven`) to assert the
  v3.6.0 contract: every phase reaches `done`, no phase error is
  left over, and `specs/0001-*/spec.md` contains the
  `[host-agent: fill in ...]` marker. The previous test asserted the
  v3.5.1 failure path; the rename is the contract change.
- **`scripts/smoke-test.sh`** — version whitelist accepts
  `v3.6.0` (was missing `v3.5.1` between v3.5.0 and this release).
- **`cmd/radiant/cmd_setup_mcp.go`** — fixed a long-standing
  `fmt.Printf` arity bug in the `--dry-run` branch (had 3 args, 2
  expected — printed without ever surfacing the content).

### Lesson (carried forward)

**Graceful degrade is only graceful when the degraded path still
does useful work.** v3.5.1 fell back to "echo the same thing and
call it success" — the user discovered the lie 4 minutes later
when their project had empty `docs/`, `specs/`, `scripts/`. Empty
placeholders are worse than a clear failure: they produce the same
surface area as a real scaffold without any of the payload.

For every future fallback: ask "what does the user actually GET?"
If the answer is "the same surface area with no payload", the
fallback is a hollow stub masquerading as graceful — and it should
not ship.

### Verified

- `go test ./...` — all packages PASS.
- `go test -run TestRunPossessWithBackend` — PASS (regression for
  the v3.5.1 failure).
- `make smoke` → `17/17 OK` (whitelist bumped to v3.6.0).
- `make test-agents` → `12/12 PASS` (cross-agent install matrix).
- Hand-trace: `radiant mcp possess --task="…" --workdir=…` against
  a `~/.radiant-harness/agent-capabilities.json` with
  `codex.supports_sampling=false` produces the full templated
  scaffold (`.radiant-harness/CONTEXT.md`, `spec.md`, `tasks.md`,
  `scripts/run.sh`, `docs/README.md`, `.radiant-harness/handoff.md`,
  `.radiant-harness/verify.md`); state machine reaches `done` for
  every phase; spec.md carries the `[host-agent: fill in …]`
  marker the next agent looks for.

## [3.5.1] — 2026-06-29 — Possession flow works on every host

Two interlocking bugs surfaced in production on 2026-06-29 with
Hermes+mimo and Codex+GPT-5 on a real credit-risk case
(`case_modelagem_risco_credito_menu_flex_candidato.zip`). Both were
prompted by the case itself; both made `mcp__radiant__possess` look
broken to the host agent (it created the empty `.radiant-harness/` /
`specs/` scaffold and exited without producing any artifacts).

**Bug A — `-32601` from `sampling/createMessage` (Codex).** Codex
GPT-5 returns JSON-RPC "method not found" when the harness asks
for sampling. We had marked `AgentCodex.SupportsSampling = true`
on faith in v3.0.0 without empirical verification. Once we see -32601,
the possession flow should NOT abort; it should fall back to a
deterministic stub so the host agent can drive the work using its
own tools.

**Bug B — XML hallucination on mimo (Hermes).** The four phase
prompts (`discover` / `plan` / `execute` / `verify`) all instructed
the LLM to invoke tools it didn't have access to:

  - plan: _"Write to specs/.../tasks.md **using Write tool**"_
  - execute: _"Write files **with Write tool**. Run the gates **with Bash**."_

Sampling params don't carry a `tools` array (intentionally — the
v3.3.0 architecture splits planning from execution; the host does
the writing). When the LLM sees the prompt and tries to honor it,
function-calling-aware models respond with structured calls and fail,
function-calling-less models (mimo / Xiaomi MiMo) generate `<function=…>`
XML as text and the harness dies on parsing it.

### Fixed

- **`internal/llm/sampling.go`** — added `ErrSamplingUnsupported`
  sentinel + `IsSamplingUnsupported(err)` helper. When the host
  replies with JSON-RPC `-32601` for `sampling/createMessage`,
  `Dispatch()` wraps the error so callers can branch.
- **`cmd/radiant/cmd_mcp_possess.go`** —
  - `phasePrompts` rewritten: all four phases are now **text-only**.
    The LLM outputs Markdown / fenced code blocks / VERDICT lines;
    it never asks for tools it doesn't have. Host agent reads the
    output and applies it with its own Read/Write/Bash.
  - `runPossessWithBackend` detects `ErrSamplingUnsupported`, logs a
    one-shot warning, and short-circuits remaining phases to a
    deterministic `[stub — host sampling unsupported]` placeholder.
    The state file is still written, the artifact list still
    includes `specs/…/`, the host can read the placeholders and
    fill them in.
- **`internal/hostdetect/hostdetect.go`** — `agentSignature.SupportsSampling`
  is now explicitly documented as **declared, not verified**. The
  empirical check is in the sampling dispatch layer.

### Verified

- `make test-agents` → 12 agents; 12 PASS, 0 FAIL (regression-free).
- `make smoke` → all green.

### Known limitations

- The `sampling.unsupported` fallback puts placeholders in the state
  but doesn't generate specs/ tasks.md / verify-report.md from the
  host agent's own tool calls. A real Codex run still produces the
  crate — see Codex's 2026-06-29 credit-risk run for a worked
  example. Future work: have the possession CLI nudge the host agent
  to fill the stubs in via a single recovery tool call.
- We still don't send a `tools` array in `samplingParams`. Adding it
  requires implementations in `Chat()`, the rotation/JSON of
  sampling params, and a per-host capability negotiation. Deferred
  until a real agent exposes function calling capability.

## [3.5.0] — 2026-06-29 — `make test-agents` (12/12 PASS)

Adds a **cross-agent install/validation matrix** that simulates each of
the 12 supported host agents (Claude, Cursor, Hermes, Kimi, OpenClaw,
Codex, Cline, OpenCode, Windsurf, Zed, VS Code Copilot, MiniMax Code)
in a sandbox HOME + project, runs `radiant setup-mcp`, then verifies
the config landed at the expected path **and** `radiant doctor --mcp`
correctly detects the entry.

The Sprint 5 deliverable is the matrix tool itself — running it
already surfaced six real layer-coordination bugs that had been
hidden since v3.2.0:

  - `cmd_setup_mcp.go` and `cmd_doctor.go::mcpConfigPath` used **different
    agent names** (`"claude"` vs hostdetect's `"claude-code"`,
    `"mavis-code"` vs `"MiniMax-code"`). Aligned both layers to hostdetect.
  - OpenCode probe in `probeRadiantEntry` looked up a literal `"<test>"`
    placeholder; replaced with `"radiant"`.
  - OpenClaw probe walked `mcpServers.radiant`; OpenClaw actually stores
    `mcp.servers.radiant`. Added dedicated case.
  - Windsurf and Zed were in `cmd_setup_mcp` and `mcpConfigPath` but had
    **no fingerprint in `internal/hostdetect`**. Added both with
    matching env-var signatures.
  - `cmd_setup_mcp` for `cursor/windsurf/zed/vscode` hardcoded `cwd`
    even with `--global`; `scripts/test-agents.sh` now creates a
    sandbox `proj/` so we see those writes.

### Added

- **`scripts/host-agent-matrix.json`** — declarative env-var +
  config-path matrix for all 12 agents. Schema-versioned (`schema_version: 2`).
- **`scripts/test-agents.sh`** — runs the matrix; emits a Markdown
  report at `.radiant-harness/agent-matrix.md` (and JSON to stdout
  with `--json`). Per-agent bash scripts are generated via Python
  heredoc so env vars with spaces / paths / special characters round-trip
  cleanly. Each agent block starts with an `unset` prelude over **every**
  env var seen across the matrix, so leakage between agents never wins.
- **`make test-agents`** Makefile target — entry point for the matrix.
- **Aliases**: `scripts/test-agents.sh one <agent>` runs a single agent
  for debugging; `RADIANT=path/to/bin scripts/test-agents.sh ...` lets
  the user point at any local build.

### Changed

- `cmd/radiant/cmd_doctor.go::mcpConfigPath`: agent IDs now match
  `internal/hostdetect.AgentID`; paths now match `cmd/radiant/cmd_setup_mcp.go::mcpConfigFor`.
- `cmd/radiant/cmd_doctor.go::probeRadiantEntry`: openclaw walks
  `mcp.servers.radiant`; opencode walks `mcp.radiant`.
- `internal/hostdetect/hostdetect.go`: added `AgentWindsurf` and
  `AgentZed` (env var + parent-binary fingerprints) so they're no longer
  invisible to `radiant host-info` / `radiant doctor --mcp`.
- `scripts/smoke-test.sh`: accepts any 3.3.x / 3.4.x / 3.5.x release.

### Verified

```
$ make test-agents
12 agents; 12 PASS, 0 FAIL.
```

(Sprint 5 began at **2/12 PASS**. Each FAIL was a real upstream defect
in `cmd_doctor` or `internal/hostdetect`, not a script bug — the
matrix was the diagnostic tool.)

## [3.4.0] — 2026-06-29 — `radiant test-case`

Adds the **single most diagnostic command in the project** —
`radiant test-case <case.zip|dir>` — which drives the harness end-to-end
against a real subprocess with simulated sampling latency, exactly
reproducing the failure mode observed on 2026-06-29 with Hermes
mimo/xiaomi and Codex GPT-5.

### Added

- **`internal/casetest/` package** — small, no-deps code that:
  - extracts a `.zip` (with zip-slip guard) or accepts a directory;
  - reads the user prompt from `CONTEXT.md` / `context.md` / `README.md` /
    `case.md` / `case_description.md` (in that priority);
  - spawns `radiant mcp serve` as a real subprocess via stdio pipes;
  - drives the full JSON-RPC possession flow;
  - replies to `sampling/createMessage` requests after a configurable
    `cold-start-ms` (default 25 s — Hermes' actual cold start);
  - emits a Markdown report with per-phase timing, sampling call
    counts, the final assistant message, and a full event log;
  - sets `Outcome` (`success` | `critical_failure` | `incomplete`) by
    parsing the harness's `Exit:` field or counting completed phases.
- **`radiant test-case <path>` command** (`cmd/radiant/cmd_test_case.go`):
  - flags: `--cold-start-ms`, `--jitter-ms`, `--sampling-timeout`,
    `--profile`, `--report <path>`, `--keep-unpacked`, `--timeout`;
  - registered as a **public** command (no `RADIANT_INTERNAL` gate), on
    par with `radiant mcp self-test` and `radiant doctor`;
  - exits 0 only when the harness exits `success`.

### Verified — cold-start matrix on a menu_flex-shaped case

| cold-start | sampling calls | elapsed |
|------------|----------------|---------|
| 500 ms     | 4              | 2.01 s  |
| 2 s        | 4              | 8.01 s  |
| 5 s        | 4              | 20.0 s  |
| 25 s (Hermes-real) | 4     | 100.0 s |

The 25 s cold-start run reproduces exactly the path Henrique saw break on
Hermes. With the test-case harness, that path completes in 100 s —
comfortably below the harness's 130 s sampling timeout and any host's
300 s tool-call timeout.

### Verified — bug found and fixed during implementation

Implementing `test-case` exposed two regressions in the harness that
would have hit real hosts sooner or later:

1. `readJSONWithTimeout` was passing a `timeout` argument to
   `bufio.Scanner.Scan()`-equivalent code that ignored it entirely —
   the call could block forever. Fixed by extracting the read into a
   dedicated goroutine + channel with `time.After(timeout)` select.
2. Repeat runs against the same case dir would short-circuit to
   `phases done, success` because the harness cached possession
   state by `task_id`. The test-case driver now `os.RemoveAll`s the
   `state/` dir before spawning the harness subprocess.

### Fixed — `internal/casetest` (housekeeping)
- Bumped `var version` to `3.4.0` and the smoke-test assertion to
  accept any 3.4.x release.

### Verified
- `make smoke` → 17/17 OK
- `go test ./...` → 30 packages pass; 0 fail
- `radiant test-case <dir>` with realistic Hermes latency → 4 sampling
  calls, 100 s elapsed, Exit: success on 25 s × 4

[3.4.0]: https://github.com/quant-risk/radiant-harness/releases/tag/v3.4.0

---

## [3.3.2] — 2026-06-29 — install.sh PREFIX auto-mkdir

### Fixed
- **`install.sh`** now `mkdir -p`s the PREFIX directory before
  `install(1)`. Previously, calling with `PREFIX=~/.local/bin` (or any
  other non-existent path) failed at the install step with
  "install failed (does $PREFIX exist and is writable? try
  PREFIX=~/.local/bin)" — the error message itself pointed at the
  workaround instead of doing it.
  Real-world repro was `PREFIX=/tmp/install-fix-prefix bash install.sh
  --self-for-agent` from a fresh container.

### Verified — end-to-end
```bash
$ HERMES_VERSION=0.1 \
  WORKDIR=/tmp/install-fix-test \
  RADIANT_VERSION=v3.3.1 PREFIX=/tmp/install-fix-prefix \
  bash install.sh --agent=hermes --self-for-agent
==> downloading radiant-darwin-arm64
==> verifying SHA256
==> SHA256 OK
==> installing to /tmp/install-fix-prefix/radiant   # was: "install failed"
==> wiring MCP for host: hermes
==> agent-bootstrap files written to: /tmp/install-fix-test/.radiant-harness/
==> NEXT STEP for the agent in this directory:
==>   send /reload-mcp in this chat
```

- 5/5 MCP possession runs (URL shortener case)
- `make smoke` 17/17 OK
- 30 unit-test packages pass; 0 fail

[3.3.2]: https://github.com/quant-risk/radiant-harness/releases/tag/v3.3.2

---

## [3.3.0] — 2026-06-29 — Possession is the only way

This release rewrites the agent-side contract so that `mcp__radiant__possess`
is the single mandated path for non-trivial work. The previous
`radiant_run(goal=…)` exposed the entire autonomous loop as one MCP
tool call — it worked against synthetic sampling responders but failed
with real hosts (Hermes mimo/xiaomi 20–40 s cold start blew past the
300 s outer tool-call timeout; Codex GPT-5 didn't even see the tool).
The decomposition below makes each MCP tool bounded so the host agent
stays in control and no single call can time out.

### Added
- **`install.sh --self-for-agent`** — emits a host-specific restart
  hint (e.g. `send /reload-mcp in this chat` for Hermes,
  `restart the shell session that runs claude` for Claude Code)
  plus writes three bootstrap files to the project directory so the
  *next* agent that opens it has the contract at hand:
  - `.radiant-harness/AGENTS.md` — what to do, in Markdown
  - `.radiant-harness/INIT.json` — the same contract, machine-readable
  - `.radiant-harness/NEXT.txt` — a single-line next-step prompt
  This is the canonical "AI agent just received this repo" path.
- **`install.sh --workdir=<path>`** — control where the bootstrap
  files are written (default: `$WORKDIR` if set, else `$PWD`).

### Changed
- **Decomposed MCP surface.** Replaced the single `radiant_run` tool with
  a small, bounded-tool grammar the host agent calls step by step:
  - `mcp__radiant__skill_list()` — enumerate the 69 bundled skills.
    Always call this once before `possess` on a non-trivial task.
  - `mcp__radiant__skill_load(name)` — return the SKILL.md + frontmatter
    of one skill.
  - `mcp__radiant__possess(task, workdir?, profile?)` — main entry. The
    harness drives the agent through discover → plan → execute →
    verify, ONE sampling/createMessage round-trip per phase, with state
    persisted to `.radiant-harness/state/possess-<task-id>.json` between
    phases so a timeout or crash can resume from where it stopped.
  - `mcp__radiant__phase_status(task_id)` — return the persisted
    progress so the user can see trace / artifacts / gate results.
  - `radiant_run` is kept as a thin alias (`task` ← `goal`) for back-compat.
- **Each phase is bounded.** `cmd_mcp_possess.go` defines `discover`,
  `plan`, `execute`, `verify` as a fixed sequence. Each phase's prompt
  starts with an unambiguous `## radiant-phase: <name>` marker so the
  host (or our synthetic test host) can map a sampling request to the
  right phase without ambiguity.
- **JSON-RPC notifications silenced.** `notifications/initialized`,
  `notifications/cancelled`, `notifications/progress` are no longer
  answered with an error frame (`-32601 method not found`). Per
  JSON-RPC 2.0, a notification (no `id`) MUST NOT be answered;
  previously the error response was throwing off the next valid
  response on the same stdio line.
- **CLI escape hatches gated.** All non-public CLI commands
  (`radiant loop`, `radiant run`, `radiant fleet`, `radiant model`,
  `radiant profile`, `radiant evaluate`, `radiant drift`, `radiant spec`,
  `radiant product`, etc.) now refuse to run unless `RADIANT_INTERNAL=1`
  is set. The host agent now has no way to drive the harness via shell
  — the only path is `mcp__radiant__possess` via MCP.
  Public commands always available without `RADIANT_INTERNAL`:
  - `radiant setup-mcp [--agent=...] [--global]`
  - `radiant mcp` (serve, self-test, possess CLI mirror)
  - `radiant host-info`
  - `radiant doctor [--mcp]`
  - `radiant update`
- **`install.sh` now auto-wires MCP.** `--agent=<name>` (or `--setup-mcp`)
  runs `radiant setup-mcp --agent=<name> --global` after install,
  detects the host via `radiant host-info`, and writes the full Hermes
  sampling block when applicable.
- **`radiant --help`** header now explicitly tells incoming agents to
  scroll to the new **"🤖 For AI agents"** section.

### Added
- **`radiant mcp possess`** — debug/CLI mirror of `mcp__radiant__possess`
  (hidden subcommand, no sampling — useful for self-test and CI).
- **AGENTS.md bootstrap.** On first `radiant_possess` in a fresh project
  directory, the harness writes `AGENTS.md` + `docs/` + `specs/` +
  `scripts/` + `.agent-context/` with explicit instructions for any
  future AI agent that opens the directory.
- **State persistence between phases.** Each phase's output and
  status is written to `.radiant-harness/state/possess-<id>/state.json`
  atomically; calling `radiant_possess(task=…, workdir=…)` again
  with the same (workdir, task) tuple resumes from the failed or
  incomplete phase rather than restarting.
- **README "🤖 For AI agents" section.** Step-by-step recipe an agent
  follows verbatim: install + wire MCP → restart → drive via
  `mcp__radiant__possess`. Plus a failure-mode table.

### Verified — 5/5 MCP possession runs in /tmp/v330-case-N
Each run produced `AGENTS.md`, `docs/`, `specs/`, `scripts/`,
`.agent-context/`, `.radiant-harness/state/possess-<id>/state.json`,
with all four phases (`discover`, `plan`, `execute`, `verify`)
completed and recorded with their distinct outputs:

```
run 1  PASS
run 2  PASS
run 3  PASS
run 4  PASS
run 5  PASS
```

### Verified — install.sh --agent=hermes end-to-end
```text
$ curl -fsSL .../install.sh | bash -s -- --agent=hermes
==> downloading radiant-darwin-arm64
==> verifying SHA256
==> SHA256 OK
==> installing to /usr/local/bin/radiant
==> wiring MCP for host: hermes
  ✓ hermes     → /Users/.../.hermes/config.yaml

$ HERMES_VERSION=0.1 radiant doctor --mcp
  agent             hermes (confidence 100)
  config path       /Users/.../.hermes/config.yaml
  radiant entry     true
  sampling.enabled  true
  sampling.timeout  120s
  mcp timeout       300s
  verdict = OK
```

### Verified — internal gate
```bash
$ radiant loop start "x"           # without RADIANT_INTERNAL
radiant "loop" is an internal helper. To run the harness, the host agent
must invoke the MCP tool
  mcp__radiant__possess(task="<the user's prompt>", workdir="<cwd>")

$ RADIANT_INTERNAL=1 radiant loop start "x"
✓ Loop starting
```

[3.3.0]: https://github.com/quant-risk/radiant-harness/releases/tag/v3.3.0

---

## [3.2.9] — 2026-06-29 — Diagnosis is one command

### Added
- **`radiant mcp self-test`** — boots a child `radiant mcp serve` process,
  sends `initialize` + `tools/list` over stdio, returns PASS/FAIL with
  timing per call. Detects regressions in the MCP wire-up without needing
  a wired host agent. Exits non-zero on failure. Use:
  ```bash
  radiant mcp self-test            # 6 ms expected when wire-up is OK
  radiant mcp self-test --timeout 15s
  ```
- **`radiant doctor --mcp`** — inspects the host agent's MCP config file
  for an entry pointing to `radiant`. For Hermes (which requires an
  explicit `sampling:` block) it verifies `sampling.enabled` and reports
  both the sampling timeout and the outer MCP timeout. Reports the
  expected config path, whether it exists/writable/parseable, whether
  `radiant` is registered, and where to look to fix problems. Three
  report states:
  - **OK** — fully wired
  - **FAIL: not wired** — config file exists but no radiant entry
  - **FAIL: sampling not enabled** — radiant entry exists but Hermes
    will drop sampling/createMessage calls until the user runs
    `radiant setup-mcp --agent=hermes --global`
  - **FAIL: config not parseable** — file is corrupt YAML/JSON

### Verified — three Hermes `doctor --mcp` scenarios

OK (sampling enabled, all green):
```
$ HERMES_VERSION=0.1 HOME=/tmp/... radiant doctor --mcp
  agent                   hermes (confidence 100)
  config path             /tmp/.../.hermes/config.yaml
  path exists             true
  path writable           true
  config parseable        true
  radiant entry           true
  sampling.enabled        true
  sampling.timeout        120s
  mcp timeout             300s
  verdict = OK
```

FAIL (radiant present but `sampling:` missing):
```
  ✗  radiant entry exists but the `sampling:` block is missing or disabled
  Fix:
     add a `sampling: { enabled: true, timeout: 120, max_tool_rounds: 5 }` block under mcp_servers.radiant (re-run `radiant setup-mcp --agent=hermes --global` to write it for you)
  verdict = FAIL
```

FAIL (no radiant entry at all):
```
  ✗  radiant is not registered as an MCP server in this config
  Fix:
     run: radiant setup-mcp --agent=hermes
  verdict = FAIL
```

### Verified — MCP possession 5/5
```
run 1  Exit: success   build+test=PASS
run 2  Exit: success   build+test=PASS
run 3  Exit: success   build+test=PASS
run 4  Exit: success   build+test=PASS
run 5  Exit: success   build+test=PASS
```

### Verified — `mcp self-test`
```
$ radiant mcp self-test
radiant mcp self-test: PASS
  server         : radiant-harness 3.2.8
  tools          : radiant_run
  initialize     : 4 ms
  tools/list     : 0 ms
  total          : 6ms
```

### Fixed — `_test.go` file-name gotcha
The new `cmd_mcp_selftest.go` was originally written as
`cmd_mcp_self_test.go`. The Go toolchain treats any `*_test.go` file as a
test file and excludes it from `go build`. Renamed to drop the
underscore-test-infix and renamed the function from
`registerMCPSelfTestCmd` to match.

[3.2.9]: https://github.com/quant-risk/radiant-harness/releases/tag/v3.2.9

---

## [3.2.8] — 2026-06-29 — Hermes: works out of the box

### Changed
- **`cmd/radiant/cmd_mcp_serve.go`** adds three new flags:
  - `--cwd=<path>` — set the working directory before booting the loop.
    Empty (default) auto-detects project root by walking up from `$PWD`
    looking for `rad.yaml`, `.git`, `go.mod`, `package.json`,
    `Cargo.toml`, `pyproject.toml`, `setup.py`, `pom.xml`, `build.gradle`,
    `Gemfile`, or `composer.json`. Replaces the per-agent
    `radiant-mcp-<project>` shell wrapper every Hermes user had to write
    manually.
  - `--sampling-timeout=<duration>` — per-call timeout for
    sampling/createMessage. Go duration syntax (`90s`, `2m`, `1500ms`).
    Default: **120 s when an MCP host is wired** (was 5 s — that 5 s
    fallback was killing the 3rd call of any long possession loop when
    Hermes' underlying model had cumulative latency). Override via
    `RADIANT_SAMPLING_TIMEOUT` env var. Without an MCP host wired the
    legacy 5 s fallback still applies so plain CLI invocations fail fast.
  - `--model-hint=<name>` — MCP `modelPreferences.hint.name` (equivalent
    to `$RADIANT_MODEL`). Empty by default.
- **`internal/llm/sampling.go`** — `SamplingOptions` gains a `Timeout
  time.Duration`. The legacy `defaultSamplingTimeout = 5s` is still the
  fallback when this is zero, so non-MCP callers (`radiant loop`,
  `radiant run` from a shell) fail fast as before. The error message now
  uses whatever timeout was actually applied.

### Fixed
- **`cmd_setup_mcp.go` case `"hermes"`** + **`mergeHermesConfig`** now
  write the full Hermes sampling block (`sampling.enabled: true`,
  `sampling.timeout: 120`, `sampling.max_tokens_cap: 8192`,
  `sampling.max_tool_rounds: 5`) + outer `timeout: 300` to
  `~/.hermes/config.yaml`. Before this fix, the user had to edit
  `config.yaml` manually (via `pip install pyyaml` + a Python one-liner)
  to enable sampling — without it, Hermes silently drops
  `sampling/createMessage` calls and the harness exits with
  `critical_failure`. `radiant setup-mcp --agent=hermes --global` now
  produces a configuration that works on the first restart.
- **`cmd_setup_mcp_per_agent.go`** — `hermesEntry` struct gains
  `Timeout`, `Cwd`, `Sampling` fields (YAML-tagged so they round-trip).

### Verified — MCP possession 5/5
End-to-end MCP possession in 5 consecutive fresh runs against an empty
repo case (`build a tiny URL shortener in Go`), driven via Python MCP
host:

```
run 1  Exit: success   build+test=PASS
run 2  Exit: success   build+test=PASS
run 3  Exit: success   build+test=PASS
run 4  Exit: success   build+test=PASS
run 5  Exit: success   build+test=PASS

=== result: 5/5 ===
```

### Verified — Hermes setup-mcp dry-run + real write
Given a pre-existing `~/.hermes/config.yaml` with `model: xiaomi`,
`terminal: …`, `browser: …`, `radiant setup-mcp --agent=hermes --global`
now writes:

```yaml
mcp_servers:
  radiant:
    command: /usr/local/bin/radiant
    args: [mcp, serve]
    timeout: 300
    sampling:
      enabled: true
      timeout: 120
      max_tokens_cap: 8192
      max_tool_rounds: 5
```

…while preserving every other top-level key. No YAML editor required, no
Python needed.

### Docs
- **`README.md`** new **"Hermes quickstart"** section at the top of
  Installation — one copyable section with the exact 3-step recipe,
  including the resulting YAML.

[3.2.8]: https://github.com/quant-risk/radiant-harness/releases/tag/v3.2.8

---

## [3.2.7] — 2026-06-29 — installer, possession evidence, smoke fix

### Added
- **`install.sh`** — single-file one-shot installer. Detects OS/arch, downloads
  the matching binary from the latest GitHub release, installs to
  `/usr/local/bin/radiant`, verifies with SHA256SUMS, and (with
  `--setup-mcp`) wires MCP into the detected host agent. Replaces the
  multi-step curl/chmod/setup-mcp recipe.
- **`README.md` "Quickstart"** is now a one-liner: `curl -fsSL
  raw.githubusercontent.com/.../install.sh | bash` plus a `--setup-mcp`
  optional flag.

### Changed
- **`scripts/smoke-test.sh`** now embeds `-X main.version=$(git describe ...)`
  when it builds the binary locally, so the version assertion no longer falls
  back to the hardcoded `var version = "3.2.0"` default. Without this, the
  smoke test was checking `3.2.0` even when the real binary was `v3.2.6`.
- **`CHANGELOG.md`** catches up — entries for v3.2.1 … v3.2.6 added (the gap
  was real; this is now archived here).

### Verified — MCP possession 5/5
End-to-end MCP possession in 5 consecutive fresh runs against an empty repo
case (`build a tiny URL shortener in Go`), driven via Python MCP host:

```
run 1  Exit: success   Iterations: 0  build+test=PASS
run 2  Exit: success   Iterations: 0  build+test=PASS
run 3  Exit: success   Iterations: 0  build+test=PASS
run 4  Exit: success   Iterations: 0  build+test=PASS
run 5  Exit: success   Iterations: 0  build+test=PASS

=== result: 5/5 ===
```

Each run produced `main.go` + `main_test.go` from scratch, all 4 acceptance
criteria satisfied, `go build ./...` PASS, `go test ./...` PASS.

### Verified — installer
On macOS arm64:

```text
$ $BIN --version
v3.2.6-1-gf56efaf-dirty

$ make smoke    # 17/17 OK
OK: version reports 'v3.2.6-1-gf56efaf-dirty'
OK: no HTTP-LLM symbols in bin/radiant
OK: no API key references in bin/radiant --help
OK: command 'setup-mcp' present
OK: command 'mcp' present
OK: command 'host-info' present
OK: setup-mcp mentions 'claude' / 'cursor' / 'codex' / 'hermes' /
                           'kimi' / 'openclaw' / 'cline' / 'opencode'
OK: binary size: 10972050 bytes (≤ 15 MB)
```

[3.2.7]: https://github.com/quant-risk/radiant-harness/releases/tag/v3.2.7

---

## [3.2.6] — 2026-06-29 — document the possession flow

### Added
- **`README.md` "The 'possession' flow (for AI agents)"** section. Documents
  the four-phase loop the harness drives on a host agent via MCP
  `sampling/createMessage`: discover → plan → execute → verify. Explains
  exactly which response format the host must emit back to the harness
  (`VERDICT: APPROVED|REJECTED` for the verifier phase, `REVIEW: PASS|FAIL`
  for the post-convergence review panel).

### Notes
- This is a **documentation-only release.** No source changes. Bumps the
  install expectation so users who follow the README now understand the
  agent-side protocol.

[3.2.6]: https://github.com/quant-risk/radiant-harness/releases/tag/v3.2.6

---

## [3.2.5] — 2026-06-29 — fix MCP possession loop (3 bugs)

### Fixed
- **`internal/loop/review.go` `ParseReviewResponse`** now accepts
  `VERDICT: APPROVED|REJECTED` in addition to `REVIEW: PASS|FAIL`. Some host
  models (and our own MCP host Python script) emit the same verdict shape for
  both phases; the parser was rejecting them with `Exit: critical_failure`
  before any gate could run.
- **`internal/loop/verifier.go` `ParseVerifierResponse`** now uses a
  first-word match (`strings.Fields()[0] == "approved"`) instead of an exact
  equality. LLMs commonly append prose or escape characters after the
  verdict line ("VERDICT: APPROVED — gates green"); the exact match was
  trapping the harness in `consecutive_failures ≥ 3` and exiting.
- **`internal/loop/cycle.go` `validTransitions`** table now includes
  `PhaseVerify → PhaseDiscover`. Without this, a successful verify returned
  the state machine to verify and the loop deadlocked.

### Verified
After fix, fresh runs from an empty repo (counter MCP case) were
**3/3 `Exit: success`**.

[3.2.5]: https://github.com/quant-risk/radiant-harness/releases/tag/v3.2.5

---

## [3.2.4] — 2026-06-29 — copied example purged, security regex hardened

### Removed
- **`examples/pulse/`** was a copy from a different project (file with
  `github.com/Fortvna/...` package path) that was checked in by mistake.
  Deleted.

### Fixed
- **`internal/cmd_security.go`** regex patterns: added `\b` word boundaries so
  that words like `task-tracker-for-personal-use` no longer trigger the
  OpenAI key matcher as a false positive.

[3.2.4]: https://github.com/quant-risk/radiant-harness/releases/tag/v3.2.4

---

## [3.2.3] — 2026-06-29 — MiniMax Code as the 12th host agent

### Added
- **`internal/hostdetect/hostdetect.go`** recognises MiniMax Code via the
  `$MINIMAX_CODE_VERSION` / `$MINIMAX_CODE_HOME` / `$MINIMAX_CODE_CONFIG` /
  `$MINIMAX_PROJECT_ROOT` env vars. The 12 supported agents are now:
  Claude Code, Cursor, Windsurf, Zed, VS Code Copilot, OpenAI Codex, OpenCode,
  Hermes, Kimi CLI, OpenClaw, Cline, **MiniMax Code**.
- **`cmd/radiant/cmd_setup_mcp.go`** writes `.MiniMax/mcp.json` when the
  detected host is MiniMax Code.

[3.2.3]: https://github.com/quant-risk/radiant-harness/releases/tag/v3.2.3

---

## [3.2.2] — 2026-06-29 — deadlock, residue, and frontmatter fixes

### Fixed
- **`internal/llm/sampling.go`** had no timeout when no MCP host context was
  wired. Plain shell users (`radiant loop`, `radiant run`) would hang
  forever. Added a 5 s deadline; non-MCP calls now fail fast with a clear
  "no MCP host" error instead of deadlocking.
- **`cmd/radiant/cmd_run.go`** — the `--api-key` guard was removed from the
  Light binary. CLI usage: `radiant run --goal ... --max-iter ...`. Any
  reference to `RADIANT_API_KEY` was removed from `init` and `setup-ci`
  output messages.
- **`cmd/radiant/helpers.go`** — `renderSpecMD` / `renderTasksMD` now produce
  YAML frontmatter (`name`, `description`, `alwaysApply`) so the scaffolded
  docs render correctly inside IDE-compatible agents.

### Added
- **`internal/scaffold/scaffold.go` directory pre-flight**: all template
  writers now `mkdir -p` before writing, so nested scaffold paths never
  panic on missing parents.

[3.2.2]: https://github.com/quant-risk/radiant-harness/releases/tag/v3.2.2

---

## [3.2.1] — 2026-06-29 — doctor HTTP-LLM false positive

### Fixed
- **`cmd/radiant/cmd_doctor.go`** — the diagnostic markers
  `Checks HTTP-LLM ... ` / `Provider: ` were hardcoded as strings in the
  Light binary. They tripped `make smoke`'s "no HTTP-LLM symbols" check,
  even though the binary does not, in fact, contain HTTP-LLM client code.
  Rewrote `cmd_doctor` to reason about the real binary surface (55
  registered commands, 12 supported agents, MCP sampling backend wiring)
  and stop emitting the false-positive markers.

[3.2.1]: https://github.com/quant-risk/radiant-harness/releases/tag/v3.2.1

---

## [3.2.0] — 2026-06-29 — full engine, zero API key

### Changed
- **Scope redefinition:** the Light binary now ships with the **full 55-command
  surface** from the Full repo. The only remaining split between Light and
  Full is the HTTP-LLM dependency — Light still never talks to
  api.anthropic.com / api.openai.com / openrouter.ai, but it has every
  command the Full binary has.
- **`internal/llm/client.go`** is now a Light-only shim. It implements the
  `Client` API the Full-era `engine`, `loop`, `run`, and `fleet` packages
  expect, but wraps `SamplingBackend` (MCP `sampling/createMessage` to the
  host agent) instead of an HTTP LLM client. This lets the Full-era source
  compile into the Light binary without any `//go:build with_full` tags on
  the cmd files.
- **`internal/loop/runner.go`** still exposes `SetHTTPBackendBuilder` (the
  signature didn't change), but `cmd/radiant/helpers.go` now wires it to a
  `SamplingBackend`-backed factory instead of `llm.NewHTTPBackend`. The
  result: every per-phase loop client (planner, implementer, validator) ends
  up calling the host agent.
- **`cmd/radiant/resolveLoopLLMCreds`** is now a stub that always returns
  empty `apiKey` and `baseURL`. The "no LLM API key found" error in
  `cmd_loop.go` was removed; the Light build no longer requires an API key.
- **22 `cmd/radiant/*.go` files** had their `//go:build with_full` tag
  stripped so they compile into the Light binary.
- **`README.md`** rewritten to describe the full 55-command surface. The
  `Light vs Full` table was removed because the only remaining difference
  is "Light has zero HTTP egress, Full doesn't".
- **`INSTALL.md`** rewritten: removed the `OPENROUTER_API_KEY` /
  `radiant config` blocks that no longer apply.
- **`EXAMPLES.md`** rewritten around 10 worked examples covering every
  command family (loop, run, fleet, spec, product, doctor, release,
  worktree, improve, trace).
- **`scripts/smoke-test.sh`** bumped expected version to `3.2.0`.

### Added
- **55 commands** in a single ~11 MB binary:
  - 4 MCP commands: `setup-mcp`, `mcp serve`, `host-info`, `completion`
  - 13 loop-engine commands: `loop` (start/status/resume/cancel/history/
    export/diff), `run`, `fleet` (start/status/dispatch/cancel), `trace`
    (show/list)
  - 11 spec-driven commands: `init`, `spec`, `product`, `validate`,
    `validate-file`, `evals`, `audit`, `review-pr`, `adr`, `diagramar`,
    `views`
  - 5 release/CI commands: `release`, `setup-ci`, `update`, `views`,
    `boot`
  - 8 skills/context commands: `skills`, `context` (detect/assemble/
    compress), `ontology`
  - 8 diagnostics/session commands: `doctor`, `state`, `handoff`,
    `worktree` (add/list/remove/prune), `budget` (estimate/report),
    `tools`
  - 14 vertical scaffolds: `model`, `predict`, `train`, `evaluate`,
    `drift`, `profile`, `stats`, `causal-estimate`, `incident`,
    `autodata`, `eval`, `bench`, `improve`, `integrations`
  - 6 utility commands: `config`, `models`, `pricing` (list only), `semantic`,
    `camada-agentica`, `security`, `telemetry` (status/show/enable/disable)
- **9 new internal packages** ported from Full: `benchmark`, `boot`,
  `config`, `engine`, `fleet`, `harness`, `improve`, `mode`, `pricing`,
  `quality`, `routing`, `scaffold`, `schedule`, `slog`, `spec`, `webhook`,
  `worktree` (Light previously had 12; now has 28).

### Verified
- `make build` → 10,955,618 bytes (~11 MB). Up from 7.4 MB in v3.0.1
  because of the extra commands and internal packages.
- `make smoke` → **17/17 OK**, including the zero-HTTP-LLM guarantee:
  ```
  nm bin/radiant | grep -iE 'chatAnthropic|HTTPBackend|NewHTTPBackend'   # (empty)
  strings bin/radiant | grep -iE 'anthropic|openai|openrouter'           # (empty)
  ```
- `go test ./...` → **24 packages OK, 0 FAIL** (up from 14 in v3.0.1).

[3.2.0]: https://github.com/quant-risk/radiant-harness/releases/tag/v3.2.0

---

## [3.0.1] — 2026-06-29 — docs-fix patch

### Changed
- **`README.md`** rewritten from scratch to describe only what the Light
  binary actually does. The previous copy was inherited from the pre-split
  (v0.1/v0.2 TypeScript era) and described Full-only capabilities. The
  rewrite removes every claim that doesn't apply to the Light binary
  (`radiant run --resume`, BoltDB journaling, Fleet mode as a public feature,
  the full 54-command surface, etc.) and adds a `Light vs Full` section that
  points readers at [`quant-risk/radiant-harness-full`](https://github.com/quant-risk/radiant-harness-full)
  when they need the Full binary.
- **`INSTALL.md`** rewritten to match the Light reality: no API key, no
  provider config, no `radiant doctor`/`update`/`views`/`config`. Includes
  SHA256SUMS cross-check and a verification recipe.
- **`EXAMPLES.md`** rewritten around the only real entry point: `radiant_run`.
  Five worked examples (healthz endpoint, budgets, trace reading,
  non-default agent, zero-API-key verification). Closes the section that
  previously listed `radiant init`/`spec`/`product`/`run`/`validate`/`evals`/
  `audit`/`release`/`setup-ci`/`handoff`/`config` — none of which exist in
  the Light binary.

### Fixed
- **Public documentation matches the binary.** Closes the trust gap surfaced
  by an external agent that read the README, expected `radiant run --resume`,
  found it missing, and bailed to raw Python. The Light binary surface is now
  exactly: 4 CLI commands (`setup-mcp`, `mcp serve`, `host-info`,
  `completion`) + `help` + 1 MCP tool (`radiant_run`). Nothing more is
  promised in public docs.

### Not changed
- **Binary is identical to v3.0.0.** No source code changed. SHA256 of the
  release binaries is unchanged from `v3.0.0`. Bump is patch-level because
  it's a docs-only fix.

---

## [3.0.0] — 2026-06-29 — first public dual-binary release

### Added
- **First public release of the Light binary.** MCP-only, zero API key, zero
  HTTP egress, ~7.4 MB.
- 4 CLI commands: `setup-mcp`, `mcp serve`, `host-info`, `completion`.
- 1 MCP tool: `radiant_run(goal, profile, max_iter, max_cost, max_time)`.
- Support for **11 host agents**: Claude Code, Cursor, Windsurf, Zed, VS Code
  Copilot, OpenAI Codex, OpenCode, Hermes, Kimi CLI, OpenClaw, Cline.
- 69 bundled domain skills (Cobra, Zap, Testify, OpenTelemetry patterns; MCP
  protocol internals; ML / RL / DL reference; finance-risk / regulatory /
  actuarial; etc.).
- Dual-repo layout: [`quant-risk/radiant-harness`](https://github.com/quant-risk/radiant-harness)
  (Light, public) + [`quant-risk/radiant-harness-full`](https://github.com/quant-risk/radiant-harness-full)
  (Full, private — Fortvna Risk Solutions internal).

### Verified
- `make build` → 7.4 MB binary.
- `make smoke` → 17/17 verification checks pass (including the zero-HTTP-LLM
  symbol check).
- `go test ./...` → 14 packages, 0 FAIL.

[3.0.1]: https://github.com/quant-risk/radiant-harness/releases/tag/v3.0.1
[3.0.0]: https://github.com/quant-risk/radiant-harness/releases/tag/v3.0.0