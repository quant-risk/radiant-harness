# radiant-harness

> **A vendor-neutral autonomous development harness for any LLM.**
> Shipped as **two physically separate binaries** from one source tree.
> Works with Claude Code, Cursor, Hermes, Codex, OpenCode, Cline, Kimi CLI, OpenClaw, VS Code Copilot, and any MCP-compatible agent.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Version](https://img.shields.io/badge/version-3.0.0-blue.svg)](CHANGELOG.md)
[![Builds](https://img.shields.io/badge/builds-2%20%28light+%2B+full%29-brightgreen.svg)](docs/LIGHT-VS-FULL.md)
[![Hosts](https://img.shields.io/badge/hosts-9%20agents%20detected-purple.svg)](docs/HOST-AGENTS.md)
[![Tests](https://img.shields.io/badge/tests-1%2C190%2B%20pass-success.svg)](CHANGELOG.md)
[![Go](https://img.shields.io/badge/go-1.22%2B-blue.svg)](go.mod)

---

## Two binaries from one source

This repo is the **monorepo** for `radiant-harness`. It produces
two distinct binaries via Go build tags:

| Binary    | Size      | Inference source           | API key | Commands                                                                 |
|-----------|-----------|----------------------------|---------|--------------------------------------------------------------------------|
| `radiant-light`  | ~10 MB | MCP sampling (host agent) only | **no**  | `setup-mcp`, `mcp serve`, `host-info`                                |
| `radiant-full`   | ~14 MB | HTTP LLM **and** MCP sampling  | yes     | everything (loop, run, fleet, audit, evals, scaffolds, etc.)          |

```bash
# Light: no API key infrastructure whatsoever
go build -tags light_only -o radiant-light ./cmd/radiant

# Full: everything (same as prior versions)
go build -o radiant-full ./cmd/radiant
```

Read [docs/LIGHT-VS-FULL.md](docs/LIGHT-VS-FULL.md) for the full
split rationale, `nm`-verified symbol separation, and how to publish
each artifact to its own GitHub repo.

---

## Quick start

### I'm running inside an agent (Claude Code, Cursor, etc.)

Use the **Light** binary. It possesses your agent via MCP sampling —
no API key needed.

```bash
# 1. Build (or download a release)
go build -tags light_only -o radiant ./cmd/radiant

# 2. Wire into your agent (auto-detects what's installed)
./radiant setup-mcp

# 3. Restart your agent and start using radiant
#    e.g. in Claude Code: "use radiant-harness to fix the auth bug"
./radiant --version        # → 3.0.0-light
```

Any tool call inside the agent becomes a `radiant_run` MCP call,
which samples your agent's LLM (no API key, no HTTP egress).

### I'm running autonomous (CI, local dev, no agent)

Use the **Full** binary with an API key.

```bash
export RADIANT_OPENROUTER_API_KEY=sk-or-...   # or OPENAI_API_KEY, ANTHROPIC_API_KEY
go build -o radiant ./cmd/radiant
./radiant loop start "ship the on-call dashboard"
```

---

## What is `radiant-harness`?

A CLI that turns any LLM into an **autonomous development loop**:
Discover → Plan → Execute → Verify → Persist. Verifier is always
a separate LLM call (so the same LLM can't approve its own work).

**Core engines** (all available in Full; Light exposes the MCP surface
that drives the same engines inside the host agent):

| Engine            | What it does |
|-------------------|-------------|
| **Loop Engine**   | Crash-safe `Discover→Plan→Execute→Verify→Persist` cycle. |
| **Fleet Engine**  | Parallel multi-agent dispatch with shared state + auto-retry. |
| **Context Engine** | Detects project domain; loads 3-10 relevant skills. |
| **Semantic Model** | Curated metric/regulation layer (e.g. CMN 4.966, IFRS 9). |
| **MCP Server**    | Full harness exposed as MCP tools (`radiant_run`, etc.). |

Works with any OpenAI-compatible API: Claude, GPT-4o, Gemini,
Mistral, OpenRouter, xAI, Groq, local models.

---

## Repository layout

```
.
├── cmd/radiant/                        CLI source (all subcommands)
│   ├── main.go                          Light entrypoint (//go:build light_only)
│   ├── main_full.go                     Full entrypoint (//go:build !light_only)
│   ├── cmd_setup_mcp.go                 11-agent config writer
│   ├── cmd_mcp_serve.go                 MCP server (both builds)
│   ├── cmd_mcp_runtime.go               Light-only MCP server impl
│   ├── cmd_mcp_runtime_full.go          Full MCP server impl (more tools)
│   ├── cmd_host_info.go                 radiant host-info (both builds)
│   ├── cmd_loop.go, cmd_run.go, ...    Full-only (tagged !light_only)
│   └── helpers.go                       Full-only helpers
│
├── internal/
│   ├── llm/                             LLM client + backend abstraction
│   │   ├── types.go                    shared types (Model, Message, ...)
│   │   ├── presets.go                  PresetModels + GetPreset + ListPresets
│   │   ├── backend.go                  Backend interface + SamplingBackend
│   │   ├── backend_http.go             HTTPBackend (Full only, !light_only)
│   │   └── anthropic.go, client.go     Full only
│   │
│   ├── loop/                            Loop engine + budget + verifier
│   ├── mcpbridge/                       MCP tool-bridge adapter
│   ├── config/, ontology/, semantic/   Semantic layer
│   ├── policy/, routing/, pricing/     Cost + policy + routing
│   ├── scaffold/, skill/, schedule/    Scaffolds, skills, scheduling
│   └── ...  (28 packages total)
│
├── internal/hostdetect/                 Sprint 79: runtime agent detection
│
├── docs/
│   ├── LIGHT-VS-FULL.md                 Split architecture + publishing guide
│   ├── HOST-AGENTS.md                   9-agent detection matrix
│   ├── MODES.md                         Light/Full behaviour
│   ├── SPRINT74-PLAN.md, ...           Per-sprint design docs
│   └── validation-report-*.md          Per-sprint verification
│
├── .goreleaser.yml                       Release config (Full by default)
├── Makefile                              `make`, `make test`, `make release`
├── Dockerfile                             Multi-stage build
└── README.md                             (you are here)
```

---

## Build

### Light (no API key code, ~10 MB)

```bash
go build -tags light_only -o bin/radiant-light ./cmd/radiant
```

Cross-compile:

```bash
GOOS=linux   GOARCH=amd64 go build -tags light_only -o bin/radiant-light-linux-amd64     ./cmd/radiant
GOOS=darwin  GOARCH=arm64 go build -tags light_only -o bin/radiant-light-darwin-arm64    ./cmd/radiant
GOOS=windows GOARCH=amd64 go build -tags light_only -o bin/radiant-light-windows-amd64.exe ./cmd/radiant
```

### Full (~14 MB, requires API key)

```bash
go build -o bin/radiant ./cmd/radiant
```

### Verify the split (zero HTTP-LLM symbols in Light)

```bash
nm bin/radiant-light | grep -iE 'chatAnthropic|HTTPBackend|NewHTTPBackend'
# (must return 0 results)
```

### Run tests

```bash
go test -count=1 ./...                              # Full: 31 packages
go test -count=1 -tags light_only ./...           # Light: 29 packages
```

---

## Commands

### Light

| Command                          | What it does                                                  |
|----------------------------------|---------------------------------------------------------------|
| `radiant setup-mcp`              | Auto-detect agent + write MCP config (11 agents supported).   |
| `radiant setup-mcp --agent=claude|cursor|...` | Target a single agent.                         |
| `radiant setup-mcp --global`     | Write to `~/.claude/settings.json` etc. instead of project-level. |
| `radiant mcp serve`              | Start MCP server on stdio (sampling).                        |
| `radiant host-info`              | Print detected host agent + confidence.                       |
| `radiant host-info --json`       | Machine-readable version.                                    |
| `radiant host-info --verbose`    | Show all matched env vars.                                    |
| `radiant --version`              | Prints `3.0.0-light`.                                         |

### Full (everything from Light, plus)

The Full binary registers every subcommand the harness has ever
shipped. The most-used ones:

```bash
radiant init .                       # scaffold a project (CONTEXT.md, AGENTS.md)
radiant spec "ship X"               # start a feature (spec.md + tasks.md)
radiant loop start "fix the auth"   # autonomous feedback loop
radiant run "add tests"              # one-shot full run with trace export
radiant fleet start ...             # multi-agent parallel work
radiant audit                       # project conformity check
radiant evals                        # AC→test coverage report
radiant release 0.4.0                # 7-step release pipeline
radiant doctor                      # diagnose radiant environment
```

Plus: `adr`, `camada-agentica`, `bench`, `causal-estimate`, `config`,
`context`, `diagramar`, `drift`, `evaluate`, `harness`, `host-info`,
`mcp-serve`, `pricing`, `product`, `release-pr`, `review-pr`,
`scaffold-*`, `security`, `semantic`, `session`, `setup-mcp`,
`skills`, `spec`, `telemetry`, `tools`, `worktree`, ... — see
`radiant --help`.

---

## How `setup-mcp` works

`radiant setup-mcp` writes the MCP config file your agent reads.
It supports 11 agents:

| Agent         | Config file (project)              | Format      |
|---------------|-------------------------------------|-------------|
| Claude Code   | `.mcp.json`                         | JSON-std    |
| Cursor        | `.cursor/mcp.json`                  | JSON-std    |
| Windsurf      | `.windsurf/mcp.json`                | JSON-std    |
| Zed           | `.zed/settings.json`                | JSON-std    |
| VS Code       | `.vscode/mcp.json`                  | JSON-std    |
| Codex (OpenAI)| `.codex/config.toml`                | TOML        |
| OpenCode      | `.opencode/config.json`             | JSON-nested |
| Hermes        | `.hermes/config.yaml`               | YAML        |
| Kimi CLI      | `~/.kimi/mcp.json` (global)         | JSON-std    |
| OpenClaw      | `.openclaw/openclaw.json`           | JSON-nested |
| Cline         | `~/.cline/mcp.json` (global)         | JSON-std    |

Auto-detect runs from the current working directory's markers
(`.claude/`, `.codex/`, `.hermes/`, etc.). Global fallback flags
(`~/.kimi`, `~/.cline`) cover CLI-only tools.

See [`docs/LIGHT-VS-FULL.md`](docs/LIGHT-VS-FULL.md) for the full
detection rules.

---

## How `mcp serve` works

`radiant mcp serve` is the MCP server entry point. Read
[`docs/MODES.md`](docs/MODES.md) for the Light/Full rationale.

Wire into your agent with `setup-mcp`, then restart the agent.
Any tool call from the agent that hits `radiant_run` will:

1. Be sent to the harness via JSON-RPC 2.0 over stdio.
2. The harness executes `loop.Run(goal)`.
3. **Light:** every LLM call is `sampling/createMessage` back to
   the host agent (no API key required).
4. **Full:** every LLM call is a direct HTTP call to the configured
   provider (API key required).
5. The harness returns the trace as the tool result.

The full trace lives at `.radiant-harness/traces/<run-id>.jsonl`
once the run finishes.

---

## Auto-detected host agents

Use `radiant host-info` to see which agent is currently driving
the harness (whether the agent's MCP server is running or not).
Detection works in 3 layers:

1. **Env-var fingerprint** (high confidence).
2. **`/proc/<ppid>/comm` walk** (medium confidence).
3. **PID trace** (low confidence; future).

See [`docs/HOST-AGENTS.md`](docs/HOST-AGENTS.md) for the matrix of
which env vars and binaries each agent uses, and how to add a new
agent.

Supported today:

- Claude Code (`CLAUDE_CODE_*`)
- Cursor (`CURSOR_*`)
- Hermes (`HERMES_*`)
- Kimi CLI (`KIMI_*`)
- OpenClaw (`OPENCLAW_*`)
- Codex (`CODEX_*`)
- Cline (`CLINE_*`)
- OpenCode (`OPENCODE_*`)
- VS Code Copilot (`VSCODE_*`)

Adding a new agent is a 5-line edit in `internal/hostdetect/hostdetect.go`.

---

## Documentation

| Doc                                     | What's in it                                |
|-----------------------------------------|---------------------------------------------|
| [docs/LIGHT-VS-FULL.md](docs/LIGHT-VS-FULL.md) | Build-tag split, publishing flow, verification |
| [docs/HOST-AGENTS.md](docs/HOST-AGENTS.md)     | Auto-detection matrix (9 agents)            |
| [docs/MODES.md](docs/MODES.md)                 | Light vs Full behaviour                     |
| [CHANGELOG.md](CHANGELOG.md)                   | Version history                             |
| [RELEASE-NOTES.md](RELEASE-NOTES.md)           | Per-release notes (Light + Full)            |
| [INSTALL.md](INSTALL.md)                       | Install instructions                        |
| [EXAMPLES.md](EXAMPLES.md)                     | Worked examples                             |
| [AGENTS.md](AGENTS.md)                         | Agent guidance / project memory             |
| [CLAUDE.md](CLAUDE.md)                         | Claude-specific guidance                    |
| [docs/SPRINT*.md](docs/SPRINT*.md)             | Per-sprint design docs                      |

---

## Versioning & release

`radiant-harness` follows [semver](https://semver.org/). Each
release ships both the Light and Full artifacts built from the
same source tag.

- **Major**: breaking changes to MCP wire protocol or command
  surface.
- **Minor**: new agents in `setup-mcp`, new commands.
- **Patch**: bug fixes, refactors, no API change.

Releases cut from `main` via `.goreleaser.yml`. Tags of the form
`vX.Y.Z` mark Full releases; `vX.Y.Z-light` is informational only
(Light is built from the same tag with `-tags light_only`).

Latest: **v3.0.0** (Full) and **v3.0.0-light** (Light) — first
public release of the dual-binary form.

---

## Contributing

Issues, PRs, and forks welcome.

For new agents in `setup-mcp`: edit `cmd_setup_mcp_per_agent.go`
and add a `case "agent-name":` block.
For new host-detect signatures: edit `internal/hostdetect/hostdetect.go`.

---

## License

MIT — see [LICENSE](LICENSE).
