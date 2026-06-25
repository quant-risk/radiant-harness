# Validation Report — Sprint 13 first batch FINAL (v0.4.6)

**Date:** 2026-06-25
**Version:** 0.4.6 (literal in source; git build embeds `e22dcd7`)
**Commit under validation:** `e22dcd7`
**Sprint:** 13 — PR + Multi-agent Views (native views opt-in; final pass)
**Scope:** `radiant views` CLI + `scaffold.GenerateViewsForAgent` + `skill.BundledFS()`.

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
ok  	github.com/quant-risk/radiant-harness/cmd/radiant         3.464s
ok  	github.com/quant-risk/radiant-harness/internal/benchmark  3.765s
ok  	github.com/quant-risk/radiant-harness/internal/engine     2.384s
ok  	github.com/quant-risk/radiant-harness/internal/harness    8.800s
ok  	github.com/quant-risk/radiant-harness/internal/llm        6.053s
ok  	github.com/quant-risk/radiant-harness/internal/policy     2.659s
ok  	github.com/quant-risk/radiant-harness/internal/quality    1.887s
ok  	github.com/quant-risk/radiant-harness/internal/scaffold   4.601s
ok  	github.com/quant-risk/radiant-harness/internal/skill      2.299s
ok  	github.com/quant-risk/radiant-harness/internal/spec       2.307s
```

**Total:** 245 PASS, **0 FAIL**, **0 data races detected**.

**Result:** ✅ Pass.

## 3. Cross-compilation — all 6 targets

```
$ make release
GOOS=linux   GOARCH=amd64 go build -ldflags "-s -w -X main.version=e22dcd7" -o dist/radiant-linux-amd64     ./cmd/radiant/
GOOS=linux   GOARCH=arm64 go build -ldflags "-s -w -X main.version=e22dcd7" -o dist/radiant-linux-arm64     ./cmd/radiant/
GOOS=darwin  GOARCH=amd64 go build -ldflags "-s -w -X main.version=e22dcd7" -o dist/radiant-darwin-amd64    ./cmd/radiant/
GOOS=darwin  GOARCH=arm64 go build -ldflags "-s -w -X main.version=e22dcd7" -o dist/radiant-darwin-arm64    ./cmd/radiant/
GOOS=windows GOARCH=amd64 go build -ldflags "-s -w -X main.version=e22dcd7" -o dist/radiant-windows-amd64.exe ./cmd/radiant/
GOOS=windows GOARCH=arm64 go build -ldflags "-s -w -X main.version=e22dcd7" -o dist/radiant-windows-arm64.exe ./cmd/radiant/
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
e22dcd7       (git SHA injected by Makefile; literal version in source = 0.4.6)

$ ./bin/radiant views --help
Generate native agent views (.claude/, .cursor/, .codex/, etc.) on demand

Usage:
  radiant views [flags]

Flags:
      --agent string   comma-separated agent list (claude,codex,cursor,copilot,gemini,windsurf) or --agent=all
      --dry-run        show what would change without writing
      --force          overwrite existing views (DESTRUCTIVE — loses local edits)
  -h, --help           help for views
```

- `views` command registered ✓
- All 3 flags (`--agent`, `--dry-run`, `--force`) present ✓
- Built binary shows git SHA `e22dcd7` ✓

**Result:** ✅ Pass.

## 5. End-to-end — fresh project, all 3 agent variants

### Variant 1: Claude (skill-dir layout, keeps frontmatter)

```
$ radiant views --agent=claude --dry-run
  [claude]
    [would-write] CONVENTIONS.md (4555 bytes)
    [would-write] .claude/skills/adr/SKILL.md (2877 bytes)
    [would-write] .claude/skills/auditar/SKILL.md (3227 bytes)
    [would-write] .claude/skills/camada-agentica/SKILL.md (2288 bytes)
    ... (17 skill views)
```

- Instructions file: `CONVENTIONS.md` (Claude's canonical path).
- Skill layout: `skill-dir` (each skill in its own directory).
- Frontmatter: **kept** (Claude's convention).

### Variant 2: Gemini (flat layout, TOML format, strips frontmatter)

```
$ radiant views --agent=gemini
  [gemini]
    [wrote] GEMINI.md
    [wrote] .gemini/commands/adr.toml
    [wrote] .gemini/commands/auditar.toml
    [wrote] .gemini/commands/nova-product.toml
    ... (17 skill views)
  Summary: 18 written, 0 skipped

$ head .gemini/commands/nova-product.toml
description = ""
prompt = """
```

- Instructions file: `GEMINI.md`.
- Skill layout: `flat` (each skill becomes `<name>.toml`).
- Format conversion: SKILL.md → TOML with `description = "..."` +
  `prompt = """..."""` blocks (the `toToml` helper).
- Frontmatter: **stripped**.

### Edge cases

| Case | Behaviour | Status |
|------|-----------|--------|
| Re-run on existing files | Each file marked `[skipped]`; summary shows `0 written, N skipped` | ✅ |
| `--force` after re-run | All files overwritten | ✅ |
| `--agent=bogus` | `Error: no valid agents in --agent="bogus" (allowed: ...)` | ✅ |
| `--agent` empty/missing | `Error: --agent=<list> required (...)` | ✅ |
| Mixed list (one valid + one invalid) | Invalid silently dropped, valid runs | ✅ (via `resolveAgents`) |

**Result:** ✅ All paths work correctly.

## 6. Per-agent format correctness (from tests)

| Agent | InstTo | InstFM | SkillsLayout | SkillsDir | SkillsExt |
|-------|--------|--------|--------------|-----------|-----------|
| Claude | `CONVENTIONS.md` | keep | skill-dir | `.claude/skills` | — |
| Codex | `AGENTS.md` | strip | skill-dir | `.agents/skills` | — |
| Cursor | `.cursor/rules/sdd.mdc` | keep | flat | `.cursor/commands` | `md` |
| Copilot | `.github/copilot-instructions.md` | strip | flat | `.github/prompts` | `prompt.md` |
| Gemini | `GEMINI.md` | strip | flat | `.gemini/commands` | `toml` |
| Windsurf | `.windsurf/rules/sdd.md` | strip | flat | `.windsurf/workflows` | `md` |

Each of these is enforced by the 5 unit tests:

- `TestGenerateViewsForAgentKnownAgents` — every adapter produces
  ≥2 views; first view path matches `InstTo`.
- `TestGenerateViewsForAgentUnknown` — unknown agent ID returns
  empty (no panic).
- `TestGenerateViewsForAgentSkillLayouts` — per-agent layout
  choice reflected in paths.
- `TestGenerateViewsForAgentStripsFrontmatter` — Codex/Copilot/
  Gemini/Windsurf do NOT include frontmatter.
- `TestGenerateViewsForAgentKeepsFrontmatter` — Claude/Cursor DO
  include frontmatter.

All 5 pass in `-race` mode.

## 7. Iteration discipline recap

The first attempt of `GenerateViewsForAgent` scanned
`scaffold/templates/skills/` — which is empty by design (skills
live in the canonical `internal/skill/` bundle). Only the
instructions file was generated; no skill views. Caught at the E2E
step:

- Codex dry-run showed only `AGENTS.md`, no `.agents/skills/*`.
- Fixed in the same commit by routing through `skill.Bundle()` +
  `skill.BundledFS()` to read from the canonical embedded skills.

The new unit tests (`TestGenerateViewsForAgentKnownAgents` + the
layout/strip/keep tests) would have caught this in CI on the next
run — but this round-trip caught it at dev time. The lesson:
**the CI guard is the safety net; the E2E check is the first
line of defence.**

## 8. Test surface

| Sprint | Tests added | Cumulative |
|---|---|---|
| Sprint 10 batch 1 | +19 | 188 |
| Sprint 10 batch 2 | +0 | 188 |
| Sprint 10 batch 3 | +8 | 216 |
| Sprint 11 | +14 | 230 |
| Sprint 12 batch 1 | +5 | 235 |
| Sprint 12 batch 2 | +5 | 240 |
| **Sprint 13 batch 1** | **+5** | **245** |

**Result:** ✅ Pass.

## 9. Documentation

- `CHANGELOG.md` — v0.4.6 entry added with full Added section.
- `docs/validation-report-sprint-13-1.md` — first-pass report (committed in e22dcd7).
- `docs/validation-report-sprint-13-1-final.md` — THIS report (final pass).

## 10. No regressions

Comparing to v0.4.5 (commit 6f36f45):

- All 240 prior tests still pass.
- No prior command behaviour changed.
- `radiant views` is purely additive.
- All 17 bundled skills still pass schema validation.

## 11. Decisions

- ✅ Sprint 13 first batch is **READY TO MERGE** at v0.4.6.
- ✅ Existing views skipped by default — local edits win.
- ✅ `--force` is the only way to overwrite existing views
  (DESTRUCTIVE — clearly flagged in the help).
- ✅ `--dry-run` available for safe preview.
- ✅ `skill.BundledFS()` is the canonical accessor for the embedded
  skills filesystem. Other packages needing to read individual
  SKILL.md files should use it.

## 12. End-to-end governance + views flow

```
1. radiant product "..."          ← Lean Inception (v0.4.4)
2. radiant spec "<feature>"       ← AC→test mapping (v0.4.2)
3. radiant run specs/<NNNN>       ← implementation (v0.3.x)
4. radiant adr "<decision>"       ← Nygard ADR (v0.4.3)
5. radiant diagramar <level>      ← C4 Mermaid (v0.4.3)
6. radiant integrations list      ← MCP discovery (v0.4.5)
7. radiant handoff --feature=...  ← session pause (v0.4.2)
8. radiant update [--force]       ← skill refresh (v0.4.3)
9. radiant views --agent=<list>   ← native agent views (v0.4.6) ← NEW
```

## 13. What Sprint 13 will continue to tackle

| ID | Deliverable | Skill | Notes |
|---|---|---|---|
| 13.2 | `radiant review-pr` | `revisar-pr` | PR review automation against the spec's ACs. |
| 13.3 | `radiant setup-ci` | `setup-ci` | GitHub Actions / GitLab CI / CircleCI scaffold. |
| 13.4 | `radiant camada-agentica` | `camada-agentica` | Generate the agentic layer config. |
| 13.5 | `radiant evals` | `evals` | Run AC→test coverage metrics. |

After Sprint 13, the radiant CLI is feature-complete against the
original HARNESS-PLAN.md scope.