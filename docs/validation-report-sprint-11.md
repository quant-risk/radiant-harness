# Validation Report — Sprint 11 (v0.4.3)

**Date:** 2026-06-25
**Version:** 0.4.3
**Commit:** (pending)
**Sprint:** 11 — Discovery Phase Closure

## Scope

Three new commands completing the discovery phase of the
methodology merge documented in `docs/HARNESS-PLAN.md`:

| Deliverable | Skill that powers it | Status |
|---|---|---|
| `radiant adr "<decision>"` | `adr` | ✓ |
| `radiant update [--force]` | (CLI-native, no skill needed) | ✓ |
| `radiant diagramar <level>` | `diagramar` | ✓ |
| `skill.ExtractSkillTo(target, name, force)` | — | ✓ (helper) |
| `readFrontmatterVersion(path)` | — | ✓ (helper) |
| `generateAgentsMD()` | — | ✓ (helper) |

## Quality gates

```
$ go build ./...
(no output)

$ go vet ./...
(no output)

$ gofmt -l .
(no output)

$ CGO_ENABLED=0 go test ./... -count=1
?    github.com/quant-risk/radiant-harness/internal         [no test files]
?    github.com/quant-risk/radiant-harness/internal/scaffold [no test files]
ok   github.com/quant-risk/radiant-harness/cmd/radiant         0.195s
ok   github.com/quant-risk/radiant-harness/internal/benchmark  0.334s
ok   github.com/quant-risk/radiant-harness/internal/engine     0.483s
ok   github.com/quant-risk/radiant-harness/internal/harness    5.462s
ok   github.com/quant-risk/radiant-harness/internal/llm        4.938s
ok   github.com/quant-risk/radiant-harness/internal/policy     0.914s
ok   github.com/quant-risk/radiant-harness/internal/quality    1.067s
ok   github.com/quant-risk/radiant-harness/internal/skill      1.130s
ok   github.com/quant-risk/radiant-harness/internal/spec       1.121s

Test count: 230 PASS (up from 216 — +14 in Sprint 11)
```

## End-to-end verification

### `radiant adr`

```
$ ./bin/radiant init /tmp/test-sprint11 --all --yes
$ cd /tmp/test-sprint11
$ radiant adr "Use Postgres for jobs" --status=accepted
  ✓ created docs/architecture/adr/0002-use-postgres-for-jobs.md

$ cat docs/architecture/adr/0002-use-postgres-for-jobs.md | head -8
# 0002. Use Postgres for jobs

## Status

accepted

> Status transitions: proposed → accepted (when team agrees) →
```

- Numbering respects existing files (0001-record-architecture-decisions.md
  is the scaffold ADR; new one gets 0002).
- Nygard format: Status, Context, Decision, Consequences (Positive /
  Negative / Neutral).
- Status validation: invalid status falls back to `proposed`.

### `radiant update`

**Clean (no conflicts) path:**

```
$ radiant update --dry-run
  [unchanged] adr (local=1.0.0 bundled=1.0.0)
  [unchanged] auditar (local=1.0.0 bundled=1.0.0)
  ... (16 skills, all unchanged)
  [regenerate] AGENTS.md (always — review after update)

  Summary: 0 added, 0 updated, 0 conflict(s)

$ radiant update
  [regenerated] AGENTS.md

  Summary: 0 added, 0 updated, 0 conflict(s)
```

**Conflict path (forced by editing local frontmatter version):**

```
$ python3 -c "...edit local version to 0.9.0..."
$ radiant update
  [conflict] nova-feature (local=0.9.0 bundled=1.0.0) — pass --force to overwrite
  [regenerated] AGENTS.md

  Summary: 0 added, 0 updated, 1 conflict(s)
  Re-run with --force to overwrite local skill edits.

$ radiant update --force
  [updated] nova-feature (local=0.9.0 bundled=1.0.0)
  [regenerated] AGENTS.md

  Summary: 0 added, 1 updated, 0 conflict(s)
```

- Conflict detection works.
- `--force` correctly overwrites the changed skill only (not all 16).
- AGENTS.md always regenerated (per design).

### `radiant diagramar`

```
$ radiant diagramar container | head -10
# C4 Level 2 — Containers
#
# Break <Your System> into deployable units: web app, API, DB,
# background worker, etc. See the diagramar skill.

```mermaid
C4Container
    title Container diagram for <Your System>

    Person(user, "User", "A human who wants to <achieve goal>")

$ radiant diagramar bogus
Error: unknown level "bogus" — choose: context | container | component | code
```

- All 4 levels (`context`, `container`, `component`, `code`) produce
  valid C4-Mermaid templates.
- Unknown levels error with a helpful message.

## What this sprint unblocks

- **Sprint 12 (governance)**: can now build `radiant product` (Lean
  Inception) on top of `diagramar` (visuals for each phase) +
  `adr` (decisions during phase boundaries).
- **Sprint 13 (PR + views)**: `radiant update` is the foundation for
  auto-regenerating AGENTS.md + skill manifests when the CLI is
  upgraded via `go install` or Homebrew.
- **External integrations**: `readFrontmatterVersion` is now the
  canonical way to compare local vs bundled skill versions — reusable
  in any future tooling (e.g. a CI check that fails if the project's
  skill set is out of date with the CLI version).

## Open items / next sprint

- Sprint 12.1: `radiant product "<vision>"` — Lean Inception phases
  (Why/What/Who) producing `docs/product/`.
- Sprint 12.2: `radiant integrations list` — MCP discovery (deferred
  per HARNESS-PLAN.md).
- Sprint 13: native views auto-generation per agent (Claude/Cursor/
  Codex) on `--agent=<list>` opt-in.
- Consider promoting `readFrontmatterVersion` into `internal/skill/`
  for reuse (currently lives in `cmd/radiant/main.go` to avoid a
  YAML dependency in main).

## Decision: ready to commit

All quality gates green, all E2E paths verified, no regressions.
Commit and tag `v0.4.3`.