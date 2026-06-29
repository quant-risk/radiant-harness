<!-- Hero -->
<div align="center">

<br/>

# ✨ radiant-harness

### Wire a verifiable dev loop into any MCP agent.

**Zero API keys · Zero HTTP egress · Zero telemetry.**

Works with **Claude Code · Cursor · Hermes · Codex · Cline · Kimi · OpenCode · OpenClaw · Windsurf · Zed · VS Code Copilot** — and any MCP-compatible agent.

<br/>

![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg?style=for-the-badge)
![Version](https://img.shields.io/badge/release-v3.0.1-blueviolet?style=for-the-badge)
![Go](https://img.shields.io/badge/Go-1.22%2B-00ADD8?style=for-the-badge&logo=go&logoColor=white)
![Binary](https://img.shields.io/badge/binary-~7.4MB-success?style=for-the-badge)
![API keys](https://img.shields.io/badge/API_keys-NONE-success?style=for-the-badge)
![Telemetry](https://img.shields.io/badge/telemetry-OFF-lightgrey?style=for-the-badge)
![MCP tools](https://img.shields.io/badge/MCP_tools-1-brightgreen?style=for-the-badge)

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
                    │        radiant-harness           │
                    │                                  │
                    │  single tool:  radiant_run       │
                    │  inference:    MCP sampling      │
                    │  back to host agent              │
                    │                                  │
                    │  no API keys · no HTTP egress    │
                    │  no telemetry · no vendor lock   │
                    └──────────────────────────────────┘
```

<br/>

```bash
# 1. install (one binary, no deps)
curl -L https://github.com/quant-risk/radiant-harness/releases/latest/download/radiant-$(uname -s | tr A-Z a-z)-$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/') \
  -o /usr/local/bin/radiant && chmod +x /usr/local/bin/radiant

# 2. wire into your agent (auto-detects which one)
radiant setup-mcp

# 3. restart your agent and ask it to ship something
```

<br/>

**[Install](#-installation) · [Quickstart](#-quickstart) · [How it works](#-how-it-works) · [Commands](#-commands) · [Agents](#-supported-agents) · [FAQ](#-faq)**

</div>

---

## Why radiant-harness?

You already use an agent. It can read files, run tests, edit code.

But it loops.

It makes a change, gets the same test result back, makes the *same* change again, and burns your context window until you shut it off. There is no budget, no progress memory, no separation between "the agent that did the work" and "the agent that approved it."

**radiant-harness gives your agent a backbone.**

When the agent has a non-trivial goal, it calls `radiant_run`. The harness drives a verifiable, budgeted, persistent loop inside your agent's process — every LLM call routes back to the host agent via MCP `sampling/createMessage`. No API keys needed, because the harness doesn't talk to any LLM provider directly: it asks your agent to think.

---

## What you get

| | |
|---|---|
| **1️⃣ Single MCP tool** | `radiant_run(goal, profile, max_iter, max_cost, max_time)` — one entry point your agent calls when the task is non-trivial. |
| **🪞 Separate verifier pass** | The work-product is judged by a *different* LLM call — never the same model that wrote it. |
| **🧮 Budget engine** | Token, cost, wall-clock, and tool-call caps. The harness fails loud when the budget runs out. |
| **🔌 MCP-native** | Wire into any agent in 30 seconds with `radiant setup-mcp`. No daemon, no port to open, no DB to install. |
| **🪶 Zero footprint** | Single ~7.4 MB binary. Zero HTTP egress for LLM calls (verified at build time via `nm`/`strings`). |
| **🔓 Vendor-neutral** | Trace files are plain JSONL. Spec files are plain Markdown. Take it with you. |
| **📚 69 bundled skills** | Domain knowledge the harness can read on demand: Go architecture, MCP internals, ML, finance risk, regulatory, … |

---

## Installation

### Download a release

Pre-built binaries are available on the
[releases page](https://github.com/quant-risk/radiant-harness/releases).
Six targets are supported:

| OS | Arch | File |
|----|------|------|
| Linux | amd64 | `radiant-linux-amd64` |
| Linux | arm64 | `radiant-linux-arm64` |
| macOS | amd64 | `radiant-darwin-amd64` |
| macOS | arm64 | `radiant-darwin-arm64` |
| Windows | amd64 | `radiant-windows-amd64.exe` |
| Windows | arm64 | `radiant-windows-arm64.exe` |

**macOS / Linux:**

```bash
# macOS Apple Silicon
curl -L https://github.com/quant-risk/radiant-harness/releases/latest/download/radiant-darwin-arm64 \
  -o /usr/local/bin/radiant
chmod +x /usr/local/bin/radiant

# Linux x86_64
curl -L https://github.com/quant-risk/radiant-harness/releases/latest/download/radiant-linux-amd64 \
  -o /usr/local/bin/radiant
chmod +x /usr/local/bin/radiant

# verify
sha256sum /usr/local/bin/radiant    # cross-check with SHA256SUMS in the release
radiant --version                   # → radiant 3.0.1
```

**Windows (PowerShell):**

```powershell
Invoke-WebRequest -Uri "https://github.com/quant-risk/radiant-harness/releases/latest/download/radiant-windows-amd64.exe" -OutFile "$env:LOCALAPPDATA\Microsoft\WindowsApps\radiant.exe"
radiant --version
```

### Build from source

Requires Go 1.22+.

```bash
git clone https://github.com/quant-risk/radiant-harness
cd radiant-harness
make build         # → ./bin/radiant
```

Or with plain `go`:

```bash
CGO_ENABLED=0 go build -o radiant ./cmd/radiant
```

Cross-compile all 6 targets:

```bash
make release       # → bin/radiant-{linux,darwin,windows}-{amd64,arm64}
```

---

## Quickstart

### 1. Wire into your agent

```bash
./bin/radiant setup-mcp
```

This auto-detects which agent you have and writes the right config file. See [Supported agents](#-supported-agents) for the full list.

Force a specific agent:

```bash
./bin/radiant setup-mcp --agent=claude    # or cursor, codex, hermes, …
./bin/radiant setup-mcp --global          # write to ~/.config/<agent>/…
./bin/radiant setup-mcp --dry-run         # print what would be written
```

### 2. Restart your agent

That's it. The next time your agent gets a non-trivial task, it'll discover `radiant_run` and use it.

### 3. Verify the wire-up

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

`--json` for machine-readable output, `--verbose` to see every matched env var.

### 4. Drive a loop from your agent

From inside Claude Code (or any wired agent):

> *"use radiant-harness to add a /healthz endpoint with tests"*

Your agent calls `radiant_run`, the harness spins up the loop, every LLM call routes back to your agent via MCP `sampling/createMessage`, and you get a JSONL trace in `.radiant-harness/traces/<run-id>.jsonl`.

You can also call it directly with budgets:

```text
radiant_run(
  goal="add /healthz endpoint that returns 200 OK + JSON body",
  profile="standard",        # lean | standard | thorough
  max_iter=20,
  max_cost="2.00",
  max_time="10m"
)
```

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
  │   read bundled    break goal     tool calls    separate LLM         │
  │   skills,         into ACs +     (host agent   call to judge       │
  │   sniff repo      tasks          invokes)      the work             │
  │                                                                    │
  │   every LLM call ↘  MCP sampling/createMessage  ↙  (back to agent)│
  └────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
  ┌────────────────────────────────────────────────────────────────────┐
  │                         on disk                                    │
  │                                                                    │
  │   .radiant-harness/                                                │
  │   └── traces/<run-id>.jsonl      ← every step, every token         │
  │                                                                    │
  │   spec.md, tasks.md              ← portable Markdown artifacts     │
  └────────────────────────────────────────────────────────────────────┘
```

### The four phases (driven by the host agent)

1. **Discover** — the agent reads `CONTEXT.md`, sniffs the repo, picks 3-10 relevant bundled skills.
2. **Plan** — the agent breaks the goal into acceptance criteria + tasks with explicit gates.
3. **Execute** — the agent runs each task; on failure, retries with the failure reason as context.
4. **Verify** — a *fresh* LLM call (in the same agent session, but a separate sampling round) judges the diff vs the AC.

Every step is appended to `.radiant-harness/traces/<run-id>.jsonl` as it happens.

---

## Commands

This is the complete CLI surface of the Light binary:

| Command                          | What it does                                          |
|----------------------------------|-------------------------------------------------------|
| `radiant setup-mcp`              | Detect host agent + write MCP config.                |
| `radiant setup-mcp --agent=X`    | Force a specific agent.                              |
| `radiant setup-mcp --global`     | Write to `~/.config/<agent>/…` instead of project.   |
| `radiant setup-mcp --dry-run`    | Print the JSON/YAML config that would be written.    |
| `radiant mcp serve`              | Start the MCP server on stdio.                       |
| `radiant host-info`              | Print detected host agent + confidence.              |
| `radiant host-info --json`       | Machine-readable.                                    |
| `radiant host-info --verbose`    | Show every matched env var + process tree.           |
| `radiant --version`              | Prints `radiant 3.0.1`.                              |
| `radiant completion <shell>`     | Shell completion (bash, zsh, fish, powershell).      |
| `radiant help`                   | Complete command list.                               |

That's it. The Light binary is intentionally minimal — 4 commands, 1 MCP tool (`radiant_run`). Every other capability (`loop`, `fleet`, `boot`, `run --resume`, `doctor`, scaffolds, …) lives in the **Full** binary at [`quant-risk/radiant-harness-full`](https://github.com/quant-risk/radiant-harness-full). See [Light vs Full](#-light-vs-full) below.

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

## Light vs Full

The project ships as **two physically separate repos** from the same source tree. The split is enforced at the source level — there is no flag or config that toggles modes.

| | **Light** ([this repo](https://github.com/quant-risk/radiant-harness)) | **Full** ([`quant-risk/radiant-harness-full`](https://github.com/quant-risk/radiant-harness-full)) |
|---|---|---|
| **Use when** | You already use an MCP agent and want it to drive a verifiable loop. | You want the harness to talk to an LLM provider directly (CI, no host agent). |
| **Inference source** | Host agent via MCP `sampling/createMessage`. | Direct HTTP to OpenAI / Anthropic / OpenRouter / Mistral / xAI / Groq. |
| **CLI commands** | 4: `setup-mcp`, `mcp serve`, `host-info`, `completion` (+ `help`). | 54: Light's 4 + `loop`, `run`, `fleet`, `init`, `spec`, `product`, `validate`, `evals`, `audit`, `review-pr`, `release`, `doctor`, `boot`, `setup-ci`, `handoff`, `state`, `config`, `views`, `update`, … |
| **MCP tools** | 1: `radiant_run`. | 10+: `radiant_run` + `radiant_spec`, `radiant_adr`, `radiant_product`, `radiant_evals`, `radiant_audit`, `radiant_release`, … |
| **API keys** | None — verified at build time (no `chatAnthropic`, `HTTPBackend`, etc. symbols). | Yes — `OPENAI_API_KEY` / `ANTHROPIC_API_KEY` / `OPENROUTER_API_KEY`. |
| **HTTP egress** | None for LLM calls. | Yes, to provider endpoints. |
| **Crash-safe resume** | No (single-shot per `radiant_run` call). | Yes (BoltDB journaled state, `radiant run --resume`). |
| **Fleet mode** | No. | Yes. |
| **Bundle size** | ~7.4 MB. | ~10.7 MB. |
| **Audience** | Public — anyone with an MCP agent. | Internal — Fortvna Risk Solutions / partners. |

See [`docs/TWO-REPOS.md`](docs/TWO-REPOS.md) for the full rationale and build-tag conventions.

---

## Verify the zero-API-key split

```bash
nm bin/radiant | grep -iE 'chatAnthropic|HTTPBackend|NewHTTPBackend'
# (must return 0 results)

strings bin/radiant | grep -iE 'anthropic|openai|openrouter'
# (must return 0 results)

ls -lh bin/radiant
# ≈ 7.4 MB
```

These checks are part of `make smoke`.

### Tests

```bash
make test         # 14 packages OK, 0 FAIL
make smoke        # 17/17 verification checks pass
```

---

## Project layout

```
radiant-harness/
├── cmd/radiant/                       CLI source
│   ├── main.go                          ← default entrypoint
│   ├── cmd_setup_mcp.go                 ← 11-agent router
│   ├── cmd_setup_mcp_per_agent.go       ← per-agent merge functions
│   ├── cmd_mcp_serve.go                 ← MCP server bootstrap
│   ├── cmd_mcp_runtime.go               ← MCP server impl (radiant_run)
│   ├── cmd_host_info.go                 ← radiant host-info
│   └── mcp_types.go                     ← shared MCP wire types
│
├── internal/
│   ├── hostdetect/                      ← runtime agent detection (env + /proc)
│   ├── loop/                            ← Discover→Plan→Execute→Verify engine
│   ├── llm/                             ← Backend interface + SamplingBackend (MCP)
│   ├── mcpbridge/                       ← MCP tool bridge
│   ├── skill/  semantic/  policy/       ← skill + semantic + policy layers
│   ├── tools/                           ← file/search/edit tools
│   ├── context/  fsutil/  gaterun/
│   ├── ontology/                        ← project ontology hooks
│   └── …                                ← 12 packages total
│
├── scripts/
│   └── smoke-test.sh                    ← 17-check binary verification
│
├── docs/
│   ├── HOST-AGENTS.md                   ← detection matrix
│   ├── ARCHITECTURE.md                  ← architecture overview
│   └── TWO-REPOS.md                     ← Light vs Full rationale
│
├── Makefile                             ← make build, make release, make smoke
├── LICENSE                              ← MIT
└── README.md                            ← you are here
```

---

## FAQ

**Q: Why no API key?**
A: Every LLM call is delegated to the host agent via MCP `sampling/createMessage`. Your agent already has a model configured; the harness just drives the loop. The Light binary has zero HTTP egress for LLM calls — verified at build time via `nm`/`strings`.

**Q: I'm an agent — what tools can I call?**
A: One: `radiant_run(goal, profile?, max_iter?, max_cost?, max_time?)`. It blocks until the harness finishes, then returns the full execution trace as JSON. Use it for non-trivial tasks. For trivial ones (typo fix, single-file read), don't call it.

**Q: I'm an agent and I want to read a skill. How?**
A: The harness bundles 69 reference skills (Go architecture, MCP internals, ML, finance risk, regulatory, …). They're available on disk inside the binary. Use your file-reading tools to access them at the path your `radiant host-info` reports, or ask the harness to list them via MCP resources (when supported by your client).

**Q: Is the harness sending my code anywhere?**
A: No. The binary has zero HTTP egress for LLM calls (verified via `nm`/`strings`). The only outbound traffic is to the MCP stdio channel, which goes to your local agent.

**Q: Does it phone home?**
A: No telemetry, no analytics, no update checks. The binary is offline-first.

**Q: I need `radiant loop`, `radiant run --resume`, `radiant fleet`, `radiant doctor`, or any other CLI command. Where is it?**
A: That's the **Full** binary: [`quant-risk/radiant-harness-full`](https://github.com/quant-risk/radiant-harness-full). The Light binary is intentionally minimal — see [Light vs Full](#-light-vs-full).

**Q: Can I add a new agent to `setup-mcp`?**
A: Yes. Add a `case "<agent-name>":` block in `cmd_setup_mcp_per_agent.go` with the agent's config format and signature env vars. See [`docs/HOST-AGENTS.md`](docs/HOST-AGENTS.md).

**Q: How is this different from Claude Code's / OpenAI Codex's native loop?**
A: Three things: (1) the verifier is a separate LLM call so the same model never approves its own work, (2) it works across 11 MCP agents (not just one), (3) every step is traceable to portable JSONL.

**Q: Is it stable?**
A: Yes. First public release at v3.0.0. Semver from here.

---

## Contributing

Issues, PRs, and forks welcome.

- New agent in `setup-mcp` → edit `cmd_setup_mcp_per_agent.go`.
- New host-detect signature → edit `internal/hostdetect/hostdetect.go`.
- New MCP tool → edit `cmd_mcp_runtime.go`.
- New bundled skill → drop a directory under `internal/skill/skills/` with `SKILL.md` + `frontmatter.yaml`. Schema is enforced by `make smoke`.

---

## License

MIT — see [`LICENSE`](LICENSE).

---

<div align="center">

**Built with care in 🇧🇷**

[github.com/quant-risk/radiant-harness](https://github.com/quant-risk/radiant-harness) · [v3.0.1](https://github.com/quant-risk/radiant-harness/releases/tag/v3.0.1) · [report a bug](https://github.com/quant-risk/radiant-harness/issues) · [Full repo](https://github.com/quant-risk/radiant-harness-full)

</div>