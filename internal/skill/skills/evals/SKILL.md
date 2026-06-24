# Skill: evals

> Spec→code fidelity. Numeric score + breakdown by feature.
> Always cite evidence.

## Decision tree

```
Eval requested (scope: all|since-last-release|<spec-path>)
        │
        ▼
For each feature in scope:
        │
        ├── Read spec.md ACs
        ├── Read tasks.md coverage
        ├── Read code (link to file:line per claim)
        │
        ▼
Per AC:
        ├── covered (test exists + passes)
        ├── not-covered (no test, or test doesn't cover this AC)
        └── deviated (code does something different from AC)
        │
        ▼
Compute score: covered / total
        │
        ▼
Report with file:line evidence
```

## Workflow

### Step 1: enumerate features

Walk `specs/` and extract every `spec.md` in scope.

### Step 2: per-AC trace

For each AC:
1. Find the task that covers it (tasks.md's Coverage column)
2. Find the test file that implements it
3. Verify the test runs (or accept that it ran in the last
   validation report)
4. Compare the test's assertion against the AC's Given/When/Then

### Step 3: deviation scan

For each implementation file mentioned in the AC, check whether
the code matches the AC's behavior. If it diverges, mark as
deviated.

### Step 4: write the report

```markdown
# Evals: <scope>

## Summary

| Feature | ACs | Covered | Not Covered | Deviated | Score |
|---------|-----|---------|-------------|----------|-------|
| 0001    | 5   | 4       | 1           | 0        | 80%   |
| 0002    | 8   | 7       | 0           | 1        | 87%   |

## Detail

### 0001-jwt-auth

- AC1 ✓: tasks/2/test_login_returns_jwt.go:14
- AC2 ✓: tasks/2/test_invalid_login.go:8
- AC3 ✗ not-covered: no test for expired token rejection
- AC4 ✓: tasks/3/test_tampered_jwt.go:12

### 0002-search

- AC1 ✓: ...
- DEV-001: AC3 says p95 < 200ms; code only handles 10 users
  (search/handler.go:47)
```

## Examples

See `examples/` directory in the bundled skill for worked examples
covering common audit/eval/PR/roadmap scenarios.

## Anti-patterns

- ❌ Scores without evidence. Numbers without citations = PR.
- ❌ Run once, stop. Fidelity drifts.
- ❌ Confusing test count with coverage.

## Failure modes

| Gate | Failure | Recovery |
|------|---------|----------|
| `data-sufficient` | <3 features | Skip; suggest waiting. |
| `report-includes-evidence` | Claim without file:line | Add the citation. |

## Related skills

- `auditar` — broader conformity check
- `metricas` — uses eval scores for maturity scoring
- `revisar-pr` — per-PR fidelity check