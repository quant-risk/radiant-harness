---
name: CONVENTIONS
description: Agent conventions for the SDD pipeline. Always active.
alwaysApply: true
---

# CLAUDE.md — Agent Conventions

This project follows **Spec-Driven Development (SDD)** with the **RPI framework**
(Research → Plan → Implement). Read before implementing anything.

## The RPI Framework

Every feature follows three phases, each in its own context window:

1. **Research** — discover WHAT to build. Explore codebase, gather requirements,
   identify implicit needs. Save findings to markdown.
2. **Plan** — define HOW to build. Create spec (contract), design (if architectural),
   tasks (decomposition). This is the "long-term memory."
3. **Implement** — build it. Fresh context window. Load only spec + tasks + these
   conventions. Execute tasks, run gates, track progress.

> **Never skip Research.** Never implement without a Plan. Never reuse a Research
> context for Implementation.

## The spec is the source of truth
- Implement **from** `specs/NNNN-*/spec.md`. ACs (Given/When/Then) are the contract.
- If spec is ambiguous → **stop and ask**. Never guess.
- Never implement beyond scope. "Out of scope" is binding.
- If implementation diverges → `// SPEC_DEVIATION: <reason>` → decide with spec owner.

## Knowledge verification (never invent)
1. Codebase patterns (how it's already done here).
2. Project docs (`specs/`, `docs/`, ADRs, glossary).
3. Reference MCP (Context7 for libs) when connected.
4. Web/official docs.
5. **Not found? Say "I don't know."** Never invent API or behavior.

## Context window management
- **Smart zone: under 40%.** Above 60% = hallucination risk.
- **Base context (~15k tokens):** this file + STATE + vision + roadmap + active spec.
- **On demand:** everything else. Pull by `description` when needed.
- **New window for implementation:** after Research+Plan, open fresh context.
- **Subagents for parallelism:** independent tasks → isolated context per subagent.

## Connected tools (MCP)
> Maintained by `/integracoes`. Empty until first connection.

| MCP (`mcp__<server>__*`) | Validated account | Consuming skills |
|---|---|---|
| _none yet_ | — | — |

Rule: active connection ≠ authorization. Confirm account before read, re-confirm before write.

## Before coding — discover the tier
*Does this introduce a hard-to-reverse decision or new domain boundary?*

| Tier | When | Artifacts |
|------|------|-----------|
| **Trivial** | ≤3 files, no decision | PR only (or `quick/` for trail) |
| **Small** | Isolated feature, <10 tasks | `spec.md` + `tasks.md` |
| **Architectural** | New bounded context, irreversible decision | Full pipeline + `design.md` + ADR |

> **Dynamic escalation:** list atomic steps before coding. >5 steps or complex
> dependencies → STOP and create formal `tasks.md`.

## Ubiquitous language
- Use **exactly** the terms from `docs/glossary.md` and feature's `domain.md`.
- New term → add to glossary in same PR. No synonyms.

## Layered architecture (dependency rule)
```
interfaces → application → domain ← infrastructure
```
- `domain/` imports NOTHING from frameworks, I/O, or other layers.
- `application/` orchestrates use cases; depends only on `domain/`.
- `infrastructure/` implements ports defined in domain.
- `interfaces/` is the boundary (API/CLI/UI).
- For simple features, use the **lean template** (core + adapters).

## Identity strategy
- **Prefer:** UUIDv7 or ULID (time-ordered, preserves B-tree index performance).
- **Avoid:** UUIDv4 (random, destroys index locality).
- **Never expose:** sequential numeric IDs (enumerable = security risk).

## Definition of Done
- [ ] All ACs green **by executable gate** (not visual inspection)
- [ ] Every AC has a test that exercises its Given/When/Then
- [ ] Gate commands actually ran (check `progress.md`)
- [ ] Static analysis clean (type-check + SAST)
- [ ] No open `SPEC_DEVIATION`
- [ ] ADRs for hard-to-reverse decisions
- [ ] Glossary/context-map updated if changed
- [ ] Spec reflects what was built
- [ ] `docs/STATE.md` updated

## Working memory — `docs/STATE.md`
- **STATE.md** = volatile (in progress, next step, blockers). Updated constantly.
- **ADR** = durable (immutable decision). Created once, never edited.
- Update STATE when pausing. Read STATE when resuming. Use `/handoff`.

## Where to write
- Durable decision → new ADR (`docs/architecture/adr/`), never edit old.
- Work state / next step → `docs/STATE.md`.
- Business term → `docs/glossary.md`.
- Boundary change → `docs/architecture/context-map.md`.
