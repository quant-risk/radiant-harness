# Validation Report — Sprint 37: Token Budget & Compression

**Date:** 2026-06-26
**Sprint:** 37 of 40
**Status:** PASSED

---

## Deliverables

| File | Type | Purpose |
|------|------|---------|
| `internal/context/summarizer.go` | Go | Phase summarizer — compresses completed phase to ≤20% of original tokens |
| `internal/context/budget_profiles.go` | Go | Budget profiles (lean/standard/thorough) + per-phase estimates |
| `internal/context/sprint37_test.go` | Go tests | 18 new tests |
| `cmd/radiant/main.go` | CLI | `radiant budget estimate`, `radiant budget report`, `radiant context summarize` |

---

## Test Results

```
ok  github.com/quant-risk/radiant-harness/internal/context  0.918s  (39 tests total, 18 new)
```

### SummarizePhase (5 tests)
- `TestSummarizePhase_ReducesTokens` — summarized ≤25% of original
- `TestSummarizePhase_PreservesPhaseHeader` — "Phase summary: plan" in output
- `TestSummarizePhase_ExtractsKeyFacts` — ✓/APPROVED/blocker markers captured
- `TestSummarizePhase_TinyContent` — no panic on tiny input
- `TestSummarizePhase_EmptyContent` — graceful empty handling

### SummarizeTrace (2 tests)
- `TestSummarizeTrace_CountsEvents` — 2 ok + 1 failed = correct count
- `TestSummarizeTrace_Empty` — returns "(no trace events)"

### BudgetProfiles (6 tests)
- `TestGetProfile_Lean` — 10K tokens
- `TestGetProfile_Standard` — 50K tokens
- `TestGetProfile_Thorough` — 200K tokens
- `TestGetProfile_UnknownDefaultsToStandard` — unknown → 50K fallback
- `TestGetProfile_AllPhasesPresent` — all 5 phases have budgets in all 3 profiles
- `TestGetProfile_PhaseSumMatchesTotal` — sum of PerPhase == TotalTokens

### EstimateSpec (4 tests)
- `TestEstimateSpec_AllPhasesCovered` — discover/plan/execute/verify/persist all present
- `TestEstimateSpec_ExecuteLargerThanDiscover` — execute typical > discover typical
- `TestEstimateSpec_EmptySpecHasFallback` — 500-token floor for empty spec
- `TestFormatEstimate_ContainsAllPhases` — all phases + TOTAL in table output

---

## New CLI Commands

### `radiant budget estimate [spec-file] [--profile=lean|standard|thorough]`
```
Budget estimate — profile: standard (50000 tokens total)

Phase        Min    Typical      Max   Budget  Fits?
------------ -------- -------- -------- -------- ------
discover      500     1000     2000     3000    yes
plan         1500     3000     5000     8000    yes
execute      3000     6000    12000    25000    yes
verify       1000     2000     4000    10000    yes
persist       300      500     1000     4000    yes
------------ -------- -------- -------- -------- ------
TOTAL               13500            50000    yes
```

### `radiant budget report <run-id>`
Aggregates `TokensIn/Out` from trace JSONL per phase.

### `radiant context summarize --phase=<phase>`
Reads CONTEXT.md, applies `SummarizePhase`, writes back.
Preserves key facts (decisions, ✓/✗, blockers).

---

## Architecture

### Phase Summarization Strategy
```
Input (100% tokens) → 3-pass reduction:
  1. Extract key facts (≤8 lines with decision/outcome/blocker keywords)
  2. Strip <!-- phase:done --> completed blocks (always free)
  3. Condense body: first 40% of lines + "...(condensed)..." + last 20%
  4. Hard compress if still over budget

Target: ≤20% of original tokens, minimum 50 tokens
```

### Budget Profile Contract
Each profile guarantees: `sum(PerPhase.values) == TotalTokens`

---

## Regression

All Sprints 33–37 green:
```
ok  internal/benchmark   0.389s
ok  internal/boot        0.667s
ok  internal/context     0.918s   ← 39 tests (18 new)
ok  internal/loop        2.003s
ok  internal/policy      2.607s
ok  internal/quality     2.881s
ok  internal/scaffold    3.130s
ok  internal/skill       3.036s
ok  internal/spec        2.988s
```

Pre-existing failures: `internal/engine`, `internal/harness`, `internal/llm`
(dyld: missing LC_UUID on Darwin 25.5.0 — not caused by Sprint 37).

---

## Next: Sprint 38

Self-Improvement Engine:
- `internal/improve/` — analyze failure traces → propose skill edits → validate on held-out tasks
- `radiant improve analyze <run-id>` — extract failure patterns
- `radiant improve propose --skill=<name>` — propose instruction patches
