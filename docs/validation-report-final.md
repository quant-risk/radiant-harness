# Radiant Harness — Final Validation Report

Date: 2026-06-24

## Build & Test

| Check | Result |
|-------|--------|
| go build | ✓ zero errors |
| go vet | ✓ zero warnings |
| go test | ✓ 57/57 passing |
| Binary | ✓ 8.9MB, v0.2.0 |
| Module path | ✓ `radiant-harness` (no external org) |
| External org refs | ✓ zero in source |

## Commands

| Command | Result |
|---------|--------|
| radiant --help | ✓ shows 3 commands |
| radiant --version | ✓ returns 0.2.0 |
| radiant init --all --yes | ✓ 140 files, 15 skills, 6 agents |
| radiant validate | ✓ audit OK, fidelity OK |

## Test Breakdown

| Package | Tests | Coverage |
|---------|-------|----------|
| harness | 41 | context window, RPI budget, token estimation, state machine, orchestrator, agent detection, protocols, capabilities, logger, integration |
| quality | 5 | audit, fidelity, frontmatter, missing alwaysApply |
| spec | 2 | spec parsing, AC tokens |
| **Total** | **57** | |

## Architecture

| Component | File | Lines | Status |
|-----------|------|-------|--------|
| CLI | cmd/radiant/main.go | ~210 | production-ready |
| Types | internal/types.go | ~170 | complete |
| Spec parser | internal/spec/spec.go | ~115 | complete |
| Tasks parser | internal/spec/tasks.go | ~110 | complete |
| Audit | internal/quality/audit.go | ~130 | complete |
| Fidelity | internal/quality/fidelity.go | ~100 | complete |
| Mermaid | internal/quality/mermaid.go | ~100 | complete |
| Validate | internal/quality/validate.go | ~130 | complete |
| Scaffold | internal/scaffold/scaffold.go | ~230 | complete |
| Adapters | internal/scaffold/adapters.go | ~50 | complete |
| Orchestrator | internal/harness/orchestrator.go | ~280 | complete |
| State machine | internal/harness/state.go | ~180 | complete |
| Agent runner | internal/harness/agent.go | ~180 | complete |
| Context window | internal/harness/context.go | ~100 | complete |
| Token estimator | internal/harness/tokens.go | ~90 | complete |
| Protocols | internal/harness/protocols.go | ~280 | complete |
| Logger | internal/harness/log.go | ~200 | complete |

## What's Different from Competitors

| Feature | Radiant Harness | TLC Spec Driven | GitHub Spec Kit | Superpowers |
|---------|----------------|-----------------|-----------------|-------------|
| Language | Go (single binary) | Markdown | Markdown | Markdown |
| Orchestrator | ✓ goroutines | ✗ | ✗ | ✗ |
| Feedback loop | ✓ auto-correct | ✗ | ✗ | ✗ |
| State machine | ✓ 8 states | partial | ✗ | ✗ |
| Context budget | ✓ smart/dumb | ✗ | ✗ | ✗ |
| Agent teams | ✓ parallel | ✗ | ✗ | ✗ |
| Protocols | ✓ 6 agents | ✗ | ✗ | ✗ |
| Validation | ✓ AC→test | ✓ | partial | ✗ |
| Logger | ✓ slog + hooks | ✗ | ✗ | ✗ |
| Token estimator | ✓ word-aware | ✗ | ✗ | ✗ |
| Tests | 57 | 0 | 0 | 0 |
| Distribution | go install | npx | npx | npx |

## Commits (19)

```
0253b1a fix: module path — neutral 'radiant-harness'
dfdcc69 chore: add .DS_Store to gitignore
adb53c7 fix: module path — radiant-agro → igoruehara
584a8c1 feat: protocols, structured logging, integration tests
268c028 feat: improved token estimation, structured logging, edge case tests
8531a38 chore: remove duplicate templates/ directory
c3175e5 feat: complete harness engine — all 10 components
dc4b2dc fix: --all flag + embed templates
ce15994 feat: rewrite in Go — full harness
587a6aa docs: ADR-0002 — rewrite in Go
c84cdc1 docs: PostgreSQL 18+/MySQL 9+ UUIDv7
986b4a0 docs: research references
8cd1423 feat: complete rewrite — all 15 skills
3535822 feat: research-driven improvements
077aac4 docs: consolidated validation report
8bb629c fix+test: round 5 — CLI subprocess + EEXIST
ff787f6 test: round 4 — update e2e, --force, --all
f71ea21 fix+docs: round 3 — golden example
964822d test: round 2 — edge cases
84efa2d docs: validation report round 1
e149f48 feat: radiant-harness v0.1.0 (TypeScript)
```
