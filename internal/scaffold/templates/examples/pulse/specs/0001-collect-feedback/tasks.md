---
name: tasks
description: Tasks — Collect feedback (golden example).
alwaysApply: false
---

# Tasks — Collect feedback

| #  | Task                          | Covers AC      | Gate (command)   | Status |
|----|-------------------------------|----------------|------------------|--------|
| 1  | Validate input                | AC-2 · AC-3    | `node --test`    | done   |
| 2  | Store and return `id`         | AC-1           | `node --test`    | done   |

## Test plan
- Acceptance: one test per AC in `src/feedback.test.mjs` (`AC-1`, `AC-2`, `AC-3`).

## Definition of Done
- [x] AC-1, AC-2, AC-3 green by gate (`node --test`)
- [x] No open `SPEC_DEVIATION`
