# Sprint 73 — setup-mcp: Codex + OpenCode auto-detect (v2.43.0)

> **Status**: In progress
> **Branch**: `feature/light-full-release` (continuation)
> **Target version**: v2.43.0
> **Estimated scope**: 1 sprint focado (small)

---

## Motivation

In v2.42.0, `radiant setup-mcp` auto-detects 5 MCP-capable agents:

- Claude Code (`.claude/settings.json`)
- Cursor (`.cursor/mcp.json`)
- Windsurf (`.windsurf/mcp.json`)
- Zed (`.zed/settings.json`)
- VSCode (`.vscode/mcp.json`)

User feedback (Sprint 73 input) flagged that **Codex** (OpenAI's CLI)
and **OpenCode** (sst/opencode) are missing. Both have public MCP
support and ship config files the harness can write to.

Sprint 73 adds both to the auto-detect list. The operator gets
`radiant setup-mcp` working out-of-the-box on more agents without
needing to know file paths.

### What about the other agents the user listed?

- **Codex** (OpenAI): adding ✓
- **Hermes**: ambiguous — NousResearch Hermes is a model family,
  not an MCP host. If user means a specific Hermes agent, ask.
- **MiniMax code**: not a recognised public MCP host. Could be
  internal to Fortvna — needs info.
- **Kimi code** (Moonshot): `kimi-cli` exists but public MCP
  support is not confirmed in stable releases. Skip until
  upstream stabilises.
- **Open code** (opencode/sst): adding ✓
- **Open claw**: not a recognised public MCP host. Skip.
- **LangChain / LangGraph**: **these are Python frameworks for
  building agents, not MCP hosts themselves**. The harness
  integrates with the *agents* built using these frameworks, not
  with the frameworks directly. Operators building LangChain
  agents can wrap the harness as an MCP tool — that's the
  integration path. Adding LangChain/LangGraph to `setup-mcp`
  would be misleading (they don't read MCP config files).

---

## Goals

| # | Goal | Acceptance |
|---|------|------------|
| G1 | `setup-mcp` auto-detects Codex (`~/.codex/config.toml` or `.codex/config.toml`) | Test: presence of `.codex/` triggers Codex config |
| G2 | `setup-mcp` writes Codex-compatible MCP entry | Test: TOML output matches Codex schema |
| G3 | `setup-mcp` auto-detects OpenCode (`.opencode/config.json` or `~/.config/opencode/config.json`) | Test: presence of either triggers OpenCode config |
| G4 | `setup-mcp` writes OpenCode-compatible MCP entry | Test: JSON output matches OpenCode schema |
| G5 | `--agent` flag accepts `codex` and `opencode` explicitly | Manual smoke test |
| G6 | Updated docs/AGENTS.md lists all 7 supported agents | Doc updated |

### Out of scope (carried to Sprint 74+)

- Hermes-specific support (needs agent clarification)
- MiniMax-specific support (needs format info)
- Kimi CLI support (waiting on upstream MCP stability)
- OpenClaw support (not a recognised agent)
- LangChain/LangGraph support (frameworks, not MCP hosts)

---

## Design

### Codex config schema

Codex (OpenAI's CLI, formerly `codex`) stores MCP config in TOML:

```toml
# ~/.codex/config.toml  (global)
# OR  .codex/config.toml  (project)

[mcp_servers.radiant]
command = "/usr/local/bin/radiant"
args = ["mcp", "serve"]
# optional:
env = { RADIANT_LOG_LEVEL = "info" }
startup_timeout_ms = 5000
```

`mergeCodexConfig` reads the existing TOML (if any), parses with
`github.com/pelletier/go-toml/v2` (already in go.mod? — check), adds
the `[mcp_servers.radiant]` block, and writes back.

### OpenCode config schema

OpenCode (sst/opencode) stores MCP config in JSON:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "radiant": {
      "type": "local",
      "command": ["/usr/local/bin/radiant", "mcp", "serve"],
      "environment": {}
    }
  }
}
```

Note: OpenCode uses `"mcp"` (not `"mcpServers"`) and `"command"` is an
array (not a string). The `type` field distinguishes `local` (subprocess)
from `remote` (HTTP). We always write `local`.

`mergeOpenCodeConfig` handles this.

### Detection paths

| Agent | Project-level | Global |
|-------|---------------|--------|
| Claude Code | `.claude/settings.json` | `~/.claude/settings.json` |
| Cursor | `.cursor/mcp.json` | `~/.cursor/mcp.json` |
| Windsurf | `.windsurf/mcp.json` | `~/.windsurf/mcp.json` |
| Zed | `.zed/settings.json` | `~/.zed/settings.json` |
| VSCode | `.vscode/mcp.json` | `~/.vscode/mcp.json` |
| **Codex** (NEW) | `.codex/config.toml` | `~/.codex/config.toml` |
| **OpenCode** (NEW) | `.opencode/config.json` | `~/.config/opencode/config.json` |

---

## Files

| File | Change | LOC est. |
|------|--------|----------|
| `cmd/radiant/cmd_setup_mcp.go` | MODIFY — add codex + opencode | +120 |
| `cmd/radiant/cmd_setup_mcp_test.go` | NEW — tests for codex + opencode | +200 |
| `docs/AGENTS.md` | MODIFY — list updated | +20 |
| `CHANGELOG.md` | MODIFY — v2.43.0 entry | +50 |
| `RELEASE-NOTES.md` | MODIFY — v2.43.0 entry | +50 |
| `cmd/radiant/main.go` | MODIFY — version bump | +1 |

**Total estimate: ~440 LOC** (320 new code + tests, ~120 docs).

---

## Test matrix

### Codex

| # | Test | Asserts |
|---|------|---------|
| 1 | `TestSetupMCP_Codex_ProjectConfig` | Presence of `.codex/` triggers Codex |
| 2 | `TestSetupMCP_Codex_GlobalConfig` | `--global` writes to `~/.codex/config.toml` |
| 3 | `TestSetupMCP_Codex_TOMLFormat` | Output is valid TOML with `[mcp_servers.radiant]` |
| 4 | `TestSetupMCP_Codex_PreservesExisting` | Existing entries in mcp_servers kept |
| 5 | `TestSetupMCP_Codex_AlreadyConfigured` | Detects existing `radiant` entry, refuses without --force |

### OpenCode

| # | Test | Asserts |
|---|------|---------|
| 1 | `TestSetupMCP_OpenCode_ProjectConfig` | Presence of `.opencode/` triggers OpenCode |
| 2 | `TestSetupMCP_OpenCode_GlobalConfig` | `--global` writes to `~/.config/opencode/config.json` |
| 3 | `TestSetupMCP_OpenCode_JSONFormat` | Output is valid JSON with `mcp.radiant` |
| 4 | `TestSetupMCP_OpenCode_PreservesExisting` | Existing config preserved |
| 5 | `TestSetupMCP_OpenCode_CommandArray` | `command` is an array (not string) |

### Agent dispatch

| # | Test | Asserts |
|---|------|---------|
| 1 | `TestResolveMCPAgents_NewAgents` | `codex` and `opencode` recognised |
| 2 | `TestSetupMCP_AgentFlag_New` | `--agent=codex,opencode` works |

---

## Risks

| Risk | Mitigation |
|------|------------|
| `pelletier/go-toml` not in go.mod | Check before adding — fall back to manual TOML emission if needed |
| Codex config schema changes between versions | Schema is stable since v0.1; pinning to current schema with note |
| OpenCode config format differs from assumed schema | Will verify against latest opencode docs; update if needed |

---

## Commit plan

Single commit on `feature/light-full-release`:

```
feat(setup-mcp): Sprint 73 — Codex + OpenCode auto-detect (v2.43.0)
```

Pass criteria: `go vet ./...` clean, `go test -count=1 -v ./...`
green (982+ tests), cross-compile 3/3 platforms.

---

**Status at plan write**: Sprint 68 (v2.42.0) committed at `d2ef8d5`
+ validation report `4b798f8`. Sprint 73 implementation in progress.