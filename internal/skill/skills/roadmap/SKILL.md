---
name: roadmap
description: Build or review a Now/Next/Later roadmap with quick wins.
---

# Skill: Roadmap (Now / Next / Later)

Builds or reviews the project roadmap using Now/Next/Later horizons.
Principle: **low-risk quick wins first** for traction and team confidence.

## Phase 1 — Load context (Research)

1. `read_file` `docs/product/roadmap.md` (if exists — this is a review run).
2. `read_file` `docs/product/vision.md` — what we're building toward.
3. `read_file` `docs/architecture/assessment.md` (brownfield) — known debts and gaps.
4. Check `specs/` for in-progress features: `search_files` pattern `specs/*/spec.md`.
5. If MCP connected (Jira/Linear), fetch current sprint items and backlog for cross-reference.

> Delegate Jira/Linear fetches to a subagent. Only bring the summary into this context.

## Phase 2 — Identify candidates (Plan)

List all items that could appear on the roadmap:

1. **From vision/MVP:** features needed to reach MVP (greenfield).
2. **From assessment:** debts/gaps that block SDD adoption (brownfield).
3. **From specs in progress:** features currently being implemented.
4. **From backlog:** items the user or Jira/Linear reports as queued.

For each candidate, note:
- **Value** — what the team/user gains (business value or risk reduction).
- **Effort** — S/M/L rough estimate.
- **Dependencies** — what must be done first.
- **Risk** — low/med/high.

## Phase 3 — Prioritize into horizons (Plan)

Sort candidates into three horizons:

- **Now (current cycle):** items with highest value-to-effort ratio and low risk. Quick wins. Include any in-progress spec.
- **Next (next cycle):** items that depend on "Now" items or have medium effort.
- **Later (future):** large efforts, research items, nice-to-haves.

Rules for placement:
- Brownfield: **SDD adoption goes in Now** — it's the foundation for everything else. Specifically: fill `TESTING.md`, set up CI gates (`/setup-ci`), generate `context-map.md`.
- No item in "Now" without a defined "Done when" criteria.
- Maximum 5 items in "Now" — force prioritization.

## Phase 4 — Generate roadmap.md (Implement)

1. Fill `docs/product/roadmap.md` from `docs/product/_templates/roadmap.template.md`.
2. Each row: `| Item | Value | Effort | Owner | Dependencies | Done when |`
3. Write the roadmap objective (1-2 sentences: what we aim to achieve this period and how we measure).

## Phase 5 — Review with user

1. Present the roadmap table.
2. Ask: "Any item in the wrong horizon? Any missing candidate?"
3. Adjust based on feedback.
4. Update `docs/STATE.md`: roadmap reviewed, date.

## Rules

- **Idempotent:** re-running preserves existing items, updates status based on current `specs/` and progress.
- **Quick wins first.** Don't let a large refactoring block small wins that build momentum.
- Confirm with the user before committing the roadmap — it's a shared team artifact.
- Keep effort estimates rough (S/M/L). Detailed estimation happens in `/nova-feature` per item.
