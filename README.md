<!-- Hero -->
<div align="center">

<br/>

# ✨ radiant-harness

### The autonomous dev harness — wired to whatever agent you're using.

**Zero API keys · Zero HTTP egress · Zero telemetry. 55 commands. 11 agents.**

Works with **Claude Code · Cursor · Hermes · Codex · Cline · Kimi · OpenCode · OpenClaw · Windsurf · Zed · VS Code Copilot** — and any MCP-compatible agent.

<br/>

![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg?style=for-the-badge)
![Version](https://img.shields.io/badge/release-v3.2.0-blueviolet?style=for-the-badge)
![Go](https://img.shields.io/badge/Go-1.22%2B-00ADD8?style=for-the-badge&logo=go&logoColor=white)
![Binary](https://img.shields.io/badge/binary-~11MB-success?style=for-the-badge)
![API keys](https://img.shields.io/badge/API_keys-NONE-success?style=for-the-badge)
![Telemetry](https://img.shields.io/badge/telemetry-OFF-lightgrey?style=for-the-badge)
![Commands](https://img.shields.io/badge/commands-55-brightgreen?style=for-the-badge)

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
                    │  55 commands · 11 agents         │
                    │  inference:    MCP sampling      │
                    │  back to host agent              │
                    │                                  │
                    │  no API keys · no HTTP egress    │
                    │  no telemetry · no vendor lock   │
                    └──────────────────────────────────┘
```

<br/>

```bash
# 1. install (single binary, no deps)
curl -L https://github.com/quant-risk/radiant-harness/releases/latest/download/radiant-$(uname -s | tr A-Z a-z)-$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/') \
  -o /usr/local/bin/radiant && chmod +x /usr/local/bin/radiant

# 2. wire into your agent (auto-detects which one)
radiant setup-mcp

# 3. use from any shell
radiant loop start "add /healthz endpoint"
radiant run specs/0001-foo
radiant fleet start "migrate auth"
```

<br/>

**[Install](#-installation) · [Quickstart](#-quickstart) · [Commands](#-commands) · [How it works](#-how-it-works) · [Agents](#-supported-agents) · [FAQ](#-faq)**

</div>

---

## Why radiant-harness?

You already use an agent. It can read files, run tests, edit code. But it loops: makes the same change, gets the same error back, burns your context window until you shut it off.

**radiant-harness gives your agent a backbone.** A single binary with **55 commands** that drive the same loop your agent would do by hand — but with a budget, crash-safe state, and a separate verifier that never lets the same model approve its own work.

The unique constraint: **no API key**. radiant never talks to an LLM provider directly. Every inference is delegated to whatever agent you're running, via MCP `sampling/createMessage`. Claude Code, Cursor, Hermes, Cline, Kimi, anything. The host agent's credentials, the host agent's model — your code never leaves the loop you control.

---

## What you get

| | |
|---|---|
| **🔁 Crash-safe loop** | `radiant loop` — start, status, resume. Every step is journaled; Ctrl-C, OOM, network drops are recoverable. |
| **⚡ One-shot run** | `radiant run specs/<id>` — the same engine in single-shot mode for a specific spec. |
| **🪞 Separate verifier** | The work-product is judged by a *different* LLM call — never the same model that wrote it. |
| **🧮 Budget engine** | Token, cost, wall-clock, and tool-call caps. Fails loud when exhausted. |
| **🪜 Fleet mode** | `radiant fleet` — Planner + Implementer + Verifier + Summarizer in parallel. |
| **📋 Spec-driven dev** | `radiant spec`, `radiant product`, `radiant init` — CONTEXT.md / spec.md / tasks.md scaffolding. |
| **✅ Verification suite** | `radiant validate`, `radiant evals`, `radiant audit`, `radiant review-pr` — AC↔test coverage, gate results. |
| **🚀 Release & CI** | `radiant release`, `radiant setup-ci` — version bump + cross-compile + GitHub Actions / GitLab CI / CircleCI. |
| **🩺 Doctor** | `radiant doctor` — diagnose agent, MCP wiring, zero-HTTP guarantee. |
| **🔌 MCP-native** | `radiant setup-mcp` wires into any of 11 agents in 30 seconds. Becomes a single `radiant_run` tool. |
| **📚 69 bundled skills** | Domain knowledge the harness can read on demand: Go architecture, MCP internals, ML, finance risk, regulatory, … |
| **🪶 Zero footprint** | Single ~11 MB binary. Zero HTTP egress for LLM calls (verified at build time via `nm`/`strings`). |
| **🔓 Vendor-neutral** | Trace files are plain JSONL. Spec files are plain Markdown. Take it with you. |

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
radiant --version                   # → radiant 3.2.0
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

### 2. Use it from any shell

```bash
# The loop engine — multi-step, crash-safe, verifiable
radiant loop start "add /healthz endpoint that returns 200 OK with JSON body"

# One-shot run for a specific spec
radiant run specs/0001-add-healthz

# Fleet: parallel agents (Planner + Implementer + Verifier + Summarizer)
radiant fleet start "migrate from REST to gRPC"

# Doctor: diagnose the wire-up
radiant doctor

# Spec scaffolding
radiant spec "rate-limit middleware" --ac="AC1: 100 req/min per IP" --ac="AC2: returns 429 over quota"

# Lean Inception
radiant product "API observability for small dev teams" --mvp-weeks=6
```

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

### 4. Drive a loop from your agent (the MCP path)

From inside Claude Code (or any wired agent):

> *"use radiant-harness to add a /healthz endpoint with tests"*

Your agent calls `radiant_run`, the harness spins up the loop, every LLM call routes back to your agent via MCP `sampling/createMessage`, and you get a JSONL trace at `.radiant-harness/traces/<run-id>.jsonl`.

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
  │   ├── traces/<run-id>.jsonl      ← every step, every token         │
  │   └── spec.md, tasks.md          ← portable Markdown artifacts     │
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

This is the complete CLI surface — **55 commands** in one binary:

### Loop engine

| Command | What it does |
|---|---|
| `radiant loop start <goal>` | Start a new feedback loop. |
| `radiant loop status` | Show running loops. |
| `radiant loop resume <id>` | Resume a crashed loop from BoltDB. |
| `radiant loop cancel <id>` | Cancel a running loop. |
| `radiant loop history` | Past loop runs. |
| `radiant loop export <id>` | Export the trace as Markdown. |
| `radiant loop diff <id>` | Show what changed during the loop. |
| `radiant run specs/<id>` | One-shot run against a specific spec. |
| `radiant fleet start <goal>` | Multi-agent parallel coordination. |
| `radiant fleet status` | Show fleet runs. |
| `radiant fleet dispatch` | Add tasks to a running fleet. |
| `radiant trace show <id>` | Inspect the reasoning trace. |
| `radiant trace list` | List available traces. |

### Spec-driven dev

| Command | What it does |
|---|---|
| `radiant init` | Scaffold the SDD pipeline (AGENTS.md + skills). |
| `radiant spec "<intent>"` | Create spec.md + tasks.md for a feature. |
| `radiant product "<why>"` | Start a Lean Inception. |
| `radiant validate specs/<id>` | Validate SDD conformity. |
| `radiant validate-file <path>` | Validate a scaffolded plan or spec. |
| `radiant evals` | AC↔test coverage (fidelity) across all specs. |
| `radiant audit` | Project layout, AC traceability, ADR validity. |
| `radiant review-pr specs/<id>` | Generate pr-review.md. |
| `radiant adr` | Create an Architecture Decision Record. |
| `radiant diagramar` | C4 Mermaid diagram template. |
| `radiant views [--agent=…]` | Generate native agent views. |

### Release & CI

| Command | What it does |
|---|---|
| `radiant release <version>` | Version bump + tests + cross-compile + commit + tag. |
| `radiant setup-ci --provider=github` | Generate CI workflow. |

### Skills & docs

| Command | What it does |
|---|---|
| `radiant skills list` | List bundled skills. |
| `radiant skills validate` | Validate skill schema. |
| `radiant update` | Refresh bundled skills + AGENTS.md. |
| `radiant context detect` | Detect project domain. |
| `radiant context assemble` | Build minimal CONTEXT.md. |
| `radiant context compress` | Compress CONTEXT.md. |
| `radiant ontology export` | Export the harness ontology. |
| `radiant boot` | Print a minimal project manifest. |

### Diagnostics & session

| Command | What it does |
|---|---|
| `radiant doctor` | Diagnose the radiant environment. |
| `radiant state` | Show current session state. |
| `radiant handoff` | Pause: write state to .radiant-harness/state.md. |
| `radiant worktree add` | Isolated git worktree for parallel agents. |
| `radiant budget estimate` | Token budget estimation. |
| `radiant tools list` | Inspect the tool registry. |

### Vertical scaffolds (MCP sampling)

| Command | What it does |
|---|---|
| `radiant model` | Scaffold a model spec. |
| `radiant predict` | Model serving request. |
| `radiant train` | Training plan. |
| `radiant evaluate` | Evaluation plan. |
| `radiant drift` | Drift monitoring. |
| `radiant profile` | Data profile. |
| `radiant stats` | Hypothesis-test plan. |
| `radiant causal-estimate` | Causal analysis. |
| `radiant incident` | Start an incident. |
| `radiant autodata` | Auto-author a skill from a prompt. |
| `radiant eval` | Run a prompt N times for latency/cost. |
| `radiant bench` | Benchmark vs TLC, Spec Kit, OpenSpec, Superpowers. |
| `radiant improve` | Self-improvement engine from traces. |
| `radiant integrate` | Read-only listing of declared MCP integrations. |

### MCP & agents

| Command | What it does |
|---|---|
| `radiant setup-mcp [--agent=X]` | Detect agent + write MCP config. |
| `radiant mcp serve` | Start the MCP server on stdio. |
| `radiant host-info [--json] [--verbose]` | Print detected host agent. |
| `radiant completion <shell>` | Shell completion (bash, zsh, fish, powershell). |
| `radiant help` | Complete command list. |

### Low-level (utility)

| Command | What it does |
|---|---|
| `radiant config` | View current profile (no API key, just budget defaults). |
| `radiant models` | List model presets (read-only). |
| `radiant pricing list` | LLM pricing table (read-only; no HTTP fetch). |
| `radiant semantic resolve <term>` | Resolve a business term against the semantic model. |
| `radiant camada-agentica` | Audit the agentic layer (AGENTS.md, native views). |
| `radiant security` | Hardcoded secrets + sensitive file perms. |
| `radiant telemetry status` | Local-only usage stats (off by default). |

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

## The zero-API-key guarantee

```bash
nm bin/radiant | grep -iE 'chatAnthropic|HTTPBackend|NewHTTPBackend'
# (must return 0 results)

strings bin/radiant | grep -iE 'anthropic|openai|openrouter'
# (must return 0 results)

ls -lh bin/radiant
# ≈ 11 MB
```

These checks are part of `make smoke` and run on every commit.

### Tests

```bash
make test         # 24 packages OK, 0 FAIL
make smoke        # 17/17 verification checks pass
```

---

## Project layout

```
radiant-harness/
├── cmd/radiant/                       CLI source (29 files)
│   ├── main.go                          ← entrypoint, registers all 55 commands
│   ├── cmd_setup_mcp.go                 ← 11-agent router
│   ├── cmd_mcp_runtime.go               ← MCP server (single tool: radiant_run)
│   ├── cmd_loop.go, cmd_run.go, cmd_fleet.go  ← loop engine wrappers
│   ├── cmd_spec.go                      ← spec, adr, diagramar, product, views, …
│   ├── cmd_audit.go                     ← audit, camada-agentica, evals, release, security
│   ├── cmd_doctor.go, cmd_diagnose.go   ← diagnostics
│   ├── cmd_telemetry.go                 ← telemetry + stats + model + predict + train + …
│   ├── helpers.go                       ← shared scaffolding helpers
│   └── …                                ← 29 files total
│
├── internal/
│   ├── loop/                            ← Discover→Plan→Execute→Verify engine
│   ├── engine/                          ← Universal SDD harness engine
│   ├── llm/                             ← Backend interface + SamplingBackend (MCP)
│   │                                      + Client shim (Light-only)
│   ├── mcpbridge/                       ← MCP tool bridge
│   ├── hostdetect/                      ← runtime agent detection
│   ├── skill/ semantic/ policy/         ← skill + semantic + policy layers
│   ├── tools/                           ← file/search/edit tools
│   ├── config/ pricing/ routing/        ← profile, pricing, model routing
│   ├── context/ fsutil/ gaterun/        ← context engine, fs utilities
│   ├── fleet/ harness/ worktree/        ← multi-agent fleet + git worktrees
│   ├── scaffold/ spec/ ontology/        ← SDD scaffolding
│   ├── boot/ mode/ quality/ routing/    ← project bootstrap, mode, quality gates
│   ├── schedule/ webhook/ slog/ types.go ← scheduling + observability
│   └── …                                ← 28 packages total
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
├── CHANGELOG.md                         ← version history
├── LICENSE                              ← MIT
└── README.md                            ← you are here
```

---

## FAQ

**Q: Why no API key?**
A: Every LLM call is delegated to the host agent via MCP `sampling/createMessage`. Your agent already has a model configured; the harness just drives the loop. The binary has zero HTTP egress for LLM calls — verified at build time via `nm`/`strings`.

**Q: I'm an agent — what tools can I call?**
A: One: `radiant_run(goal, profile?, max_iter?, max_cost?, max_time?)`. It blocks until the harness finishes, then returns the full execution trace as JSON. Use it for non-trivial tasks. For trivial ones (typo fix, single-file read), don't call it.

**Q: How do I run a loop from my shell instead of from inside an agent?**
A: `radiant loop start "your goal"`. The harness drives the loop itself and calls `sampling/createMessage` to whatever agent you wired in via `radiant setup-mcp`. If no agent is connected, you get a clear "run `radiant setup-mcp` first" error.

**Q: Is the harness sending my code anywhere?**
A: No. The binary has zero HTTP egress for LLM calls (verified via `nm`/`strings`). The only outbound traffic is to the MCP stdio channel, which goes to your local agent.

**Q: Does it phone home?**
A: No telemetry, no analytics, no update checks. The binary is offline-first.

**Q: Can I add a new agent to `setup-mcp`?**
A: Yes. Add a `case "<agent-name>":` block in `cmd_setup_mcp_per_agent.go` with the agent's config format and signature env vars. See [`docs/HOST-AGENTS.md`](docs/HOST-AGENTS.md).

**Q: Can I add a new command?**
A: Yes. Either add a `root.AddCommand(yourCmd)` to an existing `registerXxxCmds()` in `cmd/radiant/`, or create a new file with a `registerYourCmds(root *cobra.Command)` function and call it from `main.go`.

**Q: How is this different from Claude Code's / OpenAI Codex's native loop?**
A: Three things: (1) the verifier is a separate LLM call so the same model never approves its own work, (2) it works across 11 MCP agents (not just one), (3) every step is traceable to portable JSONL.

**Q: Is it stable?**
A: Yes. First public release at v3.0.0; v3.2.0 adds the full 55-command surface back into Light. Semver from here.

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

[github.com/quant-risk/radiant-harness](https://github.com/quant-risk/radiant-harness) · [v3.2.0](https://github.com/quant-risk/radiant-harness/releases/tag/v3.2.0) · [report a bug](https://github.com/quant-risk/radiant-harness/issues) · [Full repo (internal)](https://github.com/quant-risk/radiant-harness-full)

</div>