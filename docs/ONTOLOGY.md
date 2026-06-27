# Ontology Layer (Sprint 41)

The ontology is the harness **world model**: a formal, explicit graph of the
concepts the harness reasons about, the relations between them, and the axioms
that constrain valid states.

## Why it exists

Before this layer, the harness's domain concepts were scattered across packages
as independent Go structs — `Task` defined twice ([types.go:65](../internal/types.go),
[fleet/store.go:23](../internal/fleet/store.go)), `Phase` three times — with no
shared semantic layer. Domain detection was heuristic classification (weights →
label), not ontological reasoning.

Research on agent harnesses converges on the same point: *grounding LLM
operations in a shared object ontology mitigates hallucination, loss of context,
and unverifiable reasoning.* The ontology is that shared object model — the
queryable "how do the project's concepts relate" that the agent consults instead
of re-deriving structure from filesystem guesses.

The Go structs in other packages remain the runtime **projections** of these
entities; `internal/ontology` is the semantic **schema** they all answer to.

## Model

### Entities (the nouns)

| Kind | Meaning |
|------|---------|
| `spec` | A specification — the unit of intended work |
| `task` | A unit of work derived from a spec |
| `gate` | A verification command that must pass |
| `skill` | A capability that governs one or more domains |
| `domain` | A project domain (finance, ml, backend, …) |
| `phase` | A loop state (discover, plan, execute, verify, persist, …) |
| `verdict` | The outcome a gate emits (approved / rejected) |
| `agent` | A worker that claims and executes tasks |
| `run` | A single loop execution |
| `artifact` | A file or output produced by work |

### Relations (the edges)

```
spec   -contains->    task
task   -verified_by-> gate
gate   -belongs_to->  domain
skill  -governs->     domain
phase  -produces->    artifact
agent  -claims->      task
task   -touches->     artifact
run    -executes->    phase
gate   -emits->       verdict
phase  -precedes->    phase     (loop state machine)
```

### Axioms (the rules)

1. **verify-before-persist** — a task reaches `persist` only through `verify`.
   The maker never grades its own work.
2. **every-skill-governs-a-domain** — no orphan skills.
3. **domains-exist** — every governed domain is a declared entity.
4. **terminal-phases** — `done` is terminal (no outbound transitions).

`radiant ontology validate` runs all axioms; the default schema has 0 violations.

## How it's consumed

- **Context Engine** — `OntologySkillsForDomain(domain)` selects skills by the
  `governs` relation. The heuristic `skillDomains` table in `registry.go` is
  cross-checked against the ontology by `TestRegistryMatchesOntology`, so the
  two can never drift — the ontology is the single source of truth.
- **Loop phase validation** — `ValidateTransition(from, to)` answers whether a
  phase move is legal by consulting `precedes` edges, the semantic equivalent of
  the hardcoded `validTransitions` table in `cycle.go`.
- **Boot / LLM hand-off** — `radiant boot --world-model` appends the compact
  world model (~300 tokens) so any LLM gets the project's semantic schema as its
  entry context, not just a file listing.

## CLI

```bash
radiant ontology export            # full outline (entities + relation counts)
radiant ontology export --compact  # ~300-token world model for LLM context
radiant ontology validate          # check axioms (0 violations = consistent)
radiant ontology skills <domain>   # skills that govern a domain (semantic routing)

radiant boot --world-model         # boot manifest + appended world model
```

## Compact world model (what the LLM sees)

```
HARNESS WORLD MODEL
entities: agent, artifact, domain, gate, phase, run, skill, spec, task, verdict
schema:
  spec -contains-> task
  task -verified_by-> gate
  gate -belongs_to-> domain
  skill -governs-> domain
  phase -produces-> artifact
  agent -claims-> task
  task -touches-> artifact
  run -executes-> phase
  gate -emits-> verdict
  phase -precedes-> phase
axioms:
  - a task reaches persist only through verify (maker never grades own work)
  - every skill governs at least one declared domain
  - every governed domain is a declared domain entity
  - done and failed are terminal (no outbound precedence except failed→retry)
```
