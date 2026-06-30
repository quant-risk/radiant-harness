---
name: design
description: Technical Design Doc — 5 axes + dependency tables, solution, risks, and roadmap. Pull when designing an architectural feature.
alwaysApply: false
---

# Technical Design Doc — <feature name>

> **Tier:** architectural · **Status:** draft | in review | approved
> **Author:** <name> · **Reviewers:** <names> · **Date:** <YYYY-MM-DD>
> Answers: **how** at the system level. Required for architectural tier.

## Links and artifacts
| Artifact                 | Where                   | Link                        |
|--------------------------|-------------------------|-----------------------------|
| Design page              | Confluence / Notion     | <url>                       |
| Issue / epic             | Jira / Linear           | <PROJ-123>                  |
| Spec · Product · Domain  | repository              | `./spec.md` · `./product.md` · `./domain.md` |

## Functionality context
<Current state, constraints, why now. The problem this feature solves (link `product.md`).>

## Goals / Non-goals
**Goals**
- <measurable technical goal>

**Non-goals**
- <out of scope for this design>

## Proposed design
<Solution. Diagrams (C4/sequence — generate with `/diagramar`), components, data flow,
API contracts, data model. Show boundaries with existing bounded contexts.>

## 5-axis coverage
> Every technical decision passes through these 5 axes. Fill what applies; mark "no impact" for the rest.

### 1. Tech stack
<New languages, frameworks, libs, or services. Versions. Diverges from standard stack? Justify.>
### 2. Base architecture
<How it fits in layers and bounded contexts. New boundary? New aggregates/ports? Integration pattern.>
### 3. Infra
<New resources (queue, cache, DB), environments, IaC, cost. Deploy, feature flag, **safe rollback**.>
### 4. Quality
<Test strategy and what covers the ACs. Gates: coverage, contract test, performance, security.>
### 5. Observability
<Metrics, logs, tracing, alerts. SLO/SLI. How does telemetry prove it works?>

## Dependency map
| Dependency           | Type        | Description                  | Key methods / endpoints        |
|----------------------|-------------|-------------------------------|-------------------------------|
| <e.g. Payments API>  | REST / gRPC | <charges and refunds>         | `POST /charges` · `GET /charges/{id}` |

## Alternatives considered
> The most valuable section — shows the trade-off was thought through.

| Alternative  | Pros | Cons | Why (not) chosen |
|--------------|------|------|------------------|
| A (chosen)   |      |      |                  |
| B            |      |      |                  |

## Risks
| Risk   | Description          | Prob. × Impact    | Mitigations |
|--------|---------------------|-------------------|-------------|
| <risk> | <why it happens>    | medium × high     | <what to do> |

## Open questions
- [ ] <pending decision — who answers, by when>

> Hard-to-reverse decisions made here → record as ADR in `docs/architecture/adr/`.
