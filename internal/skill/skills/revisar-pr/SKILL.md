# Skill: revisar-pr

> PR review gate: spec alignment, AC coverage, gates, deviations.

## Decision tree

```
PR opened (or invoked manually)
        │
        ▼
Fetch diff (via GitHub MCP or input)
        │
        ▼
For each AC in spec.md:
    ├── Code in diff implements it? ──── yes ──► ✓
    ├── No code, but task covers it? ──── skip ──► not implemented yet
    └── Code diverges from AC? ──► SPEC_DEVIATION
        │
        ▼
Run every gate in tasks.md on the PR branch
        │
        ▼
Write pr-review.md
```

## Workflow

### Step 1: fetch the diff

If GitHub MCP is connected, fetch via `mcp__github__get_pr_diff`. If
not, ask the user to paste the diff.

### Step 2: AC-by-AC check

For each AC in spec.md:
1. Find the files that should implement it
2. Search the diff for changes to those files
3. Mark:
   - **Implemented** — code matches the AC
   - **Missing** — no code touches the relevant files
   - **Deviated** — code exists but does something different

### Step 3: run gates

If the agent has shell access, check out the PR branch and run
each gate. Otherwise, ask the user to confirm gate status.

### Step 4: SPEC_DEVIATION

For each deviation, write a SPEC_DEVIATION entry:

```markdown
### SPEC_DEVIATION-001: AC3 says "p95 < 200ms" but tests cover only 10 users

- **Files**: auth/middleware_test.go
- **What's missing**: load test at 100 concurrent users
- **Recommended action**: extend test or revise AC3
```

### Step 5: write pr-review.md

```markdown
# PR review: <NNNN> — <short title>

## Summary

| Category | Status |
|----------|--------|
| ACs implemented | 4/5 |
| Gates passing | 5/5 |
| SPEC_DEVIATION | 1 |

## Recommendation

- [ ] Approve
- [x] Request changes (SPEC_DEVIATION-001)

## Suggested PR comment

> SPEC_DEVIATION-001: AC3 mentions p95 < 200ms under 100 concurrent
> users, but the test only runs at 10. Either extend the test or
> revise the AC. The PR as-is does not satisfy AC3.
```

## Examples

See `examples/` directory in the bundled skill for worked examples
covering common audit/eval/PR/roadmap scenarios.

## Anti-patterns

- ❌ Approving without reading spec.md. The spec IS the contract.
- ❌ Confusing "looks good" with "matches spec". Check the diff.
- ❌ Skipping deviations. Document them.

## Failure modes

| Gate | Failure | Recovery |
|------|---------|----------|
| `spec-pr-align` | AC missing in code | Block merge. |
| `gates-pass` | Gate fails | Block merge. |
| `deviations-documented` | Drift, no doc | Block merge until documented. |

## Related skills

- `validar` — validates the spec against implementation after merge
- `nova-feature` — produced the spec being reviewed