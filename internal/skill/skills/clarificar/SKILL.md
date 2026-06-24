---
name: clarificar
description: Relentless interview to sharpen spec/design before building.
---

# Skill: Clarify (Relentless Interview)

Interviews to sharpen a plan, spec, or design **before** building.
Transforms diffuse intent into shared, testable understanding. One question at a time.

## Principles

- **One question at a time.** Ask, **wait for the answer**, then ask the next.
- **Walk the decision tree.** Each answer opens or closes branches. Resolve dependencies in order.
- **Always propose a recommended answer.** Don't interrogate in a vacuum — show your reasoning.
- **Explore before asking.** If the answer is in the codebase, docs, or MCP, discover it yourself first.
- **Dig until testable.** "Fast" or "secure" doesn't close a branch. Define the measurable threshold.

## Phase 1 — Frame the target (Research)

1. Identify what needs clarification: a spec? a design decision? a feature request?
2. `read_file` the relevant doc (`spec.md`, `design.md`, `vision.md`, or the user's raw request).
3. Build a mental decision tree: what decisions are blocking? What depends on what?
4. Note what you already know from the codebase/docs — don't ask about those.

> Context budget: read only the target doc + the most directly referenced files. Keep under 20k tokens so you have room for the interview.

## Phase 2 — Interview (one question at a time)

For each unresolved branch in priority order:

1. **Explore first:** `search_files` the codebase or `read_file` relevant code to see if the answer already exists.
2. **Formulate one question** with a recommended answer:
   > "Should the retry policy be exponential backoff or fixed interval?
   > I recommend exponential backoff (base 2, max 30s) because we call rate-limited APIs.
   > The codebase already uses this pattern in `src/infrastructure/http-client.ts`."
3. **Wait for the response.** Do not ask the next question until this one is answered.
4. **Record the answer.** Note whether it closes the branch or opens new ones.
5. **Check testability.** If the answer is vague ("make it robust"), push for a concrete threshold:
   > "What does 'robust' mean here? Can we define it as: survives 100 concurrent requests
   > with p99 latency under 500ms?"

## Phase 3 — Consolidate (Plan)

1. Summarize all decisions in a table:

| # | Decision | Recommended | Chosen | Testable? |
|---|----------|-------------|--------|-----------|
| 1 | Retry policy | Exponential backoff | Exponential | Yes — max 3 retries, 30s cap |

2. Flag any branch that remains unresolved — these are blockers for the spec.
3. If clarifying for a feature, update `specs/NNNN-<name>/spec.md` ACs with refined thresholds.
4. If clarifying for a design, update `design.md` with the decisions.

## Phase 4 — Handoff

1. Save decisions to `docs/STATE.md` decisions log.
2. If hard-to-reverse decisions were made, propose creating ADRs.
3. Offer to write results directly into the relevant spec/design file.
4. Propose next step: "Spec is now testable. Run `/nova-feature` Phase 2 to formalize?"

## Rules

- **Never ask more than one question at a time.** Multi-part questions get partial answers.
- **Always propose.** A bare question forces the user to do your job. Show your work.
- **Never ask what you can discover yourself.** Check codebase, docs, MCPs first.
- **Vague answers don't close branches.** Push until every decision has a testable criterion.
- If the interview exceeds ~15 questions, save progress to STATE.md and propose a follow-up session.
