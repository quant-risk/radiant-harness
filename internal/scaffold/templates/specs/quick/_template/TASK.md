---
name: quick-task
description: Lightweight trail (trivial tier). Use for small task with a trace.
alwaysApply: false
---

# Quick Task — NNN-<slug>

> **Lightweight trail** (Trivial tier: ≤3 files, no decision). Leaves a trace without the full pipeline.
> ⚠️ If listing steps exceeds ~5, or a dependency/hard-to-reverse decision appears,
> **upgrade tier**: create `specs/NNNN-<name>/` with `spec.md` + `tasks.md`.

- **What:** <one sentence>
- **Why / origin:** <bug, issue, request>
- **Steps:**
  - [ ] <atomic step>
  - [ ] <atomic step>
- **Gate:** `<test command that proves it's done>` (see `docs/engineering/TESTING.md`)
