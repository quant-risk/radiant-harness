---
name: setup-ci
description: Create or adjust CI/CD pipeline that materializes SDD gates.
---

# Skill: Setup CI/CD (SDD Gate Materialization)

Creates or adjusts the CI/CD pipeline so that SDD gates run on every PR/MR.
Detects the CI provider and generates the appropriate pipeline file. **Idempotent.**

## Phase 1 — Detect CI provider (Research)

1. Check for existing CI config:
   - `search_files` pattern `.github/workflows/*.yml` → **GitHub Actions**.
   - `read_file` `.gitlab-ci.yml` → **GitLab CI**.
   - `read_file` `Jenkinsfile` → **Jenkins**.
2. If none found, ask the user which provider to use. Default: GitHub Actions.
3. `read_file` `docs/engineering/TESTING.md` — extract gate commands (unit, integration, lint, static analysis, coverage).
4. Check `package.json` scripts or equivalent for the actual test/lint/build commands.

> If TESTING.md is empty, stop and ask the user to fill it first. CI mirrors TESTING.md.

## Phase 2 — Define pipeline stages (Plan)

Map SDD gates to CI stages. Failure in any stage **blocks merge**:

| CI Stage | Gate from TESTING.md | Blocking? |
|----------|---------------------|-----------|
| `lint` | `<lint command>` | Yes |
| `typecheck` | `<type-check command>` | Yes |
| `test-unit` | `<unit test command>` | Yes |
| `test-integration` | `<integration test command>` | Yes |
| `static-analysis` | `<SAST command>` (semgrep/codeql) | Yes (blocking findings only) |
| `coverage` | `<coverage command>` (min X%) | Yes if below threshold |
| `sdd-audit` | `node scripts/audit-esteira.mjs .` | Yes |

2. Determine trigger: `on: pull_request` (gate before merge) + `on: push: main` (deploy stage).
3. Determine deploy stage: does the project auto-deploy? If yes, add deploy after gates pass.

## Phase 3 — Generate pipeline file (Implement)

### GitHub Actions (`.github/workflows/ci.yml`)

```yaml
name: CI
on: [pull_request]
jobs:
  gates:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with: { node-version: '20' }
      - run: npm ci
      - run: <lint command>
      - run: <typecheck command>
      - run: <unit test command>
      - run: <coverage command>
      - run: <SAST command>
      - run: node scripts/audit-esteira.mjs .
```

4. Adapt commands to the detected stack (Node, Go, Python, etc.). Use actual commands from TESTING.md — don't invent.
5. Add dependency caching to speed up runs.
6. Publish coverage report and static analysis as PR artifacts.

## Phase 4 — SDD audit gate

1. Ensure `scripts/audit-esteira.mjs` exists. If not, generate a minimal version that checks:
   - Every `src/` change has a corresponding `specs/` entry.
   - No code without spec (except `quick/` tier).
2. Add as the final CI stage — it's the SDD conformity gate.

## Phase 5 — Validate and document

1. Commit the pipeline file. Trigger a test run (push a branch, open a draft PR).
2. Verify all stages pass. Fix any command errors.
3. Update `docs/engineering/TESTING.md`: note "CI runs these exact commands."
4. Record pipeline choice as **ADR** if structural.
5. Update `docs/STATE.md`: CI pipeline configured.

## Rules

- **No secrets in pipeline files.** Use `${{ secrets.X }}` (GitHub) or `$CI_*` (GitLab) variables.
- **CI mirrors TESTING.md.** If you add a gate to TESTING.md, add it to CI. Keep them in sync.
- Confirm with the user before enabling auto-deploy. Manual deploy is safer as default.
- Confirm before generating — this affects every future PR.
