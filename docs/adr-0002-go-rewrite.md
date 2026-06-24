# ADR-0002: Rewrite in Go — from Spec Scaffold to Full Harness

- **Status:** accepted
- **Date:** 2026-06-24
- **Deciders:** Henrique

## Context

The initial implementation was a TypeScript/Node.js scaffold for Spec-Driven Development.
It provided templates, skills, adapters for 6 AI agents, and quality gate scripts.

However, research (14 videos, OpenAI/Anthropic/Martin Fowler blog posts, papers) revealed
that what we built was a **spec scaffold** (feed-forward), not a **harness** (feed-forward + feedback).

A true harness requires:
- Orchestrator managing implementation + validation as separate processes
- Validator agent running in isolated context (not a subagent of implementer)
- Auto-correction loop (fail → fix → re-test, with retry limit)
- Agent teams with real parallelism (goroutines, not just async)
- State machine tracking progress across sessions
- Bootstrap reconstructing context between sessions

TypeScript/Node.js is inadequate for this because:
- No single-binary distribution (requires Node runtime)
- Concurrency model (Promises/async) is inferior to goroutines for agent orchestration
- Not the standard for serious CLI tools in the industry

## Decision

Rewrite entirely in Go with the following architecture:

```
cmd/radiant/           CLI (cobra)
internal/
  scaffold/            Spec scaffold (templates, adapters) — from TypeScript
  harness/             Harness engine (orchestrator, validator, feedback, teams, state) — NEW
  spec/                Spec parsers (spec.md, tasks.md, STATE.md) — NEW
  quality/             Quality scripts (audit, mermaid, fidelity) — from TypeScript
templates/             SDD templates (Markdown) — reused as-is
```

Key design principles:
1. **Single binary** — `go build` produces one file, no runtime deps
2. **Goroutines for agent teams** — native concurrency, not async wrappers
3. **Process isolation** — validator runs as separate process, not subagent
4. **Feedback loop** — auto-correction with configurable retry limit
5. **State machine** — explicit states (idle → research → plan → implement → validate → done)
6. **Templates stay as Markdown** — universal, work with any AI agent

## Consequences

- **+** Single binary distribution (curl, brew, go install)
- **+** Native concurrency for agent orchestration
- **+** True harness with feedback loops
- **+** Modern, sophisticated, elegant
- **-** Complete rewrite (but templates/skills reused)
- **-** Go learning curve for contributors
- **-** npm distribution replaced by binary distribution

## References

- OpenAI: Harness Engineering
- Anthropic: Harness Design for Long-Running Apps
- Martin Fowler: Harness Engineering
- Valdemar Neto: Harness Engineering video (84K views)
- Navigation Paradox paper (2026)
