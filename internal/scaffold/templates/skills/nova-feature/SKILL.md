---
name: nova-feature
description: Use to open a new feature in SDD pattern. Follows RPI: Research → Plan → Implement. Acione com /nova-feature.
---

# Skill: New Feature (SDD + RPI Loop)

Opens and conducts a feature through the pipeline using the **RPI framework**:
Research → Plan → Implement. Each phase has its own context window.

## Phase 1 — Research (discovery)

Goal: understand WHAT to build before planning HOW.

1. Gather context: PRD, design docs, existing code, stakeholder input.
2. If using MCPs (Jira/Confluence), fetch related issues/docs.
3. Explore the codebase — find existing patterns, related modules, boundaries.
4. Identify implicit requirements: error handling, concurrency, edge cases.
5. **Save research findings to markdown** (`specs/NNNN-<name>/research.md`).
   This is the "long-term memory" — future sessions won't need to re-research.

> Context budget: research can use up to 40% of the window. If larger,
> delegate exploration to a subagent and save only the output.

## Phase 2 — Plan (specification)

Goal: create the contract that drives implementation.

1. Calculate next number: max `NNNN` in `specs/` + 1, 4 digits.
2. Ask short name in kebab-case → folder `specs/NNNN-<name>/`.
3. Determine tier: Trivial / Small / Architectural.
4. Fill artifacts by tier:
   - **Trivial:** `quick/NNN-slug/TASK.md`
   - **Small:** `spec.md` + `tasks.md`
   - **Architectural:** `product.md` → `design.md` → `domain.md` → `spec.md` → `tasks.md`
5. Fill top-down through gates. Stop at each gate for review.
6. **Each AC must map to a test in tasks.md** — this is non-negotiable.
7. **Save the plan** and close this context window.

> After planning, OPEN A NEW CONTEXT WINDOW for implementation.
> The plan is self-contained — the implementer doesn't need the research.

## Phase 3 — Implement (execution)

Goal: execute the plan with minimal context.

1. Open a fresh context window.
2. Load: CLAUDE.md + the specific `spec.md` + `tasks.md` (NOT the research).
3. Execute tasks in order. For parallelizable tasks, use subagents.
4. After each task: **run the gate command** (from tasks.md). Gate MUST pass.
5. If gate fails: fix and re-run. Do NOT mark as done without green gate.
6. Track progress in `specs/NNNN-<name>/progress.md`.

> Context budget: implementation should stay under 30% of the window.
> If a task is too large, break it into smaller sub-tasks.

## Phase 4 — Close

1. Run `/validar` to verify all ACs pass.
2. Register relevant decisions as ADR.
3. Update glossary/context-map if changed.
4. Update `docs/STATE.md` with next step.

## Rules

- **Never implement without a plan.** Research → Plan → Implement is mandatory.
- **Never reuse a research context for implementation.** New window = clean context.
- **Gates are executable, not decorative.** `node --test` must actually run.
- **Each AC maps to exactly one test.** No AC without coverage.
- Confirm before outward-facing actions (create issue, publish).
