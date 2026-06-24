# Skill: metricas

> Delivery maturity: Lead Time, Throughput, AC coverage, gate
> pass rate, deviation rate. Blameless tone.

## Decision tree

```
Metrics requested (window: 30d|90d|sprint|all)
        │
        ▼
Gather data:
        - spec.md created_at timestamps
        - last commit hash on each feature
        - gate pass/fail history
        - SPEC_DEVIATION entries
        │
        ▼
≥5 features completed? ──── no ──► "Insufficient data; report skipped"
        │
        ▼ yes
Compute:
        - Lead Time = merge_time - spec_creation_time
        - Throughput = features / sprint
        - AC coverage = green_acs / total_acs
        - Gate pass rate = green_gates / total_gates
        - Deviation rate = deviations / features
        │
        ▼
Maturity score (1-5) per CD conventions
        │
        ▼
Write report
```

## Workflow

### Step 1: gather data

Walk `specs/` and `.radiant-harness/`. Extract:
- `spec.md` first commit timestamp
- Last commit timestamp on the spec directory
- Gate pass/fail from validation reports
- SPEC_DEVIATION counts from spec.md

### Step 2: compute metrics

Standard formulas:

```text
Lead Time (median) = median(merge_time - spec_creation_time)
Throughput = features_completed / sprints_in_window
AC Coverage = ACs_with_green_gate / total_ACs
Gate Pass Rate = green_gates / total_gates
Deviation Rate = deviation_entries / features_completed
```

### Step 3: maturity score (CD-inspired)

| Score | Criteria |
|-------|----------|
| 1 | <50% AC coverage, no tests, frequent deviations |
| 2 | Tests exist, low coverage, Lead Time >2 weeks |
| 3 | Tests cover most ACs, Lead Time 1-2 weeks |
| 4 | High coverage, Lead Time <1 week, low deviations |
| 5 | Continuous delivery: <1 day Lead Time, >90% coverage, near-zero deviations |

### Step 4: write the report

Blameless tone. No individual names. Focus on systems.

## Examples

### Example 1: 90-day window

**Output**: report shows Lead Time median 8 days, Throughput 6
features/sprint, AC coverage 87%, Gate pass rate 92%, Deviation
rate 0.3 per feature. Maturity score 3.

## Anti-patterns

- ❌ Naming individuals. Blameless.
- ❌ Acting on <5 features. Too noisy.
- ❌ Conflating velocity with quality.

## Failure modes

| Gate | Failure | Recovery |
|------|---------|----------|
| `data-sufficient` | <5 features | Skip the report. Suggest waiting. |
| `blameless-tone` | Names appear | Rewrite; replace with "the team", "this sprint", etc. |

## Related skills

- `auditar` — provides data for metrics
- `validar` — per-feature gate results