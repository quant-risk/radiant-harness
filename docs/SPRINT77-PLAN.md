# Sprint 77 — v2.47.0 — `helpers.go` extraction: PR review block

## Goal

Continue the debt-reduction rhythm (Sprints 74 + 76). Extract the
PR-review block from `cmd/radiant/helpers.go` (2948 LOC) into a
new themed file `cmd/radiant/cmd_pr_review.go`.

## Current state

`cmd/radiant/helpers.go` is at **2948 LOC** (was 3894 after Sprint
74 trimmed out security + scaffolds). The PR review block is one
of the largest single-purpose themes remaining:

```
cmd/radiant/helpers.go  (2948 LOC total)
├── cmd scaffolding helpers        (resolveAgents, slugify, ...)
├── spec / tasks helpers           (renderSpecMD, renderTasksMD, ...)
├── evals helpers                  (runEvals, computeFeatureCoverage, ...)
├── MCP serve / dispatch           (runMCPServe, handleMCPRequest, ...)
├── incident helpers               (runIncident, renderIncidentDoc, ...)
├── telemetry helpers              (runTelemetryRotate, ...)
├── MCP run-* helpers              (mcpRunFull, mcpRunHTTP, ...)
├── ★ PR review block ★            (~290 LOC, contiguous)
│   ├── runReviewPR               (1747)
│   ├── parseAcceptanceCriteria   (1832)
│   ├── parseGatesFromTasks       (1862)
│   ├── countDiffFiles            (1891)
│   └── renderPRReview            (1898)
├── integrations helpers           (runIntegrationsList, renderIntegrationsDoc, ...)
├── inception / personas / ADR     (renderInception, renderPersonasTemplate, renderADR, ...)
├── autodata helpers               (runAutodata, parseAutodataResponse, ...)
├── model resolve helpers          (resolveModelSilent, resolveModel, ...)
├── runDoctor                      (~115 LOC)
├── runEval                        (single function, large)
└── ... misc
```

## Target state

```
cmd/radiant/helpers.go              ~2658 LOC  (−290, −10%)
cmd/radiant/cmd_pr_review.go        ~290 LOC   (NEW)
```

The block to extract contains:

- **2 types**: `gateResult`, `acceptanceCriterion`
- **5 functions**:
  - `runReviewPR(specPath, diffPath, runGates, outPath string) error`
  - `parseAcceptanceCriteria(specMD string) []acceptanceCriterion`
  - `parseGatesFromTasks(tasksMD string) []string`
  - `countDiffFiles(diff string) int`
  - `renderPRReview(slug string, acs []acceptanceCriterion, ...) string`

The block is self-contained: it parses spec/tasks markdown, optionally
runs gates, and produces `pr-review.md`. It depends on stdlib only.

## Why this candidate

- **Biggest single-themed block** in helpers.go (besides MCP run-* and
  telemetry, which both span many subcommands and would be over-eager
  to split).
- **Self-contained**: doesn't depend on helper-only types defined
  elsewhere.
- **9 tests already exist** in `cmd/radiant/main_test.go` for the
  extracted functions (`TestParseAcceptanceCriteria*`,
  `TestParseGatesFromTasks*`, `TestCountDiffFiles`,
  `TestRenderPRReview*`). Zero test edits required — same names,
  same package.
- **Caller stays put**: `cmd_spec.go:364` calls `runReviewPR(...)`;
  no caller edits needed (same package, same identifiers).

## Caller unchanged

`cmd_spec.go` registers the `review-pr` subcommand:
```go
cmd := &cobra.Command{
    Use:   "review-pr <spec-path>",
    ...
    RunE: func(cmd *cobra.Command, args []string) error {
        ...
        return runReviewPR(args[0], diffPath, runGates, out)
    },
}
```

After extraction, `runReviewPR` lives in `cmd_pr_review.go`. The
call site doesn't change — same package, same identifier, just a
different file. This is identical to the Sprint 74 pattern with
`runStatsScaffold` (extracted to `cmd_scaffolds.go`, called from
`cmd_spec.go`).

## Files

- `cmd/radiant/helpers.go` — trim from 2948 → ~2658 LOC
- `cmd/radiant/cmd_pr_review.go` — NEW, ~290 LOC
- `cmd/radiant/main.go` — version bump (`2.46.0` → `2.47.0`)
- `CHANGELOG.md` — v2.47.0 entry
- `RELEASE-NOTES.md` — v2.47.0 notes

## What's NOT in this sprint

- **No new features.** Pure refactor — same behaviour, same callers.
- **No test additions.** All 9 existing tests stay unmodified;
  total test count is unchanged.
- **No `cmd_spec.go` edits.** The `review-pr` subcommand registration
  stays where it is.
- **No other extractions.** Two more candidates left for future
  sprints (integrations ~75 LOC, incident ~150 LOC, autodata ~225
  LOC, runDoctor ~115 LOC, evals ~225 LOC).
