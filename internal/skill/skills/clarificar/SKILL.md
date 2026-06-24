# Skill: clarificar

> Sharpens ambiguous specs through structured interview. ONE
> question per turn. Each answer narrows the spec until ACs are
> testable.

## Decision tree

```
Ambiguity reported (by user or detected in spec)
        │
        ▼
Read the ambiguous AC verbatim
        │
        ▼
Is the phrase in glossary.md? ─── no ──► Add to glossary OR
        │                                rephrase AC
        ▼ yes
Is it measurable (can write a test assertion)? ─── yes ──► Rewrite AC
        │ no                                                 confirm with user
        ▼
Ask ONE question (the smallest disambiguating question)
        │
        ▼
Answer received
        │
        ├── Resolves the ambiguity ─► Rewrite the AC
        │
        └── Doesn't resolve ──► Ask follow-up (max 5 per session)
                                    │
                                    ▼
                              5 reached? ───► Escalate: spec is broken;
                                                  re-scope or split into
                                                  multiple features
```

## Workflow

### Step 1: read the ambiguity exactly as stated

Don't paraphrase. Quote the spec text, the user's words, or the
code line. The more precise the starting point, the fewer questions
needed.

### Step 2: check the glossary

If the ambiguous term is not in `docs/glossary.md`, that's a real
problem — the language isn't ubiquitous yet. Either:

1. Add the term to the glossary with a precise definition, OR
2. Rephrase the AC using terms already in the glossary.

Option 2 is faster; option 1 is more honest.

### Step 3: ask ONE question

The question should be the **smallest disambiguating question** —
the one that, once answered, makes the rest obvious. Bad questions:

- ❌ "What did you mean by 'fast enough'?" (too vague)
- ❌ "Should this be a feature or architecture?" (too broad)
- ❌ "Can you describe the full feature?" (too open)

Good questions:

- ✅ "When the spec says 'fast enough', do you mean p99 < 100ms,
  or p95 < 200ms, or something else?"
- ✅ "Is this change reversible in <1 day, or does it touch the
  database schema?"
- ✅ "When the user 'submits', do they click a button, or does
  pressing Enter also count?"

### Step 4: rewrite the AC

Once answered, rewrite the AC in Given/When/Then form. Show the user:

```markdown
### Before
AC: the system should be fast

### After
AC3: response time under load
- **Given** 100 concurrent users making requests
- **When** they hit POST /api/orders
- **Then** p99 latency is < 200ms (measured at the load balancer)
```

Don't apply the change silently. Ask: "Does this match what you
meant?"

### Step 5: continue or stop

If the new AC is clean: stop. Don't keep asking.

If new questions surface from the rewrite: loop back to step 3.
Hard cap: 5 questions in one clarificar session. After 5, escalate.

## Examples

### Example 1: ambiguous performance AC

**Input**: `ambiguity="AC: API responses are fast"` in
`specs/0003-search/spec.md`

**Q1**: "When you say 'fast', what's the threshold — p95 < 200ms,
p99 < 500ms, or something else? And under what load — 10 users,
100, or 1000 concurrent?"

**Answer**: "p95 < 200ms at 100 concurrent users."

**Rewrite**:
```markdown
### AC3: search latency under load
- **Given** 100 concurrent users searching the catalog
- **When** each sends a POST /api/search
- **Then** p95 response time is < 200ms
```

### Example 2: untestable verb

**Input**: `ambiguity="AC: graceful error handling"`

**Q1**: "When the API fails, what does 'graceful' look like to you
— a 503 status code, a user-visible error message in the UI, both,
or something else?"

**Answer**: "503 with a JSON body like {error: 'temporarily unavailable', retry_after: 30}."

**Rewrite**:
```markdown
### AC7: graceful upstream failure
- **Given** the upstream payment API returns 500
- **When** the user attempts checkout
- **Then** the response is 503 with body
  {"error":"temporarily unavailable","retry_after":30}
```

### Example 3: same AC, two meanings

**Input**: `ambiguity="AC: users can edit their profile"` (might mean
"change name" or "change email" or "change avatar")

**Q1**: "When you say 'edit their profile', which fields specifically
— name only, email only, all fields, or some subset?"

**Answer**: "Email and display name. Avatar comes later."

**Rewrite**:
```markdown
### AC2: profile editing (email + display name)
- **Given** the user is logged in
- **When** they PUT /api/profile with {email, display_name}
- **Then** the response is 200 with the updated profile

## Non-goals
- Avatar upload (separate spec planned)
```

## Anti-patterns

### ❌ Multi-question turns

```text
"Before I rewrite this, two questions:
 (1) What threshold?
 (2) Under what load?
 (3) Does p95 vs p99 matter?"
```

Three questions → three shallow answers. Ask one. Wait.

### ❌ "Best-guess" rewrites without confirmation

```text
"I rewrote AC3 to say 'p95 < 200ms at 100 users'. Hope that's right."
```

Wrong. Show the candidate. Wait for the user to confirm. They might
have meant p99, not p95. The 5 seconds of confirmation saves an
implementation iteration.

### ❌ Over-interviewing (more than 5 questions)

If after 5 questions the AC is still ambiguous, the spec is
fundamentally broken. Don't keep asking — escalate:

> "I've asked 5 questions and the AC is still ambiguous. This
> usually means the feature itself isn't well-defined. Should we
> re-scope it into smaller features, or pause this one entirely?"

### ❌ Editing without reading

Don't paraphrase the ambiguous AC. The exact words matter — the
ambiguity might be in a single word that the user actually meant
in a precise technical sense.

## Failure modes

| Gate | Failure | Recovery |
|------|---------|----------|
| `question-count` | >5 questions asked | Stop. Escalate to kickoff or split the feature. |
| `ac-resolved` | User says "I don't know" | Don't push. Either drop the AC (move to non-goals) or mark it as an open question in spec.md with owner + due date. |
| `ac-resolved` | User keeps changing answer | Take the LAST answer as the decision. Don't loop forever. Document in spec.md that this AC changed scope. |
| (no gate) | User wants to skip clarification and just ship | Warn them. If they insist, mark the AC as "ambiguous" in spec.md and proceed. Better to ship with a known gap than to ship a guess. |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `nova-feature` | Invoked when this skill finds ambiguities in newly-authored specs |
| `kickoff` | Invoked when ambiguities indicate the entire scope needs re-shaping |
| `validar` | Invoked after this skill resolves ACs, to confirm the spec is now complete |