---
name: STATE
description: Volatile working memory — progress, decisions, blockers, context bookmarks.
alwaysApply: true
---

# STATE — Living project memory

> Working memory **between sessions** (humans and agents). **Volatile**: updated constantly.
> Unlike **ADR** (durable, immutable). Structural decision → ADR; work state → here.
> Update when **pausing/ending**; read when **resuming**. Use the `/handoff` skill.

**Last updated:** <YYYY-MM-DD> by <name>

## Current sprint / active feature
> What's open now. ONE feature at a time. Be specific.
- Active: `specs/NNNN-<name>/` — <current phase: research | plan | implement | validate>
- Sprint goal: <one sentence>
- Progress: <X/Y tasks done>

## Next concrete action
> The NEXT thing to do when resuming. Specific and atomic.
- <e.g. "implement AC-3 in the Stripe client adapter">
- <NOT "continue the feature" — that's too vague>

## Decisions log (chronological)
> Decisions made during implementation. Hard-to-reverse → create ADR and link.
- <YYYY-MM-DD: decision — impact — [ADR-NNNN](adr/NNNN-*.md) if applicable>

## Blockers
> What's blocking progress. Each has an owner and an unblock path.
- [ ] <what blocks · who/how unblocks · since when>

## Context bookmarks
> Files/sections that are important for the current work. Saves re-discovery time.
- <path/to/file> — <why it matters>
- <path/to/doc> — <what to look for>

## Deferred ideas / backlog
- <idea → trigger to reconsider>

## Loose todos
- [ ] <task that doesn't fit a spec yet>
