---
name: TESTING
description: Gate commands and test conventions. Pull when coding, validating, or setting up CI.
alwaysApply: false
---

# TESTING — How to verify the project

> **Single source of gate commands** and test conventions. What **DoD**, **CI**, and
> **subagents** consume to prove a task/feature is done — without visual inspection.
> Filled in the kickoff (Quality axis) and kept alive.

## How to run
| Level          | Command                   | When |
|----------------|---------------------------|------|
| Unit           | `<command>`               | always, fast |
| Integration    | `<command>`               | adapters / repos / contracts |
| Acceptance (UAT) | `<command>`             | one test per `AC-N` in spec |
| Lint / format  | `<command>`               | pre-commit / CI |
| Static analysis | `<command>` (type-check, complexity, SAST) | CI — no blocking findings |
| Coverage       | `<command>` (min `<X>%`, generates report) | CI — report attached to PR |

## Conventions
- Pyramid: many unit tests, fewer integration, few acceptance.
- **Each `AC-N` in spec has an acceptance test that is its gate.** Name the test with the ID
  (`test_AC_1_*` / `AC-1: ...`) for spec → test traceability.
- Domain doesn't spin up infra; integration uses `<testcontainer / edge mock>`.
- **Static analysis** (choose by stack): type-check (`<mypy/tsc/…>`), complexity/smells and
  **SAST/security** (`<sonar/codeql/semgrep/…>`). Define what's **blocking** (bars merge)
  vs **warning** (enters as trend in `metrics.md`, doesn't block).

## Gates (executable Definition of Done)
- A **task** only becomes `done` when its **Gate (command)** in `tasks.md` passes.
- A **feature** only merges when all ACs are green + lint + **static analysis clean**
  (no blocking findings) + minimum coverage.
- **CI runs exactly these commands** — failure blocks merge.
