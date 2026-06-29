---
name: spec
description: Feature contract (acceptance criteria). Active while the feature is in progress.
alwaysApply: false
---

# Spec — <feature name>

> **Source of truth.** Status: draft | in review | approved | implemented
> The acceptance criteria are (a) the contract with the business, (b) the test oracle,
> (c) the prompt for the AI agent to implement. Write them to be executable.

## Summary
<One sentence: what the system will start doing.>

## Acceptance criteria
> Given/When/Then format. Each criterion must be testable and unambiguous.
> **Each `AC-N` is a traceable ID:** it appears in `tasks.md` (column "Covers AC"), in the
> acceptance test that validates it, and in the commit message. Do not renumber implemented ACs.

### AC-1: <scenario title>
- **Given** <state/precondition>
- **When** <action/event>
- **Then** <observable and verifiable result>

### AC-2: <title>
- **Given** …
- **When** …
- **Then** …

## Decision matrix (optional)
> Use **when the rule combines multiple factors** (flags, states, roles, modes). A truth table
> is denser, less ambiguous, and cheaper in tokens than the same rule in prose — and **each row
> becomes a test case**. Link each row to its `AC-N`.

| Factor A | Factor B | … | Expected result | AC |
|----------|----------|---|-----------------|------|
| <value>  | <value>  | … | <observable action> | AC-1 |

## Edge cases and errors
- <invalid input → expected behavior>
- <concurrency, timeout, dependency failure → expected behavior>

## Out of scope
> Binding. Do not implement anything here.
- <…>

## Traceability
- Product: `./product.md`
- Design: `./design.md` (if architectural tier)
- Domain: `./domain.md`
- Related ADRs: <links>
