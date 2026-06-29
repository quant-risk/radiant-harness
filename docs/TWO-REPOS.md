# Two-repo split (Sprint 80+)

`radiant-harness` ships as **two physically separate Git repos** from the same source tree:

## Repositories

| Repo                                              | Build                                                | When to use it                                                                 |
|---------------------------------------------------|------------------------------------------------------|--------------------------------------------------------------------------------|
| [`quant-risk/radiant-harness`](https://github.com/quant-risk/radiant-harness)       | **Light** (default, no API key infrastructure)        | You already use an MCP-compatible agent (Claude Code, Cursor, Hermes, …). The harness drives the agent's LLM via MCP sampling/createMessage. |
| [`quant-risk/radiant-harness-full`](https://github.com/quant-risk/radiant-harness-full) | **Full** (HTTP LLM transport, fleet, scaffolds, …)   | You want the harness to talk to an LLM provider directly. Autonomous mode in CI / dev / no host agent. |

## What each repo contains

### `radiant-harness` (Light, public)

- Single binary `radiant` (`v3.0.0`)
- 4 commands: `setup-mcp`, `mcp serve`, `host-info`, `completion`
- ~7.2 MB, 0 HTTP-LLM symbols, 0 API key code paths
- All inference comes from the host agent via MCP `sampling/createMessage`

### `radiant-harness-full` (Full, internal)

- Single binary `radiant` (`v3.0.0-full` in tag), built with `-tags with_full`
- 54 commands: every Light command plus `init`, `spec`, `loop`, `run`, `fleet`, `audit`, `evals`, `release`, `doctor`, etc.
- ~10.7 MB with HTTP transport wired up to Claude / OpenAI / OpenRouter / Mistral / xAI / Groq
- Verifier is a separate LLM call (different model role)
- Crash-safe state via BoltDB
- Fleet mode for parallel multi-agent work

## Build tags

The two repos share source files but use opposite build-tag conventions:

| Source file with tag...    | Compiles in...                              |
|----------------------------|---------------------------------------------|
| `//go:build !with_full`    | Light only                                  |
| `//go:build with_full`     | Full only                                   |
| no tag                     | Both                                        |

- **Light repo**: removes all `with_full`-tagged files (Full sibling is gone).
- **Full repo**: keeps both, but the **default `make build`** adds `-tags with_full` so users get the Full binary. `make build-light` builds the Light subset.

## Why two repos?

The public Light repo is the artifact users download when they want a self-driving loop inside an existing agent. It's minimal, has zero API key code paths, and is verified at the artifact level (no HTTP-LLM symbols via `nm`).

The Full repo has everything: HTTP transport to LLM providers, fleet mode, scaffolds, audit, evals, release pipeline, the full 54-command surface. It's the binary you use in CI or when you don't have a host agent.

Both ship from the same source tree (the Full repo is the full source, including the Light subset).

## Local layout

```
projects/
├── radiant-harness-main/        ← Light repo (this one)
│   └── 225 files, HEAD on commit with all Full code purged
└── radiant-harness-full/        ← Full repo
    └── 518 files, HEAD on commit that includes both Light + Full
```

The Full repo is a clone of the Light repo's pre-purge commit (`4c873fa`), with the dual-target Makefile and Full README added.