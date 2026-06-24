# Skill: handoff

> Pause/resume session via state.md. Two modes: pause (writes
> current position) and resume (reads and summarizes).

## Decision tree

```
mode arrives
        │
        ├── pause ──► Capture: feature, tier, next_command,
        │                blockers, open_questions
        │             Write state.md (atomically — temp + rename)
        │             Print: "Resume by running: <next_command>"
        │
        └── resume ──► Read state.md
                       │
                       ├── present ──► Print summary, suggest
                       │                next_command
                       │
                       └── missing ──► Ask user what's in flight
```

## Workflow

### Pause

1. Determine current position:
   - `current_feature`: which `specs/<NNNN>-<slug>/` is open?
   - `tier`: trivial / feature / architecture?
   - `next_command`: literal `radiant <subcmd> ...` to resume
   - `blockers`: who/what is blocking?
   - `open_questions`: things awaiting user input
2. Write state.md with all fields.
3. Print the resume command to the user.

### Resume

1. Read state.md.
2. Verify it parses (frontmatter-style Markdown).
3. Print:
   - "Last session: <date> — <summary>"
   - "Current feature: <NNNN>-<slug> (tier=<tier>)"
   - "Next command: <next_command>"
   - "Blockers: <list>"
4. If `next_command` is non-empty, suggest running it.

## Examples

### Example 1: pause mid-feature

**Input**: `mode="pause"`, `note="blocked on PR review"`

**state.md produced**:
```markdown
# State

## Current position
- current_feature: 0002-jwt-auth
- tier: feature
- next_command: radiant run specs/0002-jwt-auth --continue
- blockers:
  - "PR review on auth middleware (assigned to alice)"
- open_questions:
  - question: "Should we add rate limiting to /auth/login?"
    owner: alice
    due: 2026-06-30
- last_session: 2026-06-24
- last_updated: 2026-06-24T22:30:00Z
```

### Example 2: resume

**Input**: `mode="resume"`

**Output**:
```
Last session: 2026-06-24
Current feature: 0002-jwt-auth (tier=feature)
Next command: radiant run specs/0002-jwt-auth --continue
Blockers:
  - PR review on auth middleware (assigned to alice)
Run the next command? [Y/n]
```

## Anti-patterns

- ❌ Closing window without handoff. Next session has to guess.
- ❌ Vague next_step ("continue"). Be specific: literal command.
- ❌ Forgetting blockers. If you're stuck, say so.

## Failure modes

| Gate | Failure | Recovery |
|------|---------|----------|
| `state-parsed` | state.md is malformed | Offer to repair from defaults. Don't silently rewrite. |
| `next-action-explicit` | next_command is empty | Ask user: what should the next session do? |

## Related skills

- `nova-feature` — typically called after handoff resumes a feature
- `metricas` — uses state.md timestamps for Lead Time measurement |