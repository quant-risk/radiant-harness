# Validation Report — Sprint 75: v2.45.0 — Hermes + Kimi + OpenClaw + Cline

> **Date:** 2026-06-29
> **Project version:** v2.45.0
> **Branch:** `feature/light-full-release`
> **Base:** `9ab312b` (Sprint 75 commit)
> **Status:** PASSED — ready to merge

---

## TL;DR

Sprint 75 extends `radiant setup-mcp` to auto-detect four more
MCP-capable agents, each researched against published documentation
(no fabricated "not supported" claims):

- **Hermes** (NousResearch, 205k stars) — YAML config.
- **Kimi CLI** (Moonshot AI, 9.1k stars) — JSON, global only.
- **OpenClaw** (250k stars) — JSON, nested under `mcp.servers`.
- **Cline** (VSCode/CLI) — JSON, with the official `disabled` /
  `autoApprove` shape.

The harness now supports **11 agents across 4 config formats**
(was 7 / 3).

| Metric | Value |
|--------|-------|
| Commits on branch | ahead of base (`9b28e77`) |
| New commits in this release | **1** (`9ab312b`) |
| Agents added | 4 (hermes, kimi, openclaw, cline) |
| Agents intentionally skipped | 2 (langchain, langgraph — frameworks, not MCP hosts) |
| Tests | **1190 PASS, 0 confirmed FAIL** across 31 packages |
| Tests added | **+18** (from v2.44.0's 1172) |
| Files changed | 5 modified, 1 new |
| LOC delta | +1165 / −7 |
| Cross-compile | linux/{amd64,arm64}, darwin/{amd64,arm64}, windows/amd64 — all OK |
| `go vet ./...` | clean |
| New deps | 0 (`gopkg.in/yaml.v3` was already a project dep) |

---

## Build / Vet / Test

```bash
$ go vet ./...
EXIT=0   (silent — clean)

$ go build -o /tmp/radiant ./cmd/radiant
-rwxr-xr-x  14M  /tmp/radiant    # darwin/arm64 host

$ /tmp/radiant --version
2.45.0

$ go test -count=1 -v ./... | grep -cE "^--- PASS|^    --- PASS|^        --- PASS"
1190

$ go test -count=1 -v ./... | grep -cE "^--- FAIL|^    --- FAIL|^        --- FAIL"
0   # pre-existing `internal/fleet.TestRunAllContextCanceled` may
    # intermittently appear; documented in
    # `validation-report-sprint-56-57.md`, not a regression.
```

### Cross-compile matrix

```bash
$ GOOS=linux   GOARCH=amd64   go build -o /tmp/radiant-linux-amd64     ./cmd/radiant   # 15M OK
$ GOOS=linux   GOARCH=arm64   go build -o /tmp/radiant-linux-arm64     ./cmd/radiant   # 14M OK
$ GOOS=darwin  GOARCH=amd64   go build -o /tmp/radiant-darwin-amd64    ./cmd/radiant   # 15M OK
$ GOOS=darwin  GOARCH=arm64   go build -o /tmp/radiant-darwin-arm64    ./cmd/radiant   # 14M OK
$ GOOS=windows GOARCH=amd64   go build -o /tmp/radiant-windows-amd64.exe ./cmd/radiant   # 15M OK
```

All five platforms built cleanly. No link-time surprises despite the
added `gopkg.in/yaml.v3` import.

---

## Coverage matrix (Sprint 75 → post-Sprint 75)

| Agent     | Stars  | Format | Project-level                  | Global                         | Sprint |
|-----------|--------|--------|--------------------------------|--------------------------------|--------|
| Claude Code| —      | JSON   | `.claude/settings.json`        | `~/.claude/settings.json`      | 35     |
| Cursor     | —      | JSON   | `.cursor/mcp.json`             | `~/.cursor/mcp.json`           | 35     |
| Windsurf   | —      | JSON   | `.windsurf/mcp.json`           | `~/.windsurf/mcp.json`         | 35     |
| Zed        | —      | JSON   | `.zed/settings.json`           | `~/.zed/settings.json`         | 35     |
| VSCode     | —      | JSON   | `.vscode/mcp.json`             | `~/.vscode/mcp.json`           | 35     |
| Codex      | —      | TOML   | `.codex/config.toml`           | `~/.codex/config.toml`         | 73     |
| OpenCode   | —      | JSON-nested | `.opencode/config.json`    | `~/.config/opencode/config.json` | 73   |
| Hermes     | 205k   | YAML   | `.hermes/config.yaml`          | `~/.hermes/config.yaml`        | **75** |
| Kimi CLI   | 9.1k   | JSON   | (no project-level MCP)         | `~/.kimi/mcp.json`             | **75** |
| OpenClaw   | 250k   | JSON-nested | `.openclaw/openclaw.json`  | `~/.openclaw/openclaw.json`    | **75** |
| Cline      | —      | JSON   | (CLI form is global-only)      | `~/.cline/mcp.json`            | **75** |

---

## Per-agent smoke tests

```bash
$ # Project-level (auto-detect .hermes/ and .openclaw/)
$ mkdir .hermes .openclaw
$ radiant setup-mcp --agent=hermes --dry-run
  [dry-run] hermes → /tmp/smoke-test/.hermes/config.yaml
mcp_servers:
    radiant:
        command: /usr/local/bin/radiant
        args:
            - mcp
            - serve

$ radiant setup-mcp --agent=openclaw --dry-run
  [dry-run] openclaw → /tmp/smoke-test/.openclaw/openclaw.json
{
  "mcp": {
    "servers": {
      "radiant": {
        "command": "/usr/local/bin/radiant",
        "args": ["mcp", "serve"]
      }
    }
  }
}

$ # Global-only (Kimi, Cline)
$ radiant setup-mcp --agent=kimi --dry-run
  [dry-run] kimi → ~/.kimi/mcp.json
{
  "mcpServers": {
    "radiant": {
      "command": "/usr/local/bin/radiant",
      "args": ["mcp", "serve"]
    }
  }
}

$ radiant setup-mcp --agent=cline --dry-run
  [dry-run] cline → ~/.cline/mcp.json
{
  "mcpServers": {
    "radiant": {
      "command": "/usr/local/bin/radiant",
      "args": ["mcp", "serve"],
      "disabled": false,
      "autoApprove": []
    }
  }
}
```

All four emit syntactically valid output that each agent's loader
can consume.

---

## Per-agent research summary

### Hermes Agent (NousResearch)

**Sources consulted:**
- https://github.com/NousResearch/hermes-agent (205k ⭐, 37k forks)
- https://github.com/NousResearch/hermes-agent/blob/main/cli-config.yaml.example
- https://hermes-agent.nousresearch.com/docs/user-guide/features/mcp
- https://lobehub.com/skills/nousresearch-hermes-agent-native-mcp

**Verified shape:**
```yaml
mcp_servers:
  time:
    command: "uvx"
    args: ["mcp-server-time", "--utc"]
```

**Implementation:** `mergeHermesConfig` parses the full file via
`gopkg.in/yaml.v3` into `map[string]any`, locates (or creates) the
`mcp_servers` key, sets `radiant`, and re-marshals. All other keys
(`model`, `terminal`, `browser`, `agent`, ...) round-trip verbatim.

**Test cases:**
- `TestMergeHermesConfig_NewFile` — emit radiant block from scratch.
- `TestMergeHermesConfig_PreservesExisting` — preserves `model`,
  `terminal`, and the existing `time` MCP server.
- `TestMergeHermesConfig_ReplacesExisting` — replaces old radiant
  path, doesn't duplicate the entry.

### Kimi CLI (Moonshot AI)

**Sources consulted:**
- https://github.com/MoonshotAI/kimi-cli (9.1k ⭐, 1.1k forks)
- https://github.com/MoonshotAI/kimi-cli/blob/main/src/kimi_cli/cli/mcp.py
  - `get_global_mcp_config_file()` = `get_share_dir() / "mcp.json"`
  - `get_share_dir()` = `Path.home() / ".kimi"` (or `$KIMI_SHARE_DIR`)

**Verified shape:**
```json
{
  "mcpServers": {
    "context7": {
      "url": "https://mcp.context7.com/mcp",
      "headers": { "CONTEXT7_API_KEY": "..." }
    }
  }
}
```

**Implementation:** `mergeKimiMCP` is a standard `mcpServers` JSON
merge. The `--global` flag is always implicit because Kimi has no
project-level MCP file. The path is `~/.kimi/mcp.json` (or the env
override path), same as `share.py`.

**Test cases:**
- `TestMergeKimiMCP_NewFile`
- `TestMergeKimiMCP_PreservesExisting` — preserves `context7`,
  adds `radiant`.

### OpenClaw

**Sources consulted:**
- https://github.com/openclaw/openclaw (250k ⭐)
- https://docs.openclaw.ai/cli/mcp
- Open issue: "Claude CLI runner ignores user-configured `mcp.servers`
  from openclaw.json" — confirms the path is exactly
  `mcp.servers.<name>`.

**Verified shape:**
```json
{
  "channels": { "telegram": { "token": "..." } },
  "gateway": { "port": 18789 },
  "mcp": {
    "sessionIdleTtlMs": 600000,
    "servers": {
      "context7": { "command": "uvx", "args": ["context7-mcp"] }
    }
  }
}
```

**Implementation:** `mergeOpenClawJSONConfig` preserves every
top-level key (`channels`, `gateway`, ...) and every sibling of
`mcp.servers` (`sessionIdleTtlMs`, future feature flags), only
touching `mcp.servers.radiant`. Uses `map[string]json.RawMessage`
round-trip — same pattern as `mergeOpenCodeConfig` from Sprint 73.

**Test cases:**
- `TestMergeOpenClawJSONConfig_NewFile`
- `TestMergeOpenClawJSONConfig_PreservesSiblings` — preserves
  `sessionIdleTtlMs` and an existing `context7`.
- `TestMergeOpenClawJSONConfig_PreservesTopLevel` — preserves
  `channels` and `gateway`.

### Cline

**Sources consulted:**
- https://docs.cline.bot/mcp/mcp-overview
- CLI MCP wizard: `cline mcp` (add / edit / enable / disable).
- VS Code extension-managed file (separate path, intentionally not
  addressed by `setup-mcp`).

**Verified shape:**
```json
{
  "mcpServers": {
    "local-server": {
      "command": "node",
      "args": ["/path/to/server.js"],
      "env": { "API_KEY": "..." },
      "disabled": false,
      "autoApprove": []
    }
  }
}
```

**Implementation:** `mergeClineConfig` is a standard `mcpServers`
JSON merge, but emits `disabled: false` and `autoApprove: []` on
every entry to match the documented shape exactly. The VS Code
extension-managed file is intentionally NOT addressed.

**Test cases:**
- `TestMergeClineConfig_NewFile` — verifies both `disabled: false`
  and `autoApprove: []` are emitted.
- `TestMergeClineConfig_PreservesExisting` — preserves
  `local-server`, adds `radiant`.

### Detection tests (Sprint 75 additions)

- `TestResolveMCPAgents_DetectsHermes`
- `TestResolveMCPAgents_DetectsOpenClaw`
- (Kimi and Cline detection tests are intentionally omitted; they
  require polluting `~/.kimi/` or `~/.cline/` in the user's real
  home and we chose not to. Their behaviour is covered by
  `mcpConfigFor` global-path tests.)

### Routing tests (Sprint 75 additions)

- `TestMCPConfigFor_Hermes_Project` (validates YAML quote-style)
- `TestMCPConfigFor_Hermes_Global`
- `TestMCPConfigFor_Kimi`
- `TestMCPConfigFor_OpenClaw_Project`
- `TestMCPConfigFor_OpenClaw_Global`
- `TestMCPConfigFor_Cline` (validates `disabled` + `autoApprove`)

---

## Files modified

```
 CHANGELOG.md                      |  +67   (v2.45.0 entry)
 RELEASE-NOTES.md                  | +161   (v2.45.0 release notes)
 cmd/radiant/cmd_setup_mcp.go      | +327   (4 handlers + 4 merge funcs)
 cmd/radiant/cmd_setup_mcp_test.go | +511   (18 new tests)
 cmd/radiant/main.go               |   ±2   (version 2.44.0 → 2.45.0)
 docs/SPRINT75-PLAN.md             | +120   (NEW; design doc)
```

Net: **+1165 / −7 LOC across 6 files**.

---

## What was NOT added (and why)

### LangChain / LangGraph

Skipped deliberately. They are Python frameworks for *building* MCP
clients, not MCP host runtimes themselves. Users wanting LangChain
integration wrap `radiant mcp serve` from inside their LangChain
agent:

```python
from mcp import StdioServerParameters, stdio_client

params = StdioServerParameters(
    command="/usr/local/bin/radiant",
    args=["mcp", "serve"],
)
# ... use params in your LangChain MCP adapter
```

This is the same pattern as wrapping any other stdio MCP server.

### OpenClaw's Hermes bridge (`openclaw acp`)

`openclaw acp` and the embedded runtime use the *same* `mcp.servers`
config; writing radiant there is sufficient. The ACP-aware path
(below `mcp/acp` in OpenClaw's docs) reads from the same registry.

### Cline VS Code extension config (managed via UI)

The VS Code extension manages its own `cline_mcp_settings.json` via
the Cline panel UI. `radiant setup-mcp` intentionally does NOT touch
that file — there's no stable schema-published path for it (the file
lives at `~/.cline/data/settings/cline_mcp_settings.json` but Cline
recommends editing via the UI). CLI users get the proper
`~/.cline/mcp.json` instead.

---

## Backward compatibility

- All 7 previously-supported agents behave identically.
- Auto-detect order: project-local markers first (claude/cursor/.../
  codex/opencode/hermes/openclaw), then global-only fallback
  (~/.kimi, ~/.cline). Existing users see no change.
- Existing config files written by radiant are valid format for each
  respective agent — verified by reading the published docs for each
  agent (Hermes CLI config example, Kimi `MCPConfig` validator,
  OpenClaw `mcp` config schema, Cline official examples).

---

## Known limitations

- **One flaky pre-existing test:** `internal/fleet.TestRunAllContextCanceled`
  alternates PASS/FAIL on timing (documented in
  `validation-report-sprint-56-57.md`). Re-running `go test -count=1`
  on `./internal/fleet/` alone passes eventually; not a regression
  from this sprint.

- **YAML quote style for Hermes:** `gopkg.in/yaml.v3` emits
  unquoted scalars for simple paths (e.g. `command: /usr/local/bin/radiant`)
  and quoted strings for special chars (e.g. `command: "p a t h"`).
  Both styles are accepted by Hermes's YAML parser (PyYAML). The
  routing test was made quote-tolerant: it checks for the path
  substring and the `command:` key, not the exact quote style.

- **Kimi project-level MCP:** Kimi's CLI doesn't support project-level
  MCP configs at all (the `kimi mcp` commands only write the global
  file). `--global` is implicit for that agent; we document this in
  the command `Long` and in the Sprint 75 plan doc.

- **Cline extension path:** The VS Code extension-managed file path
  is not addressed (UI-managed, schema internal to Cline). Cline CLI
  users get `~/.cline/mcp.json` and that file is documented upstream
  as the canonical CLI path.

---

## Verification checklist

- [x] `go vet ./...` clean
- [x] `go build ./...` clean (5 platforms cross-compiled)
- [x] `go test -count=1 ./...` — 1190 PASS, 0 confirmed FAIL
- [x] 18 new tests passing (4 agents × new/preserve + 1 replace for hermes + 2 detection + 6 routing)
- [x] Smoke test for all 4 new agents (`--dry-run`) emits valid config
- [x] `radiant --version` reports `2.45.0`
- [x] `radiant setup-mcp --help` lists all 11 agents in the description and the `--agent` flag
- [x] CHANGELOG and RELEASE-NOTES updated
- [x] Plan doc added (`docs/SPRINT75-PLAN.md`)
- [x] git commit `9ab312b` lands cleanly, working tree clean
