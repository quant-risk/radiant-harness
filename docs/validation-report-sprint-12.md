# Validation Report — Sprint 12 first batch (v0.4.4)

**Date:** 2026-06-25
**Version:** 0.4.4
**Commit under validation:** (pending — this commit)
**Sprint:** 12 — Governance Phase opening
**Scope:** `nova-product` skill (new, 17th bundled skill) + `radiant product` CLI.

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
?   	github.com/quant-risk/radiant-harness/internal/scaffold [no test files]
ok  	github.com/quant-risk/radiant-harness/cmd/radiant         1.739s
ok  	github.com/quant-risk/radiant-harness/internal/benchmark  1.292s
ok  	github.com/quant-risk/radiant-harness/internal/engine     1.933s
ok  	github.com/quant-risk/radiant-harness/internal/harness    7.783s
ok  	github.com/quant-risk/radiant-harness/internal/llm        6.520s
ok  	github.com/quant-risk/radiant-harness/internal/policy     2.645s
ok  	github.com/quant-risk/radiant-harness/internal/quality    2.471s
ok  	github.com/quant-risk/radiant-harness/internal/skill      2.944s
ok  	github.com/quant-risk/radiant-harness/internal/spec       1.747s
```

**Total:** 235 PASS, **0 FAIL**, **0 data races detected**.

**Result:** ✅ Pass.

## 3. Cross-compilation — all 6 targets

```
$ make release
GOOS=linux   GOARCH=amd64 go build -ldflags "-s -w -X main.version=c3cf7ef-dirty" -o dist/radiant-linux-amd64     ./cmd/radiant/
GOOS=linux   GOARCH=arm64 go build -ldflags "-s -w -X main.version=c3cf7ef-dirty" -o dist/radiant-linux-arm64     ./cmd/radiant/
GOOS=darwin  GOARCH=amd64 go build -ldflags "-s -w -X main.version=c3cf7ef-dirty" -o dist/radiant-darwin-amd64    ./cmd/radiant/
GOOS=darwin  GOARCH=arm64 go build -ldflags "-s -w -X main.version=c3cf7ef-dirty" -o dist/radiant-darwin-arm64    ./cmd/radiant/
GOOS=windows GOARCH=amd64 go build -ldflags "-s -w -X main.version=c3cf7ef-dirty" -o dist/radiant-windows-amd64.exe ./cmd/radiant/
GOOS=windows GOARCH=arm64 go build -ldflags "-s -w -X main.version=c3cf7ef-dirty" -o dist/radiant-windows-arm64.exe ./cmd/radiant/
✓ Release binaries in dist/

$ ls dist/ | wc -l
6
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

## 4. End-to-end — `radiant product`

```
$ ./bin/radiant product "Internal SLA dashboard for support team" --mvp-weeks=4
  ✓ created docs/product/inception.md
  ✓ created docs/product/personas.md

  Next steps (Lean Inception phases — work them in order):
    1. Why   — persona + job-to-be-done + alternative
    2. What  — brainstorm features (untagged)
    3. Who   — fill personas.md (2-4 personas)
    4. How   — technical / business approach (1-2 paragraphs)
    5. When  — Q1 MVP / Q2 Growth / Q3+ Vision
    6. Where — bounded contexts (new vs existing)
    7. Cut the MVP (3-7 features max) and run `radiant spec <feature>` per MVP item.

  MVP target: 4 weeks.

$ grep "^## " docs/product/inception.md
## 1. Why
## 2. What (untagged brainstorm)
## 3. Scope triage
## 4. Who (personas)
## 5. How
## 6. When
## 7. Where (bounded contexts)
## MVP cut

$ grep "MVP target\|**4 weeks**" docs/product/inception.md
Target MVP timeline: **4 weeks**.
```

- All 7 sections + the MVP cut heading present.
- `--mvp-weeks=4` correctly propagated into the When phase.
- Both files written atomically (temp + rename).

**Result:** ✅ Pass.

## 5. New skill validation

```
$ go test ./internal/skill/ -count=1 -run TestAllBundledSkillsValidateCleanly -v 2>&1 | tail -5
--- PASS: TestAllBundledSkillsValidateCleanly/roadmap (0.00s)
--- PASS: TestAllBundledSkillsValidateCleanly/setup-ci (0.00s)
--- PASS: TestAllBundledSkillsValidateCleanly/validar (0.00s)
PASS
```

The `nova-product` skill passes all 10 schema rules:

1. Name pattern ✓
2. Semver version ✓
3. Tier eligibility closed set ✓
4. Input uniqueness + valid types ✓ (after `int` → `number` fix)
5. Output uniqueness + valid types ✓
6. Gate uniqueness ✓
7. Required fields present ✓
8. SKILL.md non-empty ✓
9. Name matches directory ✓
10. SKILL.md sections (Decision tree, Workflow, Examples, Anti-patterns, Failure modes, Related skills) ✓

**Result:** ✅ Pass — 17 skills bundled and validated cleanly.

## 6. Test surface — what Sprint 12 added

| Test file | Tests added | Coverage |
|---|---|---|
| `cmd/radiant/main_test.go` | +5 | renderInception (4) + renderPersonasTemplate (1) |
| **Total** | **+5** | Sprint 12 first batch: 235 PASS (was 230) |

All new tests follow the existing pattern: pure-function tests,
no filesystem side effects, no LLM calls.

## 7. Iteration discipline

The first iteration of `nova-product` failed validation — the
`mvp_weeks` input was typed `int`, which is not in the schema's
allowed set (`string | number | enum | object | path`). The CI
guard (`TestAllBundledSkillsValidateCleanly`) caught it before
the binary was ever built. One round-trip fix:

```yaml
- name: mvp_weeks
- type: int          # before
+ type: number       # after (allowed by schema rule 4)
```

This is exactly the CI guard working as designed — never ship a
binary with broken skills.

## 8. No regressions

All 230 prior tests still pass. No prior command behaviour
changed. `nova-product` is purely additive (new skill + new
command + 2 new helpers).

## 9. Decisions

- ✅ Sprint 12 first batch is **READY TO MERGE** at v0.4.4.
- ✅ No follow-up fixes required.
- ✅ The `mvp_weeks` default of 8 weeks is documented in the
  nova-product skill (`description: "Default: 8"`) and in the
  CLI flag default (`Flags().Int("mvp-weeks", 8, ...)`).

## 10. What Sprint 12 will continue to tackle

Per `docs/ROADMAP.md` and the methodology-merge plan:

| ID | Deliverable | Skill | Status |
|---|---|---|---|
| 12.2 | `radiant integrations list` | `integracoes` | Next (skill exists, CLI hook pending) |
| 12.3 | `--brownfield` flag for `kickoff` | `kickoff` | Next (LLM-driven detection of existing stack) |
| 12.4 | `radiant mapear` (C4 Level 1 from codebase) | `mapear` | Next (auto-extract modules + deps) |
| 12.5 | `radiant audit` (project conformity) | `auditar` | Next |
| 12.6 | `radiant metrics` (AC→test coverage) | `metricas` | Next |

This unblocks **Sprint 13**: PR review + native views
auto-generation per agent. The complete governance flow becomes:
`radiant product` (vision) → `radiant spec` (feature) →
`radiant run` (implementation) → `radiant adr` (decisions) →
`radiant diagramar` (visuals) → `radiant handoff` (pause) →
`radiant update` (refresh) → `radiant review-pr` (review).