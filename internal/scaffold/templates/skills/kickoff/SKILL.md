---
name: kickoff
description: Constitute a project: interview, map, and generate roadmap.
---

# Skill: Project Kickoff (Lean Inception + SDD)

Constitutes a project: interviews, maps, proposes, and generates a roadmap.
Follows RPI: Research (detect mode, gather context) → Plan (interview, decide axes) → Implement (generate docs).

## Phase 1 — Detect mode (Research)

1. Inspect the directory: `read_file` manifests (`package.json`, `go.mod`, `Cargo.toml`), `src/` with real code, git history, existing `docs/`.
2. If `src/` is empty or absent and git history is < 10 commits → **Greenfield**.
   If real code + history exists → **Brownfield**. Mixed → **Hybrid**.
3. Confirm detected mode with the user before proceeding.

> Keep this phase lean. Delegate broad codebase exploration to a subagent if the repo is large.

## Phase 2A — Greenfield path (Plan: Lean Inception)

Interview one question at a time (see `/clarificar` principles). Fill in order:

1. **Vision** → generate `docs/product/vision.md` (product vision canvas).
2. **Personas** → identify primary/secondary, their goals and pains.
3. **Journey mapping** → key user journeys from trigger to outcome.
4. **MVP canvas** → generate `docs/product/mvp-canvas.md`: what's in, what's out, success metrics.

## Phase 2B — Brownfield path (Plan: as-is map)

1. Run `/mapear` to auto-detect stack, architecture, bounded contexts.
2. Review the generated `assessment.md` with the user — confirm gaps and risks.
3. Capture undocumented historical decisions as retroactive ADRs (`docs/architecture/adr/`).

## Phase 3 — Technical kickoff (5 axes) — both paths

Interview each axis, propose a recommended answer, confirm with user:

| Axis | Questions | Output doc |
|------|-----------|------------|
| **Tech stack** | Language, framework, persistence, messaging | `vision.md` / `assessment.md` |
| **Architecture** | Monolith? Modules? Services? Bounded contexts? | `context-map.md` via `/diagramar` |
| **Infra** | Cloud provider, container strategy, environments | `design.md` or infra section |
| **Quality** | Test framework, coverage minimum, static analysis | `docs/engineering/TESTING.md` |
| **Observability** | Logs, metrics, tracing, alerting, SLOs | observability section in design |

## Phase 4 — Generate artifacts (Implement)

1. Fill `docs/engineering/TESTING.md` — gate commands per test level.
2. Run `/integracoes` — discover team tools, propose MCP connections.
3. Run `/camada-agentica` — propose rules, subagents, skills, workflows.
4. Run `/roadmap` — generate `docs/product/roadmap.md` with Now/Next/Later.
5. Initialize `docs/STATE.md` with kickoff date, current phase: `plan`.
6. Commit all generated docs with message `chore: kickoff — project constitution`.

## Rules

- **One question at a time.** Never dump a multi-axis form. Interview sequentially.
- **Always propose a recommended answer** based on detected stack and docs — don't ask open-ended.
- **Idempotent:** re-running `/kickoff` updates existing docs, doesn't overwrite decisions.
- Delegate file-heavy exploration to subagents to keep the context window under 40%.
- Confirm with the user before any outward-facing action (creating issues, publishing).
