---
name: handoff
description: Pause or resume sessions via STATE.md context handoff.
---

# Skill: Session Handoff (Pause / Resume)

Maintains project continuity across sessions via `docs/STATE.md`.
PAUSE saves state; RESUME reconstructs context and proposes the next step.

## PAUSE mode — end or pause a session

### Phase 1 — Capture current state

1. Identify what was being worked on: which feature (`specs/NNNN-*/`), which phase (research/plan/implement/validate).
2. Record progress: `search_files` in the feature's `tasks.md` for completed vs pending tasks.
3. Note the **next concrete action** — specific and atomic. Not "continue the feature."
   Good: "Implement AC-3 in the Stripe adapter — the spec is at `specs/0003-payment/spec.md`."
   Bad: "Keep working on payments."

### Phase 2 — Update STATE.md

Write to `docs/STATE.md`:

1. **Active feature:** `specs/NNNN-<name>/` — current phase.
2. **Next concrete action:** the single next step (see above).
3. **Decisions log:** any decisions made this session. Hard-to-reverse → note "needs ADR."
4. **Blockers:** what's blocking, who/what unblocks it, since when.
5. **Context bookmarks:** files/sections important for resuming — saves re-discovery time.
   - `<path/to/file>` — why it matters for the current work.
6. **Deferred ideas:** ideas not acted on, with a trigger to reconsider.
7. **Date + author:** `Last updated: YYYY-MM-DD by <name>`.

### Phase 3 — Commit

1. Commit `docs/STATE.md` (and any spec progress) with message `chore: handoff — pause session`.
2. Confirm to the user: "State saved. Next session, run `/handoff` to resume."

---

## RESUME mode — start a new session

### Phase 1 — Recompose context

1. `read_file` `docs/STATE.md` — the volatile working memory.
2. `read_file` `docs/product/vision.md` — why we're building this.
3. `read_file` `docs/product/roadmap.md` — current priorities.
4. `read_file` the active feature's `spec.md` — the contract.
5. Read any files listed in STATE.md "Context bookmarks."
6. Check for unread decisions: `read_file` any ADRs created since last session.

> Context budget: load only STATE + vision + roadmap + active spec + bookmarks. Stay under 15k tokens. Everything else is on-demand.

### Phase 2 — Summarize

Present a concise summary:
```
## Resuming — <feature name>

**Last session:** <date>
**Phase:** <research | plan | implement | validate>
**Progress:** <X/Y tasks done>

**Next action:** <the concrete next step from STATE.md>

**Open decisions:** <any from STATE.md decisions log>
**Blockers:** <any from STATE.md blockers>
```

### Phase 3 — Propose next step

1. Based on the next action and current phase, propose the immediate step:
   - Phase `research` → "Continue research on <X>, or move to planning?"
   - Phase `plan` → "Spec is at AC-N, want to review before implementation?"
   - Phase `implement` → "Next task is <X>. Gate command is <Y>. Shall I proceed?"
   - Phase `validate` → "Ready to run /validar on feature NNNN?"
2. **Confirm before executing.** Don't auto-start work — the user may have different priorities.
3. After confirmation, open a fresh context window for implementation if entering that phase.

## Rules

- **STATE.md is volatile memory.** Update constantly during PAUSE — don't rely on recall.
- **ADR is durable memory.** Structural decisions go to ADR, not just STATE.
- **Be specific.** "Next action" must be atomic and point to exact files. Vague next steps waste the next session.
- **Context bookmarks save time.** List the 2-3 files that matter most for resuming.
- On RESUME, if STATE.md is stale (> 7 days), flag it: "STATE.md hasn't been updated since <date>. Is it current?"
