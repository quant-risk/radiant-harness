# Skill: setup-ci

> Generate CI workflow that enforces radiant gates on every PR.

## Decision tree

```
CI setup requested
        │
        ▼
Which provider?
        │
        ├── github ──► .github/workflows/esteira.yml
        │
        ├── gitlab ──► .gitlab-ci.yml
        │
        └── circleci ──► .circleci/config.yml
        │
        ▼
Gates: validate, audit, tests, build
Secrets: API keys via provider's secret store
```

## Workflow

### Step 1: detect provider

Check for `.github/`, `.gitlab-ci.yml`, `.circleci/`. Default to
detected provider; user can override.

### Step 2: write the workflow

For GitHub Actions, the workflow runs:
- `radiant validate specs/` on every PR
- `radiant audit` on every push to main
- Project tests (npm test / go test / etc.)

### Step 3: configure secrets

Document which secrets the user needs to set:
- `OPENROUTER_API_KEY` (or `OPENAI_API_KEY`, etc.)
- `GITHUB_TOKEN` (auto-provided by Actions)

NEVER hardcode secrets in the workflow YAML.

## Examples

### Example 1: GitHub Actions

**Output**: `.github/workflows/esteira.yml` running `radiant
validate` on every PR, `radiant audit` on every push to main.

## Anti-patterns

- ❌ Hardcoded secrets.
- ❌ Skipping radiant gates.
- ❌ CI only on push (bypasses review).

## Failure modes

| Gate | Failure | Recovery |
|------|---------|----------|
| `gates-present` | Workflow missing a gate | Add it. |
| `secrets-referenced` | Hardcoded secret | Replace with `${{ secrets.NAME }}`. |

## Related skills

- `auditar` — runs as a CI gate
- `validar` — runs as a CI gate