---
name: auditar
description: Validate SDD conformity via structural scripts and judgment.
---

# Skill: Audit SDD Pipeline

Validates that the project conforms to the SDD pipeline. Two layers:
**structural** (deterministic, scriptable) and **semantic** (requires judgment).

## Phase 1 — Structural check (deterministic)

Run automated validators. These are pass/fail — no judgment needed.

1. **Pipeline audit script:**
   ```
   terminal: node scripts/audit-esteira.mjs .
   ```
   Checks: required docs exist, frontmatter valid, directory structure matches SDD layout.

2. **Mermaid validation** (if diagrams exist):
   ```
   terminal: node scripts/validate-mermaid.mjs .
   ```
   Checks: all Mermaid blocks in `docs/architecture/diagrams.md` parse without errors.

3. **Frontmatter validation:** scan all `docs/**/*.md` and `specs/**/*.md`:
   - Every doc has `name` and `description` in frontmatter.
   - Pipeline docs have `alwaysApply` field.
   - Skills have `name` + `description` (no `alwaysApply`).

4. Record structural results: ✅ pass / ❌ fail with specific file and error.

> If audit scripts don't exist yet, note it as a gap and generate a minimal version.

## Phase 2 — Semantic check (judgment)

These require reading and reasoning — a script can't do them.

### 2a. Traceability
- For each feature in `specs/`, verify: every `AC-N` in `spec.md` has a corresponding task in `tasks.md`.
- For each `AC-N`, `search_files` test directories for the AC ID — confirm a test exists.
- Flag orphans: AC without task, task without AC, test without AC.

### 2b. Orphan specs
- List all `specs/NNNN-*/` directories.
- For each, check: does implementation exist in `src/`? Is there a merged PR?
- Flag specs with no implementation (stale) or implementation with no spec (rogue).

### 2c. Living docs freshness
- `read_file` `docs/glossary.md` — are the terms still used in code? `search_files` for each term in `src/`.
- `read_file` `docs/architecture/context-map.md` — do the bounded contexts match the actual `src/` structure?
- Flag stale entries: terms no longer in code, contexts that merged or split.

### 2d. Pending DoD violations
- `search_files` pattern `SPEC_DEVIATION` across `src/` and `specs/` — list all open deviations.
- Check `docs/STATE.md` for features marked "done" — do they have open deviations? That's a DoD violation.

### 2e. Link integrity
- `search_files` pattern `\]\(` in all docs — extract markdown links.
- Verify each link target exists (`read_file` or `search_files` for the path).
- Flag broken links.

## Phase 3 — Generate audit report (Implement)

1. Compile results into a structured report:

```
## SDD Pipeline Audit — <date>

### Structural: PASS / FAIL
- [✅/❌] audit-esteira.mjs: <details>
- [✅/❌] validate-mermaid.mjs: <details>
- [✅/❌] frontmatter: <n> issues

### Semantic findings
| Category | Finding | Severity | Recommendation |
|----------|---------|----------|----------------|
| Traceability | AC-3 in spec 0002 has no test | high | Add test_AC_3 |
| Orphan | spec 0001 has no PR | med | Implement or archive |
| Living docs | "Widget" in glossary, not in code | low | Remove or update |

### Overall: CONFORMANT / NON-CONFORMANT
```

2. Save report to `docs/STATE.md` (audit section) or a dedicated `docs/audit-<date>.md`.
3. Present findings to the user with prioritized fix list.

## Rules

- **Structural first, semantic second.** Fix structural failures before evaluating semantics.
- **Evidence-based.** Every finding cites the file and search that revealed it.
- **Non-conformant is actionable.** Each finding has a specific fix recommendation.
- Re-running `/auditar` refreshes the report — it's not a one-time gate.
- Don't fix issues during the audit — that's a separate step. Report, then plan fixes.
