# Sprint 74 — helpers.go extractions (security + scaffolds) (v2.44.0)

> **Status**: In progress
> **Branch**: `feature/light-full-release` (continuation)
> **Target version**: v2.44.0
> **Estimated scope**: 1 sprint focado (hygiene — debt reduction)

---

## Motivation

`cmd/radiant/helpers.go` is currently **3894 lines**, the largest
file in the repository by an order of magnitude. Most of its bulk
is unrelated domain logic that just happens to share a file:

- Security audit (`runSecurity`, `scanSecrets`, `scanPerms`,
  `renderSecurityReport`) — ~300 LOC
- ML scaffolds (`runStatsScaffold`, `runCausalEstimateScaffold`,
  `runModelScaffold`, `runPredictScaffold`, `runTrainScaffold`,
  `runEvaluateScaffold`, `runDriftScaffold`, `runProfileScaffold`) —
  ~600 LOC
- PR review (`runReviewPR`, `parseAcceptanceCriteria`,
  `parseGatesFromTasks`, `countDiffFiles`, `renderPRReview`) — ~400 LOC
- Telemetry (already extracted to `cmd_telemetry.go` in earlier sprint)
- Integrations (`runIntegrationsList`, `renderIntegrationsDoc`) — ~150 LOC
- Incidents (`runIncident`, `renderIncidentDoc`) — ~150 LOC
- Product inception (`renderInception`, `renderPersonasTemplate`) — ~150 LOC
- ADR (`nextADRSequence`, `renderADR`) — ~100 LOC
- Evals (`runEvals`, `computeFeatureCoverage`, `renderEvalsReport`) — ~250 LOC

Plus the Sprint 67 MCP plumbing — `runMCPServe`,
`handleMCPRequest`, `mcpRunFull`, `mcpRunHTTP`, `mcpRunWithBackend`,
`callMCPTool` — ~900 LOC.

Sprint 74 starts the cleanup. Two extractions this sprint:

1. **security.go** (~300 LOC) — pulls `runSecurity` and helpers out
   of `helpers.go` into a themed file alongside the existing
   `cmd_audit.go` (which currently inlines the security command
   registration).
2. **scaffolds.go** (~600 LOC) — pulls the eight `run*Scaffold`
   functions out of `helpers.go` into a new file alongside the
   existing `cmd_spec.go` family.

Both extractions preserve the existing public API
(`runSecurity`, `runModelScaffold`, etc.) so callers in other
files don't change. The move is internal — same package, same
function names.

Future sprints tackle PR review, telemetry, integrations, etc.

---

## Goals

| # | Goal | Acceptance |
|---|------|------------|
| G1 | `cmd_security.go` exists with `runSecurity`, `scanSecrets`, `scanPerms`, `renderSecurityReport`, `securityFinding` | All functions moved from helpers.go |
| G2 | `cmd_security.go` has `registerSecurityCmd(root *cobra.Command)` | Function added; cmd_audit.go's inline registration removed |
| G3 | `radiant security` produces identical output to v2.43.0 | Manual smoke test (with `--dry-run` or fixture) |
| G4 | `cmd_scaffolds.go` exists with the eight `run*Scaffold` functions + helpers | All functions moved |
| G5 | `helpers.go` drops from 3894 → ~3000 lines | Diffstat confirms reduction |
| G6 | All tests still green | `go test -count=1 ./...` 997 PASS |
| G7 | Cross-compile 3/3 OK | linux/amd64, darwin/arm64, windows/amd64 |
| G8 | `radiant security` and one `radiant model` / `radiant train` smoke run | Manual smoke test |

### Out of scope (Sprint 75+)

- PR review extraction (`runReviewPR` + helpers, ~400 LOC)
- Integrations extraction (`runIntegrationsList`, ~150 LOC)
- Incident extraction (`runIncident`, ~150 LOC)
- Product inception extraction (`renderInception`, `renderPersonasTemplate`)
- ADR extraction (`nextADRSequence`, `renderADR`)
- Evals extraction (`runEvals`, `computeFeatureCoverage`, `renderEvalsReport`)
- MCP plumbing extraction (`runMCPServe` etc., ~900 LOC) — biggest of all, deserves its own sprint

---

## Files

| File | Change | LOC est. |
|------|--------|----------|
| `cmd/radiant/cmd_security.go` | NEW — `runSecurity`, `scanSecrets`, `scanPerms`, `renderSecurityReport`, `securityFinding`, `registerSecurityCmd` | 320 |
| `cmd/radiant/cmd_scaffolds.go` | NEW — eight `run*Scaffold` functions + `runStatsScaffold`, `runCausalEstimateScaffold`, helpers | 600 |
| `cmd/radiant/cmd_audit.go` | MODIFIED — remove inline security block (now in cmd_security.go) | −25 |
| `cmd/radiant/helpers.go` | MODIFIED — remove moved functions | −920 |
| `docs/SPRINT74-PLAN.md` | NEW — this file | 200 |
| `CHANGELOG.md` | MODIFIED — v2.44.0 entry | +50 |
| `RELEASE-NOTES.md` | MODIFIED — v2.44.0 entry | +40 |

**Total estimate: ~265 LOC net** (gain ~960 from new files, lose ~920 from helpers.go, gain ~225 in docs).

---

## Test strategy

The moved functions don't change their public signatures or
behaviour — they just change physical location. Existing tests
should pass unchanged:

- `internal/quality` tests exercise secret scanning patterns.
- No tests exist for the scaffold functions directly (they're
  integration-style CLI commands); smoke-tested manually.

This sprint adds **one test** in `cmd_security_test.go` to verify
the file compiles and registers cleanly. The bulk of validation
is "tests still pass + smoke tests of CLI behaviour".

If a function's behaviour was untested, extracting it doesn't
change that. Future sprints add tests for the scaffolds.

---

## Risks

| Risk | Mitigation |
|------|------------|
| Function has hidden cross-file dependency (a private helper from helpers.go) | Extract all referenced helpers together; smoke-test the CLI command after |
| Move breaks the cobra registration ordering | `registerSecurityCmd` is invoked in `cmd_audit.go` — preserve the call order |
| `helpers.go` import dependencies change | All moved functions use only stdlib + cobra — same imports |

---

## Commit plan

Single commit on `feature/light-full-release`:

```
refactor(cmd): Sprint 74 — extract security + scaffolds from helpers.go (v2.44.0)
```

Pass criteria: `go vet ./...` clean, `go test -count=1 -v ./...`
green (997+ tests), `helpers.go` < 3100 lines, cross-compile 3/3
platforms, `radiant security` and `radiant model` smoke tests pass.

---

**Status at plan write**: Sprint 73 (v2.43.0) committed at `ed41ffb`
+ validation report `464ecb3`. Sprint 74 implementation in progress.