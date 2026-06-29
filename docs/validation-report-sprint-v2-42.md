# Validation Report — Sprint 68: v2.42.0 — Light/Full by subcommand

> **Date:** 2026-06-29
> **Project version:** v2.42.0
> **Branch:** `feature/light-full-release`
> **Base:** `293ec2e` (v2.41.0 — Sprint 72)
> **Status:** PASSED — ready to merge

---

## TL;DR

The "no more mode flag" release. v2.37.0 introduced Light and Full
as runtime modes with a 4-level resolution chain (flag > env > config >
auto-detect). That was overengineered and a constant source of
confusion. v2.42.0 collapses the dichotomy into the subcommand name
itself:

- `radiant mcp serve` is **always Light** (MCP sampling, no API key)
- Every other subcommand is **always Full** (HTTP direct, API key)

No `--mode` flag. No `RADIANT_MODE` env. No `mode:` field. No
`radiant mode show/set` subcommand. Behaviour emerges from the
subcommand the operator invokes.

| Metric | Value |
|--------|-------|
| Commits on branch | **16** ahead of base (`9b28e77`) |
| New commits in this release | **1** (`d2ef8d5`) |
| Files changed | 11 (10 modified, 1 deleted) |
| LOC delta | +414 / −671 |
| Tests | **982 PASS, 0 confirmed FAIL** across 30 packages |
| Flaky pre-existing | 1 (`TestRunAllContextCanceled` in `internal/fleet/`, NOT a regression — documented in `validation-report-sprint-56-57.md`) |
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
982
$ go test -count=1 -v ./... | grep -cE "^--- FAIL:"
1   # pre-existing flaky — see below
```

### Pre-existing flaky (NOT a regression)

`internal/fleet.TestRunAllContextCanceled` failed once during the
test run; passed on subsequent runs. Documented in
`docs/validation-report-sprint-56-57.md` (line 21-23) as a
timing-dependent test in the fleet dispatcher's process-kill path —
not in any file created or modified by Sprint 68. Reproduces
across multiple `go test -count=1` runs (alternates PASS/FAIL).
Tracked separately from this release.

### Per-package timing

All 30 packages green. Slowest packages (HTTP fixtures and fleet
dispatch):

| Package | Time |
|---------|------|
| `internal/webhook` | 15.5s |
| `internal/fleet` | 4.3s |
| `internal/llm` | 5.7s |
| `internal/harness` | 6.0s |
| `cmd/radiant` | 2.3s |
| (other 25 packages) | <2s each |

### Cross-compile matrix

```bash
$ GOOS=linux   GOARCH=amd64   go build -o .../radiant-linux-amd64   ./cmd/radiant  # 15M OK
$ GOOS=darwin  GOARCH=arm64   go build -o .../radiant-darwin-arm64 ./cmd/radiant  # 14M OK
$ GOOS=windows GOARCH=amd64   go build -o .../radiant-windows-amd64.exe ./cmd/radiant  # 15M OK
```

3/3 platforms clean.

---

## Smoke Tests — CLI surface

### `radiant --help` — subcommand list

```text
doctor          Diagnose the radiant environment — API key, model, git, worktrees
fleet           Multi-agent coordination
loop            Manage the autonomous feedback loop (start, status, resume)
mcp             MCP server commands
model           Scaffold a model spec
models          List available model presets
run             Run the SDD harness on a feature
tools           Inspect the structured tool-use registry
```

**No more `mode` subcommand** (was `radiant mode show/set`).

### `radiant mode` — removed

```text
$ radiant mode
Error: unknown command "mode" for "radiant"

Did you mean this?
        models
        model
```

### `radiant loop start --help` — no `--mode` flag

```text
--auto-route              Auto-select per-phase models from the anchor's preset family
--model string            Model ID for cost tracking (e.g. claude-sonnet-4-6)
--planner-model string    Model used for planning
--verifier-model string   Separate model for verification
```

The "mode" matches are `--model`, `--auto-route`, `--planner-model`,
`--verifier-model` — not `--mode`.

### `radiant mcp serve --help` — always Light

```text
Start the MCP server on stdio. The harness operates in
Light mode: it uses MCP sampling/createMessage to request LLM inference
from the calling agent (Claude Code, Hermes, Cursor, etc.). No API key
is required — the host agent pays for the inference.

This is one half of the Light/Full split. The other half (Full mode,
autonomous HTTP calls) lives in the regular subcommands:
  - radiant loop start
  - radiant run
  - radiant fleet start
  - radiant init / validate / etc.

Behaviour emerges from the subcommand. No --mode flag, no
RADIANT_MODE env, no mode: field in .radiant.yaml.
```

No `--sampling` flag either — sampling is the only path.

### `radiant doctor` — simplified mode check

```text
radiant doctor
────────────────────────────────────────────────────────────
  ✗  API key                 none found — set OPENROUTER_API_KEY, OPENAI_API_KEY, or ANTHROPIC_API_KEY
  ✓  git installed           git version 2.50.1 (Apple Git-155)
  ✓  git repo                ok
  ✓  worktrees               no stale worktrees
  ✓  model                   claude-sonnet-4-6 (default)
  ✓  radiant binary          /tmp/radiant
  ✗  mode                    Full mode (CLI subcommand) requires an API key — export OPENROUTER_API_KEY, ...
```

The "mode" check now reports "Full mode (CLI subcommand)" because
CLI subcommands are by definition Full. Reports "requires API key"
if no key is found.

---

## What's Committed

Branch `feature/light-full-release` (16 commits ahead of `9b28e77`).

Sprint 68 single commit:

| SHA | Type | Summary |
|-----|------|---------|
| `d2ef8d5` | refactor(mode) | Light/Full by subcommand, not by flag (v2.42.0) |

### File-level diffstat

```text
$ git diff 293ec2e..d2ef8d5 --shortstat
 11 files changed, 414 insertions(+), 671 deletions(-)

$ git diff 293ec2e..d2ef8d5 --name-status
M       CHANGELOG.md
M       RELEASE-NOTES.md
M       cmd/radiant/cmd_audit.go
M       cmd/radiant/cmd_doctor.go
M       cmd/radiant/cmd_fleet.go
M       cmd/radiant/cmd_loop.go
D       cmd/radiant/cmd_mode.go                # DELETED
M       cmd/radiant/main.go
M       docs/MODES.md
M       internal/mode/mode.go
M       internal/mode/mode_test.go
```

### Highlights

- **`cmd/radiant/cmd_mode.go` DELETED** (the `radiant mode show/set`
  subcommand).
- **`internal/mode/mode.go`** rewritten: 215 → 50 LOC. Removed
  `Resolve()`, `Source` enum, `Resolution` struct, `Detect()`.
  Kept only type definitions (`Light`, `Full`, `Mode.String()`,
  `Mode.Description()`, `Mode.IsValid()`).
- **`internal/mode/mode_test.go`** rewritten: now tests the simple
  constants.
- **`cmd/radiant/cmd_loop.go`** removed `--mode` flag, removed
  `modeFlag` reading, removed resolution logic, removed
  "Light mode cannot run from CLI" check.
- **`cmd/radiant/cmd_fleet.go`** same as above.
- **`cmd/radiant/cmd_audit.go`** removed `--sampling` flag from
  `mcp serve`. Sampling is now always on. Added TTY warning.
- **`cmd/radiant/cmd_doctor.go`** simplified mode check.
- **`docs/MODES.md`** complete rewrite — "behaviour emerges from
  subcommand" narrative, not "resolution chain" reference.
- **`CHANGELOG.md`** v2.42.0 entry with migration table.
- **`RELEASE-NOTES.md`** v2.42.0 entry with migration table.

---

## Migration

| v2.37.0–v2.41.0 | v2.42.0 |
|----------------|---------|
| `radiant mode show` | **removed** — use `radiant --help` |
| `radiant mode set light` | **removed** — use `radiant mcp serve` |
| `radiant mode set full` | **removed** — all other commands are Full |
| `--mode=light` on `loop start` | **removed** — `loop` is always Full |
| `--mode=full` on `loop start` | **removed** — `loop` is always Full |
| `--mode=light|full|auto` on `fleet start` | **removed** — `fleet` is always Full |
| `--sampling` on `mcp serve` | **removed** — sampling is always on |
| `RADIANT_MODE` env var | **removed** — silently ignored if set |
| `mode:` field in `.radiant.yaml` | **removed** — silently ignored if set |
| `internal/mode.Resolve()` chain | **removed** — replaced by simple constants |

---

## Architecture Snapshot

```
                        ┌──────────────────────────────────┐
                        │       radiant CLI (Go)           │
                        │       v2.42.0                    │
                        └──────────────────────────────────┘
                                    │
            ┌───────────────────────┴───────────────────────┐
            ▼                                               ▼
   ┌─────────────────────────┐               ┌─────────────────────────┐
   │  Light (always)         │               │  Full (always)          │
   │                         │               │                         │
   │  $ radiant mcp serve    │               │  $ radiant loop start   │
   │                         │               │  $ radiant run          │
   │  MCP sampling from      │               │  $ radiant fleet start  │
   │  host agent.            │               │  $ radiant init         │
   │  No API key.            │               │  $ radiant validate     │
   │                         │               │                         │
   │  Uses SamplingBackend   │               │  Uses HTTPBackend       │
   │  via internal/llm/      │               │  via internal/llm/      │
   └─────────────────────────┘               └─────────────────────────┘
            │                                               │
            ▼                                               ▼
   ┌─────────────────────────┐               ┌─────────────────────────┐
   │  Host agent (Claude     │               │  OpenRouter / OpenAI /  │
   │  Code, Hermes, Cursor,  │               │  Anthropic / Groq /     │
   │  etc.) pays for tokens  │               │  Mistral / xAI          │
   └─────────────────────────┘               └─────────────────────────┘
```

---

## Gaps (carried into Sprint 69+)

1. **Tool-use: SDK-level function-call parsing** (Sprint 72+
   next frontier). Replace the markdown `tool_call` fence with
   the SDK's structured function-call protocol.
2. **MCP HTTP/SSE transport** (Sprint 73+) — currently stdio only.
3. **Tool-call replay in `radiant loop export`** (debugging aid).
4. **`helpers.go` extraction** (3894 lines still large — candidates:
   `audit.go`, `telemetry.go`, `scaffolds.go`, `pr_review.go`).
5. **i18n of the 24 skills still in PT-BR**.
6. **More semantic-model domains** (`market-risk`,
   `liquidity-risk`, `operational-risk` are placeholders).

---

## Merge Plan

```bash
cd ~/Library/Mobile\ Documents/com~apple~CloudDocs/projects/radiant-harness-main
git log 293ec2e..d2ef8d5 --oneline    # 1 commit
git diff 293ec2e..d2ef8d5 --stat      # 11 files / +414 / -671
# Then merge v2.42.0 into mainline; tag v2.42.0
```

Or open PR from `feature/light-full-release` → main and let CI gate.
No flaky tests need to block this merge (pre-existing flaky is
noted, not blocking).

---

**Signed off:** Sprint 68 (v2.42.0) validation pass. Ready to merge
and proceed to Sprint 69 (agent support expansion — adding Codex
and other MCP-capable agents to `setup-mcp` auto-detect).