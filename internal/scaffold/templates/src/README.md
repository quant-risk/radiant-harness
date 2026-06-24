---
name: src
description: DDD layer rule. Pull when structuring or implementing code.
alwaysApply: false
---

# src/ — Layered Structure (Tactical DDD)

Language-agnostic. Create equivalent subfolders/modules in your stack, but
**preserve the dependency rule**: arrows point only inward.

```
interfaces ──► application ──► domain ◄── infrastructure
```

| Layer              | Responsibility                                   | Can depend on         |
|--------------------|--------------------------------------------------|-----------------------|
| `domain/`          | Entities, value objects, events, rules/invariants | **nothing** (no framework/IO) |
| `application/`     | Use cases, orchestration, ports (interfaces)     | `domain/`             |
| `infrastructure/`  | Repos, adapters, integrations (implements ports) | `domain/`, `application/` |
| `interfaces/`      | Boundary: API, CLI, UI, controllers              | `application/`        |
