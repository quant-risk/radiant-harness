---
name: revisar-pr
description: SDD conformity gate on PR/MR — checks process, not bugs.
---

# Skill: Review PR/MR (SDD Conformity Gate)

Checks **process conformity** — not code quality or bugs. For bugs/style, use a separate code review.
This gate answers: "Did this change follow the SDD pipeline?"

## Phase 1 — Identify change scope (Research)

1. Get the PR/MR diff: `terminal: git diff main...HEAD --name-only` (or use MCP `mcp__github__*`).
2. Categorize changed files:
   - `src/**` → code changes (require spec traceability).
   - `docs/**` → doc changes (check if living docs need update).
   - `specs/**` → spec changes (check if approved).
   - `.claude/**`, `scripts/**` → infra changes (check if intentional).
3. If no `src/` files changed → skip to verdict (doc-only PRs need lighter checks).
4. Identify which spec(s) this PR belongs to: match branch name or changed files to `specs/NNNN-*/`.

> Context budget: only read the diff file list + the matching spec. Don't review every line of code.

## Phase 2 — SDD conformity checklist (Plan)

Run each check. Mark pass/fail with evidence:

### 2a. Spec exists and is approved
- [ ] The changed `src/` files map to a `specs/NNNN-*/spec.md`.
- [ ] That spec has acceptance criteria (Given/When/Then).
- [ ] No code changes exist without a corresponding spec (except `quick/` trivial tier).

### 2b. Traceability: every touched AC has a test
- [ ] For each `AC-N` in the spec, a test exists (check naming: `test_AC_N_*` or `AC-N:`).
- [ ] `search_files` in test directories for each AC ID — confirm coverage.
- [ ] Flag any AC touched by the diff but lacking a test.

### 2c. Gates green
- [ ] Run gate commands from `docs/engineering/TESTING.md`:
  ```
  terminal: <unit test command>
  terminal: <lint command>
  terminal: <static analysis command>
  ```
- [ ] All pass. CI status is green (check via `mcp__github__*` if connected).

### 2d. No open SPEC_DEVIATION
- [ ] `search_files` pattern `SPEC_DEVIATION` in the diff — any new deviations?
- [ ] If found, verify each has a resolution (code fixed or spec updated + ADR).

### 2e. ADRs for hard-to-reverse decisions
- [ ] If the PR introduces a new pattern, dependency, or architectural change → ADR exists in `docs/architecture/adr/`.

### 2f. Living docs and scope
- [ ] Glossary/context-map updated if language or boundary changed.
- [ ] Nothing from the spec's "Out of scope" section was implemented.

## Phase 3 — Verdict (Implement)

Produce a clear result:

**APPROVE** if all checks pass. Note minor suggestions as non-blocking comments.

**CHANGES REQUIRED** if any check fails. List each failure with evidence:
> ❌ AC-3 has no test — `search_files` found no `test_AC_3` in test dirs.
> ❌ Open SPEC_DEVIATION at `src/api/handler.ts:42` — code retries 5x, spec says 3x.
> ❌ No ADR for new dependency `stripe` (hard-to-reverse decision).

If MCP connected, offer to post the verdict as a comment on the PR/MR.

## Rules

- **Check process, not bugs.** Don't review code quality — that's a different review.
- **Evidence-based.** Every check cites the file or search result that proves it.
- **Scope-bound.** Only check what this PR changes. Don't expand to unrelated issues.
- If CI is still running, note it as pending — don't block on incomplete runs.
