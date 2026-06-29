# Host Agents Reference

This document lists every host agent that radiant-harness can
auto-detect in runtime (Sprint 79+), the detection fingerprint
used, and whether the agent supports MCP sampling/createMessage
(needed for "possession" without an API key).

> **Reading the matrix:** "Env key" is the env var the agent sets
> when it's running. "PPID match" is the parent binary name pattern
> that the agent's CLI process usually has. "Sampling" = yes means
> the agent can answer MCP sampling requests, so the harness can
> possess the agent's LLM via `radiant mcp serve`.

| Agent             | Env key (any of)                                  | PPID match                | Sampling | Code path                 |
|-------------------|----------------------------------------------------|--------------------------|----------|---------------------------|
| Claude Code       | `CLAUDE_CODE_ENTRY`, `CLAUDE_CODE_SSE_PORT`, `CLAUDE_CODE_PID` | `claude-code`, `claude`, `Claude` | yes      | Light + Full              |
| Cursor            | `CURSOR_TRACE_ID`, `CURSOR_HOME`, `CURSOR_USER_DATA_DIR` | `cursor`, `cursor-server`, `Cursor`, `Cursor.exe` | yes      | Light + Full              |
| Hermes            | `HERMES_VERSION`, `HERMES_HOME`, `HERMES_AGENT_HOME`     | `hermes-agent`, `hermes-cli` | yes    | Light + Full              |
| Kimi CLI          | `KIMI_SHARE_DIR`, `KIMI_VERSION`, `KIMI_CONFIG_DIR`     | `kimi`, `kimi-cli`         | yes    | Light + Full              |
| OpenClaw          | `OPENCLAW_GATEWAY_URL`, `OPENCLAW_VERSION`, `OPENCLAW_WORKSPACE` | `openclaw`, `openclaw-cli` | yes | Light + Full            |
| Codex             | `CODEX_HOME`, `CODEX_THREAD_ID`, `CODEX_RUNTIME`, `CODEX_THREAD_ENV` | `codex`, `codex-cli` | yes      | Light + Full              |
| Cline             | `CLINE_USER`, `CLINE_VERSION`, `CLINE_WORKSPACE`        | `cline`, `cline-host`     | yes    | Light + Full              |
| OpenCode          | `OPENCODE_HOME`, `OPENCODE_VERSION`, `OPENCODE_CONFIG`  | `opencode-cli`           | yes    | Light + Full              |
| VS Code Copilot   | `VSCODE_PID`, `VSCODE_IPC_HOOK_CLI`, `VSCODE_CWD`       | `Code Helper`, `code`     | yes    | Light + Full              |

## Confidence scoring

| Layer | Boost | Range    |
|-------|-------|----------|
| 0 env hits + 0 PPID hits | 0  | "no signal" |
| 1 env hit                  | 75 | "High confidence" (minimum) |
| 2 env hits                 | 90 | "High confidence" |
| 3+ env hits                | 100 (capped) | "Certain" |
| 0 env hits, 1 PPID match   | 50 | "Medium confidence" |

The agent with the highest confidence across all detected matches
is reported. Ties go to the first in the `knownAgents` order.

## What uses the detection

| Sprint | Use                                                |
|--------|----------------------------------------------------|
| 79     | `radiant host-info` command (display only)          |
| 80     | `internal/llm/pick.go` — `PickBackend(cfg)` picks SamplingBackend when host supports it |
| 81     | Apply PickBackend to every Full subcommand |

## Adding a new agent

Edit `internal/hostdetect/hostdetect.go`:

```go
// 1. Add the constant.
const AgentFoo AgentID = "foo"

// 2. Add to the order list.
var knownAgents = []AgentID{
    ...
    AgentFoo,
}

// 3. Add the signature (find env vars by spawning `env` inside the agent).
var signatures = map[AgentID]agentSignature{
    ...
    AgentFoo: {
        ID:               AgentFoo,
        EnvVars:          []string{"FOO_AGENT", "FOO_VERSION"},
        ParentBinaries:   []string{"foo", "foo-cli"},
        SupportsSampling: true, // or false if agent doesn't support MCP
    },
}
```

Then add at least one test in `internal/hostdetect/hostdetect_test.go`:

```go
func TestDetect_Foo(t *testing.T) {
    d := stubDetector(
        map[string]string{"FOO_AGENT": "1.0"},
        "", 1000, 999,
    )
    if got := d.Detect(); got.Agent != AgentFoo {
        t.Errorf("got %q, want foo", got.Agent)
    }
}
```

That's it — the new agent will show up in `radiant host-info` and
be available for Sprint 80's PickBackend.
