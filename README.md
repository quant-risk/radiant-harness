<!-- Hero -->
<div align="center">

<br/>

# ✨ radiant-harness

### Self-driving loops for any LLM agent.

**Vendor-neutral. Zero API keys. Zero telemetry.**

Works with **Claude Code · Cursor · Hermes · Codex · Cline · Kimi · OpenCode · OpenClaw · VS Code Copilot** — and any MCP-compatible agent.

<br/>

![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg?style=for-the-badge)
![Version](https://img.shields.io/badge/release-v3.0.0-blueviolet?style=for-the-badge)
![Go](https://img.shields.io/badge/Go-1.22%2B-00ADD8?style=for-the-badge&logo=go&logoColor=white)
![Binary](https://img.shields.io/badge/binary-~10MB-success?style=for-the-badge)
![API keys](https://img.shields.io/badge/API_keys-NONE-success?style=for-the-badge)
![Telemetry](https://img.shields.io/badge/telemetry-OFF-lightgrey?style=for-the-badge)
![Tests](https://img.shields.io/badge/tests-1,190%2B_passing-brightgreen?style=for-the-badge)

<br/>

```text
                    ┌──────────────────────────────────┐
                    │           your agent             │
                    │  Claude · Cursor · Hermes · …    │
                    └────────────────┬─────────────────┘
                                     │  MCP over stdio
                                     │  (JSON-RPC 2.0)
                                     ▼
                    ┌──────────────────────────────────┐
                    │        radiant-harness            │
                    │                                  │
                    │  Discover → Plan → Execute       │
                    │        → Verify → Persist        │
                    │                                  │
                    │  no API keys · no HTTP egress    │
                    │  no telemetry · no vendor lock   │
                    └──────────────────────────────────┘
```

<br/>

```bash
git clone https://github.com/quant-risk/radiant-harness
cd radiant-harness
make build
./bin/radiant setup-mcp
```

<br/>

**[Install](#-installation) · [Quickstart](#-quickstart) · [How it works](#-how-it-works) · [Agents](#-supported-agents) · [Build](#-build-from-source) · [FAQ](#-faq)**

</div>

---

## Why radiant-harness?

You already use an agent. It can read files, run tests, edit code.

But it loops.

It makes a change, gets the same test result back, makes the *same* change again, and burns your context window until you shut it off. There is no budget, no progress memory, no separation between "the agent that did the work" and "the agent that approved it."

**radiant-harness gives your agent a backbone.**

A verifiable, budgeted, persistent `Discover → Plan → Execute → Verify` loop that doesn't lose its place — and a *separate* verifier pass so the same model never approves its own work.

And because we don't carry any LLM client code, we don't need your API keys. Your agent already has a brain. We just keep the loop honest.

---

## What you get

| | |
|---|---|
| **🧠 Spec-driven development** | `CONTEXT.md` + `spec.md` + `tasks.md` scaffolding for every feature. |
| **🔁 Crash-safe loop** | Discover → Plan → Execute → Verify → Persist. Survives Ctrl-C, OOM, network drops. |
| **🪞 Separate verifier** | A second LLM call judges the work — never the same model that wrote it. |
| **🧮 Budget engine** | Token, cost, wall-clock, and tool-call caps. Fail loud when exceeded. |
| **🔌 MCP-native** | Wire into any agent in 30 seconds. Becomes a single `radiant_run` tool. |
| **🧰 Fleet mode** | Parallel agents with shared state, auto-retry, contention detection. |
| **📦 Semantic layer** | Curated metric/regulation models (CMN 4.966, IFRS 9, Basel) load on demand. |
| **🪶 Zero footprint** | ~10 MB binary. No daemon. No HTTP egress. No telemetry. |
| **🔓 Zero lock-in** | Trace files are plain JSONL. Spec files are plain Markdown. Take it with you. |

---

## Installation

### Download a release

```bash
# macOS (Apple Silicon)
curl -L https://github.com/quant-risk/radiant-harness/releases/download/v3.0.0/radiant-darwin-arm64 \
  -o /usr/local/bin/radiant
chmod +x /usr/local/bin/radiant

# Linux x86_64
curl -L https://github.com/quant-risk/radiant-harness/releases/download/v3.0.0/radiant-linux-amd64 \
  -o /usr/local/bin/radiant
chmod +x /usr/local/bin/radiant
```

### Build from source

```bash
git clone https://github.com/quant-risk/radiant-harness
cd radiant-harness
make build         # → ./bin/radiant
```

Or with plain `go`:

```bash
go build -o radiant ./cmd/radiant
```

---

## Quickstart

### 1. Wire into your agent

```bash
./bin/radiant setup-mcp
```

This auto-detects which agent you have (Claude Code? Cursor? Hermes?) and writes the right config file:

| If you use…           | It writes…                                |
|-----------------------|-------------------------------------------|
| Claude Code           | `.mcp.json`                               |
| Cursor                | `.cursor/mcp.json`                        |
| Windsurf              | `.windsurf/mcp.json`                      |
| Zed                   | `.zed/settings.json`                      |
| VS Code Copilot       | `.vscode/mcp.json`                        |
| OpenAI Codex          | `.codex/config.toml`                      |
| OpenCode              | `.opencode/config.json`                   |
| Hermes                | `.hermes/config.yaml`                     |
| OpenClaw              | `.openclaw/openclaw.json`                 |
| Kimi CLI              | `~/.kimi/mcp.json`                        |
| Cline                 | `~/.cline/mcp.json`                       |

Force a specific agent:

```bash
./bin/radiant setup-mcp --agent=claude    # or cursor, codex, hermes, …
./bin/radiant setup-mcp --global          # write to ~/.config/<agent>/…
```

### 2. Restart your agent

That's it. The next time you ask your agent to do something non-trivial, it'll discover `radiant_run` and use it.

### 3. Verify it can see you

```bash
./bin/radiant host-info
```

Output:

```text
detected agent     : Claude Code
confidence         : 100
signals matched    : CLAUDE_CODE_ENTRYPOINT, CLAUDE_CODE_SHELL_PREFIX
process tree       : /Users/you/.npm/_npx/.../claude (pid 12345)
```

### 4. Drive a loop from your agent

From inside Claude Code (or any wired agent):

> *"use radiant-harness to add a /healthz endpoint with tests"*

Your agent will call `radiant_run`, the harness spins up the loop, every LLM call routes back to your agent via MCP `sampling/createMessage`, and you get a trace in `.radiant-harness/traces/<run-id>.jsonl`.

---

## How it works

```text
  ┌────────────────────────────────────────────────────────────────────┐
  │                          your agent                                │
  │                                                                    │
  │   1. user says "ship X"                                            │
  │   2. agent decides this is non-trivial → calls radiant_run         │
  │                                                                    │
  └─────────────────────────────────┬──────────────────────────────────┘
                                    │ MCP stdio (JSON-RPC 2.0)
                                    ▼
  ┌────────────────────────────────────────────────────────────────────┐
  │                       radiant-harness                               │
  │                                                                    │
  │   ┌─────────┐    ┌─────────┐    ┌─────────┐    ┌─────────┐         │
  │   │Discover │ →  │  Plan   │ →  │ Execute │ →  │ Verify  │ → …     │
  │   └─────────┘    └─────────┘    └─────────┘    └─────────┘         │
  │       │              │              │              │               │
  │       ▼              ▼              ▼              ▼               │
  │   .radiant-harness/  spec.md    tool calls    separate LLM          │
  │   CONTEXT.md         tasks.md   (Bash, Edit,  call to judge        │
  │                                  Read, …)     the work              │
  │                                                                    │
  │   every LLM call ↘  MCP sampling/createMessage  ↙  (back to agent)│
  └────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
  ┌────────────────────────────────────────────────────────────────────┐
  │                         on disk                                    │
  │                                                                    │
  │   .radiant-harness/                                                │
  │   ├── traces/<run-id>.jsonl      ← every step, every token         │
  │   ├── state/<run-id>.bolt        ← crash-safe resume state         │
  │   └── reports/<run-id>.md        ← human-readable verdict          │
  │                                                                    │
  │   CONTEXT.md, spec.md, tasks.md   ← portable, take them anywhere   │
  └────────────────────────────────────────────────────────────────────┘
```

### The four phases

1. **Discover** — read `CONTEXT.md`, sniff the project, load 3-10 relevant skills.
2. **Plan** — break the goal into acceptance criteria + tasks with explicit gates.
3. **Execute** — run each task; on failure, retry with the failure reason as context.
4. **Verify** — a *fresh* LLM call (or the host agent's model) judges the diff vs the AC.

The loop is **crash-safe**: every step is journaled to BoltDB before it runs. Power-off mid-run? `radiant run --resume` picks up exactly where it left off.

---

## Supported agents

`setup-mcp` knows about **11 agents** today. Auto-detect scans your working directory for `.claude/`, `.codex/`, `.hermes/`, etc. and global fallbacks (`~/.kimi`, `~/.cline`) for CLI-only tools.

| Agent             | Config file               | Format         | Detect signal                          |
|-------------------|---------------------------|----------------|----------------------------------------|
| **Claude Code**   | `.mcp.json`               | JSON           | `CLAUDE_CODE_*` env                    |
| **Cursor**        | `.cursor/mcp.json`        | JSON           | `CURSOR_*` env                         |
| **Windsurf**      | `.windsurf/mcp.json`      | JSON           | (project marker)                       |
| **Zed**           | `.zed/settings.json`      | JSON           | (project marker)                       |
| **VS Code Copilot** | `.vscode/mcp.json`      | JSON           | `VSCODE_*` env                         |
| **OpenAI Codex**  | `.codex/config.toml`      | TOML           | `CODEX_*` env                          |
| **OpenCode**      | `.opencode/config.json`   | JSON (nested)  | `OPENCODE_*` env                       |
| **Hermes**        | `.hermes/config.yaml`     | YAML           | `HERMES_*` env                         |
| **Kimi CLI**      | `~/.kimi/mcp.json`        | JSON           | `KIMI_*` env                           |
| **OpenClaw**      | `.openclaw/openclaw.json` | JSON (nested)  | `OPENCLAW_*` env                       |
| **Cline**         | `~/.cline/mcp.json`       | JSON           | `CLINE_*` env                          |

Run `./bin/radiant host-info --verbose` to see which env vars matched for your session.

---

## Build from source

### Build from source

```bash
# Native
make build
./bin/radiant --version    # → radiant 3.0.0

# Cross-platform
make release                  # → bin/radiant-{linux,darwin,windows}-{amd64,arm64}
```

### Verify the zero-API-key split

```bash
nm bin/radiant | grep -iE 'chatAnthropic|HTTPBackend|NewHTTPBackend'
# (must return 0 results)

strings bin/radiant | grep -iE 'anthropic|openai|openrouter'
# (must return 0 results)

ls -lh bin/radiant
# ≈ 10 MB
```

### Tests

```bash
make test         # 31 packages OK, 0 FAIL
make smoke        # 17/17 verification checks pass
```

---

## Commands

| Command                          | What it does                                          |
|----------------------------------|-------------------------------------------------------|
| `radiant setup-mcp`              | Detect agent + write MCP config.                      |
| `radiant setup-mcp --agent=X`    | Force a specific agent.                               |
| `radiant setup-mcp --global`     | Write to `~/.config/<agent>/…` instead of project.    |
| `radiant mcp serve`              | Start the MCP server on stdio.                        |
| `radiant host-info`              | Print detected host agent + confidence.               |
| `radiant host-info --json`       | Machine-readable.                                     |
| `radiant host-info --verbose`    | Show every matched env var + process tree.            |
| `radiant --version`              | Prints `radiant 3.0.0`.                         |
| `radiant completion <shell>`     | Shell completion (bash, zsh, fish, powershell).       |
| `radiant help`                   | Complete command list.                                |

---

## Project layout

```
radiant-harness/
├── cmd/radiant/                       CLI source
│   ├── main.go                          ← default entrypoint
│   ├── cmd_setup_mcp.go                 ← 11-agent router
│   ├── cmd_setup_mcp_per_agent.go       ← per-agent merge functions
│   ├── cmd_mcp_serve.go                 ← MCP server (JSON-RPC 2.0 over stdio)
│   ├── cmd_mcp_runtime.go               ← MCP server impl
│   ├── cmd_host_info.go                 ← radiant host-info
│   └── helpers.go                       ← shared helpers
│
├── internal/
│   ├── hostdetect/                      ← runtime agent detection (env + /proc)
│   ├── loop/                            ← Discover→Plan→Execute→Verify engine
│   ├── mcpbridge/                       ← MCP tool bridge
│   ├── config/  ontology/  semantic/    ← semantic layer
│   └── …                                ← 25 packages total
│
├── scripts/
│   └── smoke-test.sh                    ← 17-check binary verification
│
├── docs/
│   ├── HOST-AGENTS.md                   ← detection matrix
│   └── validation-report-*.md           ← per-sprint reports
│
├── Makefile                             ← make build, make release, make smoke
├── LICENSE                              ← MIT
├── CHANGELOG.md
└── README.md                            ← you are here
```

---

## FAQ

**Q: Why no API key?**
A: Every LLM call is delegated to the host agent via MCP `sampling/createMessage`. Your agent already has a model configured; we just drive the loop.

**Q: Is the harness sending my code anywhere?**
A: No. The binary has zero HTTP egress for LLM calls (verified via `nm`/`strings`). The only outbound traffic is to the MCP stdio channel, which goes to your local agent.

**Q: Does it phone home?**
A: No telemetry, no analytics, no update checks. The binary is offline-first.

**Q: Can I add a new agent to `setup-mcp`?**
A: Yes. Add a `case "<agent-name":` block in `cmd_setup_mcp_per_agent.go` with the agent's config format and signature env vars. See [docs/HOST-AGENTS.md](docs/HOST-AGENTS.md).

**Q: How is this different from Claude Code's / OpenAI Codex's native loop?**
A: Three things: (1) verifier is a separate LLM call so the same model never approves its own work, (2) crash-safe resume via BoltDB journaling, (3) every step traces to portable JSONL.

**Q: Is it stable?**
A: Yes. First public release at v3.0.0. Semver from here.

---

## Contributing

Issues, PRs, and forks welcome.

- New agent in `setup-mcp` → edit `cmd_setup_mcp_per_agent.go`.
- New host-detect signature → edit `internal/hostdetect/hostdetect.go`.
- New MCP tool → edit `cmd_mcp_runtime.go`.

---

## License

MIT — see [LICENSE](LICENSE).

---

<div align="center">

**Built with care in 🇧🇷**

[github.com/quant-risk/radiant-harness](https://github.com/quant-risk/radiant-harness) · [v3.0.0](https://github.com/quant-risk/radiant-harness/releases/tag/v3.0.0) · [report a bug](https://github.com/quant-risk/radiant-harness/issues)

</div>