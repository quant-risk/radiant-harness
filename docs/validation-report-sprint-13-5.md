# Validation Report — Sprint 13 fifth batch (v0.5.0)

**Date:** 2026-06-25
**Version:** 0.5.0
**Commit under validation:** (pending — this commit)
**Sprint:** 13 — PR + Multi-agent Views (AC→test coverage metrics)
**Scope:** `radiant evals` CLI + coverage helpers + 5 tests.

**Milestone:** this is the last planned deliverable in the
methodology merge defined in `docs/HARNESS-PLAN.md`. After this
commit, the radiant CLI is feature-complete against the original
scope. Version bumped to 0.5.0 to mark the release boundary.

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
ok  	github.com/quant-risk/radiant-harness/cmd/radiant         1.698s
ok  	github.com/quant-risk/radiant-harness/internal/benchmark  2.789s
ok  	github.com/quant-risk/radiant-harness/internal/engine     1.880s
ok  	github.com/quant-risk/radiant-harness/internal/harness    7.390s
ok  	github.com/quant-risk/radiant-harness/internal/llm        7.864s
ok  	github.com/quant-risk/radiant-harness/internal/policy     2.553s
ok  	github.com/quant-risk/radiant-harness/internal/quality    3.068s
ok  	github.com/quant-risk/radiant-harness/internal/scaffold   4.356s
ok  	github.com/quant-risk/radiant-harness/internal/skill      1.860s
ok  	github.com/quant-risk/radiant-harness/internal/spec       1.935s
```

**Total:** 268 PASS, **0 FAIL**, **0 data races detected**.

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
| darwin/amd64 | ✅ |
| windows/amd64 | ✅ |
| windows/arm64 | ✅ |

**Result:** ✅ 6/6 targets build clean.

## 4. End-to-end — `radiant evals`

Test fixture: 2 features (0001-login fully covered, 0002-reports uncovered).

```
$ radiant evals
  ✓ wrote docs/evals-report.md

  Features: 2
  ACs: 4 total, 3 claimed-covered (75%)

  Per-feature scores (worst first):
    0002-reports — 0/1 (0%)
    0001-login — 3/3 (100%)

  ⚠ fidelity below 80% — review uncovered ACs above
```

- Walks `specs/` ✓
- Parses ACs from spec.md (`### AC1`, `### AC2`, ...) ✓
- Parses tasks.md Coverage column ✓
- Per-feature scores sorted worst-first ✓
- Aggregate fidelity computed correctly ✓
- Warning printed when fidelity < 80% ✓

### Edge cases

| Case | Behaviour |
|------|-----------|
| No `specs/` directory | `Error: no specs directory found — initialize with 'radiant init' or 'radiant spec'` |
| Empty tasks.md | ACs scored as uncovered (0%) |
| No ACs in spec | Feature skipped (no coverage to compute) |
| Skip `_templates/` and `quick/` | Implemented in walk logic |

**Result:** ✅ All paths + edge cases work.

## 5. Methodology merge — complete

Per `docs/HARNESS-PLAN.md`, the 4-phase methodology merge is now
fully shipped:

| Sprint | Theme | Range | Status |
|--------|-------|-------|--------|
| 10 | Foundation (skill runtime, 16 skills, schema spec, init/state/handoff/spec) | v0.4.0–0.4.2 | ✅ |
| 11 | Discovery (adr, update, diagramar) | v0.4.3 | ✅ |
| 12 | Governance (product, integrations list) | v0.4.4–0.4.5 | ✅ |
| 13 | PR + multi-agent views (views, review-pr, setup-ci, camada-agentica, evals) | v0.4.6–0.5.0 | ✅ |

**Total deliverable count:** 24 commands + 17 bundled skills +
1 open schema spec (MIT) — every line item from
`docs/HARNESS-PLAN.md` is shipped.

## 6. Test surface

| Sprint | Tests added | Cumulative |
|---|---|---|
| Sprint 10 batch 1 | +19 | 188 |
| Sprint 10 batch 2 | +0 | 188 |
| Sprint 10 batch 3 | +8 | 216 |
| Sprint 11 | +14 | 230 |
| Sprint 12 batch 1 | +5 | 235 |
| Sprint 12 batch 2 | +5 | 240 |
| Sprint 13 batch 1 | +5 | 245 |
| Sprint 13 batch 2 | +9 | 254 |
| Sprint 13 batch 3 | +6 | 260 |
| Sprint 13 batch 4 | +3 | 263 |
| **Sprint 13 batch 5** | **+5** | **268** |

Sprint 13.5 tests:

- `TestComputeFeatureCoverageAllCovered` — 2 ACs both in tasks.md
  → 2/2, score 1.0, 0 uncovered.
- `TestComputeFeatureCoveragePartial` — 3 ACs but tasks.md only
  mentions 2 → 2/3, uncovered=[AC3].
- `TestComputeFeatureCoverageNoTasksMD` — no tasks.md → 0/2 covered.
- `TestRenderEvalsReportIncludesSections` — all 4 sections +
  feature names + aggregate row.
- `TestRenderEvalsReportComputesAggregate` — empty feature (0 ACs)
  contributes 0 to aggregate (no division by zero).

All 5 pass in `-race` mode.

## 7. Decisions

- ✅ Sprint 13 fifth batch is **READY TO MERGE** at v0.5.0.
- ✅ MVP is "claimed coverage" (does tasks.md list this AC?).
  Real verification (test passes + covers AC's Given/When/Then)
  is the LLM's job via the evals skill.
- ✅ Per-AC evidence is a TODO row in the report — the LLM
  fills in file:line citations per the skill's anti-pattern
  ("Reporting fidelity scores without evidence").
- ✅ Aggregate fidelity threshold at 80% emits a warning to
  stdout (not just to the file) so it surfaces in CI logs.
- ✅ Sort by score ascending (worst-first) so users see what
  needs attention immediately.

## 8. End-to-end flow now complete (13 steps)

```
1. radiant product "..."          ← Lean Inception (v0.4.4)
2. radiant spec "<feature>"       ← AC→test mapping (v0.4.2)
3. radiant run specs/<NNNN>       ← implementation (v0.3.x)
4. radiant adr "<decision>"       ← Nygard ADR (v0.4.3)
5. radiant diagramar <level>      ← C4 Mermaid (v0.4.3)
6. radiant integrations list      ← MCP discovery (v0.4.5)
7. radiant handoff --feature=...  ← session pause (v0.4.2)
8. radiant update [--force]       ← skill refresh (v0.4.3)
9. radiant views --agent=<list>   ← native agent views (v0.4.6)
10. radiant review-pr <spec>      ← PR review scaffold (v0.4.7)
11. radiant setup-ci              ← CI workflow (v0.4.8)
12. radiant camada-agentica       ← agentic layer audit (v0.4.9)
13. radiant evals                 ← AC→test coverage (v0.5.0) ← NEW
```

## 9. Future work (post-methodology-merge)

The methodology merge is feature-complete, but the CLI can keep
growing. Open work, in priority order:

1. **Wire evals + audit + camada-agentica into the CI workflow**
   (the setup-ci template currently runs them as separate steps —
   unify into a single `radiant check` that fails the build if
   any of them report below threshold).
2. **`since-last-release` scope for evals** — requires git state
   awareness (parse `git log --tags` to find the last tag).
3. **Unify AGENTS.md generation** — audit surfaced drift between
   `scaffold`'s template and `generateAgentsMD()`. Pick one as
   canonical.
4. **MCP `serve` command** — explicit `radiant mcp serve` for
   agents that prefer MCP over stdio. Deferred per HARNESS-PLAN.md.
5. **First-class release artefacts** — `radiant release v0.X.Y`
   that runs evals + audit + tests + build + cross-compile +
   git tag in one command.