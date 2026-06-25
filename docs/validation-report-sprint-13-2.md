# Validation Report — Sprint 13 second batch (v0.4.7)

**Date:** 2026-06-25
**Version:** 0.4.7
**Commit under validation:** (pending — this commit)
**Sprint:** 13 — PR + Multi-agent Views (PR review half)
**Scope:** `radiant review-pr` CLI + AC/gate parsers + diff stats.

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
ok  	github.com/quant-risk/radiant-harness/cmd/radiant         2.367s
ok  	github.com/quant-risk/radiant-harness/internal/benchmark  1.473s
ok  	github.com/quant-risk/radiant-harness/internal/engine     1.984s
ok  	github.com/quant-risk/radiant-harness/internal/harness    5.786s
ok  	github.com/quant-risk/radiant-harness/internal/llm        5.703s
ok  	github.com/quant-risk/radiant-harness/internal/policy     2.711s
ok  	github.com/quant-risk/radiant-harness/internal/quality    2.525s
ok  	github.com/quant-risk/radiant-harness/internal/scaffold   3.450s
ok  	github.com/quant-risk/radiant-harness/internal/skill      1.616s
ok  	github.com/quant-risk/radiant-harness/internal/spec       1.221s
```

**Total:** 254 PASS, **0 FAIL**, **0 data races detected**.

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

## 4. End-to-end — `radiant review-pr`

Test fixture: a JWT auth spec with 2 ACs and 2 gates (`true` +
`go test ./auth/...`).

### Mode 1: basic (no diff, no run-gates)

```
$ radiant review-pr specs/0007-jwt-auth
  ✓ wrote specs/0007-jwt-auth/pr-review.md
  ACs found: 2
  Gates found: 2

  Next: open specs/0007-jwt-auth/pr-review.md and fill in AC↔code semantic check (use the revisar-pr skill).
```

- ACs parsed from `### AC<n>:` headers ✓
- Gates parsed from the Gate column ✓
- Output file: `<spec-path>/pr-review.md` (default) ✓

### Mode 2: with `--diff`

```
$ radiant review-pr specs/0007-jwt-auth --diff=sample.diff
  ✓ wrote specs/0007-jwt-auth/pr-review.md
  ACs found: 2
  Gates found: 2
  Diff: 1 files, 8 lines
```

- Diff stats recorded (1 file, 8 lines) ✓
- Summary table includes diff row ✓

### Mode 3: with `--run-gates`

```
$ radiant review-pr specs/0007-jwt-auth --run-gates --output=/tmp/pr-review.md
  ✓ wrote /tmp/pr-review.md
  ACs found: 2
  Gates found: 2
  Gates executed: 1/2 passed
```

- Gate commands executed via `sh -c` ✓
- Pass/fail recorded with output excerpt ✓
- Example: `true` passes (✓ pass), `go test ./auth/...` fails because
  `go` is not in the test shell's PATH (✗ fail with output: "sh: go:
  command not found")

### Generated report structure

```markdown
# PR review: 0007-jwt-auth
## Summary
| Metric | Value |
| ACs in spec | 2 |
| Gates in tasks | 2 |
| Gates executed | 1/2 passed |
## Recommendation
- [ ] Approve
- [ ] Request changes
- [ ] Needs spec revision
## AC coverage
| AC | Title | Implemented | Notes |
| AC1 | valid login returns a JWT | TODO | TODO |
| AC2 | invalid login returns 401 | TODO | TODO |
## Gate results
| Gate | Status | Output |
| `true` | ✓ pass | (silent) |
| `go test ./auth/...` | ✗ fail | sh: go: command not found |
## SPEC_DEVIATION
[template for LLM to fill in]
## Suggested PR comment
[copy-paste ready]
```

**Result:** ✅ All 3 modes produce correct output.

## 5. Parser coverage (unit tests)

| Test | Coverage |
|------|----------|
| `TestParseAcceptanceCriteriaBasic` | 2 ACs from `### AC1:` and `### AC2:` headers, IDs uppercased, titles extracted |
| `TestParseAcceptanceCriteriaEmpty` | No `### AC` headers → 0 ACs |
| `TestParseAcceptanceCriteriaCaseInsensitive` | `### ac1:` → `AC1` (uppercased) |
| `TestParseGatesFromTasks` | 2 backticked gates from Gate column; `—` placeholder excluded |
| `TestParseGatesFromTasksEmpty` | No table → 0 gates |
| `TestCountDiffFiles` | 2 files from `diff --git` headers in a unified diff |
| `TestRenderPRReviewIncludesSections` | All 6 sections present + ACs included + revisar-pr skill reference |
| `TestRenderPRReviewGatePassFail` | ✓ pass / ✗ fail correctly rendered |
| `TestRenderPRReviewWithDiffStats` | Diff stats embedded ("3 files, 42 lines") |

All 9 pass in `-race` mode.

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
| **Sprint 13 batch 2** | **+9** | **254** |

## 7. Decisions

- ✅ Sprint 13 second batch is **READY TO MERGE** at v0.4.7.
- ✅ MVP is template-based — the semantic AC↔code check is left as
  TODO rows in the report, to be filled in by the LLM (via the
  `revisar-pr` skill). This matches the skill's design: it owns the
  semantic review; the CLI owns the reproducible scaffold.
- ✅ `--run-gates` is opt-in (default OFF) because gate commands
  may have side effects. CI runs the command with `--run-gates`
  and bails on any ✗ fail; local dev runs without.
- ✅ `--diff` is opt-in (default OFF) — the report is useful even
  without a diff (just ACs + gates status).

## 8. Iteration discipline recap

First build attempt had a type mismatch: the `runReviewPR` body
declared an anonymous struct `{Name, Passed, Err}` while
`renderPRReview` accepted the same fields in a named `gateResult`
type. Compile error caught by `go build`:
> cannot use results (variable of type []gateResult) as []struct
> {Name string; Passed bool; Err string} value in argument to
> renderPRReview

Fixed by:
1. Promoting the anonymous struct to a named `gateResult` type
   at file scope.
2. Removing the duplicate inline declaration in `runReviewPR`.

The lesson: **prefer named types for any struct that crosses
function boundaries** — Go's structural typing doesn't help when
two anonymous structs have identical fields.

## 9. What Sprint 13 will continue to tackle

| ID | Deliverable | Skill | Notes |
|---|---|---|---|
| 13.3 | `radiant setup-ci` | `setup-ci` | GitHub Actions / GitLab CI / CircleCI scaffold. |
| 13.4 | `radiant camada-agentica` | `camada-agentica` | Generate the agentic layer config. |
| 13.5 | `radiant evals` | `evals` | Run AC→test coverage metrics. |

After Sprint 13, the radiant CLI is feature-complete against the
original HARNESS-PLAN.md scope.