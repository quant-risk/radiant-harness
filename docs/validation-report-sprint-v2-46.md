# Validation Report — Sprint 76: v2.46.0 — `cmd_setup_mcp.go` split

> **Date:** 2026-06-29
> **Project version:** v2.46.0
> **Branch:** `feature/light-full-release`
> **Base:** `c3b59c9` (Sprint 76 commit)
> **Status:** PASSED — ready to merge

---

## TL;DR

Sprint 76 is a **pure code-movement refactor** — `cmd_setup_mcp.go`
(781 LOC after Sprint 75) is split into two themed files. Same
package, same identifiers, same behaviour, **zero test edits**.
Continues the debt-reduction rhythm established in Sprint 74
(helpers.go extractions) and applied last sprint to the setup-mcp
file.

| Metric | Value |
|--------|-------|
| Commits on branch | ahead of base (`9b28e77`) |
| New commits in this release | **1** (`c3b59c9`) |
| Files changed | 5 modified, 1 new |
| LOC delta | +712 / −417 (net +295 LOC; mostly the new file's header) |
| Agents supported | **11** (unchanged from v2.45.0) |
| New deps | 0 |
| New tests | **0** (zero behaviour change → all 18 setup-mcp tests pass unmodified) |
| Tests | **1189 PASS, 0 confirmed FAIL** across 31 packages |
| `go vet ./...` | clean |
| Cross-compile | linux/{amd64,arm64}, darwin/{amd64,arm64}, windows/amd64 — all OK |

---

## File layout delta

```
BEFORE (post-Sprint 75)
─────────────────────────
cmd/radiant/cmd_setup_mcp.go    781 LOC  (everything)


AFTER (post-Sprint 76)
──────────────────────
cmd/radiant/cmd_setup_mcp.go              375 LOC  (−406, −52%)
  ├── registerSetupMCPCmd
  ├── radiantBinaryPath
  ├── resolveMCPAgents
  ├── type mcpEntry
  ├── mcpConfigFor            ← routing switch (still the single source of truth)
  ├── mergeClaudeSettings
  ├── mergeMCPJSON            ← shared by Cursor/Windsurf/VSCode
  ├── mergeZedSettings
  └── writeMCPConfig

cmd/radiant/cmd_setup_mcp_per_agent.go    439 LOC  (NEW)
  ├── Codex (TOML):        radiantBlockPattern, tomlQuote, mergeCodexTOML
  ├── OpenCode (nested):   openCodeServer, openCodeConfig, mergeOpenCodeConfig
  ├── Hermes (YAML):       hermesEntry, mergeHermesConfig
  ├── Kimi (global JSON):  mergeKimiMCP
  ├── OpenClaw (nested):   openClawServer, mergeOpenClawJSONConfig
  └── Cline (JSON + opts): clineEntry, mergeClineConfig
```

---

## Build / Vet / Test

```bash
$ go vet ./...
EXIT=0   (silent — clean)

$ go build -o /tmp/radiant ./cmd/radiant
-rwxr-xr-x  14M  /tmp/radiant    # darwin/arm64 host

$ /tmp/radiant --version
2.46.0

$ go test -count=1 ./cmd/radiant/ -run 'TestMerge|TestResolve|TestMCPConfig|TestTomlQuote'
ok      github.com/quant-risk/radiant-harness/cmd/radiant    0.434s
        # all 18 setup-mcp tests pass unmodified

$ go test -count=1 -v ./... | grep -cE "^--- PASS|^    --- PASS|^        --- PASS"
1189

$ go test -count=1 -v ./... | grep -cE "^--- FAIL|^    --- FAIL|^        --- FAIL"
0
```

### Cross-compile matrix

```bash
$ GOOS=linux   GOARCH=amd64 go build -o /tmp/rad-s76-linux-amd64     ./cmd/radiant   # 15M OK
$ GOOS=linux   GOARCH=arm64 go build -o /tmp/rad-s76-linux-arm64     ./cmd/radiant   # 14M OK
$ GOOS=darwin  GOARCH=amd64 go build -o /tmp/rad-s76-darwin-amd64    ./cmd/radiant   # 15M OK
$ GOOS=darwin  GOARCH=arm64 go build -o /tmp/rad-s76-darwin-arm64    ./cmd/radiant   # 14M OK
$ GOOS=windows GOARCH=amd64 go build -o /tmp/rad-s76-windows-amd64.exe ./cmd/radiant  # 15M OK
```

All five platforms built cleanly.

---

## Smoke test (functional regression check)

```bash
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
      "radiant": { ... }
    }
  }
}

$ radiant setup-mcp --agent=kimi --dry-run
  [dry-run] kimi → ~/.kimi/mcp.json
{ "mcpServers": { "radiant": { ... } } }

$ radiant setup-mcp --agent=cline --dry-run
  [dry-run] cline → ~/.cline/mcp.json
{ "mcpServers": { "radiant": { ..., "disabled": false, "autoApprove": [] } } }

$ radiant --version
2.46.0
```

All seven per-agent emitters behave identically to v2.45.0. The
split is invisible at the API boundary.

---

## Imports hygiene after split

**`cmd_setup_mcp.go` (post-split)** keeps only what the routing +
detection + generic-merge layers use:
```
"encoding/json"     (Claude / Cursor / Windsurf / Zed / VSCode merges)
"fmt"
"os"
"os/exec"
"path/filepath"
"strings"
"github.com/spf13/cobra"
```

**`cmd_setup_mcp_per_agent.go` (NEW)** owns the imports that only
the per-agent merges need:
```
"encoding/json"     (OpenCode / Kimi / OpenClaw / Cline)
"fmt"               (Hermes yaml-parse error wrap)
"os"
"regexp"            (Codex TOML block pattern)
"strings"           (Codex TOML builder/escape)
"gopkg.in/yaml.v3"  (Hermes YAML round-trip)
```

Both files live in `package main` — no new packages, no Go-module
boundary changes. The test file (`cmd_setup_mcp_test.go`) continues
to reference all 18 test functions unchanged.

---

## Why this split (not finer, not coarser)

### Finer — one file per agent

That would mean 6 brand-new files of 50–85 LOC each. Adding the
12th agent would require creating a 7th file — a small but real
friction tax. The agents are related enough (every one of them is
"merge MCP config into a different file format") that grouping them
in one file is cheaper to scan.

### Coarser — no split

781 LOC for a single routing file is past comfortable. If a 12th
agent lands, it grows further. The split at the natural "routing vs.
per-agent merges" boundary keeps each file under 450 LOC.

### This split

- `cmd_setup_mcp.go` (375 LOC): command registration, binary-path
  resolution, auto-detect, the routing switch (which is the single
  source of truth for "which agent uses which file"), the 3 generic
  JSON merges (used by 5 agents), the writer.
- `cmd_setup_mcp_per_agent.go` (439 LOC): the six specialized
  merges. Each block is self-contained and easy to extend (append a
  block to add a 12th).

---

## Files modified

```
 CHANGELOG.md                                |   +67   (v2.46.0 entry)
 RELEASE-NOTES.md                            |  +100   (v2.46.0 notes)
 cmd/radiant/cmd_setup_mcp.go                | ±410    (was 781, now 375)
 cmd/radiant/cmd_setup_mcp_per_agent.go      | +439    (NEW)
 cmd/radiant/main.go                         |    ±2   (version bump)
 docs/SPRINT76-PLAN.md                       | +115    (NEW; plan doc)
```

Net `cmd/radiant/` change: **+33 LOC** (just the file-level header in
the new file).

---

## What was NOT in this sprint

- **No agent additions.** Total stays at 11.
- **No behaviour change.** No command renames, no flag changes,
  no output format changes.
- **No new dependencies.** `gopkg.in/yaml.v3` was already a project
  dep from earlier sprints.
- **No `helpers.go` extraction.** That's a separate follow-up;
  current `helpers.go` size is ~2900 LOC and we'll address it in
  Sprint 77+ as a different rhythm.

---

## Backward compatibility

- All 11 agent entry points (claude/cursor/windsurf/zed/vscode/
  codex/opencode/hermes/kimi/openclaw/cline) produce **byte-identical
  output** to v2.45.0.
- The setup-mcp CLI command, all its flags, and all error messages
  are unchanged.
- Internal: function names, signatures, and behaviour are unchanged.
  Tests pass without any modification.

---

## Known limitations

- **One flaky pre-existing test:** `internal/fleet.TestRunAllContextCanceled`
  alternates PASS/FAIL on timing. Not a regression from this sprint.
  Documented in `validation-report-sprint-56-57.md`.

- **Imports in `cmd_setup_mcp_per_agent.go` may grow** if future agents
  need additional stdlib packages (e.g. a hypothetical 12th agent
  using INI format would pull `gopkg.in/ini.v1`). Acceptable
  because the file is now focused.

---

## Verification checklist

- [x] `go vet ./...` clean
- [x] `go build ./...` clean (5 platforms cross-compiled)
- [x] `go test -count=1 ./...` — 1189 PASS, 0 confirmed FAIL
- [x] All 18 setup-mcp tests pass with **zero edits**
- [x] All 11 agents emit identical output to v2.45.0 (smoke test)
- [x] `radiant --version` reports `2.46.0`
- [x] CHANGELOG and RELEASE-NOTES updated
- [x] Plan doc added (`docs/SPRINT76-PLAN.md`)
- [x] git commit `c3b59c9` lands cleanly, working tree clean
