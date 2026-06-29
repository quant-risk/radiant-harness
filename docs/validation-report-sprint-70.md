# Validation Report — Sprint 70: Tool Use Wire-up Parte 2 (v2.39.0)

> **Date:** 2026-06-29
> **Project version:** v2.39.0
> **Branch:** `feature/light-full-release`
> **Base:** `3a0394d` (v2.38.0)
> **Status:** PASSED — ready to merge

---

## TL;DR

Sprint 70 closes the read half of the read-write pair (`read_file`)
and adds the first search primitive (`search_code`). The LLM can
now inspect state before mutating it and grep the project tree
without round-tripping through the shell. Sprint 69's `write_file`
plus Sprint 70's two tools give us 3 concrete structured tools —
enough to replace most ad-hoc shell-out patterns the executor
previously needed.

| Metric | Value |
|--------|-------|
| Commits on this branch | **12** (3 from v2.37.0 plan, 9 features/docs) |
| New commits in Sprint 70 | **1** (1a911d9) |
| New tools wired | **2** (`read_file`, `search_code`) |
| Packages | **28 green**, 0 confirmed failures |
| Tests | **970 PASS** (`go test -count=1 -v ./...`) |
| Pre-existing flaky | 1 (`TestRunAllContextCanceled` in `internal/fleet/`, not a regression) |
| New tests in this release | **+23** (from 947 in v2.38.0) |
| Files changed | 11 — **+1,401 / −19 LOC** |
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
970
$ go test -count=1 -v ./... | grep -cE "^--- FAIL:"
1   # pre-existing flaky; see below
```

### Pre-existing flaky (NOT a regression)

`internal/fleet.TestRunAllContextCanceled` (dispatch_test.go:327)
fails intermittently with `expected non-zero exit for killed
process`. Documented in `docs/validation-report-sprint-56-57.md`
(line 21-23) as a timing-dependent test in the fleet dispatcher's
process-kill path — not in any file created or modified by Sprint
70. Reproduces across multiple `go test -count=1` runs (alternates
PASS/FAIL). Tracked separately from this release.

### Per-package breakdown

| Package | Time | Notes |
|---------|------|-------|
| `cmd/radiant` | 2.34s | 26 commands + tools/list subcommand |
| `internal/engine` | 0.40s | (Sprint 69 changes — unchanged) |
| `internal/fleet` | 6.86s | slowest relative — contains the flaky test |
| `internal/fsutil` | 0.39s | (Sprint 69 — unchanged) |
| `internal/loop` | 1.15s | RealRegistry now wires 3 tools |
| `internal/tools` | 1.05s | (Sprint 69 — unchanged) |
| `internal/tools/fs` | 1.39s | **+ read_file, search_code** |
| `internal/webhook` | 16.19s | slowest absolute (HTTP fixtures) |
| (other 20 packages) | ... | unchanged from v2.38.0 |

### Cross-compile matrix

```bash
$ GOOS=linux   GOARCH=amd64   go build -o .../radiant-linux-amd64   ./cmd/radiant  # 15M OK
$ GOOS=darwin  GOARCH=arm64   go build -o .../radiant-darwin-arm64 ./cmd/radiant  # 14M OK
$ GOOS=windows GOARCH=amd64   go build -o .../radiant-windows-amd64.exe ./cmd/radiant  # 15M OK
```

3/3 platforms clean.

---

## Smoke Tests — Tool registry CLI

Validated against `/tmp/radiant` (darwin/arm64 host binary).

### `radiant tools list --real`

```text
NAME            DESCRIPTION                                                  PARAMS
----            -----------                                                  ------
write_file      Write content to a file at the given path (project-relati... 2
read_file       Read the contents of a file at the given path (project-re... 1
search_code     Search the project for a regex pattern. Returns matching ... 4
```

**3 concrete tools** (was 1 in v2.38.0). `run_gate` remains a stub
for v2.40.0 (Sprint 71).

### `radiant tools list` (default registry)

```text
NAME            DESCRIPTION                                                  PARAMS
----            -----------                                                  ------
run_gate        Run a quality gate command (go test, go vet, etc.). Retur... 1
read_file       Read the contents of a file at the given path. Path must ... 1
write_file      Write content to a file at the given path. Creates parent... 2
search_code     Search the project for a regex pattern. Returns matching ... 2
```

**4 advertised tools** (3 stubs + 1 stub for `run_gate`). Back-compat
preserved — operators inspecting the v2.37.0 surface area see the
same shape.

### `radiant mode show`

```text
Mode:    light
Source:  detected
Reason:  no API key found, defaulting to Light
```

### `radiant semantic list`

```text
Available domains:
  credit-risk            Credit Risk Metrics (Basileia + IFRS 9 + CMN 4.966) (7 metrics, v1.0.0)
  market-risk            (no model embedded)
  liquidity-risk         (no model embedded)
  operational-risk       (no model embedded)
```

---

## What's Committed

Branch `feature/light-full-release` (12 commits ahead of `9b28e77`).

Sprint 70 single commit:

| SHA | Type | Summary |
|-----|------|---------|
| `1a911d9` | feat(tool-use) | read_file + search_code concrete; RealRegistry now 3 tools (v2.39.0) |

### File-level diffstat

```text
$ git diff 3a0394d..1a911d9 --shortstat
 11 files changed, 1401 insertions(+), 19 deletions(-)
```

Highlights:
- `internal/tools/fs/read_file.go` (+109) — ReadFileTool concrete
- `internal/tools/fs/read_file_test.go` (+209) — 12 tests
- `internal/tools/fs/search_code.go` (+247) — SearchCodeTool concrete
- `internal/tools/fs/search_code_test.go` (+247) — 11 tests
- `internal/tools/fs/helper.go` (+22) — shared absProjectDir + joinPath
- `internal/loop/real_registry.go` (+7) — wires 3 tools
- `docs/SPRINT70-PLAN.md` (+180) — plan this release executed
- `docs/TOOL-USE.md` (+240) — read_file + search_code sections
- `CHANGELOG.md`, `RELEASE-NOTES.md` — v2.39.0 entries
- `cmd/radiant/main.go` — version bump to 2.39.0

---

## Test coverage detail — new in Sprint 70

### `internal/tools/fs/read_file_test.go` (12 tests)

| # | Test | What it asserts |
|---|------|-----------------|
| 1 | `TestReadFile_HappyPath` | Reads content, returns correct bytes/lines counts |
| 2 | `TestReadFile_NoTrailingNewline` | Line counter handles missing trailing newline |
| 3 | `TestReadFile_EmptyFile` | Returns bytes=0, lines=0 |
| 4 | `TestReadFile_MissingFile` | Returns "file not found" structured error |
| 5 | `TestReadFile_Directory` | Returns "is a directory" error |
| 6 | `TestReadFile_RejectsUnsafePath` | `../escape.txt`, `../../etc/passwd` rejected |
| 7 | `TestReadFile_RejectsSymlinkedProjectSubdir` | Symlink escape caught |
| 8 | `TestReadFile_RejectsOversizeFile` | File > 4 MiB → structured error |
| 9 | `TestReadFile_RejectsEmptyPath` | Empty/whitespace path rejected |
| 10 | `TestReadFile_RejectsMalformedArgs` | Malformed JSON rejected |
| 11 | `TestReadFile_Annotate` | Annotate() returns trace-friendly map |
| 12 | `TestReadFile_ViaRegistry` | Roundtrip through tools.Registry |

### `internal/tools/fs/search_code_test.go` (11 tests)

| # | Test | What it asserts |
|---|------|-----------------|
| 1 | `TestSearchCode_FindsMatches` | Returns correct file/line; skips `.git`, `.radiant-harness`, `node_modules` |
| 2 | `TestSearchCode_FindsMultipleMatchesInSameLine` | All occurrences on one line reported, correct columns |
| 3 | `TestSearchCode_NoMatches` | Returns empty matches, count=0 |
| 4 | `TestSearchCode_InvalidRegex` | Compile error surfaces as structured error |
| 5 | `TestSearchCode_EmptyPattern` | Empty/whitespace pattern rejected |
| 6 | `TestSearchCode_RespectsScope` | Custom `path` arg limits search |
| 7 | `TestSearchCode_RespectsIncludeGlob` | `*.md` filter works |
| 8 | `TestSearchCode_RejectsUnsafeScope` | `../outside` search root rejected |
| 9 | `TestSearchCode_RespectsMaxResults` | Cap enforced, `truncated=true` |
| 10 | `TestSearchCode_Annotate` | Annotate() returns trace-friendly map |
| 11 | `TestSearchCode_ViaRegistry` | Roundtrip through tools.Registry |

---

## Architecture snapshot (Sprint 70 additions)

```
                        ┌──────────────────────────────────┐
                        │       radiant CLI (Go)           │
                        │       v2.39.0                    │
                        └──────────────────────────────────┘
                                    │
            ┌───────────────────────┼───────────────────────┐
            ▼                       ▼                       ▼
   ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
   │  Light mode     │    │  Full mode      │    │  --mode auto    │
   │  (default)      │    │  (opt-in)       │    │  (resolved)     │
   └─────────────────┘    └─────────────────┘    └─────────────────┘
            │                       │
            ▼                       ▼
   ┌─────────────────┐    ┌─────────────────┐
   │ MCP sampling/   │    │ HTTP backend    │
   │ createMessage   │    │ OpenRouter/     │
   │ → host agent    │    │ OpenAI/         │
   │   credentials   │    │ Anthropic/...   │
   │ Zero API key    │    │ API key required│
   └─────────────────┘    └─────────────────┘
                                    │
                                    ▼
                          ┌─────────────────┐
                          │ Loop engine     │
                          │ (Discover→Plan  │
                          │  →Execute→      │
                          │  Verify→Persist)│
                          └─────────────────┘
                                    │
                ┌───────────────────┼───────────────────┐
                ▼                   ▼                   ▼
        ┌─────────────┐     ┌─────────────┐     ┌─────────────┐
        │ Tool        │     │ Semantic    │     │ Pricing     │
        │ registry    │     │ model       │     │ catalog     │
        │ (Sprint 70) │     │ (credit-    │     │ (Sprint 68) │
        │             │     │  risk)      │     │             │
        │ write_file  │     │             │     │             │
        │ read_file   │     │             │     │             │
        │ search_code │     │             │     │             │
        │ (run_gate   │     │             │     │             │
        │  → S71)     │     │             │     │             │
        └─────────────┘     └─────────────┘     └─────────────┘
```

---

## Gaps (carried into Sprint 71+)

1. **`run_gate` concrete implementation** (Sprint 71) — needs
   `internal/gaterun.RunShellGate` wrapper + `internal/policy.GateBinaries`
   allowlist re-export.

2. **MCP tool-bridge adapter** (Sprint 71) — register MCP server
   tools directly into `tools.Registry`.

3. **Anthropic/OpenAI/Gemini function-call native parsing** (Sprint 72)
   — replace the markdown `tool_call` fence with the SDK's
   structured function-call protocol. Same `Registry.Call` interface,
   different extractor.

4. **Tool-call replay in `radiant loop export`** (Sprint 72) —
   debugging aid for failed runs.

5. **Schema validation beyond JSON type-check** (Sprint 73) —
   min/max length, regex on string args, etc.

6. **Helper extraction from `helpers.go`** — still 3894 lines, candidates
   remain: `audit.go`, `telemetry.go`, `scaffolds.go`, `pr_review.go`.

---

## Compatibility Notes

- **No breaking changes.** New tools are opt-in via the existing
  `Engine.ToolRegistry` wiring.
- **Back-compat preserved:** LLM outputs that contain only
  `write_file` tool calls keep working unchanged.
- **`--no-tools`** still forces the legacy code-block path
  (Sprint 69 — unchanged).
- **`engine.PathIsSafe`** retained as a thin wrapper.

---

## Merge Plan

```bash
cd ~/Library/Mobile\ Documents/com~apple~CloudDocs/projects/radiant-harness-main
git log 3a0394d..1a911d9 --oneline        # 1 commit
git diff 3a0394d..1a911d9 --stat          # 11 files / +1401 / -19
# Then merge v2.39.0 into mainline; tag v2.39.0
```

Or open PR from `feature/light-full-release` → main and let CI gate.
Flaky test (`TestRunAllContextCanceled`) is pre-existing — flag for
separate cleanup, do not block this merge.

---

**Signed off:** Sprint 70 validation pass. Ready to merge and proceed
to Sprint 71 (`run_gate` closes the trio).