# Validation Report — Sprint 13 first batch (v0.4.6)

**Date:** 2026-06-25
**Version:** 0.4.6
**Commit under validation:** (pending — this commit)
**Sprint:** 13 — PR + Multi-agent Views (native views opt-in half)
**Scope:** `radiant views` CLI + `scaffold.GenerateViewsForAgent` helper.

---

## 1. Build hygiene

```
$ go build ./...
(clean)

$ go vet ./...
(clean)

$ gofmt -l .
(clean)
```

**Result:** ✅ Pass.

## 2. Race-detector tests

```
$ CGO_ENABLED=0 go test ./... -count=1 -race -timeout=180s
?   	github.com/quant-risk/radiant-harness/internal         [no test files]
ok  	github.com/quant-risk/radiant-harness/cmd/radiant         2.140s
ok  	github.com/quant-risk/radiant-harness/internal/benchmark  2.459s
ok  	github.com/quant-risk/radiant-harness/internal/engine     1.524s
ok  	github.com/quant-risk/radiant-harness/internal/harness    7.572s
ok  	github.com/quant-risk/radiant-harness/internal/llm        6.044s
ok  	github.com/quant-risk/radiant-harness/internal/policy     2.264s
ok  	github.com/quant-risk/radiant-harness/internal/quality    1.240s
ok  	github.com/quant-risk/radiant-harness/internal/scaffold   3.455s
ok  	github.com/quant-risk/radiant-harness/internal/skill      2.009s
ok  	github.com/quant-risk/radiant-harness/internal/spec       1.929s
```

**Total:** 245 PASS, **0 FAIL**, **0 data races detected**.

**Result:** ✅ Pass.

## 3. Cross-compilation — all 6 targets

```
$ make release
GOOS=linux   GOARCH=amd64 go build ... -o dist/radiant-linux-amd64
GOOS=linux   GOARCH=arm64 go build ... -o dist/radiant-linux-arm64
GOOS=darwin  GOARCH=amd64 go build ... -o dist/radiant-darwin-amd64
GOOS=darwin  GOARCH=arm64 go build ... -o dist/radiant-darwin-arm64
GOOS=windows GOARCH=amd64 go build ... -o dist/radiant-windows-amd64.exe
GOOS=windows GOARCH=arm64 go build ... -o dist/radiant-windows-arm64.exe
✓ Release binaries in dist/
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

## 4. End-to-end — `radiant views`

### Codex (skill-dir layout)

```
$ radiant views --agent=codex --dry-run
  [codex]
    [would-write] AGENTS.md (4442 bytes)
    [would-write] .agents/skills/adr/SKILL.md (2877 bytes)
    [would-write] .agents/skills/auditar/SKILL.md (3227 bytes)
    [would-write] .agents/skills/camada-agentica/SKILL.md (2288 bytes)
    ... (17 skill views total: 17 bundled skills + 1 instructions file)

$ radiant views --agent=codex
  Summary: 18 written, 0 skipped
```

### Cursor (flat layout with .md extension)

```
$ radiant views --agent=cursor
  [cursor]
    [wrote] .cursor/rules/sdd.mdc
    [wrote] .cursor/commands/adr.md
    [wrote] .cursor/commands/auditar.md
    ... (17 skill files)
  Summary: 18 written, 0 skipped
```

### Edge cases

| Case | Behaviour |
|------|-----------|
| Re-run on existing files | `[skipped]` per file; summary shows `0 written, N skipped` |
| `--force` after re-run | All files overwritten |
| `--agent=bogus` | `Error: no valid agents in --agent="bogus" (allowed: ...)` |
| `--agent` empty/missing | `Error: --agent=<list> required (...)` |

**Result:** ✅ All paths work correctly.

## 5. Iteration discipline recap

First attempt of `GenerateViewsForAgent` scanned
`templates/skills/` — which is empty by design (skills live in
the canonical `internal/skill/` bundle). Only the instructions
file was generated; no skill views. Caught during E2E:

- Codex dry-run showed only `AGENTS.md`, no `.agents/skills/*`.
- Fixed by routing through `skill.Bundle()` + `skill.BundledFS()`
  to read from the canonical embedded skills.

The fix is in the same commit — caught at the E2E step before
shipping, not by a unit test (a unit test for "the scaffold
should produce N skill views per agent" would have caught this
faster; the new `TestGenerateViewsForAgentKnownAgents` does
exactly that).

## 6. Test surface

| Sprint | Tests added | Cumulative |
|---|---|---|
| Sprint 10 batch 1 | +19 | 188 |
| Sprint 10 batch 2 | +0 | 188 |
| Sprint 10 batch 3 | +8 | 216 |
| Sprint 11 | +14 | 230 |
| Sprint 12 batch 1 | +5 | 235 |
| Sprint 12 batch 2 | +5 | 240 |
| **Sprint 13 batch 1** | **+5** | **245** |

Sprint 13.1 tests:

- `TestGenerateViewsForAgentKnownAgents` — every adapter produces
  at least 2 views (instructions + ≥1 skill); the first view's
  path matches the adapter's `InstTo`.
- `TestGenerateViewsForAgentUnknown` — unknown agent ID returns
  empty (no panic).
- `TestGenerateViewsForAgentSkillLayouts` — per-agent layout
  choice is reflected in paths: Claude/Codex → `skill-dir`
  (ends with `/SKILL.md`), others → `flat` (`.md` / `.prompt.md`
  / `.toml`).
- `TestGenerateViewsForAgentStripsFrontmatter` — Codex, Copilot,
  Gemini, Windsurf do NOT include YAML frontmatter in their
  instructions file.
- `TestGenerateViewsForAgentKeepsFrontmatter` — Claude and Cursor
  DO include frontmatter (Cursor `.mdc` requires it).

All 5 pass in `-race` mode.

## 7. Decisions

- ✅ Sprint 13 first batch is **READY TO MERGE** at v0.4.6.
- ✅ Existing views skipped by default — local edits win.
- ✅ `--force` is the only way to overwrite existing views
  (DESTRUCTIVE — clearly flagged in the help).
- ✅ `--dry-run` available for safe preview.

## 8. What Sprint 13 will continue to tackle

| ID | Deliverable | Skill | Notes |
|---|---|---|---|
| 13.2 | `radiant review-pr` | `revisar-pr` | PR review automation against the spec's ACs. |
| 13.3 | `radiant setup-ci` | `setup-ci` | GitHub Actions / GitLab CI / CircleCI scaffold. |
| 13.4 | `radiant camada-agentica` | `camada-agentica` | Generate the agentic layer config. |
| 13.5 | `radiant evals` | `evals` | Run AC→test coverage metrics. |

After Sprint 13, the radiant CLI is feature-complete against the
original HARNESS-PLAN.md scope.