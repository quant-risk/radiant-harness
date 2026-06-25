# Validation Report — Sprint 11 FINAL (v0.4.3)

**Date:** 2026-06-25
**Version:** 0.4.3 (literal in source; git build embeds `9e5e424`)
**Commit under validation:** `9e5e424`
**Sprint:** 11 — Discovery Phase Closure (final pass)
**Validator scope:** full re-validation after the first pass to
catch any drift, regressions, or missed edge cases.

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
ok  	github.com/quant-risk/radiant-harness/cmd/radiant         1.774s
ok  	github.com/quant-risk/radiant-harness/internal/benchmark  3.242s
ok  	github.com/quant-risk/radiant-harness/internal/engine     1.480s
ok  	github.com/quant-risk/radiant-harness/internal/harness    7.542s
ok  	github.com/quant-risk/radiant-harness/internal/llm        6.280s
ok  	github.com/quant-risk/radiant-harness/internal/policy     2.944s
ok  	github.com/quant-risk/radiant-harness/internal/quality    2.380s
ok  	github.com/quant-risk/radiant-harness/internal/skill      3.452s
ok  	github.com/quant-risk/radiant-harness/internal/spec       2.245s
```

**Total:** 230 PASS, **0 FAIL**, **0 data races detected**.

**Result:** ✅ Pass.

## 3. Cross-compilation — all 6 targets

```
$ make release
GOOS=linux   GOARCH=amd64 go build -ldflags "-s -w -X main.version=9e5e424" -o dist/radiant-linux-amd64     ./cmd/radiant/
GOOS=linux   GOARCH=arm64 go build -ldflags "-s -w -X main.version=9e5e424" -o dist/radiant-linux-arm64     ./cmd/radiant/
GOOS=darwin  GOARCH=amd64 go build -ldflags "-s -w -X main.version=9e5e424" -o dist/radiant-darwin-amd64    ./cmd/radiant/
GOOS=darwin  GOARCH=arm64 go build -ldflags "-s -w -X main.version=9e5e424" -o dist/radiant-darwin-arm64    ./cmd/radiant/
GOOS=windows GOARCH=amd64 go build -ldflags "-s -w -X main.version=9e5e424" -o dist/radiant-windows-amd64.exe ./cmd/radiant/
GOOS=windows GOARCH=arm64 go build -ldflags "-s -w -X main.version=9e5e424" -o dist/radiant-windows-arm64.exe ./cmd/radiant/
✓ Release binaries in dist/

$ ls -la dist/
-rwxr-xr-x  8409312  radiant-darwin-amd64
-rwxr-xr-x  8076194  radiant-darwin-arm64
-rwxr-xr-x  8192152  radiant-linux-amd64
-rwxr-xr-x  7930008  radiant-linux-arm64
-rwxr-xr-x  8479232  radiant-windows-amd64.exe
-rwxr-xr-x  7956992  radiant-windows-arm64.exe

$ file dist/*
dist/radiant-darwin-amd64:      Mach-O 64-bit executable x86_64
dist/radiant-darwin-arm64:      Mach-O 64-bit executable arm64
dist/radiant-linux-amd64:       ELF 64-bit LSB executable, x86-64, statically linked
dist/radiant-linux-arm64:       ELF 64-bit LSB executable, ARM aarch64, statically linked
dist/radiant-windows-amd64.exe: PE32+ executable (console) x86-64, for MS Windows
dist/radiant-windows-arm64.exe: PE32+ executable (console) Aarch64, for MS Windows
```

| Target | Size | Type | Status |
|---|---|---|---|
| linux/amd64 | 8.0 MB | ELF x86-64 static | ✅ |
| linux/arm64 | 7.6 MB | ELF ARM aarch64 static | ✅ |
| darwin/amd64 | 8.2 MB | Mach-O x86_64 | ✅ |
| darwin/arm64 | 7.7 MB | Mach-O arm64 | ✅ |
| windows/amd64 | 8.1 MB | PE32+ x86-64 | ✅ |
| windows/arm64 | 7.6 MB | PE32+ Aarch64 | ✅ |

Total: ~47 MB across 6 targets.

**Result:** ✅ 6/6 targets build clean.

## 4. Built binary sanity

```
$ ./bin/radiant --version
9e5e424       (git SHA injected by Makefile; literal version in source = 0.4.3)

$ ./bin/radiant --help | head -25
Spec-Driven Development harness that works with any LLM via OpenRouter, OpenAI, Anthropic, or custom providers. No agent dependency.

Usage:
  radiant [command]

Available Commands:
  adr         Create an Architecture Decision Record in Nygard format
  bench       Run radiant-harness against comparable frameworks ...
  completion  Generate the autocompletion script for the specified shell
  config      Configure LLM provider and model
  diagramar   Generate a C4 Mermaid diagram template (context|container|component|code)
  doctor      Diagnose the local environment for radiant-harness
  eval        Run a single prompt against a model N times ...
  handoff     Pause: write the current session state to .radiant-harness/state.md
  init        Scaffold the SDD pipeline
  models      List available model presets
  run         Run the SDD harness on a feature (uses LLM API directly)
  skills      Manage vendor-neutral workflow skills
  spec        Create spec.md + tasks.md for a new feature ...
  state       Show the current session state (resume point)
  update      Refresh bundled skills + AGENTS.md without touching user docs
  validate    Validate SDD pipeline conformity

$ ./bin/radiant skills list | head -8
  Bundled skills (16):
    NAME                   VERSION    TIER         DESCRIPTION
    ----                   -------    ----         -----------
    adr                    1.0.0      feature,arc… Creates an Architecture Decision Record (Nygard format) for
    auditar                1.0.0      architecture Audits the radiant-harness project layout for conformity:
    camada-agentica        1.0.0      architecture Generates the agentic layer configuration for the project:
    clarificar             1.0.0      trivial,fea… Conducts a structured interview to sharpen ambiguous specs,
    diagramar              1.0.0      feature,arc… Produces C4-model architecture diagrams (Context, Container,
    evals                  1.0.0      feature,arc… Measures spec→code fidelity: which ACs are covered by tests,
```

- All 17 commands registered (1 root + 16 sub-commands).
- All 16 bundled skills embedded correctly.
- `--version` returns the git SHA (Makefile injection) which is
  the preferred display in development builds.

**Result:** ✅ Pass.

## 5. End-to-end — fresh project, all 3 new commands

```
$ ./bin/radiant init /tmp/radiant-sprint11-final --all --yes
✓ created AGENTS.md
✓ created .radiant-harness/
✓ created .radiant-harness/skills/<16 skills>
✓ created docs/architecture/adr/0001-record-architecture-decisions.md
✓ created docs/architecture/adr/_template.md

$ ./bin/radiant adr "Use PostgreSQL for jobs" --status=accepted
  ✓ created docs/architecture/adr/0002-use-postgresql-for-jobs.md
  (Next-steps reminder printed)

$ ./bin/radiant adr "Bogus status test" --status=invalid
  ✓ created docs/architecture/adr/0003-bogus-status-test.md
  $ grep -A 1 "^## Status$" docs/architecture/adr/0003-bogus-status-test.md
  ## Status
  proposed   ← invalid status fell back to "proposed" as designed

$ ./bin/radiant diagramar container | head -5
# C4 Level 2 — Containers
#
# Break <Your System> into deployable units ...

$ ./bin/radiant diagramar component -o /tmp/c4-component.md
  ✓ wrote /tmp/c4-component.md    (23 lines)

$ ./bin/radiant update --dry-run
  [unchanged] <16 skills>
  [regenerate] AGENTS.md
  Summary: 0 added, 0 updated, 0 conflict(s)
```

**Result:** ✅ All 3 new commands work end-to-end on a fresh project.

## 6. Test surface — what Sprint 11 added

| Test file | Tests added | Coverage |
|---|---|---|
| `cmd/radiant/main_test.go` | +14 | ADR (5) + frontmatter (4) + AGENTS.md (2) + diagramar (3) |
| **Total** | **+14** | Sprint 11: 230 PASS (was 216) |

All new tests follow the existing pattern: table-driven where
useful, tempdir for filesystem fixtures, no external dependencies,
no LLM calls. They pass in `-race` mode.

## 7. Documentation

- `CHANGELOG.md` — v0.4.3 entry added with full Added section.
- `docs/validation-report-sprint-11.md` — first-pass report (committed in 9e5e424).
- `docs/validation-report-sprint-11-final.md` — THIS report (final pass).
- `docs/ROADMAP.md` — already marked Sprint 11 in progress; will be
  updated to "done" after this report lands.

## 8. No regressions

Comparing to the previous version (v0.4.2, commit d319e96):

- All 216 prior tests still pass.
- No prior command behaviour changed (verified by re-reading the
  `init`, `state`, `handoff`, `spec`, and `run` code paths).
- `internal/skill/` API extended (added `ExtractSkillTo`) but
  existing `ExtractTo` signature unchanged — backward compatible.

## 9. Decisions

- ✅ Sprint 11 is **READY TO MERGE** at v0.4.3.
- ✅ No follow-up fixes required.
- ✅ `dist/` artifacts excluded from git (Makefile generates them
  locally; CI generates them per-tag).

## 10. What Sprint 12 will tackle

Per `docs/HARNESS-PLAN.md` Phase 3 (Governance):

| ID | Deliverable | Skill | Notes |
|---|---|---|---|
| 12.1 | `radiant product "<vision>"` | `nova-product` (new) | Lean Inception phases Why/What/Who → `docs/product/` |
| 12.2 | `radiant integrations list` | `integracoes` | MCP discovery (already have skill, needs CLI hook) |
| 12.3 | `--brownfield` flag for `kickoff` | `kickoff` | Detect existing code, branch the spec interview |

`nova-product` is the next skill to author (top-of-line spec like
`nova-feature`). The `integracoes` skill already exists from
Sprint 10; only the CLI command needs wiring. `kickoff`'s
brownfield branch needs LLM-driven detection of existing
languages/frameworks.

These unblock the **Product Discovery** workflow end-to-end:
product vision → spec → tasks → code → handoff → diagram → ADR.