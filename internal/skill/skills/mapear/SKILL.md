# Skill: mapear

> Map an existing codebase to bounded contexts, debts, and the
> radiant layout gap.

## Decision tree

```
Codebase path provided
        │
        ▼
Detect primary language + framework
        │
        ▼
Walk the directory tree (file counts, layer boundaries)
        │
        ▼
Identify bounded contexts (cluster by import graph)
        │
        ▼
Detect debts (missing tests, TODOs, deprecation warnings)
        │
        ▼
Compare to radiant layout → gap report
        │
        ▼
Write assessment.md + context-map.md
```

## Workflow

### Step 1: detect stack

Look at the file extensions, package files (`package.json`, `go.mod`,
`Cargo.toml`, `requirements.txt`), and CI configs. Identify:
- Primary language
- Framework (web framework, ORM, test framework)
- Build / run command

### Step 2: walk the tree

For each top-level directory, count files and identify the layer
(domain, application, infrastructure, presentation). Don't list
every file — show the shape.

### Step 3: cluster bounded contexts

Group directories into bounded contexts. Heuristics:
- Same prefix (`billing/`, `users/`, `inventory/`)
- Same owner (look at git log for that directory)
- Same external system they integrate with

### Step 4: detect debts

Search for:
- TODO/FIXME comments
- Files with deprecation warnings
- Missing test coverage (count test files vs source files)
- Circular dependencies (rough check)

Don't hide these. The map is a tool for decisions, not PR.

### Step 5: gap to radiant layout

Compare to the expected radiant structure:
- `AGENTS.md`?
- `.radiant-harness/state.md`?
- `docs/glossary.md`?
- `docs/architecture/context-map.md`?
- `specs/` with `spec.md` + `tasks.md`?

Note what's missing.

## Examples

### Example 1: brownfield Go service

**Output**: assessment.md reports Go 1.22, Gin, PostgreSQL,
3 bounded contexts (api, db, worker), 12 TODOs, 23% test coverage,
no radiant artifacts.

### Example 2: greenfield React app

**Output**: assessment.md reports TypeScript, Next.js 14,
Tailwind, 1 bounded context (the whole app — too coarse),
no TODOs but also no tests, no radiant artifacts.

## Anti-patterns

- ❌ Inventorying every file. The map is about shape.
- ❌ Hiding debts. They bite later.
- ❌ Imposing layout on day 1. Map first, propose deltas second.

## Failure modes

| Gate | Failure | Recovery |
|------|---------|----------|
| `stack-detected` | Unrecognized stack | List both candidates + reasoning; ask user. |
| `contexts-bounded` | Code has no clear boundaries | That's a real finding; mapear just discovered the project's biggest risk. |

## Related skills

- `kickoff` — uses mapear output for brownfield path
- `diagramar` — produces visual context-map from the same data
- `audit` — checks radiant-layout compliance separately