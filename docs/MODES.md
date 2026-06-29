# radiant-harness — Operating Modes

> Pick the mode that matches how you want inference to happen. Both modes
> use the same `radiant` binary. The choice is a deployment-time decision,
> not a build-time one.

The harness runs in one of two modes. Each has the same autonomous loop,
the same state machine, the same budget brakes, the same verifier, the
same skill system. What differs is **who provides the LLM inference**.

## Quick decision

```
┌─ Are you already inside Claude Code / Cursor / Hermes / Codex / Copilot?
│
├─ YES → Light mode (zero setup beyond `radiant setup-mcp`)
│        The host agent provides inference. No API key needed.
│
└─ NO  → Full mode (CI, cron, batch, or standalone)
         You provide the API key. Harness calls the provider directly.
```

```
You have an LLM API key but no agent session?
  → Full mode. `export OPENROUTER_API_KEY=sk-...` and go.

You have an agent session (Claude Code etc.) and no API key?
  → Light mode. `radiant setup-mcp` and call `radiant_run` from the agent.

You have both?
  → Either works. Light is cheaper (no double-billing); Full is more
    portable (works anywhere, no agent dependency).
```

## Light — harness possesses the agent

In Light mode the harness treats the host agent as its inference backend.
The host agent (Claude Code, Hermes Agent, Cursor, Codex, Copilot) holds
the credentials and runs the actual model call. The harness emits a JSON-RPC
`sampling/createMessage` request over the MCP transport and waits for the
host to do the inference and return the result.

**Use Light when:**
- You already pay for Claude Code, Cursor, or another MCP-capable agent.
- You want zero new credentials.
- You want the agent's existing context (recent files, open tabs, project
  notes) to influence the harness's LLM calls naturally.
- You're prototyping and want to skip API key plumbing.

**Setup:**
```bash
radiant setup-mcp --agent=claude   # one-time, registers MCP server
# inside your agent session:
> use radiant-harness to: add input validation to /api/users
# the agent invokes radiant_run, which is in Light mode by default
```

**Architecture:**
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

The harness owns the state machine and the orchestration. The agent
provides the intelligence.

## Full — autonomous, no agent

In Full mode the harness calls LLM HTTP endpoints directly. It owns its
own credentials, its own cost tracking, its own budget brakes. The host
agent (if any) is irrelevant to the harness's operation.

**Use Full when:**
- Running in CI/CD without an interactive agent.
- Running on a schedule (cron, systemd timer, Kubernetes CronJob).
- You want full control over model selection, cost, and provider routing.
- You're behind a firewall that only allows outbound HTTPS to specific
  providers.
- You need to audit each LLM call independently (compliance, CMN 4.966).

**Setup:**
```bash
export OPENROUTER_API_KEY=sk-...   # or OPENAI_API_KEY, ANTHROPIC_API_KEY
radiant config --provider=openrouter --model=claude-sonnet-4-6
radiant loop start "fix the race condition in dispatch.go"
```

**Architecture:**
```
User → radiant loop start "<goal>"
         ↓
      harness loop (in-process, no agent involved)
         ├─ DISCOVER → HTTP POST /v1/chat/completions → OpenRouter/Claude → discovery
         ├─ PLAN     → HTTP POST → plan
         ├─ EXECUTE  → HTTP POST → code
         └─ VERIFY   → HTTP POST → verdict
         ↓
      persist + export trace
```

The harness owns everything end-to-end.

## Resolution

When you run `radiant loop start`, the mode is resolved in this order:

| Priority | Source | Example |
|----------|--------|---------|
| 1 | `--mode=light\|full\|auto` flag | `radiant loop start "..." --mode=full` |
| 2 | `RADIANT_MODE` env var | `export RADIANT_MODE=full` |
| 3 | `.radiant.yaml` `mode:` field | `mode: full` |
| 4 | Auto-detect | See below |

`radiant mode show` displays the resolved mode and where it came from.

**Auto-detect logic (priority order):**
1. If a radiant MCP config is found (project `.claude/settings.json`,
   `~/.claude/settings.json`, project `.cursor/mcp.json`, `.windsurf/`,
   `.zed/`, `.vscode/`) → **Light** (assume agent session).
2. If any LLM API key is set in env (`OPENROUTER_API_KEY`,
   `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, etc.) → **Full**.
3. Default → **Light** (safe: clearer error if no MCP available than
   leaking an API key requirement).

## What works in each mode

| Capability | Light | Full |
|------------|:-----:|:----:|
| `radiant loop start` from CLI | ❌ (use MCP) | ✅ |
| `radiant_run` from agent | ✅ | ❌ (won't be called) |
| `radiant fleet start` from CLI | ❌ (use MCP) | ✅ |
| `radiant_fleet_start` from agent | ✅ | ❌ |
| `radiant doctor` | ✅ | ✅ |
| `radiant boot` | ✅ | ✅ |
| `radiant context detect` | ✅ (free, no LLM) | ✅ |
| `radiant validate` | ✅ (free, no LLM) | ✅ |
| `radiant views` | ✅ (free, no LLM) | ✅ |
| `radiant setup-mcp` | ✅ | ✅ (after install) |
| `radiant pricing` | ✅ | ✅ |
| CI/cron usage | ❌ | ✅ |

The CLI-only commands (`doctor`, `boot`, `context`, `validate`, `views`,
`setup-mcp`, `pricing`) work in both modes because they don't issue
LLM calls.

## Pricing implications

- **Light**: you pay your host agent's normal subscription (Claude Code,
  Cursor Pro, etc.). The harness adds no LLM cost on top.
- **Full**: you pay the provider directly per token. The harness tracks
  every call and reports cost in `radiant loop status` and `radiant loop
  export`.

## Choosing per-task

Sometimes a single project benefits from both modes. Examples:

- **Local dev with an agent** → Light. Fast iteration, no key management.
- **Nightly batch job** → Full. Runs without any agent session.
- **Customer demo with human-in-loop** → Light. The demo operator is
  in an agent; you want their context to inform the harness.
- **Compliance audit on a regulated dataset** → Full. Every LLM call
  is logged independently in your provider dashboard.

You can configure each project independently via `.radiant.yaml`.

## Forcing a mode

To force Light mode regardless of environment:
```bash
export RADIANT_MODE=light
radiant loop start "..."     # will require MCP context, errors otherwise
```

To force Full mode:
```bash
export RADIANT_MODE=full
export OPENROUTER_API_KEY=sk-...
radiant loop start "..."
```

To set the default per-project:
```bash
radiant mode set full    # writes .radiant.yaml
```

## What's next

- `docs/MODES.md` (this file) — the user-facing mode guide
- `internal/mode/mode.go` — the resolver (≤200 lines, fully tested)
- `cmd/radiant/cmd_mode.go` — `radiant mode show|set`
- `cmd/radiant/cmd_doctor.go` — mode shown in diagnostics
- `cmd/radiant/cmd_loop.go` and `cmd_fleet.go` — `--mode` flag

Both modes share the same loop engine, the same verifier, the same skill
system, the same pricing. The only thing that differs is who pays for the
tokens and which way the inference flows.