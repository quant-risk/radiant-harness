# Validation Report — Sprint 12 second batch FINAL (v0.4.5)

**Date:** 2026-06-25
**Version:** 0.4.5 (literal in source; git build embeds `d8bbe89`)
**Commit under validation:** `d8bbe89`
**Sprint:** 12 — Governance Phase (MCP read-only wiring; final pass)
**Scope:** `radiant integrations list` + 3 output modes + 3 helpers.

---

## 1. Build hygiene

```
$ go version
go version go1.22.10 darwin/arm64

$ go build ./...
(clean — no output, no warnings)

$ go vet ./...
(clean — no output)

$ gofmt -l .
(clean — no files flagged)
```

**Result:** ✅ Pass.

## 2. Race-detector tests

```
$ CGO_ENABLED=0 go test ./... -count=1 -race -timeout=180s
?   	github.com/quant-risk/radiant-harness/internal         [no test files]
?   	github.com/quant-risk/radiant-harness/internal/scaffold [no test files]
ok  	github.com/quant-risk/radiant-harness/cmd/radiant         1.486s
ok  	github.com/quant-risk/radiant-harness/internal/benchmark  2.195s
ok  	github.com/quant-risk/radiant-harness/internal/engine     1.908s
ok  	github.com/quant-risk/radiant-harness/internal/harness    7.829s
ok  	github.com/quant-risk/radiant-harness/internal/llm        6.921s
ok  	github.com/quant-risk/radiant-harness/internal/policy     3.665s
ok  	github.com/quant-risk/radiant-harness/internal/quality    3.331s
ok  	github.com/quant-risk/radiant-harness/internal/skill      4.028s
ok  	github.com/quant-risk/radiant-harness/internal/spec       2.772s
```

**Total:** 240 PASS, **0 FAIL**, **0 data races detected**.

**Result:** ✅ Pass.

## 3. Cross-compilation — all 6 targets

```
$ make release
GOOS=linux   GOARCH=amd64 go build -ldflags "-s -w -X main.version=d8bbe89" -o dist/radiant-linux-amd64     ./cmd/radiant/
GOOS=linux   GOARCH=arm64 go build -ldflags "-s -w -X main.version=d8bbe89" -o dist/radiant-linux-arm64     ./cmd/radiant/
GOOS=darwin  GOARCH=amd64 go build -ldflags "-s -w -X main.version=d8bbe89" -o dist/radiant-darwin-amd64    ./cmd/radiant/
GOOS=darwin  GOARCH=arm64 go build -ldflags "-s -w -X main.version=d8bbe89" -o dist/radiant-darwin-arm64    ./cmd/radiant/
GOOS=windows GOARCH=amd64 go build -ldflags "-s -w -X main.version=d8bbe89" -o dist/radiant-windows-amd64.exe ./cmd/radiant/
GOOS=windows GOARCH=arm64 go build -ldflags "-s -w -X main.version=d8bbe89" -o dist/radiant-windows-arm64.exe ./cmd/radiant/
✓ Release binaries in dist/

$ file dist/*
dist/radiant-darwin-amd64:      Mach-O 64-bit executable x86_64
dist/radiant-darwin-arm64:      Mach-O 64-bit executable arm64
dist/radiant-linux-amd64:       ELF 64-bit LSB executable, x86-64, statically linked
dist/radiant-linux-arm64:       ELF 64-bit LSB executable, ARM aarch64, statically linked
dist/radiant-windows-amd64.exe: PE32+ executable (console) x86-64, for MS Windows
dist/radiant-windows-arm64.exe: PE32+ executable (console) Aarch64, for MS Windows
```

| Target | Status |
|---|---|
| linux/amd64 | ✅ |
| linux/arm64 | ✅ |
| darwin/amd64 | ✅ |
| darwin/arm64 | ✅ |
| windows/amd64 | ✅ |
| windows/arm64 | ✅ |

**Result:** ✅ 6/6 targets build clean.

## 4. Built binary sanity

```
$ ./bin/radiant --version
d8bbe89       (git SHA injected by Makefile; literal version in source = 0.4.5)

$ ./bin/radiant --help | grep integration
  integrations Manage declared MCP integrations (read-only listing; never auto-configures)

$ ./bin/radiant integrations list --help
List MCP servers declared in the project's .mcp.json

Usage:
  radiant integrations list [flags]

Flags:
  -h, --help                help for list
      --json                machine-readable JSON output
      --write-docs string   also write docs/engineering/integrations.md from current MCPs (pass empty for default path)
```

- `integrations` command registered ✓
- Subcommand `list` with all 3 flags (`--json`, `--write-docs`) ✓
- Built binary shows git SHA `d8bbe89` ✓

**Result:** ✅ Pass.

## 5. End-to-end — fresh project, all 3 modes

Tested with a `.mcp.json` containing 3 servers: `github` (with
env var), `slack` (no env), `notion` (URL-only, SSE).

### Mode 1: default table

```
$ radiant integrations list
  MCP servers declared in .mcp.json (3):

    NAME                 COMMAND      ARGS (truncated)                 ENV
    ----                 -------      --------------                   ---
    github               npx          -y @modelcontextprotocol/serv... 1 vars
    notion               <http>       (none)                           0 vars
    slack                npx          -y @modelcontextprotocol/serv... 0 vars

  To validate an MCP, invoke the /integracoes skill.
  To approve and persist a new MCP, edit .mcp.json manually — this command never writes it.
```

- Sorted alphabetically by name ✓
- URL-based server (`notion`) correctly rendered as `<http>` ✓
- Safety reminder always printed ✓
- Counts env vars correctly (github=1, slack=0, notion=0) ✓

### Mode 2: `--json`

```
$ radiant integrations list --json
{
  "github": {
    "command": "npx",
    "args": ["-y", "@modelcontextprotocol/server-github"],
    "env": {"GITHUB_TOKEN": "${GITHUB_TOKEN}"}
  },
  ...
}
```

- Clean JSON, indented for readability ✓
- Suitable for piping to `jq` or other tools ✓

### Mode 3: `--write-docs`

```
$ radiant integrations list --write-docs=docs/engineering/integrations.md
  ✓ wrote docs/engineering/integrations.md

$ wc -l docs/engineering/integrations.md
      28 docs/engineering/integrations.md
```

- Markdown regenerated ✓
- 28 lines (matches design — header + table + approval log stub) ✓

### Edge cases

| Case | Behaviour | Status |
|------|-----------|--------|
| No `.mcp.json` | `Error: no .mcp.json found — invoke the /integracoes skill...` | ✅ |
| Empty `mcpServers: {}` | `(no MCP servers declared in .mcp.json)` | ✅ |
| Malformed JSON | `Error: parse .mcp.json: invalid character 'b'...` | ✅ |

**Result:** ✅ All 3 modes + 3 edge cases work correctly.

## 6. Safety guarantee verified

Per the integracoes skill's anti-patterns:
- "Auto-configuring `.mcp.json` without approval" — explicitly
  forbidden by the skill.
- This command NEVER writes `.mcp.json` — verified by code review
  of `runIntegrationsList`: the only file write is the optional
  `--write-docs` output, which is a Markdown summary, not the
  config itself.

**Result:** ✅ Safety guarantee upheld.

## 7. Test surface

| Sprint | Tests added | Cumulative |
|---|---|---|
| Sprint 10 batch 1 | +19 | 188 |
| Sprint 10 batch 2 | +0 | 188 |
| Sprint 10 batch 3 | +8 | 216 |
| Sprint 11 | +14 | 230 |
| Sprint 12 batch 1 | +5 | 235 |
| Sprint 12 batch 2 | +5 | **240** |

Sprint 12.2 tests:

- `TestRenderIntegrationsDocIncludesServers` — 2-server map → doc
  includes all server names + table header + approval log section.
- `TestRenderIntegrationsDocHandlesHTTPServer` — URL-based server
  rendered as `<http>` (no command available).
- `TestRenderIntegrationsDocEmptyMap` — empty map still emits the
  table header.
- `TestIntegrationsListReadsMCPConfig` — round-trip: write a JSON
  config → read it back → confirm `mcpServer.Command` matches.
- `TestIntegrationsListMissingFile` — confirms `os.IsNotExist` is
  returned for a missing file (the user-facing error path).

All 5 pass in `-race` mode.

## 8. Documentation

- `CHANGELOG.md` — v0.4.5 entry added with full Added section.
- `docs/validation-report-sprint-12-2.md` — first-pass report (committed in d8bbe89).
- `docs/validation-report-sprint-12-2-final.md` — THIS report (final pass).

## 9. No regressions

Comparing to v0.4.4 (commit 9329c7e):

- All 235 prior tests still pass.
- No prior command behaviour changed.
- `radiant integrations list` is purely additive (new command +
  3 new helpers + 1 new subcommand structure).
- The `nova-product` skill (Sprint 12.1) still validates cleanly.

## 10. Decisions

- ✅ Sprint 12 second batch is **READY TO MERGE** at v0.4.5.
- ✅ The read-only design is the correct one — adding/removing
  MCPs is a project-level decision that must go through the
  `integracoes` skill's approval interview (account/workspace
  boundary check). Auto-config would risk data leaks.
- ✅ JSON output enables scripting (e.g. CI checks for "is MCP
  X declared?") without coupling to the markdown layout.

## 11. End-to-end governance flow

The full flow is now complete and runnable:

```
1. radiant product "..."          ← Lean Inception (v0.4.4)
2. radiant spec "<feature>"       ← AC→test mapping (v0.4.2)
3. radiant run specs/<NNNN>       ← implementation (v0.3.x)
4. radiant adr "<decision>"       ← Nygard ADR (v0.4.3)
5. radiant diagramar <level>      ← C4 Mermaid (v0.4.3)
6. radiant integrations list      ← MCP discovery (v0.4.5) ← NEW
7. radiant handoff --feature=...  ← session pause (v0.4.2)
8. radiant update [--force]       ← skill refresh (v0.4.3)
```

## 12. What Sprint 13 will tackle

Per `docs/ROADMAP.md`:

| ID | Deliverable | Skill | Notes |
|---|---|---|---|
| 13.1 | Native views opt-in (`--agent=<list>`) | (CLI-only) | Generate `.claude/`, `.cursor/`, `.codex/`, etc. when requested. |
| 13.2 | `radiant review-pr` | `revisar-pr` | PR review automation against the spec's ACs. |
| 13.3 | `radiant setup-ci` | `setup-ci` | GitHub Actions / GitLab CI / CircleCI scaffold. |
| 13.4 | `radiant camada-agentica` | `camada-agentica` | Generate the agentic layer config. |
| 13.5 | `radiant evals` | `evals` | Run AC→test coverage metrics. |

Sprint 13 closes the **PR + multi-agent views** phase of the
methodology merge. After Sprint 13, the radiant CLI is feature-
complete against the original HARNESS-PLAN.md scope.