---
name: mapear
description: Map codebase, detect stack, and produce assessment.md.
---

# Skill: Map Codebase (Brownfield Assessment)

Auto-detects stack, architecture, and bounded contexts. Produces an as-is portrait
and gap analysis vs the SDD standard. Idempotent — re-running refreshes the assessment.

## Phase 1 — Detect stack (Research)

1. Scan for manifests using `search_files` (target: `files`, pattern: `package.json|go.mod|Cargo.toml|pyproject.toml|pom.xml|build.gradle`).
2. `read_file` each manifest to identify: language/runtime, frameworks, persistence, messaging libraries.
3. Check for infra artifacts: `Dockerfile`, `docker-compose.yml`, `.github/workflows/`, `terraform/`, `helm/`.
4. Detect CI provider: `.github/workflows/` → GitHub Actions; `.gitlab-ci.yml` → GitLab CI; `Jenkinsfile` → Jenkins.
5. Count test files (`search_files` pattern: `*.test.*|*_test.*|spec/**/*.ts`) to gauge existing test coverage.

> Delegate large scans to a subagent if the repo has > 500 files. Return only the summary table.

## Phase 2 — Detect architecture (Research)

1. Examine `src/` directory structure. Identify layering: `interfaces/`, `application/`, `domain/`, `infrastructure/`?
2. If DDD layers absent, detect the actual style (flat MVC, modular monolith, microservices).
3. Identify bounded contexts: group modules by domain responsibility. List each with its responsibility.
4. Flag dangerous couplings: cross-context imports, circular dependencies, shared mutable state.
5. Check for existing architecture docs (`docs/architecture/`) and ADRs — read them for context.

## Phase 3 — Gap analysis vs SDD (Plan)

Score each of the 5 axes against the SDD standard:

| Axis | SDD standard | Check method | Gap |
|------|-------------|--------------|-----|
| **Tech stack** | Documented, versioned | Manifests exist and are documented? | |
| **Architecture** | DDD layers, context-map | `src/` follows dependency rule? context-map.md exists? | |
| **Infra** | Containerized, IaC | Dockerfile/CI present? Environments documented? | |
| **Quality** | Gates defined, coverage min | TESTING.md exists? Test command runs green? | |
| **Observability** | Logs + metrics + tracing | Any observability setup? SLOs defined? | |

2. Rate each axis risk: `low` / `med` / `high` based on gap severity.
3. List the top 3 debts/risks with impact and recommended action.

## Phase 4 — Generate assessment.md (Implement)

1. Fill `docs/architecture/_templates/assessment.template.md` with detected data.
2. Capture undocumented historical decisions → list as candidates for retroactive ADRs.
3. Save to `docs/architecture/assessment.md`.
4. Update `docs/STATE.md`: note assessment complete, link to file.

## Rules

- **Idempotent:** re-running refreshes data; existing ADRs are never overwritten.
- **Photograph, don't judge** — the assessment is an as-is portrait. Recommendations go in roadmap, not here.
- Keep context lean: don't read every source file — sample entry points and structural files only.
- Confirm with the user before marking anything as a "debt" — some choices are intentional.
