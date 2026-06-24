# Skill: validar

> Definition of Done check. Verifies that what was built matches
> what was specified, with evidence.

## Decision tree

```
spec.md + tasks.md + implemented code in hand
        │
        ▼
Parse spec.md → list of ACs
        │
        ▼
For each AC, find the task that covers it (tasks.md)
        │
        ▼
For each covered task, run its gate command
        │
        ├── Gate fails ──► Document in validation.md, do NOT pass
        │
        └── Gate passes ──► Continue
        │
        ▼
Check: does the implementation match the spec?
        │
        ├── Match ──► AC green ✓
        │
        └── Drift ──► SPEC_DEVIATION entry required
                       │
                       ▼
                 Document drift in spec.md under "Deviations"
                 (with rationale) → user approves
        │
        ▼
Write validation.md with all results
```

## Workflow

### Step 1: parse spec.md and tasks.md

Extract:
- ACs from spec.md (each AC's name + the AC text)
- Tasks from tasks.md (each task's #, name, coverage list, gate)

Build a map: `AC → task # → gate command`.

### Step 2: run every gate

For each task, run its gate command. Capture stdout, stderr, exit
code. Each gate that exits 0 is green; non-zero is red.

If a gate can't even run (command not found, permission denied),
treat as red with a note in validation.md.

### Step 3: AC-to-test mapping check

For each AC, verify at least one task in its coverage column has a
green gate. If not, that AC has no proof — flag it.

### Step 4: spec-deviation scan

For each AC, compare the AC text against the implementation:
- Files referenced in the AC exist?
- Behavior described matches actual behavior?

If you find drift, write a `SPEC_DEVIATION` entry to spec.md:

```markdown
## Deviations

### DEV-001: AC3 says "p95 < 200ms" but tests cover only 10 users
- **Status**: documented, awaiting user decision
- **Rationale**: load test infrastructure not yet built
- **Next step**: add load test, or revise AC3 to match current
  test coverage
```

### Step 5: write validation.md

```markdown
# Validation: <NNNN> — <short title>

## Summary

| Category | Pass | Fail | Skip |
|----------|------|------|------|
| ACs      | 5    | 1    | 0    |
| Gates    | 6    | 1    | 0    |

## ACs

| AC | Title | Status | Test |
|----|-------|--------|------|
| AC1 | login returns JWT | ✓ green | tasks/2 |
| AC2 | invalid login 401  | ✓ green | tasks/2 |
| AC3 | expired JWT 401    | ✗ red   | tasks/3 (gate failed: assertion at line 47) |
| AC4 | tampered JWT 401   | ✓ green | tasks/3 |

## Gate results

| Task | Gate | Exit | Time | Status |
|------|------|------|------|--------|
| 1 | go build | 0 | 1.2s | ✓ |
| 2 | go test ./auth | 0 | 4.1s | ✓ |
| 3 | go test ./auth | 1 | 3.8s | ✗ (see AC3) |

## SPEC_DEVIATION

- DEV-001 (if any)

## Recommendation

- [ ] Ready to merge / Ready for PR review
- [ ] Fix AC3 before merge
- [ ] Re-scope (feature is incomplete)
```

## Examples

### Example 1: feature passes validation

**Input**: spec with 5 ACs, tasks with 5 green gates.

**Output**: validation.md shows all 5 ACs green, 5 gates green,
zero deviations. Recommendation: "Ready to merge."

### Example 2: feature has 1 failing AC

**Input**: 5 ACs, 4 gates green, 1 gate fails on AC3.

**Output**: validation.md shows AC3 red with the gate output.
Recommendation: "Fix AC3 before merge."

### Example 3: feature with documented SPEC_DEVIATION

**Input**: 5 ACs, 5 gates green, but the implementation skipped
one edge case described in AC4.

**Output**: validation.md flags the deviation, AC4 marked as
"documented deviation". User must decide: ship as documented, or
extend the implementation.

## Anti-patterns

- ❌ Marking "looks right" as green. Run the actual test.
- ❌ Skipping SPEC_DEVIATION documentation. Drift without record is
  silent scope creep that compounds.
- ❌ Passing with vague summary ("all looks good"). Include actual
  test output.

## Failure modes

| Gate | Failure | Recovery |
|------|---------|----------|
| `ac-testable` | AC is not Given/When/Then | Call clarificar skill; do NOT pass validation. |
| `ac-tested` | No task covers an AC | Add a task + gate; do NOT mark green. |
| `gates-pass` | Gate exits non-zero | Surface which task + which assertion failed. Do NOT pass. |
| `spec-deviation` | Code drifted, no doc | Reject validation. Force the deviation to be documented. |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `nova-feature` | When validating a spec that doesn't exist yet — this skill needs a real spec. |
| `revisar-pr` | Called after this skill — PR review checks the spec/code alignment in a different way. |
| `metricas` | Pulled metrics from validation reports over time. |