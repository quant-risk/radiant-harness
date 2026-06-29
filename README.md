# radiant-harness

> A vendor-neutral autonomous development harness for any LLM.
> Shipped as a single binary — works with Claude Code, Cursor, Codex, Copilot, Gemini CLI, Windsurf, Hermes, and any MCP-compatible agent.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Version](https://img.shields.io/badge/version-2.37.0-blue.svg)](CHANGELOG.md)
[![Tests](https://img.shields.io/badge/tests-25_packages_green-brightgreen.svg)](CHANGELOG.md)
[![MCP](https://img.shields.io/badge/MCP-radiant__run-purple.svg)](AGENTS.md)
[![Modes](https://img.shields.io/badge/modes-light%20%7C%20full-blueviolet.svg)](docs/MODES.md)

---

## What it is

`radiant` is a CLI harness for **autonomous LLM-driven development**. One binary, zero external dependencies at runtime.

**Core engines:**

| Engine | What it does |
|--------|-------------|
| **Loop Engine** | Crash-safe `Discover→Plan→Execute→Verify→Persist` cycle. Verifier is always a separate LLM call. |
| **Fleet Engine** | Parallel multi-agent dispatch with conflict-safe shared state, concurrency cap, and auto-retry. |
| **Context Engine** | Detects project domain and lazy-loads 3–10 relevant skills (~300 tokens vs 55K for all 60). |
| **Semantic Model** | Curated "what it means here" layer — PD/LGD/EAD/RWA for credit-risk, with regulation references (CMN 4.966, IFRS 9, Basileia). |
| **MCP Server** | Exposes the full harness as MCP tools — any MCP-compatible agent calls `radiant_run` and gets results back. |

Works with any OpenAI-compatible API (Claude, GPT-4o, Gemini, Mistral, OpenRouter, local models).

---

## Choose your mode

`radiant` runs in one of two modes. Pick the one that matches how you want inference to happen:

| Mode | Who provides the LLM | When to use |
|------|---------------------|-------------|
| **Light** | Your host agent (Claude Code, Hermes, Cursor, ...) via MCP sampling | You're already paying for an agent. Zero new credentials. |
| **Full**  | The harness, via direct HTTP to OpenRouter / OpenAI / Anthropic / Groq / Mistral / xAI / custom | CI, cron, batch, or any case where you don't have an agent session. |

```bash
# Light: harness possesses the agent
radiant setup-mcp --agent=claude   # one-time
# inside your agent: "use radiant-harness to: add input validation"

# Full: autonomous, no agent
export OPENROUTER_API_KEY=sk-...
radiant loop start "add input validation to /api/users" --profile=standard
```

`radiant mode show` reports the active mode and where it was resolved from. See [`docs/MODES.md`](docs/MODES.md) for the full guide.

---

## Quickstart — 3 minutes to first autonomous run

### Option A: compile from source

```bash
git clone https://github.com/quant-risk/radiant-harness.git
cd radiant-harness
go build -o radiant ./cmd/radiant
./radiant doctor
```

### Option B: install via go

```bash
go install github.com/quant-risk/radiant-harness/cmd/radiant@latest
```

### Option C: use the pre-built binary

```bash
# macOS arm64 (Apple Silicon)
curl -L https://github.com/quant-risk/radiant-harness/releases/latest/download/radiant-darwin-arm64 -o radiant
chmod +x radiant && sudo mv radiant /usr/local/bin/
```

### First run

```bash
export ANTHROPIC_API_KEY=sk-...   # or OPENROUTER_API_KEY / OPENAI_API_KEY

radiant doctor                    # verify environment
radiant boot                      # read project manifest + AGENT PROTOCOL

radiant loop start "add rate limiting to /api/users" --profile=standard
radiant loop status               # monitor
radiant loop export <run-id>      # full trace
```

---

## Using radiant as a sub-agent (MCP)

This is the primary way to use radiant from any coding agent (Claude Code, Hermes, Cursor, etc.).

### One-time setup

Add to your agent's MCP config:

```json
{
  "mcpServers": {
    "radiant": {
      "command": "/path/to/radiant",
      "args": ["mcp-serve"]
    }
  }
}
```

Or use the auto-detection command:

```bash
radiant setup-mcp   # detects Claude Code, Cursor, Windsurf, Zed, VSCode automatically
```

### Two operating modes

| Mode | Command | How it works |
|------|---------|-------------|
| **HTTP mode** (default) | `radiant mcp-serve` | Harness makes its own LLM calls using an API key from the environment |
| **Sampling mode** | `radiant mcp-serve --sampling` | Harness uses the calling agent as its LLM — no API key needed |

**Sampling mode** is the recommended mode for Claude Code, Hermes, Cursor, and any other MCP-compatible agent. The harness "possesses" the agent: every LLM call in the loop (planner, executor, verifier) is dispatched back to the calling agent via `sampling/createMessage`. The harness controls the state machine and orchestration; the agent provides the intelligence.

```
User → Agent → radiant_run({ goal })
                  ↓
              harness loop
                  ├─ DISCOVER → sampling/createMessage → Agent reasons → discovery
                  ├─ PLAN     → sampling/createMessage → Agent plans   → plan
                  ├─ EXECUTE  → sampling/createMessage → Agent codes   → code
                  └─ VERIFY   → sampling/createMessage → Agent checks  → verdict
                  ↓
              returns full trace to Agent → User
```

### Claude Code setup (sampling mode)

```json
// .claude/settings.json
{
  "mcpServers": {
    "radiant": { "command": "radiant", "args": ["mcp-serve", "--sampling"] }
  }
}
```

### Usage after setup

The agent calls ONE tool. No extra prompt engineering needed.

```
radiant_run({ goal: "add input validation to POST /api/users" })
```

The harness runs the full loop, blocks until done, and returns the complete trace.

**Available MCP tools:**

| Tool | What it does |
|------|-------------|
| `radiant_run` | **Full loop in one call** — start + execute + export. Blocks until done. |
| `radiant_loop_start` | Start a loop (non-blocking from MCP perspective) |
| `radiant_loop_status` | Get progress of a run |
| `radiant_loop_list` | List all runs |

### Prompt to any agent

Once the MCP server is registered:

```
Read the project context and use radiant-harness to: <your goal>
```

Or even simpler — the agent reads `AGENTS.md` at session start and knows to call `radiant_run` automatically.

---

## Loop Engine

Crash-safe state machine: `idle → discover → plan → execute → verify → persist → done`

```
radiant loop start "add rate limiting to /api/users"

  iteration 1
  ├─ discover  → domain: backend, skills: [api, security]
  ├─ plan      → decompose into tasks
  ├─ execute   → write internal/api/ratelimit.go
  ├─ verify    → REJECTED: missing tests        ← separate agent, never self-grades
  ├─ execute   → write internal/api/ratelimit_test.go
  └─ verify    → APPROVED: score 0.92
     persist   → checkpoint + JSONL trace
     done      → exit reason: success
```

**Guards:** `--max-iter`, `--max-cost`, `--max-time`, `--stall-patience`

**Structured logging:** `--log-json` emits JSONL per LLM call to stdout.

**Full command reference:**

```bash
radiant loop start "<goal>" [--profile=lean|standard|thorough]
                            [--model=<id>]
                            [--max-iter=N]
                            [--max-cost=2.00]
                            [--max-time=10m]
                            [--auto-route]
                            [--log-json]
                            [--webhook-url=<url>]
radiant loop status [<run-id>] [--json]
radiant loop list
radiant loop history [--json]
radiant loop resume <run-id>
radiant loop cancel <run-id>
radiant loop export <run-id> [--format=json|md]
radiant loop diff <run-id> [--base=main] [--stat]
```

---

## Fleet Engine

Parallel multi-agent dispatch for goals that decompose into independent sub-tasks.

```bash
radiant fleet start "<goal>"
radiant fleet status <run-id> [--json]
radiant fleet summary <run-id> [--json]
radiant fleet history [--json]
radiant fleet resume <run-id>
radiant fleet retry <run-id> <task-id>
radiant fleet cancel <run-id>
radiant fleet dispatch --concurrency=4 --max-retries=2
```

**Config defaults** (`.radiant.yaml`):

```yaml
model: claude-sonnet-4-6
max_iter: 20
profile: standard
webhook_url: ""
fleet_concurrency: 4
fleet_max_retries: 2
auto_route: true
mode: full                # light | full | auto (default: auto-detect)
intensity: full           # executor intensity for lazy-executor skill
```

---

## Mode, Pricing, and Semantic Commands

```bash
# Mode (Light/Full)
radiant mode show                       # show resolved mode + source
radiant mode set light                  # persist in .radiant.yaml
radiant mode set full

# Pricing
radiant pricing list                    # tabulated model rates
radiant pricing stale                   # warn if rates > 90 days old
radiant pricing refresh                 # instructions to update rates

# Semantic model (credit-risk, market-risk, ...)
radiant semantic list                   # available domains
radiant semantic show credit-risk       # full markdown of one domain
radiant semantic resolve credit-risk RWA     # formula + regulation
radiant semantic search credit-risk basileia # fuzzy search
```

## Lazy-executor skill

The harness ships a `lazy-executor` skill (ported from the
[ponytail ladder](https://github.com/DietrichGebert/ponytail)) that
biases the executor toward the minimum code that works. Three
intensities, default `full`:

```bash
radiant loop start "..." --intensity=lite   # build what was asked, suggest lazy alt
radiant loop start "..." --intensity=full   # ladder enforced (default)
radiant loop start "..." --intensity=ultra  # YAGNI extremist
```

In projects detected as `credit-risk`, the loop runner also
auto-injects the semantic model (PD, LGD, EAD, RWA, IFRS 9
provisioning formulas) into the executor system prompt.

---

## Other commands

### Context & Boot
```bash
radiant boot                              # ≤500-token manifest + AGENT PROTOCOL
radiant boot --world-model               # + compact ontology
radiant context detect [--json]
radiant context assemble [--budget=N]
radiant context compress --budget=2000
```

### Diagnostics
```bash
radiant doctor                            # API key, git, model, worktrees
```

### Webhooks
```bash
radiant loop start "<goal>" --webhook-url=https://...
# fires: loop.done / loop.failed / fleet.task.done / fleet.done
```

### Worktrees
```bash
radiant worktree add <name>
radiant worktree list
radiant worktree remove <path>
radiant worktree prune
```

### Agent views (native files per IDE)
```bash
radiant views --agent=claude     # .claude/settings.json + skills
radiant views --agent=cursor     # .cursor/rules/*.mdc
radiant views --agent=copilot    # .github/copilot-instructions.md
radiant views --agent=gemini     # GEMINI.md
radiant views --agent=windsurf   # .windsurfrules
radiant views --agent=codex      # AGENTS.md
radiant views --agent=all --force
```

### Classic SDD workflow
```bash
radiant init . --all --yes
radiant product "..."
radiant spec "..."
radiant run specs/0001-<slug>
radiant validate specs/0001-<slug> --gates
radiant audit
radiant release v0.1.0
```

---

## Architecture

```
cmd/radiant/          ← CLI entrypoint (cobra) + MCP server
internal/loop/        ← Loop Engine: cycle, budget, verifier, tracer, PID, JSONL log
internal/fleet/       ← Fleet Engine: planner, dispatcher, store, E2E tests
internal/context/     ← Context Engine: domain detect, skill selector
internal/config/      ← .radiant.yaml project config
internal/webhook/     ← fire-and-forget HTTP POST webhooks
internal/slog/        ← structured JSONL logger
internal/boot/        ← boot manifest + AGENT PROTOCOL renderer
internal/ontology/    ← world model (domains, axioms)
internal/worktree/    ← git worktree isolation
internal/scaffold/    ← native agent view generation
internal/llm/         ← OpenAI / Anthropic / OpenRouter clients
internal/skill/       ← skill schema + bundle (60 skills, go:embed)
internal/engine/      ← SDD execution engine
internal/harness/     ← quality gates + policy enforcement
internal/spec/        ← spec + task + ADR parsing
```

Single binary, no external runtime dependencies. Skills embedded via `//go:embed`.

---

## Skills (60 bundled)

Lazy-loaded — only 3–10 loaded per session based on domain detection.

**Core:** `nova-feature`, `nova-product`, `kickoff`, `clarificar`  
**Quality:** `validar`, `auditar`, `metricas`, `evals`, `revisar-pr`  
**Architecture:** `adr`, `diagramar`, `mapear`, `camada-agentica`, `handoff`, `roadmap`  
**Finance & Risk:** `finance`, `credit-risk`, `market-risk`, `liquidity-risk`, `operational-risk`, `model-risk`, `stress-test`, `regulatory`, `actuarial`, `accounting`, `controlling`, `valuation`, `aml-kyc`, `fraud-detection`, `capital-markets`  
**ML & Data:** `ml`, `deep-learning`, `reinforcement-learning`, `causal`, `bayesian`, `stats`, `econometrics`, `synthetic-data`, `data`  
**Engineering:** `api`, `cli`, `security`, `setup-ci`, `integracoes`, `update`, `incident`  
**Domain:** `frontend`, `mobile`, `iot`, `game`, `blockchain`, `marketing`  
**Science:** `biology`, `chemistry`, `physics`, `quantum-physics`, `quantum-ml`

---

## Documentation

| Doc | What it covers |
|-----|----------------|
| [`AGENTS.md`](AGENTS.md) | Full agent onboarding — commands, profiles, rules |
| [`CLAUDE.md`](CLAUDE.md) | Claude Code specific instructions |
| [`docs/AGENT-SYSTEM-PROMPT.md`](docs/AGENT-SYSTEM-PROMPT.md) | System prompt template for external agents (Hermes, mimo, etc.) |
| [`docs/LOOP-ENGINE.md`](docs/LOOP-ENGINE.md) | Loop state machine, exit conditions |
| [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) | Full architecture deep-dive |
| [`CHANGELOG.md`](CHANGELOG.md) | Version history |

---

## License

MIT — see [LICENSE](LICENSE).
