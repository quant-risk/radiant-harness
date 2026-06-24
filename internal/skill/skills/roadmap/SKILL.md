# Skill: roadmap

> Sequence features by value × effort, dependency graph,
> quarterly milestones.

## Decision tree

```
Roadmap requested (initial | revise)
        │
        ▼
Gather candidate features (specs/_templates/ + recent
nova-feature invocations)
        │
        ▼
For each feature: estimate value (H/M/L) and effort (S/M/L)
        │
        ▼
Map dependencies (A blocks B?)
        │
        ▼
Sequence: high value + low effort first; dependencies first
        │
        ▼
Group by quarter (Q1, Q2, Q3, Q4)
        │
        ▼
Write docs/product/roadmap.md
```

## Workflow

### Step 1: gather candidates

Read `specs/_templates/` for the next-up features. Also check
`.radiant-harness/state.md` for the in-flight feature.

### Step 2: estimate value

For each candidate, ask: "If we shipped ONLY this, would the
customer notice?" High = yes, M = maybe, L = no.

### Step 3: estimate effort

T-shirt size: S (≤1 sprint), M (1-2 sprints), L (3+ sprints).
Default to the larger estimate; optimism kills roadmaps.

### Step 4: map dependencies

For each pair of candidates, ask: "Does A block B?" Build the
dependency graph.

### Step 5: sequence

Sort by:
1. No blockers go first
2. Among unblocked, high value + low effort first

### Step 6: write the roadmap

```markdown
# Roadmap

## Q1 (next)
- [ ] 0001-jwt-auth (value: H, effort: S)
- [ ] 0002-search (value: H, effort: M, depends on 0001)

## Q2
- [ ] 0003-billing (value: H, effort: L, depends on 0001, 0002)

## Q3
- [ ] 0004-refund-flow (value: M, effort: M)

## Q4
- [ ] 0005-multi-tenant (value: M, effort: L)

## Risk register
- 0005 multi-tenant: schema migrations across many tables — estimate may slip
```

## Examples

See `examples/` directory in the bundled skill for worked examples
covering common audit/eval/PR/roadmap scenarios.

## Anti-patterns

- ❌ Listing without ordering.
- ❌ Optimistic estimates.
- ❌ Ignoring dependencies.

## Failure modes

| Gate | Failure | Recovery |
|------|---------|----------|
| `sequenced` | Just a list | Sort by value × effort. |
| `estimated` | No estimates | Ask the team; use t-shirt sizes. |
| `dependencies-mapped` | Implicit | Make explicit. |

## Related skills

- `kickoff` — produces initial roadmap
- `nova-feature` — produces features for the roadmap