---
name: ADR-0001
description: Decision to use ADRs. Pull when recording or reviewing decisions.
alwaysApply: false
---

# ADR-0001: Record architecture decisions as ADRs

- **Status:** accepted
- **Date:** <YYYY-MM-DD>
- **Deciders:** <names>

## Context
Hard-to-reverse architectural decisions need durable memory. Without it,
the team reopens the same discussions and loses the *why* of old choices.

## Decision
We will use **Architecture Decision Records** (Nygard format) in `docs/architecture/adr/`.
- One file per decision, sequentially numbered: `NNNN-title.md`.
- ADRs are **immutable**. To change a decision, create a new ADR with status
  `supersedes ADR-XXXX` and mark the old one as `superseded by ADR-YYYY`.
- Create an ADR when the decision is hard-to-reverse (database choice,
  context boundary, integration protocol, cross-cutting pattern).

## Consequences
- **+** Traceability of the *why*; faster onboarding.
- **+** More objective reviews (the decision has a home).
- **-** Small overhead per decision — acceptable and limited to the architectural tier.
