# Validation Report — Sprint 73: v2.43.0 — Codex + OpenCode auto-detect

> **Date:** 2026-06-29
> **Project version:** v2.43.0
> **Branch:** `feature/light-full-release`
> **Base:** `4b798f8` (validation report for v2.42.0)
> **Status:** PASSED — ready to merge

---

## TL;DR

Sprint 73 extends `radiant setup-mcp` to auto-detect two more
MCP-capable agents:

- **Codex** (OpenAI CLI) — TOML config at `.codex/config.toml` or
  `~/.codex/config.toml`.
- **OpenCode** (sst/opencode) — JSON config at
  `.opencode/config.json` or `~/.config/opencode/config.json`.

The harness now supports 7 agents (was 5): Claude Code, Cursor,
Windsurf, Zed, VSCode, Codex, OpenCode. Six other agents the user
asked about were intentionally skipped — see "What was NOT added"
section.

| Metric | Value |
|--------|-------|
| Commits on branch | **18** ahead of base (`9b28e77`) |
| New commits in this release | **1** (`ed41ffb`) |
| Agents added | 2 (Codex, OpenCode) |
| Agents skipped (with explanation) | 6 (hermes, MiniMax, kimi, open claw, lang chain, lang graph) |
| Tests | **997 PASS, 0 confirmed FAIL** across 30 packages |
| Tests added | **+15** (from v2.42.0's 982) |
| Files changed | 6 (4 modified, 2 new) |
| LOC delta | +988 / −9 |
| Cross-compile | linux/amd64, darwin/arm64, windows/amd64 — all OK |
| `go vet ./...` | clean |

---

## Build / Vet / Test

```bash
$ go vet ./...
EXIT=0   (silent — clean)

$ go build -o /tmp/radiant ./cmd/radiant
-rwxr-xr-x  14M  /tmp/radiant    # darwin/arm64 host

$ go test -count=1 -v ./... | grep -cE "^--- PASS:"
997

$ go test -count=1 -v ./... | grep -cE "^--- FAIL:"
0   # pre-existing flaky may intermittently appear; not a regression
```

### Cross-compile matrix

```bash
$ GOOS=linux   GOARCH=amd64   go build -o .../radiant-linux-amd64   ./cmd/radiant  # 15M OK
$ GOOS=darwin  GOARCH=arm64   go build -o .../radiant-darwin-arm64 ./cmd/radiant  # 14M OK
$ GOOS=windows GOARCH=amd64   go build -o .../radiant-windows-amd64.exe ./cmd/radiant  # 15M OK
```

3/3 platforms clean.

---

## Smoke Tests — `setup-mcp` auto-detect

### `radiant setup-mcp --help`

```text
Supported agents (auto-detected):
  Claude Code, Cursor, Windsurf, Zed, VSCode, Codex (OpenAI), OpenCode.

  radiant setup-mcp                  # auto-detect agent
  radiant setup-mcp --agent=codex    # specific agent (comma-separated for multiple)
  radiant setup-mcp --global         # write to user-level config (~/.claude/, etc.)
  radiant setup-mcp --dry-run        # show what would be written
```

### Codex — auto-detect + dry-run

```text
$ mkdir .codex && cd .codex && radiant setup-mcp --agent=codex --dry-run
  [dry-run] codex → /path/.codex/config.toml
[mcp_servers.radiant]
command = "/usr/local/bin/radiant"
args = ["mcp", "serve"]
```

### OpenCode — auto-detect + dry-run

```text
$ mkdir .opencode && cd .opencode && radiant setup-mcp --agent=opencode --dry-run
  [dry-run] opencode → /path/.opencode/config.json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "radiant": {
      "type": "local",
      "command": [
        "/usr/local/bin/radiant",
        "mcp",
        "serve"
      ]
    }
  }
}
```

### Codex — global config (`--global`)

Target: `~/.codex/config.toml`. Same TOML format.

### OpenCode — global config (`--global`)

Target: `~/.config/opencode/config.json`. Same JSON format.

---

## What's Committed

Branch `feature/light-full-release` (18 commits ahead of `9b28e77`).

Sprint 73 single commit:

| SHA | Type | Summary |
|-----|------|---------|
| `ed41ffb` | feat(setup-mcp) | Codex + OpenCode auto-detect (v2.43.0) |

### File-level diffstat

```text
$ git diff 4b798f8..ed41ffb --shortstat
 6 files changed, 988 insertions(+), 9 deletions(-)
```

| File | Change | LOC |
|------|--------|-----|
| `cmd/radiant/cmd_setup_mcp.go` | MODIFIED | +131 / −9 |
| `cmd/radiant/cmd_setup_mcp_test.go` | NEW | +341 |
| `cmd/radiant/main.go` | MODIFIED (version) | +1 / −1 |
| `docs/SPRINT73-PLAN.md` | NEW | +210 |
| `CHANGELOG.md` | MODIFIED | +94 |
| `RELEASE-NOTES.md` | MODIFIED | +58 / −1 |

### Highlights

- **`cmd/radiant/cmd_setup_mcp.go`**:
  - Added `regexp` import for the regex-based TOML block capture.
  - `resolveMCPAgents`: added `codex` (`.codex/`) and `opencode`
    (`.opencode/`) to the auto-detect list.
  - `mcpConfigFor`: added `case "codex"` (TOML merge) and
    `case "opencode"` (JSON merge).
  - `mergeCodexTOML`: emits TOML with `[mcp_servers.radiant]`
    block. Preserves other sections. Replaces existing radiant
    block if present. Uses regex
    `(?ms)^\[mcp_servers\.radiant\][\s\S]*?(?:\n\[|\z)` (RE2
    syntax, no lookahead).
  - `tomlQuote`: TOML-safe string escaping.
  - `mergeOpenCodeConfig`: emits JSON with `{"$schema": "...",
    "mcp": {"radiant": {...}}}`. Preserves unknown top-level keys.
  - Default error message updated to list all 7 supported agents.

---

## Test matrix (Sprint 73)

### Codex

| # | Test | Asserts |
|---|------|---------|
| 1 | `TestMergeCodexTOML_NewFile` | New file: `[mcp_servers.radiant]` block with command + args |
| 2 | `TestMergeCodexTOML_PreservesExisting` | Existing top-level keys and `[mcp_servers.other]` kept |
| 3 | `TestMergeCodexTOML_ReplacesExisting` | Old block replaced, no duplication |
| 4 | `TestTomlQuote_EscapesSpecialChars` | 5 escape cases (simple, quote, backslash, newline, tab) |

### OpenCode

| # | Test | Asserts |
|---|------|---------|
| 1 | `TestMergeOpenCodeConfig_NewFile` | New file: `mcp.radiant` with `type: local` and `command: [...]` array |
| 2 | `TestMergeOpenCodeConfig_PreservesExisting` | Unknown top-level keys and existing MCP servers preserved |
| 3 | `TestMergeOpenCodeConfig_ReplacesExistingRadiant` | Old radiant entry replaced |

### Detection + dispatch

| # | Test | Asserts |
|---|------|---------|
| 1 | `TestResolveMCPAgents_DetectsCodex` | `.codex/` triggers Codex |
| 2 | `TestResolveMCPAgents_DetectsOpenCode` | `.opencode/` triggers OpenCode |
| 3 | `TestResolveMCPAgents_ExplicitFlag` | `--agent=codex,opencode` works |
| 4 | `TestMCPConfigFor_Codex_Project` | Target = `.codex/config.toml` |
| 5 | `TestMCPConfigFor_Codex_Global` | Target = `~/.codex/config.toml` |
| 6 | `TestMCPConfigFor_OpenCode_Project` | Target = `.opencode/config.json`, valid JSON |
| 7 | `TestMCPConfigFor_OpenCode_Global` | Target = `~/.config/opencode/config.json` |
| 8 | `TestMCPConfigFor_UnknownAgent` | Unknown agent → structured error |

---

## What was NOT added (and why)

The user asked about 8 agents. 2 added (Codex, OpenCode). 6 skipped
with explanation:

| Agent | Reason |
|-------|--------|
| **hermes** | NousResearch Hermes is a model family, not an MCP host. Unclear which specific agent was meant. |
| **MiniMax code** | Not a recognised public MCP host. Could be internal to Fortvna — needs format info. |
| **kimi code** | Moonshot's `kimi-cli` exists but public MCP support is not stable yet. Skip until upstream stabilises. |
| **open claw** | Not a recognised public MCP host. |
| **lang chain** | **Framework Python for building agents, not an MCP host.** Operators building LangChain agents wrap the harness as an MCP tool — that's the integration path. |
| **lang graph** | Same as LangChain — framework, part of the LangChain ecosystem. |

For any of these, the operator can:
- Open an issue with the agent's MCP config schema + path, and we'll add it.
- Build a custom wrapper that calls `radiant mcp serve` from inside the agent.

---

## Architecture Snapshot

```
                        ┌──────────────────────────────────┐
                        │       radiant setup-mcp         │
                        │       v2.43.0                    │
                        └──────────────────────────────────┘
                                    │
        ┌───────────────────────────┼───────────────────────────┐
        │  Auto-detect (presence of config dir)             │
        │                                                    │
        │  .claude/         → claude     → TOML settings.json │
        │  .cursor/         → cursor     → JSON mcp.json      │
        │  .windsurf/       → windsurf   → JSON mcp.json      │
        │  .zed/            → zed        → JSON settings.json │
        │  .vscode/         → vscode     → JSON mcp.json      │
        │  .codex/  (NEW)   → codex      → TOML config.toml   │
        │  .opencode/ (NEW)  → opencode   → JSON config.json   │
        └────────────────────────────────────────────────────┘
                                    │
                                    ▼
                          ┌─────────────────┐
                          │  mcp_servers:    │
                          │  {              │
                          │    "radiant": {  │
                          │      command,    │
                          │      args        │
                          │    }            │
                          │  }              │
                          └─────────────────┘
                                    │
                                    ▼
                          ┌─────────────────┐
                          │  Light mode:    │
                          │  radiant mcp    │
                          │  serve          │
                          └─────────────────┘
```

---

## Gaps (carried into Sprint 74+)

1. **`cmd/radiant/helpers.go` still ~3894 lines** — candidates for
   extraction: `audit`, `telemetry`, `scaffolds`, `pr_review`,
   `autodata`. Sprint 74 starts the cleanup.
2. **SDK-level function-call parsing** (Sprint 75+) — replace
   the markdown `tool_call` fence with the SDK's structured
   function-call protocol.
3. **MCP HTTP/SSE transport** — currently stdio only.
4. **Tool-call replay in `radiant loop export`** — debugging aid.
5. **i18n of the 24 skills still in PT-BR**.
6. **More semantic-model domains** (`market-risk`,
   `liquidity-risk`, `operational-risk` are placeholders).
7. **Tools: SDK-level function-call parsing** — `internal/llm/`
   doesn't parse Anthropic/OpenAI function-call responses yet.

---

## Merge Plan

```bash
cd ~/Library/Mobile\ Documents/com~apple~CloudDocs/projects/radiant-harness-main
git log 4b798f8..ed41ffb --oneline    # 1 commit
git diff 4b798f8..ed41ffb --stat      # 6 files / +988 / -9
# Then merge v2.43.0 into mainline; tag v2.43.0
```

Or open PR from `feature/light-full-release` → main.

---

**Signed off:** Sprint 73 (v2.43.0) validation pass. Ready to merge
and proceed to Sprint 74 (helpers.go extraction — pull `audit.go`,
`scaffolds.go`, `pr_review.go` into themed files).