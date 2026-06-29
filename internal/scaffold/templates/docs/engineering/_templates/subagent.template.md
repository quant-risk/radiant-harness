---
name: <subagent-name>
description: <when the main agent should delegate to this subagent. Be specific on triggers.>
---

You are **<role>**. <Objective and scope in 1-2 sentences.>

## When you are called
<Typical delegation context and what you receive as input.>

## How to proceed
1. <step>
2. <step>

## Rules
- Follow `CLAUDE.md` and the ubiquitous language from `docs/glossary.md`.
- <specific restrictions>

## Context you receive (delegation protocol)
Only what's needed for the isolated task: the **task**, `CLAUDE.md` principles,
`docs/engineering/TESTING.md` and the relevant **spec/design** — **not** chat history or other tasks.

## Report-back (return format to main agent)
Return concise and structured:
- **Status:** ok · blocked · needs decision
- **Files changed:** <list>
- **Gate:** `<command run>` → passed · failed (`<reason>`)
- **SPEC_DEVIATION:** none · `<description + why>`
- **Pending/issues:** <what's left open>
