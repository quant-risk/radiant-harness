---
name: tasks
description: Feature decomposition and gates. Pull when implementing.
alwaysApply: false
---

# Tasks — <feature name>

> Implementation decomposition. Each task **maps to one or more `AC-N`** (traceability
> spec → task → commit) and has an **executable gate**: the command that proves it's done.
> Mark `[P]` for tasks that can run in parallel (no interdependency).

## Plan
| #  | Task                                  | Covers AC | Depends on | Gate (command)        | Status |
|----|---------------------------------------|-----------|------------|-----------------------|--------|
| 1  | <e.g. model aggregate in domain>      | AC-1      | —          | `<domain test>`       | todo   |
| 2  | <e.g. use case in application>        | AC-1,2    | 1          | `<use case test>`     | todo   |
| 3  | <e.g. adapter/repo in infrastructure> | AC-2      | 1          | `<integration test>`  | todo   |
| 4  | <e.g. endpoint in interface> `[P]`    | AC-1,2    | 2,3        | `<acceptance test>`   | todo   |

> A task only becomes `done` when the **gate passes** — not by visual inspection.
> One commit per task. Gate commands are MANDATORY, not optional.

## Execution protocol (RPI)

1. **Research** (if needed): explore codebase, save findings to `research.md`.
2. **Plan**: fill the table above. Each AC MUST have at least one task.
3. **Implement**: open a FRESH context window. Load only spec + tasks + CLAUDE.md.
4. **Verify**: after EACH task, run its gate command. If gate fails → fix → re-run.
5. **Track**: update `progress.md` after each task.

> **Context budget**: implementation should stay under 30% of the context window.
> If the window is growing too large, open a new context and load only the next task.

## AC → Test mapping (MANDATORY)
> Every AC must have a test. No exceptions. This is verified by `/validar`.

| AC | Test file | Test name pattern | Verified |
|----|-----------|-------------------|----------|
| AC-1 | <file> | `test_AC_1_*` / `AC-1: ...` | [ ] |
| AC-2 | <file> | `test_AC_2_*` / `AC-2: ...` | [ ] |

## Test plan
- Unit: <domain invariants, value objects>
- Integration: <adapters, repos, contracts>
- Acceptance: <one test per AC in spec.md — that's the acceptance gate>

## Deviations (SPEC_DEVIATION)
> If implementation needs to diverge from spec, record here before proceeding (see `CLAUDE.md`).
- [ ] <task # · reason · resolution: fix code OR update spec/ADR>

## Definition of Done checklist
- [ ] All ACs green **by the executable gate** (not by inspection)
- [ ] Every AC has a test that exercises its Given/When/Then
- [ ] Gate commands actually ran (check `progress.md` for evidence)
- [ ] No open `SPEC_DEVIATION`
- [ ] ADRs registered for hard-to-reverse decisions
- [ ] Glossary / context-map updated if they changed
- [ ] Spec reflects what was built
- [ ] `docs/STATE.md` updated (next step / decisions)
