---
name: domain
description: DDD domain model for the feature. Pull when modeling aggregates and language.
alwaysApply: false
---

# Domain Model (DDD) — <feature name>

> Answers: what's the **language** and **model** of the business.
> Tactical DDD within the bounded context. Terms here must appear identically in code.

## Bounded Context
<Context name. Which subdomain: **core** (competitive advantage) /
**supporting** (necessary, not differentiating) / **generic** (buy off the shelf)?>

## Identity strategy
> How entities are identified. Prefer time-ordered IDs for database performance.
>
> **Recommended:** UUIDv7 or ULID (time-ordered, preserves B-tree index performance).
> **Native support:** PostgreSQL 18+, MySQL 9+ have native UUIDv7 columns.
> **Avoid:** UUIDv4 (random, destroys index locality — see Shopify payment system case study).
> **Never expose:** sequential numeric IDs (security — enumerable).
>
> For high-volume tables, a dual strategy works: internal sequential ID + external UUIDv7.
> The application layer exposes only the UUID; the DB uses the sequential ID for joins.

## Ubiquitous language
> Same vocabulary between business, spec, and code. Promote to `docs/glossary.md` global.

| Term        | Definition                                  | Do NOT confuse with |
|-------------|---------------------------------------------|---------------------|
| <Term>      | <precise meaning in this domain>            | <similar term>      |

## Aggregates, entities, and value objects
- **Aggregate `<Name>`** (root: `<Entity>`)
  - Entities: <…>
  - Value objects: <…>
  - **Invariants** (rules always true): <…>
  - Consistency boundary: <what changes together in a transaction>

## Domain events
| Event (past tense)     | Triggered when           | Who reacts          |
|------------------------|--------------------------|---------------------|
| `<Something>Happened`  | <condition>              | <context/handler>   |

## Relations with other contexts
<How this context talks to others: Customer/Supplier, Conformist,
Anti-Corruption Layer, Shared Kernel? Update `docs/architecture/context-map.md`.>
