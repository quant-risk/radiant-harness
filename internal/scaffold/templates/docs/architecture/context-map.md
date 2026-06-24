---
name: context-map
description: Bounded contexts and relations. Pull when modeling or crossing contexts.
alwaysApply: false
---

# Context Map

> DDD strategic view: the system's bounded contexts and how they relate.
> Update when a feature creates/moves boundaries. Combine with C4 diagrams if useful.

## Bounded Contexts
| Context    | Subdomain (core/supporting/generic) | Responsibility         | Owner |
|------------|--------------------------------------|------------------------|-------|
| <Context>  | core                                 | <what it decides>      | <team>|

## Relations between contexts
> DDD integration patterns: Customer/Supplier, Conformist, Anti-Corruption Layer (ACL),
> Shared Kernel, Open Host Service, Published Language.

| Upstream   | Downstream | Pattern               | Why |
|------------|------------|-----------------------|-----|
| <A>        | <B>        | Anti-Corruption Layer | <protects B's model> |

## Diagrams
High-level architecture diagrams (C4 context, containers, context map) live in
[`diagrams.md`](./diagrams.md) — generate/update with the `/diagramar` skill.
