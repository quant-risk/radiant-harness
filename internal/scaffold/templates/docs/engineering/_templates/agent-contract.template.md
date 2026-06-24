---
name: agent-contract
description: Agreement between implementer and validator agents. Create before starting a sprint.
alwaysApply: false
---

# Agent Contract — <feature/sprint name>

> Agreement between the **implementer** (builds) and the **validator** (tests).
> Created DURING planning, BEFORE implementation starts.
> Both agents reference this contract — it prevents scope drift and infinite loops.

## Scope
<What this sprint covers. Link to spec.md and tasks.md.>

## Implementer commits to
- [ ] Build exactly what the spec says — nothing more, nothing less
- [ ] Run gate commands after each task (from tasks.md)
- [ ] Mark tasks as done ONLY when gate passes
- [ ] Update progress.md after each task
- [ ] Flag SPEC_DEVIATION immediately if spec is wrong

## Validator commits to
- [ ] Test against THIS checklist — not against personal opinions
- [ ] Run the same gate commands the implementer should have run
- [ ] Verify each AC has a corresponding test that exercises it
- [ ] Report: PASS / FAIL per AC with evidence
- [ ] Do NOT suggest new features or scope changes — only validate

## AC → Test mapping (agreed before implementation)
| AC | Test file | Test name | Status |
|----|-----------|-----------|--------|
| AC-1 | <file> | <test_AC_1_*> | pending |
| AC-2 | <file> | <test_AC_2_*> | pending |

## Gate commands (both agents run these)
```
<command 1>
<command 2>
```

## Escalation
If validator fails 3 times on the same AC:
1. Implementer and validator re-read the spec together
2. If spec is ambiguous → clarify spec (human decides)
3. If spec is wrong → SPEC_DEVIATION → update spec + ADR
