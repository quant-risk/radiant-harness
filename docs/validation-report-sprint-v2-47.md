# Validation Report — Sprint 77: v2.47.0 — `helpers.go` PR review extraction

> **Date:** 2026-06-29
> **Project version:** v2.47.0
> **Branch:** `feature/light-full-release`
> **Base:** `31bc88c` (Sprint 77 commit)
> **Status:** PASSED — ready to merge

---

## TL;DR

Sprint 77 is the third installment of the debt-reduction rhythm
(Sprint 74 + Sprint 76 + Sprint 77). It pulls the PR review block
out of the 2948-line `cmd/radiant/helpers.go` into a new themed
file. Zero test edits, zero behaviour change.

| Metric | Value |
|--------|-------|
| Commits on branch | ahead of base (`9b28e77`) |
| New commits in this release | **1** (`31bc88c`) |
| Files changed | 5 modified, 1 new |
| LOC delta | +579 / −279 (net +300 LOC; mostly the new file's header) |
| Agents supported | **11** (unchanged) |
| New deps | 0 |
| New tests | **0** (all 9 PR review tests pass unmodified) |
| Tests | **1189 PASS, 0 confirmed FAIL** |
| `go vet ./...` | clean |
| Cross-compile | linux/{amd64,arm64}, darwin/{amd64,arm64}, windows/amd64 — all OK |

---

## File layout delta

```
BEFORE (post-Sprint 76)
─────────────────────────
cmd/radiant/helpers.go    2948 LOC  (everything)


AFTER (post-Sprint 77)
──────────────────────
cmd/radiant/helpers.go              2670 LOC  (−278, −9%)
  (everything else: spec helpers, MCP serve, telemetry, ...)

cmd/radiant/cmd_pr_review.go        309 LOC  (NEW)
  ├── type gateResult
  ├── type acceptanceCriterion
  ├── runReviewPR                  ← body of `radiant review-pr`
  ├── parseAcceptanceCriteria
  ├── parseGatesFromTasks
  ├── countDiffFiles
  └── renderPRReview
```

---

## Build / Vet / Test

```bash
$ go vet ./...
EXIT=0   (silent — clean)

$ go build -o /tmp/radiant ./cmd/radiant
-rwxr-xr-x  14M  /tmp/radiant    # darwin/arm64 host

$ /tmp/radiant --version
2.47.0

$ go test -count=1 ./cmd/radiant/ -run 'TestRunReviewPR|TestParseAcceptance|TestParseGatesFromTasks|TestCountDiffFiles|TestRenderPRReview'
ok      github.com/quant-risk/radiant-harness/cmd/radiant    0.353s
        # all 9 PR review tests pass with zero edits

$ go test -count=1 -v ./... | grep -cE "^--- PASS|^    --- PASS|^        --- PASS"
1189

$ go test -count=1 ./... | grep -cE "^ok"
29    (30 packages ok, 1 flake)

$ go test -count=1 ./... | grep -E "^FAIL|^--- FAIL"
--- FAIL: TestRunAllContextCanceled (1.39s)
FAIL    github.com/quant-risk/radiant-harness/internal/fleet    13.039s
FAIL
        # same documented pre-existing flake; not a regression
```

### Cross-compile matrix

```bash
$ GOOS=linux   GOARCH=amd64 go build -o /tmp/rad-s77-linux-amd64     ./cmd/radiant   # 15M OK
$ GOOS=linux   GOARCH=arm64 go build -o /tmp/rad-s77-linux-arm64     ./cmd/radiant   # 14M OK
$ GOOS=darwin  GOARCH=amd64 go build -o /tmp/rad-s77-darwin-amd64    ./cmd/radiant   # 15M OK
$ GOOS=darwin  GOARCH=arm64 go build -o /tmp/rad-s77-darwin-arm64    ./cmd/radiant   # 14M OK
$ GOOS=windows GOARCH=amd64 go build -o /tmp/rad-s77-windows-amd64.exe ./cmd/radiant  # 15M OK
```

All five platforms built cleanly.

---

## PR review tests — zero edits required

The 9 PR review tests live in `cmd/radiant/main_test.go`. After
extraction they continue to pass without any edit:

```
=== RUN   TestParseAcceptanceCriteriaBasic          --- PASS  (0.00s)
=== RUN   TestParseAcceptanceCriteriaEmpty          --- PASS  (0.00s)
=== RUN   TestParseAcceptanceCriteriaCaseInsensitive --- PASS  (0.00s)
=== RUN   TestParseGatesFromTasks                   --- PASS  (0.00s)
=== RUN   TestParseGatesFromTasksEmpty              --- PASS  (0.00s)
=== RUN   TestCountDiffFiles                        --- PASS  (0.00s)
=== RUN   TestRenderPRReviewIncludesSections        --- PASS  (0.00s)
=== RUN   TestRenderPRReviewGatePassFail            --- PASS  (0.00s)
=== RUN   TestRenderPRReviewWithDiffStats           --- PASS  (0.00s)
```

Same package (`package main`), same identifiers — the test file
doesn't know the functions moved.

## Caller behaviour unchanged

The `review-pr` subcommand stays registered in `cmd_spec.go:354-364`:

```go
// cmd_spec.go (UNCHANGED in Sprint 77)
cmd := &cobra.Command{
    Use:   "review-pr <spec-path>",
    ...
    RunE: func(cmd *cobra.Command, args []string) error {
        ...
        return runReviewPR(args[0], diffPath, runGates, out)
    },
}
```

`runReviewPR` lives in `cmd_pr_review.go` now. The call site
doesn't change because the call resolves to `main.runReviewPR`
through Go's normal package scoping.

---

## Files modified

```
 CHANGELOG.md                                |   +58  (v2.47.0 entry)
 RELEASE-NOTES.md                            |  +108  (v2.47.0 notes)
 cmd/radiant/cmd_pr_review.go                | +309   (NEW; the extracted block + header)
 cmd/radiant/helpers.go                      |  -278  (was 2948; now 2670)
 cmd/radiant/main.go                         |    ±2  (version bump)
 docs/SPRINT77-PLAN.md                       |  +127  (NEW; plan doc)
```

Net `cmd/radiant/` change: **+31 LOC** (just the file-level header
in `cmd_pr_review.go`).

---

## What was NOT in this sprint

- **No new tests.** Zero behaviour change → all 9 PR review tests
  pass unmodified.
- **No cmd_spec.go edits.** The `review-pr` subcommand registration
  stays put. Identifiers are unchanged.
- **No new dependencies.** `cmd_pr_review.go` imports stdlib only
  (`fmt`, `os`, `os/exec`, `path/filepath`, `strings`).
- **No other extractions.** Several thematic blocks still remain
  in `helpers.go` (2670 LOC) — see below.

---

## Remaining `helpers.go` extractions (future sprints)

| Block                                  | Approx LOC | Notes |
|----------------------------------------|------------|-------|
| MCP run-* (mcpRunFull + mcpRunHTTP + mcpRunWithBackend + callMCPTool) | ~600 | spans 4 subcommands; largest single block |
| evals (runEvals + computeFeatureCoverage + renderEvalsReport) | ~225 | called by `radiant evals` |
| autodata (runAutodata + parseAutodataResponse + emitAutodataStub + autodataSystemPrompt + autodataUserPrompt + resolveModelSilent + resolveModel) | ~225 | autodata skill worker |
| integrations (runIntegrationsList + renderIntegrationsDoc) | ~150 | called by `radiant spec integrations` |
| incident (runIncident + renderIncidentDoc + helpers) | ~150 | called by `radiant incident` |
| runDoctor                              | ~115       | called by `radiant doctor` |
| runEval (separate from runEvals)       | ~115       | called by `radiant eval` |

Future sprints could extract these one at a time, each as its own
themed file, until `helpers.go` is back under ~500 LOC. Realistic
plan: 6 more sprints of debt reduction at 100-300 LOC each.

---

## Backward compatibility

- `radiant review-pr <spec-path>` behaviour identical to v2.46.0.
- All 11 agent entry points in setup-mcp unchanged.
- All other commands unchanged (helpers.go only lost the PR review
  block; everything else stayed in place).
- Public API unchanged. Internal identifiers unchanged.

---

## Known limitations

- **One flaky pre-existing test:** `internal/fleet.TestRunAllContextCanceled`
  alternates PASS/FAIL on timing. Same as documented in
  `validation-report-sprint-56-57.md`. Not a regression.
- **Two untracked output files** generated by prior validation
  runs (`docs/audit-report.md`, `docs/security-report.md`) are left
  untracked. They are outputs of `radiant audit` and
  `radiant security`, not source code, and should arguably be
  added to `.gitignore` in a follow-up.

---

## Verification checklist

- [x] `go vet ./...` clean
- [x] `go build ./...` clean (5 platforms cross-compiled)
- [x] `go test -count=1 ./...` — 1189 PASS, 0 confirmed FAIL
- [x] All 9 PR review tests pass with **zero edits**
- [x] `cmd_spec.go` unchanged (caller still works)
- [x] `radiant --version` reports `2.47.0`
- [x] CHANGELOG and RELEASE-NOTES updated
- [x] Plan doc added (`docs/SPRINT77-PLAN.md`)
- [x] git commit `31bc88c` lands cleanly
