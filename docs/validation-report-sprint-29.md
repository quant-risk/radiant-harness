# Sprint 29 validation — 3 niche skills + 2 CLI commands + v0.7.0

**Commit (this report):** TBD
**Re-validates:** `3be3b80` (Sprint 28 — Harness-Quant)
**Version:** **v0.7.0** (in source; tag pending Phase E)

## What shipped

### Phase A: 3 niche skills

| Skill | Domain | Lines |
|-------|--------|-------|
| `radiant-actuarial` | Life/P&C insurance, reserving, Solvency II, IFRS 17 | ~280 |
| `radiant-tax` | CIT, transfer pricing, indirect taxes, treaties, BEPS Pillar 2 | ~310 |
| `radiant-regulatory` | BCBS, BACEN, CVM, SUSEP, ANS, Fed, EBA, ESMA, EIOPA | ~290 |

### Phase B: 2 CLI commands

| Command | Purpose |
|---------|---------|
| `radiant stats <test>` | Scaffold hypothesis-test plan (H0/H1, alpha, power, effect size, multiple-testing) |
| `radiant causal-estimate <design>` | Scaffold causal analysis (DAG mermaid template, identification assumption, sensitivity, CATE) |

Both commands produce structured Markdown scaffolds that the
user fills in. They wire the `radiant-stats`, `radiant-causal`,
and `radiant-causal-ml` skills to the CLI surface.

### Phase C: v0.7.0 — major version bump

Justified by:
- 22 new domain skills in Sprint 28 (Harness-Quant)
- 3 niche skills in Sprint 29 (vertical depth)
- 2 CLI commands on top of skill foundation
- Total: 53 skills bundled (up from 28 at start of Sprints 22-28)

## Validation

| Gate | Result |
|---|---|
| `go build ./...` | clean |
| `go vet ./...` | clean |
| `gofmt -l .` | clean |
| `go test ./... -race` | 10 packages OK |
| `TestAllBundledSkillsValidateCleanly` | **53/53 skills pass** (was 50) |
| Tests | **341 PASS, 0 FAIL** (was 337, +4 new) |
| Data races | **0** |
| Cross-compile | **6/6** |

## Iteration discipline

Three issues caught + fixed at dev time:

1. **YAML colon-in-description** (`actuarial`, `tax`, `regulatory`):
   auto-fixed via Python script (~7 occurrences).
2. **YAML list-item indentation drift**: fixed manually after
   Python script missed edge cases (~10 occurrences).
3. **`%` literal in fmt.Sprintf format string**: scaffold bodies
   contained "95% CI" which vet parses as `% C` (unknown verb).
   Fix: escape as `%%` (Go format string escape).

**All caught at dev time, not at user time.** The CI guard paid
for itself again.

## Final tally (post-Sprint 29)

- **23 CLI commands** (was 21, +2: stats, causal-estimate)
- **53 bundled skills** (was 50, +3: actuarial, tax, regulatory)
- **1 open MIT schema spec** (`docs/SKILL-SCHEMA.md`)
- **1 strategic plan doc** (`docs/HARNESS-QUANT.md`)
- **341 tests passing**, 0 FAIL, 0 data races, 6/6 cross-compile
- **0 vendor-centrism, 0 hardcoded secrets, 0 global git config mutations**
- **`v0.6.0` tag** (dogfooded earlier)
- **`v0.7.0` in source** (Phase E: tag pending)