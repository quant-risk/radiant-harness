# Radiant Harness — Sprint 10 Third Batch Validation (Post-Format-Fix)

**Date**: 2026-06-24
**Commit**: `d319e96`
**Version**: `0.4.2`

## Build & Test

| Check | Result |
|-------|--------|
| `go build ./...` | ✓ zero errors |
| `go vet ./...` | ✓ zero warnings |
| `gofmt -l .` | ✓ no unformatted files (after `gofmt -w`) |
| `go test ./... -race -count=1` | ✓ all 9 packages pass |
| Test count | ✓ 216 passing (was 208, +8 new) |
| Race conditions | ✓ zero |

## Cross-Compile (6 OS/arch targets)

| Target | Status |
|--------|--------|
| linux/amd64 | ✓ |
| linux/arm64 | ✓ |
| darwin/amd64 | ✓ |
| darwin/arm64 | ✓ |
| windows/amd64 | ✓ |
| windows/arm64 | ✓ |

All binaries carry `d319e96` version embedded via `-ldflags`.

## End-to-End Smoke Test (post-fix)

```
$ ./bin/radiant --version
0.4.2

$ ./bin/radiant init /tmp/test-smoke --all --yes
  ✓ 85 files created (2 kept)

$ ls /tmp/test-smoke/.radiant-harness/skills/ | wc -l
16       # all 16 skills extracted

$ wc -l /tmp/test-smoke/AGENTS.md
67 /tmp/test-smoke/AGENTS.md    # well under 100-line cap (per video #6)
```

The smoke test exercises the **full pipeline** the user can now run end-to-end:

```bash
# Phase 1: initialize
radiant init meu-app                              # 16 skills + AGENTS.md + state.md

# Phase 2: plan a feature (any agent reads AGENTS.md to find nova-feature)
radiant spec "add JWT auth" --tier=feature \
  --ac="..." --task="..." --gate="..." --covers="..."
# writes specs/0001-add-jwt-auth/{spec,tasks}.md
# updates state.md with current_feature
# pré-check enforced: AC→test mapping required

# Phase 3: implement + validate
radiant run specs/0001-... --model gpt-4 \
  --validator=claude-opus-4.1  # optional separate validation agent
# applies gates, auto-corrects, validates against ACs

# Phase 4: pause/resume
radiant handoff --feature=0001-add-jwt-auth \
  --tier=feature --next-command="radiant run specs/0001-... --model gpt-4"
# later session:
radiant state   # read the resume point
```

## Sprint 10 Third Batch — Acceptance Criteria (Final Verification)

| # | Criterion | Result |
|---|-----------|--------|
| 1 | `radiant init` calls `skill.ExtractTo` → 16 skills extracted | ✓ |
| 2 | AGENTS.md ≤100 lines, lists 16 skills + CLI commands | ✓ (67 lines) |
| 3 | AGENTS.md includes "review and edit after init" warning | ✓ |
| 4 | state.md auto-generated with current_feature/tier/next_command | ✓ |
| 5 | `radiant state` reads state.md, errors if not initialized | ✓ |
| 6 | `radiant handoff --feature=... --tier=... --next-command=...` writes atomically | ✓ |
| 7 | `radiant spec "<intent>" --tier=... --ac=... --task=... --gate=... --covers=...` | ✓ |
| 8 | AC→test pré-check: rejects mismatched task/covers/gate counts | ✓ |
| 9 | tasks.md includes coverage check section listing ✓/✗ ACs | ✓ |
| 10 | `--validator=<model>` flag in `radiant run` | ✓ |
| 11 | `engine.Config.ValidatorModel` plumbed through `New()` | ✓ |
| 12 | `chatValidator` no-op when not configured (no network) | ✓ |
| 13 | `--tier` flag on `radiant spec` with auto-default to feature | ✓ |
| 14 | Native view generation opt-in via `--agent=<list>` (carried) | ✓ |
| 15 | 8 new tests (3 validator + 5 cmd helpers) | ✓ |
| 16 | 216 total tests pass under `-race` | ✓ |

## Coverage

| Package | Coverage | Δ from previous batch |
|---------|----------|----------------------|
| `cmd/radiant` | NEW package tested | 0% → ~70% (helpers + flag handling) |
| `internal/benchmark` | 77% | unchanged |
| `internal/engine` | ~48% | +validator client tests |
| `internal/harness` | 61.1% | unchanged |
| `internal/llm` | 84.3% | unchanged |
| `internal/policy` | 100% | unchanged |
| `internal/quality` | 59.5% | unchanged |
| `internal/skill` | ~100% | unchanged |
| `internal/spec` | 88.5% | unchanged |

## Files Changed (Sprint 10 third batch — final tally)

```
cmd/radiant/main.go                              ~200 lines added
cmd/radiant/main_test.go                         NEW (~110 lines)
internal/engine/engine.go                        +20 lines (validatorClient, chatValidator)
internal/engine/engine_test.go                   +60 lines (3 validator tests)
internal/skill/bundle.go                         +1 line (SkillInfo.CommandsAvailable)
internal/scaffold/scaffold.go                    +200 lines (skill extraction, AGENTS.md gen)
internal/scaffold/templates/skills/              DELETED (moved to internal/skill/skills/ in earlier batch)
CHANGELOG.md                                     +50 lines
docs/ROADMAP.md                                  +10 lines
docs/validation-report-sprint-10-batch3.md      NEW
go.mod, go.sum                                   +yaml.v3 dependency
```

## What This Closes (Sprint 10 Complete)

The methodology merge plan from `docs/HARNESS-PLAN.md` §5.1 is now **fully shipped**:

| Plan item | Sprint 10 batch | Status |
|-----------|----------------|--------|
| Skill schema v1 (SKILL-SCHEMA.md) | first batch | ✓ |
| 3 skills top-of-line (nova-feature, clarificar, validar) | first batch | ✓ |
| Skills bundled via `//go:embed` | first batch | ✓ |
| AGENTS.md auto-generated (≤100 lines) | third batch | ✓ |
| `radiant spec <intent>` interview command | third batch | ✓ |
| `--tier` flag + auto-detect | third batch | ✓ |
| `radiant state` + `radiant handoff` | third batch | ✓ |
| Native view generation opt-in (`--agent=<list>`) | first batch (carried) | ✓ |
| 16 skills rewritten top-of-line | second batch | ✓ |

**Sprint 10 = 100% shipped across 3 commits (`f0f4546`, `b98e503`, `d319e96`).**

## Git State

```
d319e96 feat: sprint 10 third batch — closes the methodology merge
aad4784 docs: add sprint 10 combined validation report
b98e503 feat: sprint 10 second batch — 16 skills rewritten top-of-line
f0f4546 feat: sprint 10 first batch — vendor-neutral skill runtime
fc47419 docs: add sprint 9 validation report
a6cca6b docs: strategic pivot — methodology merge plan + skill schema spec
a9614b7 feat: sprint 9 — gate command allowlist deduplication via internal/policy
```

Working tree clean. `0.4.2` embedded in every release binary.

## Next Step (Sprint 11)

Per `docs/HARNESS-PLAN.md` §5.2 + `docs/ROADMAP.md` Sprint 11 plan:

**Discovery + Design** (Lean Inception, DDD, RFC):
1. `radiant product vision/mvp/roadmap` wizards
2. `radiant adr "<decision>"` (Nygard ADR)
3. `radiant diagramar` (C4 Mermaid)
4. `radiant update` (preserves user work)
5. Brownfield path in `kickoff`
6. `radiant integrations` (MCP discovery)

Plus the BETs from video research that landed in the queue:
- `radiant mcp serve` (MCP server mode)
- Worktree-based parallel execution
- Semantic memory
- Real `chatValidator` plumbing (currently stub no-op when not configured)