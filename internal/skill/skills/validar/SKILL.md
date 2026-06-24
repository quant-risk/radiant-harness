---
name: validar
description: Validate implementation: gates, AC-to-test mapping, and DoD.
---

# Skill: Validate Feature (UAT)

Closes the SDD loop: proves the implementation fulfills the **spec** via **executable gates**.
Run after implementation, before merge.

## Phase 1 — Load feature context (Research)

1. Identify the feature: `search_files` pattern `specs/*/` to find active specs.
2. `read_file` `specs/NNNN-<name>/spec.md` — the acceptance criteria (the contract).
3. `read_file` `specs/NNNN-<name>/tasks.md` — task list and gate commands.
4. `read_file` `docs/engineering/TESTING.md` — the gate command reference.
5. Check `specs/NNNN-<name>/progress.md` (if exists) — what's been done so far.

> Context budget: load only this feature's docs + TESTING.md. No need for the full research history.

## Phase 2 — Run gates (Implement)

1. Execute each gate command from `tasks.md` using `terminal`:

```
terminal: <unit test command from TESTING.md>
terminal: <integration test command>
terminal: <lint command>
terminal: <static analysis command>
terminal: <coverage command>
```

2. Record pass/fail for each gate. If a gate fails, do NOT proceed to Phase 3 — fix first.
3. Capture the coverage report output — this is evidence, not visual inspection.

## Phase 3 — Map AC → test (Plan + Implement)

1. For each `AC-N` in `spec.md`, find the corresponding test:

| AC | Test identifier | Gate command | Status |
|----|----------------|-------------|--------|
| AC-1 | `test_AC_1_*` or `AC-1: ...` | `<command>` | PASS / FAIL |
| AC-2 | `test_AC_2_*` | `<command>` | PASS / FAIL |

2. Methods to find tests:
   - `search_files` pattern `AC.?1|AC_1|test_AC_1` in test directories.
   - Check naming convention from TESTING.md (`test_AC_N_*` or `AC-N: ...`).
3. **Flag any AC without a test** — this is a coverage gap. The feature cannot pass validation.
4. If spec has a decision matrix, verify every row has a test.

## Phase 4 — Resolve SPEC_DEVIATION

1. `search_files` pattern `SPEC_DEVIATION` in `src/` and `specs/` — find all open deviations.
2. For each deviation, present: what the spec says, what the code does, why it diverged.
3. Resolve per CLAUDE.md rules:
   - **Fix the code** (spec wins) → make the change, re-run gate.
   - **Update the spec** (conscious decision) → edit `spec.md`, create ADR if hard-to-reverse.
   - Decision has ramifications? → run `/clarificar` to resolve before proceeding.
4. **No open SPEC_DEVIATION may remain** at end of validation.

## Phase 5 — Check Definition of Done

Walk the DoD checklist from `CLAUDE.md`:

- [ ] All ACs pass — verified by executable gate, not inspection.
- [ ] Every AC has a test.
- [ ] Gate commands actually ran (output captured).
- [ ] Coverage ≥ project minimum, report attached.
- [ ] Static analysis clean — no blocking findings.
- [ ] No open SPEC_DEVIATION.
- [ ] Hard-to-reverse decisions → ADRs registered.
- [ ] Glossary and context-map updated if changed.
- [ ] Spec reflects what was built (or deviation documented).
- [ ] `docs/STATE.md` updated.

## Phase 6 — Report and handoff

1. Present validation summary: gates green/red, AC coverage table, deviations resolved, DoD status.
2. If all green → update `docs/STATE.md`: "Feature NNNN validated, ready for PR."
3. If any red → list blockers with specific fix needed. Do not mark as validated.

## Rules

- **Gates are executable, not decorative.** `node --test` must actually run and pass.
- **Each AC maps to exactly one test.** No AC without coverage — non-negotiable.
- **No open deviations.** Resolve or escalate before declaring validation complete.
- Evidence (test output, coverage report) must be captured, not described from memory.
