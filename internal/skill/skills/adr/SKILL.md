# Skill: adr

> Architecture Decision Records (Nygard format). Records WHY a
> decision was made, not just what.

## Decision tree

```
Non-trivial technical decision made (or about to be)
        │
        ▼
Is this hard to reverse? ──── no ──► Don't ADR; just do it
        │
        ▼ yes
Is this a new pattern or a new dependency? ──── no ──► Doc in spec.md, no ADR needed
        │
        ▼ yes
Draft ADR with the 5 sections below
        │
        ▼
Number it (next NNNN in docs/architecture/adr/)
        │
        ▼
Status: proposed (until team agrees) → accepted
```

## Workflow

### Step 1: draft the 5 sections

```markdown
# NNNN. <short title>

## Status

<proposed | accepted | deprecated | superseded>

## Context

What forces are at play? What problem are we solving? What
constraints exist?

## Decision

What did we choose? (One paragraph.)

## Consequences

What becomes easier? What becomes harder? What trade-offs
did we accept?
```

### Step 2: list alternatives

In the **Context** section, list at least 2 alternatives considered,
even briefly. ADRs are valuable BECAUSE they record what was
rejected.

### Step 3: number and place

`docs/architecture/adr/NNNN-<slug>.md`. NNNN is sequential, slug
is kebab-case.

### Step 4: status transitions

- `proposed` → `accepted`: team agrees
- `accepted` → `deprecated`: superseded by ADR NNNN
- `accepted` → `superseded`: replaced, link to the new one

## Examples

### Example 1: database choice

```markdown
# 0001. Use PostgreSQL for primary data store

## Status
accepted

## Context
We need a relational database with strong consistency, mature
ORM support, and JSON columns for semi-structured data.

Alternatives considered:
- MySQL: weaker JSON support, no array types
- MongoDB: too early for document store; team lacks ops experience
- SQLite: file-based, doesn't scale to multi-instance

## Decision
Use PostgreSQL 16.

## Consequences
+ Strong consistency, JSONB, array types
+ Mature Go driver (pgx), good migration story
- Need to manage connection pooling
- Operational complexity higher than SQLite
```

## Anti-patterns

- ❌ "We use Postgres" — that's not an ADR. Missing context.
- ❌ Listing only the chosen option. ADRs record rejections.
- ❌ Skipping consequences. Both positive and negative.

## Failure modes

| Gate | Failure | Recovery |
|------|---------|----------|
| `context-written` | ADR is just the decision | Push back; rewrite context. |
| `alternatives-listed` | No alternatives shown | List at least 2; even brief. |
| `consequences-traced` | Only positive consequences | Add the negative trade-offs. |

## Related skills

- `kickoff` — generates retro-ADRs for brownfield projects
- `diagramar` — diagrams often accompany ADRs
- `nova-feature` — feature specs that change architecture should ADR first