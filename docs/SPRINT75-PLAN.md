# Sprint 75 — v2.45.0 — `setup-mcp`: Hermes + Kimi CLI + OpenClaw + Cline

## Goal

Broaden `radiant setup-mcp` auto-detect from "7 agents" (Claude, Cursor, Windsurf,
Zed, VSCode, Codex, OpenCode) to "11 agents" by adding the four major
MCP-capable agents the user explicitly named after Sprint 73:

- **Hermes** (NousResearch) — terminal-coding agent, 205k GitHub stars
- **Kimi CLI** (Moonshot AI) — terminal-coding agent, 9.1k GitHub stars
- **OpenClaw** — agentic desktop gateway with embedded MCP registry, 250k GitHub stars
- **Cline** — VSCode extension (also ships CLI), MCP-native

LangChain and LangGraph were *intentionally* skipped (Sprint 73 message) — they
are Python frameworks, not MCP hosts. Users wanting LangChain integration wrap
`radiant mcp serve` from inside their LangChain agent instead of running
`radiant setup-mcp`.

## Research summary

| Agent     | Stars  | Config file                                       | Format | Key              | Levels |
|-----------|--------|---------------------------------------------------|--------|------------------|--------|
| Hermes    | 205k   | `~/.hermes/config.yaml` / `.hermes/config.yaml`   | YAML   | `mcp_servers`    | global + project |
| Kimi CLI  | 9.1k   | `~/.kimi/mcp.json`                                | JSON   | `mcpServers`     | global only |
| OpenClaw  | 250k   | `~/.openclaw/openclaw.json` / `.openclaw/openclaw.json` | JSON | `mcp.servers` | global + project |
| Cline     | —      | `~/.cline/mcp.json`                               | JSON   | `mcpServers`     | global only |

Sources consulted:
- https://github.com/NousResearch/hermes-agent/blob/main/cli-config.yaml.example
- https://hermes-agent.nousresearch.com/docs/user-guide/features/mcp
- https://lobehub.com/skills/nousresearch-hermes-agent-native-mcp
- https://github.com/MoonshotAI/kimi-cli (`src/kimi_cli/cli/mcp.py` →
  `get_global_mcp_config_file() = get_share_dir() / "mcp.json"`,
  `get_share_dir() = Path.home() / ".kimi"` or `$KIMI_SHARE_DIR`)
- https://github.com/openclaw/openclaw + https://docs.openclaw.ai/cli/mcp
- https://docs.cline.bot/mcp/mcp-overview (CLI: `~/.cline/mcp.json`)

## Implementation details

### Hermes (`hermes`)

```go
// ~/.hermes/config.yaml is a deep YAML file (model, terminal, browser, etc.).
// We need to merge into the top-level `mcp_servers` key while preserving
// every other key exactly.
```

Uses `gopkg.in/yaml.v3` (already a project dependency). Decode into
`map[string]any`, locate-or-create `mcp_servers`, set `radiant`, encode back.

### Kimi CLI (`kimi`)

```go
// Kimi stores MCP servers ONLY in a global file:
//   <share_dir>/mcp.json where share_dir = ~/.kimi (or $KIMI_SHARE_DIR)
// No project-level override exists in Kimi.
//
// The shape matches the Claude standard:
//   { "mcpServers": { "<name>": { "command": "...", "args": [...] } } }
```

Standard JSON merge (same shape as `mergeMCPJSON`). If `global=false`, still
write the global file — Kimi doesn't have a project-level alternative.

### OpenClaw (`openclaw`)

```go
// OpenClaw uses { "mcp": { "servers": { ... } }, <other keys> }.
// The `mcp` key has many siblings (`sessionIdleTtlMs`, ...) and top-level
// config keys (`channels`, `gateway`, ...) that must be preserved.
```

JSON merge under `mcp.servers`, preserving unknown top-level keys plus
unknown siblings under `mcp`. Separate handlers because the structure is
nested one level deeper than OpenCode.

### Cline (`cline`)

```go
// Cline CLI writes to ~/.cline/mcp.json with standard mcpServers shape.
// Cline's documented entries include optional `disabled` and `autoApprove`
// fields — we emit those for shape compatibility with the official examples.
```

Standard JSON merge with `disabled`/`autoApprove` defaults.

## Files

- `cmd/radiant/cmd_setup_mcp.go` — add 4 handlers in `mcpConfigFor`, 4 cases in
  `resolveMCPAgents`, 4 new merge functions (`mergeHermesConfig`,
  `mergeKimiMCP`, `mergeOpenClawConfig`, `mergeClineConfig`)
- `cmd/radiant/cmd_setup_mcp_test.go` — 4 new test groups
- `cmd/radiant/main.go` — bump version → `2.45.0`
- `CHANGELOG.md` — v2.45.0 entry
- `RELEASE-NOTES.md` — v2.45.0 notes

## Coverage matrix (post-Sprint 75)

```
$ radiant setup-mcp --agent=claude|cursor|windsurf|zed|vscode|
                       codex|opencode|hermes|kimi|openclaw|cline
```

11 agents, 4 config formats (JSON-std, TOML, YAML, JSON-nested).
