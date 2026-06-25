# Skill: nova-product

> **Read first**. This skill starts a product. It does NOT
> implement features. After the inception document is produced
> and the MVP scope is cut, close this context and use
> `nova-feature` for each MVP feature in a fresh context.
>
> Goal: produce `docs/product/inception.md` + `docs/product/personas.md`
> that the team can use to align before any code is written. The
> MVP cut at the end of the inception is the source of truth for
> which features go into `specs/`.

## Decision tree

```
Product vision arrives
        │
        ▼
Read state.md ────────────────► Inception already in flight?
        │                              │
        ▼ no                           ▼ yes
Read glossary.md                    Surface existing inception
        │                            If conflicts, ask user
        ▼                            before continuing
Phase 1: Why (vision, problem, who)
        │
        ▼
Phase 2: What (feature list, untagged)
        │
        ▼
Scope triage (MVP / Growth / Vision)
        │
        ▼
Phase 3: Who (personas, jobs-to-be-done)
        │
        ▼
Phase 4: How (technical / business approach)
        │
        ▼
Phase 5: When (roadmap, MVP timing)
        │
        ▼
Phase 6: Where (context-map, bounded contexts)
        │
        ▼
MVP cut (3-7 features max)
        │
        ▼
Hand off to nova-feature per MVP feature
```

## Workflow

### Step 1: orient (always)

Before doing anything:

1. Read `.radiant-harness/state.md`. If `current_product` is set
   and it's NOT the same vision, surface this — the user may
   want to resume the existing inception instead of starting a
   new one.
2. Read `docs/glossary.md` so the team uses consistent
   ubiquitous language from day one.
3. Read `docs/architecture/context-map.md` if it exists. This
   tells you which bounded contexts are already in place (for
   the Where phase) and which would be new.

### Step 2: scaffold the inception directory

```bash
mkdir -p docs/product
ls docs/product/inception.md 2>/dev/null || echo "fresh"
```

If `inception.md` already exists and state.md points to it,
resume. Otherwise create a fresh template.

### Step 3: phase 1 — Why

Ask the user 3-5 questions in sequence (one at a time):

1. "Who is this product for?" (specific persona, not "everyone")
2. "What job do they hire this product to do?" (Clayton
   Christensen's jobs-to-be-done framing)
3. "What do they do today instead?" (the alternative you're
   replacing)
4. "Why now? What changed that makes this urgent?"
5. "How will you know the product succeeded?" (one metric)

Fill in the Why section of inception.md from these answers.
The vision line must fit this template:

> "We help `<persona>` do `<job>` better than `<alternative>`."

Anything that doesn't fit is not yet a vision — go back to
question 1.

### Step 4: phase 2 — What (feature brainstorm)

Brainstorm features WITHOUT judging scope. The user lists every
feature they imagine. Don't filter at this point.

A useful prompt: "What would your power user expect on day 1?
What would they expect by month 6? What about year 2?"

Record all features as bullet points in the What section,
untagged.

### Step 5: scope triage (MVP / Growth / Vision)

Walk the user through each feature and tag it:

| Tag | Definition | When it ships |
|-----|------------|---------------|
| **MVP** | The smallest set that delivers the Why. New user cannot succeed without it. | First release. |
| **Growth** | What you add once MVP proves the Why works. | 1-3 months after MVP. |
| **Vision** | The end state. Aspirational. Often changes shape based on what you learn. | 6+ months. |

For each feature, ask: "If we cut this, can a new user still
get value from the product on day 1?"

- Yes → Growth or Vision.
- No → MVP.

If more than 7 features are MVP, the user is scope-creeping.
Re-visit the Why — usually the problem is not clear enough, so
the user is trying to cover every angle.

### Step 6: phase 3 — Who (personas)

For each persona, write a paragraph in `personas.md`:

```markdown
## <Persona name>

<One sentence: who they are, where they work, what tools they
currently use.>

**Job to be done**: <what they're trying to accomplish when
they find this product.>

**Pain today**: <the cost of the current alternative.>

**Success looks like**: <how they measure whether the product
helped.>
```

Usually 2-4 personas. More than that = no focus.

### Step 7: phase 4 — How

Write 1-2 paragraphs: technical approach, business model, GTM,
or whatever "How" means for this product. Keep it short — this
is not a design doc, it's the strategic answer.

If the technical approach introduces new bounded contexts or
external integrations, flag them — they'll become the "Where"
phase.

### Step 8: phase 5 — When

Roadmap as a Gantt-style 3-line sketch:

```markdown
## When

| Quarter | Milestone | Scope |
|---------|-----------|-------|
| Q1      | MVP       | <list MVP features> |
| Q2      | Growth    | <list Growth features> |
| Q3+     | Vision    | <list Vision features> |
```

If the user can't articulate MVP timing, the scope is too big.
Cut until MVP fits in `mvp_weeks` (default 8).

### Step 9: phase 6 — Where (context-map)

For each bounded context, one line:

```markdown
| Context | Type | Notes |
|---------|------|-------|
| <name>  | new / existing | <one-line description> |
```

If most contexts are "new", the product is mostly greenfield —
expect a longer architecture phase. If most are "existing", the
product fits into a current system — scope accordingly.

### Step 10: MVP cut

Final section of inception.md:

```markdown
## MVP cut

The 3-7 features we ship first (in priority order):

1. <feature> — covers `<persona>`'s `<top job>`
2. <feature>
3. <feature>

Each becomes a spec under `specs/<NNNN>-<slug>/` via the
nova-feature skill. Do NOT bundle multiple MVP features into
one spec — one feature per spec so each can ship independently.
```

### Step 11: handoff to feature specs

1. Update `.radiant-harness/state.md`:
   - `current_product: <inception slug>`
   - `mvp_features: <count>`
   - `next_step: nova-feature`
   - `next_command: radiant spec "<MVP feature 1>"`
2. Tell the user: "Inception done. For each MVP feature, open a
   fresh context and run `radiant spec <feature>`."
3. **CLOSE THIS CONTEXT**. Don't start spec'ing in the
   inception context — that's how vision bleeds into details.

## Examples

### Example 1: greenfield B2B SaaS

**Input**: `vision="API observability for small dev teams"`,
`mvp_weeks=6`

**Why**: "We help backend engineers on teams of 3-10 debug
latency spikes without paying Datadog's per-host pricing. They
do this today by manually grep'ing logs and re-running k6
scripts."

**What (untagged)**:
- HTTP request tracing
- Latency percentile dashboards
- Error rate alerts
- Slack integration
- Custom query language
- Team workspaces
- SSO

**Scope triage**:
- MVP: HTTP request tracing, latency dashboards, error alerts
- Growth: Slack integration, team workspaces
- Vision: Custom query language, SSO

**MVP cut** (3 features): ships in Q1, 6 weeks.

**Personas**: "Solo backend dev at a startup" + "Tech lead at
a 10-person team".

**Where**: all new contexts (`tracing`, `analytics`,
`alerting`). Expect architecture sprint after inception.

### Example 2: internal tool

**Input**: `vision="dashboard for our support team to see
ticket SLA breaches in real time"`

**Why**: "We help support managers at our company see which
tickets are about to breach SLA before they do, so they can
reassign proactively. They do this today by exporting a CSV
every morning."

**What (untagged)**:
- Live SLA dashboard
- Email digest of at-risk tickets
- Slack alert on breach
- Reassignment workflow
- Manager drill-down

**Scope triage**:
- MVP: Live SLA dashboard + reassignment
- Growth: Slack alert
- Vision: Email digest

**MVP cut** (2 features): 2-week build.

**Personas**: "Support manager (single persona)".

**Where**: mostly existing — reads from the support DB, writes
back via existing API. Brownfield path; minimal new contexts.

## Anti-patterns

### ❌ Vision that names no persona

"We make the world more productive." → not a vision. No team
can ship against this. Always force the persona + job framing.

### ❌ MVP with 12+ features

If the MVP can't fit on one screen, it's not an MVP. Cut.
The product that ships beats the product that's "almost ready".

### ❌ Skipping the personas.md output

Personas are how you decide the MVP is RIGHT, not just SHIPPED.
"Build for everyone" is "build for no one". 2-4 personas, max.

### ❌ Jumping to spec during inception

If you spec a feature during inception, you optimise for the
feature instead of the product. Always finish inception first;
specs come after, in their own contexts.

### ❌ Forgetting the When phase

A roadmap with no dates is a wishlist. Even rough quarters are
better than nothing. Forces honest triage.

## Failure modes

| Gate | Failure | Recovery |
|------|---------|----------|
| `vision-clear` | Vision is vague or aspirational | Ask "who specifically? what job? what alternative?" — one at a time. |
| `scope-triaged` | Features not tagged | Walk each one through the MVP test: "cut this, can a new user succeed?" |
| `mvp-cut` | MVP > 7 features | Force-rank. Bottom half goes to Growth or Vision. |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/nova-feature` | After inception. For each MVP feature, run nova-feature in a fresh context. |
| `/kickoff` | If MVP includes architecture-tier features. kickoff owns the brownfield path. |
| `/clarificar` | If the vision doesn't fit the persona + job template. |
| `/roadmap` | After inception, to flesh out the When phase into a quarter-by-quarter plan. |
| `/mapear` | After Where phase, to build the actual context-map (C4 Level 1). |