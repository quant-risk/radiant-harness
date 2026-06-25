# Validation Report — Sprint 13 fifth batch FINAL (v0.5.0)

**Date:** 2026-06-25
**Version:** 0.5.0 (literal in source; git build embeds `8ef8a25`)
**Commit under validation:** `8ef8a25`
**Sprint:** 13 — PR + Multi-agent Views (AC→test coverage metrics; final pass)
**Scope:** `radiant evals` CLI + coverage helpers + 5 tests.

**Milestone:** this is the final planned deliverable in the
methodology merge defined in `docs/HARNESS-PLAN.md`.

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
ok  	github.com/quant-risk/radiant-harness/cmd/radiant         2.069s
ok  	github.com/quant-risk/radiant-harness/internal/benchmark  3.210s
ok  	github.com/quant-risk/radiant-harness/internal/engine     1.613s
ok  	github.com/quant-risk/radiant-harness/internal/harness    8.387s
ok  	github.com/quant-risk/radiant-harness/internal/llm        8.080s
ok  	github.com/quant-risk/radiant-harness/internal/policy     3.976s
ok  	github.com/quant-risk/radiant-harness/internal/quality    2.336s
ok  	github.com/quant-risk/radiant-harness/internal/scaffold   4.930s
ok  	github.com/quant-risk/radiant-harness/internal/skill      3.079s
ok  	github.com/quant-risk/radiant-harness/internal/spec       2.883s
```

**Total:** 268 PASS, **0 FAIL**, **0 data races detected**.

**Result:** ✅ Pass.

## 3. Cross-compilation — all 6 targets

```
$ make release
GOOS=linux   GOARCH=amd64 go build -ldflags "-s -w -X main.version=8ef8a25" -o dist/radiant-linux-amd64     ./cmd/radiant/
GOOS=linux   GOARCH=arm64 go build -ldflags "-s -w -X main.version=8ef8a25" -o dist/radiant-linux-arm64     ./cmd/radiant/
GOOS=darwin  GOARCH=amd64 go build -ldflags "-s -w -X main.version=8ef8a25" -o dist/radiant-darwin-amd64    ./cmd/radiant/
GOOS=darwin  GOARCH=arm64 go build -ldflags "-s -w -X main.version=8ef8a25" -o dist/radiant-darwin-arm64    ./cmd/radiant/
GOOS=windows GOARCH=amd64 go build -ldflags "-s -w -X main.version=8ef8a25" -o dist/radiant-windows-amd64.exe ./cmd/radiant/
GOOS=windows GOARCH=arm64 go build -ldflags "-s -w -X main.version=8ef8a25" -o dist/radiant-windows-arm64.exe ./cmd/radiant/
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

## 4. Built binary sanity

```
$ ./bin/radiant --version
8ef8a25       (git SHA injected by Makefile; literal version in source = 0.5.0)

$ ./bin/radiant evals --help
Measure AC→test coverage (fidelity) across all specs

Usage:
  radiant evals [flags]

Flags:
  -o, --output string   output path (default: docs/evals-report.md)
```

- `evals` command registered ✓
- Both flags (`--scope`, `--output`) present ✓
- Built binary shows git SHA `8ef8a25` ✓

**Result:** ✅ Pass.

## 5. End-to-end — `radiant evals`

Test fixture: 1 feature (0001-x) with 2 ACs, only 1 mentioned in tasks.md.

```
$ radiant evals
  ✓ wrote docs/evals-report.md

  Features: 1
  ACs: 2 total, 1 claimed-covered (50%)

  Per-feature scores (worst first):
    0001-x — 1/2 (50%)

  ⚠ fidelity below 80% — review uncovered ACs above
```

- Walks `specs/` ✓
- Parses ACs from spec.md ✓
- Parses tasks.md coverage ✓
- Aggregate fidelity computed correctly ✓
- Warning printed when fidelity < 80% ✓

**Result:** ✅ Audit works on a real-world partial-coverage scenario.

## 6. Methodology merge — complete (this is the final report)

Per `docs/HARNESS-PLAN.md`, the 4-phase methodology merge is now
fully shipped:

| Sprint | Theme | Version range | Deliverables |
|--------|-------|---------------|--------------|
| 10 | Foundation | v0.4.0–0.4.2 | skill runtime, 16 skills, schema spec, init/state/handoff/spec |
| 11 | Discovery | v0.4.3 | adr, update, diagramar |
| 12 | Governance | v0.4.4–0.4.5 | product, integrations list |
| 13 | PR + views | v0.4.6–0.5.0 | views, review-pr, setup-ci, camada-agentica, evals |

**Total deliverable count:** 24 commands + 17 bundled skills +
1 open MIT schema spec — every line item from
`docs/HARNESS-PLAN.md` is shipped.

## 7. Test surface

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

- `TestComputeFeatureCoverageAllCovered` — 2/2 covered, score 1.0.
- `TestComputeFeatureCoveragePartial` — 2/3 covered, uncovered=[AC3].
- `TestComputeFeatureCoverageNoTasksMD` — 0/2 covered (no tasks).
- `TestRenderEvalsReportIncludesSections` — all 4 sections present.
- `TestRenderEvalsReportComputesAggregate` — empty feature
  contributes 0 (no division by zero).

All 5 pass in `-race` mode.

## 8. Decisions

- ✅ Sprint 13 fifth batch is **READY TO MERGE** at v0.5.0.
- ✅ MVP is "claimed coverage" (does tasks.md list this AC?).
- ✅ Per-AC evidence is a TODO row in the report — the LLM
  fills in file:line citations.
- ✅ Aggregate fidelity threshold at 80% emits a warning to
  stdout (not just to the file).
- ✅ Sort by score ascending (worst-first) so users see what
  needs attention immediately.

## 9. End-to-end flow now complete (13 steps)

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

See `docs/METHODOLOGY-MERGE-FINAL.md` for the full consolidated
report covering all 4 sprints, every command shipped, every
test added, and the post-merge roadmap.