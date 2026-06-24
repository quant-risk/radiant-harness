# Skill: nova-feature

> **Read first**. This skill starts a feature. It does NOT implement
> it. After the spec and tasks are produced and the DoR is green,
> close this context and open a fresh one for implementation
> (`radiant run`).
>
> Goal: produce `specs/<NNNN>-<slug>/spec.md` + `tasks.md` that a
> different agent (or future session) can implement without
> re-reading your conversation. The spec is the handoff.

## Decision tree

```
User intent arrives
        │
        ▼
Read state.md ─────────────────────► Already in flight?
        │                                 │
        ▼ no                              ▼ yes
Read glossary.md                       Surface existing feature
        │                                 If conflicts, ask user
        ▼                                 before continuing
Tier detection
        │
        ├── Architecture? ──► /kickoff brownfield; full esteira
        │                     (product.md → design.md → domain.md
        │                      → spec.md → tasks.md)
        │
        ├── Feature? ─────► spec.md + tasks.md; tier_decided gate
        │
        └── Trivial? ─────► quick/<NNN>-<slug>/TASK.md only
                            (1 file, no spec.md; < 10 minutes work)

        ▼
Spec authoring (one AC at a time)
        │
        ▼
Coverage check (1:1 AC ↔ task mapping)
        │
        ▼
DoR gate
        │
        ▼
Write state.md resume point → hand off to radiant run
```

## Workflow

### Step 1: orient (always, every invocation)

Before doing anything else:

1. Read `.radiant-harness/state.md`. If `current_feature` is set and
   it's NOT the same intent, surface this to the user — they may
   want to resume the existing feature instead of starting a new
   one.
2. Read `docs/glossary.md` so you use the project's ubiquous
   language in the ACs.
3. Read `docs/architecture/context-map.md` (if it exists) to
   understand which bounded context the feature belongs to.

### Step 2: tier detection

Ask 1-2 questions to determine tier. Do NOT silently default.

| Question | Tier it suggests |
|----------|------------------|
| Does it introduce a new bounded context or external integration? | architecture |
| Does it require changing >10 files or cross 2+ subsystems? | architecture or feature |
| Is it a bug fix or refactor with clear scope? | trivial or feature |
| Is it <3 files, well-understood, reversible? | trivial |

When in doubt: feature. Promote to architecture if a question
about reversibility surfaces ("we can't roll this back easily").

Record the tier decision in state.md under `current_feature.tier`.

### Step 3: scaffold the feature directory

Calculate the next sequence number:

```bash
ls specs/ | grep -E '^[0-9]{4}-' | sort | tail -1
# If empty: 0001
# Otherwise: increment the highest by 1
```

Derive slug from intent: kebab-case, ≤32 chars, ASCII-only.
Examples:
- "Add JWT authentication" → `0001-jwt-auth`
- "Refactor user model" → `0002-user-model-refactor`
- "Fix login redirect bug" → `0003-login-redirect`

Create `specs/<NNNN>-<slug>/` with the right artifacts per tier:

| Tier | Files |
|------|-------|
| trivial | `specs/<NNNN>-<slug>/TASK.md` (one file, 5-line max) |
| feature | `specs/<NNNN>-<slug>/spec.md` + `tasks.md` |
| architecture | full esteira: product.md → design.md → domain.md → spec.md → tasks.md |

For architecture, invoke skill `/kickoff` instead — it owns the
full brownfield path.

### Step 4: spec authoring (feature and architecture)

Write `spec.md` with this structure (don't skip sections):

```markdown
# <NNNN> — <short title>

## Why

[2-3 sentences: what problem this solves, who feels it, why now]

## What

[Description of the user-visible behavior]

## Acceptance criteria

### AC1: <name>
- **Given** <precondition>
- **When** <action>
- **Then** <observable result>

### AC2: <name>
...

## Non-goals

[Explicit list of what this feature does NOT do. Prevents scope creep
more than ACs prevent under-implementation.]

## Out of scope (for this iteration)

[Things that are adjacent but belong to a future spec.]
```

**Rules for ACs** (rule-test each one):

- ✅ "When user submits empty email, Then form rejects with message X"
- ❌ "The system should handle edge cases gracefully"
- ❌ "It should be fast"
- ❌ "It should work correctly"

If you can't write the Given/When/Then, invoke skill `/clarificar`.

### Step 5: tasks authoring

Write `tasks.md` as a markdown table. Required columns:

```markdown
| # | Task | Covers | Gate |
|---|------|--------|------|
| 1 | <verb phrase> | AC1, AC2 | `<command that runs and exits 0>` |
| 2 | ... | AC3 | `<command>` |
```

Coverage gate: every AC appears in at least one row's `Covers` column.
No AC without coverage. No task without a gate (use `true` for trivial
no-ops).

Gate command format: shell command, allowlisted (see
`internal/policy/`). The gate MUST actually run during `radiant run`;
non-executable gates fail the run.

### Step 6: DoR check

Definition of Ready — gate `dor-completed` passes only if:

- [ ] `spec.md` exists with non-empty ACs (≥1, all Given/When/Then)
- [ ] `tasks.md` exists with AC coverage (every AC mapped)
- [ ] Non-goals section present
- [ ] Glossary terms used in ACs match `docs/glossary.md`
- [ ] Tier recorded in `state.md`
- [ ] No unresolved open questions (or list them explicitly as
      "open questions" in spec.md)

If any item missing: tell the user, fix, re-check.

### Step 7: handoff to implementation

1. Update `.radiant-harness/state.md` with:
   - `current_feature: <NNNN>-<slug>`
   - `tier: <trivial|feature|architecture>`
   - `next_step: run`
   - `next_command: radiant run specs/<NNNN>-<slug> --model <model>`
2. Tell the user the spec is ready and the recommended next command.
3. **CLOSE THIS CONTEXT**. Do not implement here. Implementation
   goes through `radiant run` with a fresh context.

## Examples

### Example 1: trivial (refactor)

**Input**: `intent="rename User.email to User.email_address for clarity"`

**Tier decision**: trivial (single rename, no behavior change).

**Output**: `specs/0001-email-rename/TASK.md`:

```markdown
# 0001 — Rename User.email → User.email_address

**Why**: avoid confusion with User.work_email.

**Steps**:
1. Update schema migration to rename column
2. Update ORM model
3. Update all references

**Gate**: `go test ./... && go vet ./...`
```

**No spec.md**, no tasks.md table. 5-minute refactor.

### Example 2: feature (most common)

**Input**: `intent="add JWT auth so users can stay logged in across
restarts"`

**Tier decision**: feature (touches 1 subsystem, ~5-10 files, needs
tests).

**Output**: `specs/0002-jwt-auth/spec.md`:

```markdown
# 0002 — JWT authentication

## Why
Users lose their session on every server restart. JWT-based
stateless auth fixes this without server-side session storage.

## What
On login, server returns a signed JWT. Client stores it (httpOnly
cookie) and includes it in subsequent requests. Server validates
signature + expiry without DB lookup.

## Acceptance criteria

### AC1: valid login returns a JWT
- **Given** a user with valid credentials
- **When** POST /auth/login with email + password
- **Then** response is 200 with Set-Cookie: session=<jwt>; HttpOnly; SameSite=Strict

### AC2: invalid login returns 401
- **Given** wrong password
- **When** POST /auth/login
- **Then** response is 401 with body {"error": "invalid credentials"}

### AC3: expired JWT is rejected
- **Given** a JWT issued >24h ago
- **When** any authenticated request
- **Then** response is 401 with body {"error": "token expired"}

### AC4: tampered JWT is rejected
- **Given** a JWT with modified payload
- **When** any authenticated request
- **Then** response is 401 with body {"error": "invalid signature"}

## Non-goals
- Refresh tokens (out of scope this iteration)
- OAuth/SSO integration
- Password reset flow

## Out of scope
- Rate limiting on /auth/login
- Account lockout after N failures
```

**Output**: `specs/0002-jwt-auth/tasks.md`:

```markdown
| # | Task | Covers | Gate |
|---|------|--------|------|
| 1 | Add JWT lib (golang-jwt/jwt) | AC1, AC3, AC4 | `go build ./...` |
| 2 | Implement POST /auth/login | AC1, AC2 | `go test ./auth/...` |
| 3 | Implement JWT middleware | AC3, AC4 | `go test ./auth/...` |
| 4 | Add httptest for the auth flow | AC1, AC2, AC3, AC4 | `go test ./auth/... -v` |
```

**Handoff**: state.md updated, user told to run
`radiant run specs/0002-jwt-auth --model claude-sonnet-4.5`.

### Example 3: architecture (when to escalate)

**Input**: `intent="integrate with the new billing system"`

**Tier decision**: architecture (new external integration, new
bounded context, hard to reverse).

**Action**: invoke skill `/kickoff` (brownfield path). It will
produce `product.md → design.md → domain.md → spec.md → tasks.md`
sequentially with gates between each.

Do NOT try to skip directly to spec.md for an architecture tier —
the design and domain docs are what make a new bounded context
safe to commit to.

## Anti-patterns

### ❌ Skipping tier detection

```text
"User wants feature X" → writes spec.md directly.
```

Wrong. Tier detection is one or two questions. If you default to
"feature", an architecture-tier change ships without ADR. Cost of
asking: 30 seconds. Cost of not asking: re-doing the implementation.

### ❌ ACs that are not testable

```markdown
### AC1: system works correctly
### AC2: response is fast enough
### AC3: handles edge cases gracefully
```

These ACs cannot be written as Given/When/Then because "correctly",
"fast enough", and "edge cases" have no measurement. They will be
delivered as "I think it's done" instead of "the test passed".

### ❌ No non-goals

A spec without non-goals is a wishlist. Non-goals force the author
to think about scope. They prevent the implementer from adding
"while I'm at it" features that weren't asked for.

### ❌ Implementation in the planning context

If you implement while planning, your planning context fills with
implementation details. The spec loses the why. The next session
can't reuse the planning context for implementation because it's
polluted. Always close the planning context and open a fresh one.

### ❌ Coverage gaps in tasks

```markdown
| 1 | Add login | AC1 | `go test` |
| 2 | Add dashboard | (none) | `go test` |
```

AC2-AC5 have no task. The user will only notice when an AC fails
in production. Every AC needs at least one task; ideally 1:1.

### ❌ Slash commands as the only entry point

This skill is invoked by humans via `radiant spec <intent>` (CLI),
by agents via reading this file directly, or by name lookup in any
tool that understands the skill schema. Not via `/nova-feature` —
that's a Claude-Code convention, not a radiant one.

## Failure modes

| Gate | Failure | Recovery |
|------|---------|----------|
| `tier-decided` | User can't decide / keeps changing mind | Offer 2-question rubric: "Does it cross a subsystem boundary? Can you roll it back in <1 day?" Use answers to pin tier. |
| `spec-testable` | ACs are vague | Invoke `/clarificar` to interview user. One question at a time. |
| `ac-coverage` | Tasks don't reference all ACs | Add missing tasks. Don't drop ACs — if an AC is dropped, document it as non-goal. |
| `dor-completed` | Glossary terms don't match | Update glossary.md (propose, ask user) OR rephrase ACs to match existing glossary. |
| `dor-completed` | Open questions remain | Either resolve them or list them as "open questions" in spec.md with owner + due date. |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/kickoff` | Tier is architecture. /kickoff owns the brownfield full-esteira path. |
| `/clarificar` | Spec has ambiguous ACs. /clarificar runs the structured interview. |
| `/validar` | After implementation. /validar runs the DoD check. |
| `/integracoes` | Before starting, if MCPs are not yet connected — /integracoes discovers them. |