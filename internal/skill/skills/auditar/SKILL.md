# Skill: auditar

> Project-wide conformity check. Doc frontmatter, links, AC
> traceability, ADRs, deviations.

## Decision tree

```
Audit requested (scope: full|docs|specs|adrs)
        │
        ▼
Walk the scope; for each file:
        │
        ├── has YAML frontmatter? ──── yes ──► parses? ──── yes ──► ✓
        │                                    │              │
        │                                    │              └── no ──► ✗ fix
        │                                    └── no ──► depends on convention
        │
        ├── cross-references another file? ──► link resolves? ──── yes ──► ✓
        │                                                       │
        │                                                       └── no ──► ✗
        │
        └── for spec.md + tasks.md:
            each AC → ≥1 task? each task → ≥1 AC?
```

## Workflow

### Step 1: walk the scope

Recursive directory walk. For each `.md` file, parse the
frontmatter (if any) and check it against the schema.

### Step 2: check links

Extract every `[text](path)` link. Resolve relative to the file.
Verify the target exists.

### Step 3: AC traceability

For every `specs/<NNNN>-<slug>/spec.md`:
- Extract ACs
- Open `tasks.md` for the same spec
- Verify: every AC has ≥1 task in its `Covers` column
- Verify: every task has ≥1 AC in its `Covers` column

### Step 4: SPEC_DEVIATION status

For every spec with a `## Deviations` section:
- Each entry should have a status (open / closed)
- Closed entries should have a resolution

### Step 5: write the report

```markdown
# Audit report

## Summary

| Severity | Count |
|----------|-------|
| Error    | 0     |
| Warning  | 2     |
| Info     | 5     |

## Findings

### [WARNING] specs/0003-search/spec.md AC3 has no covering task
- **Severity**: warning
- **Location**: specs/0003-search/spec.md:23
- **Fix**: add a task with AC3 in the Covers column, or remove AC3.

### [INFO] docs/architecture/adr/0001 has no supersession link
- **Severity**: info
- **Location**: docs/architecture/adr/0001-use-postgres.md
- **Fix**: if superseded, link to the replacement ADR.
```

## Examples

See `examples/` directory in the bundled skill for worked examples
covering common audit/eval/PR/roadmap scenarios.

## Anti-patterns

- ❌ Audit once, assume conformant. Re-audit every release.
- ❌ Ignore low-severity. They compound.
- ❌ Auto-fix without showing the diff.

## Failure modes

| Gate | Failure | Recovery |
|------|---------|----------|
| `all-frontmatter-valid` | A doc has broken frontmatter | Show the doc, ask user to fix or accept. |
| `all-links-resolve` | A cross-reference is broken | Show the broken link, ask user. |
| `traceability-1to1` | AC has no task | Show the orphan AC, ask user to add or remove. |
| `no-stale-deviations` | SPEC_DEVIATION is unresolved | Show the entry, ask user to close or document. |

## Related skills

- `validar` — per-feature DoD check (audit is project-wide)
- `revisar-pr` — per-PR check (audit is project-wide)
- `metricas` — uses audit data for maturity scoring