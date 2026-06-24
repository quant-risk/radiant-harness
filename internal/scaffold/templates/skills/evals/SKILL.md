---
name: evals
description: Evaluate spec-to-code fidelity via AC and test coverage.
---

# Skill: Evals (Spec → Code Fidelity)

Measures how faithfully the implementation follows the spec.
Scriptable checks for AC→task→test coverage, plus SPEC_DEVIATION counting.

## Phase 1 — Collect spec inventory (Research)

1. List all features: `search_files` pattern `specs/*/spec.md` — enumerate spec directories.
2. For each spec, `read_file` and extract:
   - All `AC-N` identifiers and their Given/When/Then text.
   - The spec's "Out of scope" section (if present).
3. For each spec, `read_file` `tasks.md` — extract task list and gate commands.
4. Count total ACs across all specs — this is the coverage denominator.

> Delegate per-spec reading to subagents if there are > 5 features. Each subagent returns a structured summary.

## Phase 2 — Check AC → task coverage (Plan + Implement)

For each feature, verify every AC has a corresponding task:

```
terminal: grep -r "AC-1\|AC_1" specs/0001-*/tasks.md
```

Or use `search_files` pattern `AC.?<N>` in each `tasks.md`.

Build the coverage matrix:

| Feature | Total ACs | ACs with task | ACs with test | ACs passing | Deviations | Score |
|---------|-----------|--------------|--------------|-------------|------------|-------|
| 0001-feedback | 5 | 5 | 5 | 5 | 0 | 100% |
| 0002-export | 3 | 3 | 2 | 2 | 1 | 67% |

**Score formula:** `(ACs with passing test) / (Total ACs) × 100`

## Phase 3 — Check AC → test coverage (Plan + Implement)

1. For each `AC-N`, search test files: `search_files` pattern `AC.?<N>|test_AC_<N>` in test directories.
2. Methods:
   - Naming convention from TESTING.md: `test_AC_N_*` or `AC-N: ...`.
   - If naming convention not followed, search for Given/When/Then text snippets.
3. Flag ACs without tests as **uncovered** — score impact.

## Phase 4 — Count SPEC_DEVIATION (Plan + Implement)

1. `search_files` pattern `SPEC_DEVIATION` across `src/` — find all deviation comments.
2. For each deviation, classify:
   - **Resolved** — code fixed or spec updated + ADR exists.
   - **Open** — deviation exists without resolution.
3. Count open deviations per feature. Each open deviation reduces the fidelity score.

**Adjusted score:** `base_score - (open_deviations × 10)`, floored at 0%.

## Phase 5 — Check scope adherence (Plan)

1. For each spec, read the "Out of scope" section.
2. `search_files` in `src/` for features that shouldn't exist yet — implementations beyond scope.
3. Flag any out-of-scope implementation as a scope violation.

## Phase 6 — Generate evals report (Implement)

1. Write the per-feature scorecard:

```
## Spec → Code Fidelity Report — <date>

### Overall fidelity: <avg score>%

### Per-feature breakdown
| Feature | ACs | Task coverage | Test coverage | Pass rate | Deviations | Score |
|---------|-----|---------------|---------------|-----------|------------|-------|

### Findings
- [feature] AC-N has no test — uncovered acceptance criterion.
- [feature] 2 open SPEC_DEVIATIONs — spec and code diverge.
- [feature] Implementation includes <X> which is marked out-of-scope.

### Trends (if previous report exists)
- Coverage: <prev>% → <current>% (↑/→/↓)
- Open deviations: <prev> → <current>
```

2. Save to `docs/evals-<date>.md` or update `docs/STATE.md` with summary.
3. Present to user with prioritized remediation list.

## Rules

- **Objective scoring.** The score is computed from searchable artifacts, not opinion.
- **Naming convention matters.** If tests don't follow `test_AC_N_*`, coverage detection degrades.
  Flag naming convention violations as a finding.
- **One report per run.** Compare with previous report for trends — don't overwrite history.
- Low scores are actionable, not punitive. Every finding has a specific fix.
