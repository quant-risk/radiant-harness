# Skill: kickoff

> Project constitution. Greenfield discovery or brownfield
> mapping. Routes to the right artifacts per the tier system.

## Decision tree

```
User says "start a new project" or "set up radiant"
        │
        ▼
Does the directory have an existing codebase?
        │
        ├── no ──► GREENFIELD path
        │          - Interview: vision, personas, MVP
        │          - Generate: docs/product/vision.md, mvp-canvas.md
        │          - Draft: glossary.md (≥5 terms)
        │
        └── yes ──► BROWNFIELD path
                    - Map: existing code → context-map.md
                    - Capture: existing decisions as retro-ADRs
                    - Gap: what's missing vs the radiant layout
                    - Propose: roadmap to close the gap

        ▼
Connect MCPs (call integracoes if not done)
        ▼
Run first feature with nova-feature
```

## Workflow

### Greenfield path

1. Interview (one question at a time, like clarificar):
   - "What problem does this solve, and for whom?"
   - "Who are the 2-3 personas who'll use this most?"
   - "What's the smallest version that's still useful?"
2. Write `docs/product/vision.md` — one page: problem, personas,
   success metric.
3. Write `docs/product/mvp-canvas.md` — MVP scope with persona
   mapping.
4. Draft `docs/glossary.md` — 5+ terms from the interview.
5. Move to `nova-feature` for the first feature.

### Brownfield path

1. Run `radiant mapear` against the existing code.
2. Review the generated `docs/architecture/context-map.md`.
3. For each major architectural decision visible in the code
   (database choice, framework, auth pattern), create a retro-ADR
   in `docs/architecture/adr/`.
4. Compare the radiant layout vs what's present; identify gaps.
5. Generate a `docs/product/roadmap.md` to close the gaps over N
   sprints.
6. Move to `nova-feature` for the next feature.

## Examples

### Example 1: greenfield

**Input**: `mode="greenfield"`, `project-name="billing-api"`

**Outputs**:
- `docs/product/vision.md` — "billing-api lets SMBs accept payments
  without writing PCI-compliant code"
- `docs/product/mvp-canvas.md` — 3 personas, MVP features mapped
- `docs/glossary.md` — "merchant, charge, refund, payout, dispute"

### Example 2: brownfield

**Input**: `mode="brownfield"`, `project-name="legacy-monolith"`

**Outputs**:
- `docs/architecture/context-map.md` — existing code mapped to
  4 bounded contexts
- `docs/architecture/adr/0001-keep-postgres.md` — retro-ADR for
  the existing DB choice
- `docs/product/roadmap.md` — 5-sprint plan to introduce tests,
  then modularize

## Anti-patterns

- ❌ Skipping Lean Inception. "We already know what to build" is
  rarely true; vision interviews surface hidden constraints.
- ❌ Forcing greenfield on a brownfield. The code IS the source of
  truth; interview-driven discovery ignores it.
- ❌ Writing vision without personas. Vision is empty without
  who-it-serves.

## Failure modes

| Gate | Failure | Recovery |
|------|---------|----------|
| `mode-confirmed` | User keeps flip-flopping | Default: greenfield if directory is empty, brownfield otherwise. Ask user to confirm. |
| `stakeholders-named` | User can't name personas | Walk through recent customer interactions; personas emerge from real users, not theory. |
| `contexts-bounded` | Brownfield has no clear boundaries | This is a real finding; the kickoff just discovered the project's biggest risk. Don't fake it. |
| `language-ubiquitous` | Glossary has <5 terms | The project is too small to warrant radiant, OR the team hasn't agreed on language yet. Either is a valid finding. |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `nova-feature` | After kickoff finishes — start the first feature. |
| `integracoes` | Connect MCPs before kickoff interviews if available. |
| `mapear` | Used by brownfield kickoff for codebase analysis. |
| `diagramar` | Used to produce context-map.md visuals. |