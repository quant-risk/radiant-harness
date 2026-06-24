---
name: lean-architecture
description: Lean architecture template — 2 layers (core + adapters). Use for small projects or features where full DDD ceremony is overkill.
alwaysApply: false
---

# Lean Architecture — <project/feature name>

> For features where full DDD (4 layers) is overkill. Two layers, same principles:
> business logic isolated, dependencies point inward, testable without infra.

## Structure
```
src/
├── core/           # Business logic, entities, rules, ports (interfaces)
│   ├── entities.ts
│   ├── rules.ts
│   └── ports.ts    # Interfaces for external dependencies
└── adapters/       # Implementations: DB, HTTP, CLI, UI
    ├── db.ts
    ├── api.ts
    └── cli.ts
```

## Dependency rule
```
adapters → core (never the reverse)
```

- `core/` has ZERO imports from frameworks, I/O, or adapters.
- `adapters/` implements the ports defined in `core/`.
- Swap DB/framework/UI by changing only `adapters/`.

## When to upgrade to full DDD
- New bounded context emerges → split into domain/application/infrastructure/interfaces.
- More than ~10 entities → domain layer grows too large.
- Multiple adapters need different orchestration → application layer needed.

## ID strategy
> Prefer UUIDv7 or ULID (time-ordered). Avoid UUIDv4 for indexed columns.
