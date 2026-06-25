# Sprint 26 validation — 3 domain skills (ml, game, cli)

**Commit (this report):** TBD
**Re-validates:** `dc58ce3` (Sprint 25 — release --interactive)
**Version:** v0.6.3 (in source)

## What shipped

Three new domain-specific skills bundled with the CLI:

| Skill | Lines | Coverage |
|-------|-------|----------|
| `ml` | 541 | Problem framing, data card, baseline, model selection, eval protocol, reproducibility, deployment, monitoring, model card |
| `game` | 561 | Engine choice, game loop (fixed timestep), state machines, asset pipeline, perf budgets, multiplayer netcode, save/load, platform cert |
| `cli` | 603 | POSIX conventions, argument parsing, stdout vs stderr discipline, exit codes, configuration precedence, completion, distribution, man pages |

**Total: 1705 lines** across 6 files (3 frontmatter.yaml + 3 SKILL.md).

## Skill design

Each skill follows the established schema:

**frontmatter.yaml**:
- `name` (matches directory), `version` (semver)
- `description` + `when_to_use` (markdown pipes for multi-line)
- `tier_eligible` (closed set: Trivial/Feature/Architecture)
- `inputs` (typed; required flags)
- `outputs` (artifact paths with descriptions)
- `gates` (release-blocking criteria)
- `context_provides`, `commands_available`, `related_skills`
- `anti_patterns` (markdown-pipe list)
- `author`, `license`

**SKILL.md**:
- Decision tree (ASCII flowchart)
- Step-by-step workflow (Step 1 → Step N)
- Topic-specific tables (frame budgets, distribution channels, etc.)
- 2-3 worked examples (concrete project shapes)
- Anti-patterns (❌ markers)
- Failure modes (table: failure → recovery)
- Related skills (cross-references)

## Validation

| Gate | Result |
|---|---|
| `go build ./...` | clean |
| `go vet ./...` | clean |
| `gofmt -l .` | clean |
| `go test ./... -race` | 10 packages OK |
| `TestAllBundledSkillsValidateCleanly` | **24/24 skills pass** (was 21) |
| Tests | **337 PASS, 0 FAIL** |
| Data races | **0** |
| Cross-compile | **6/6** |

## Iteration discipline

Two issues caught + fixed in this sprint (both by the existing CI
guard `TestAllBundledSkillsValidateCleanly`):

1. **`type: list` in `cli` skill** — frontmatter used `type: list`
   for `target_platforms` but the schema only allows
   `string, number, enum, object, path`. Fix: changed to
   `type: string` with a description noting it's a
   comma-separated list.

2. **`type: integer` in `game` skill** — frontmatter used `type:
   integer` for `target_fps` but the schema only allows `number`.
   Fix: changed to `type: number`.

**This is exactly the role of the CI guard.** Schema drift is
caught at test time, not at user time. Both fixes were 1-character
edits. Cost: 2 build cycles.

## Domain coverage (post-Sprint 26)

| Domain | Skill |
|--------|-------|
| Product/process | nova-feature, nova-product, kickoff, roadmap, handoff, incident |
| Discovery/design | clarificar, validar, mapear, diagramar, adr, metricas |
| Quality/correctness | auditar, evals, revisar-pr, camada-agentica |
| Infrastructure | setup-ci, integracoes |
| **Domain — Mobile** | mobile (iOS / Android / cross-platform) |
| **Domain — Data** | data (data pipelines, warehouses) |
| **Domain — Frontend** | frontend (web apps) |
| **Domain — ML** | ml (NEW — ML/AI project lifecycle) |
| **Domain — Game** | game (NEW — game development) |
| **Domain — CLI** | cli (NEW — CLI tool design) |

**6 domain skills total.** Future candidates: `radiant-api`,
`radiant-security`, `radiant-blockchain`, `radiant-iot`.

## Final tally (post-everything)

- **21 CLI commands** + **24 bundled skills** (was 21, +3) + **1 open MIT schema spec**
- **337 tests passing**, 0 FAIL, 0 data races, 6/6 cross-compile
- **0 vendor-centrism, 0 hardcoded secrets, 0 global git config mutations**
- **`v0.6.0` tag exists** (dogfooded via `radiant release v0.6.0`)
- **`v0.6.3` in source**

## Stopping point

This is a clean stopping point. The skill catalog now covers:

- **6 process skills** (kickoff through incident)
- **6 discovery/design skills** (clarify through ADR)
- **4 quality/correctness skills** (audit through camada-agentica)
- **2 infrastructure skills** (setup-ci, integracoes)
- **6 domain skills** (mobile, data, frontend, ml, game, cli)

That's a complete catalog for shipping a serious project end-to-end.
The skill schema (`docs/SKILL-SCHEMA.md`) is the open spec; any
project can author skills in this format and ship them alongside.

Remaining candidates:
- More domain skills (`radiant-api`, `radiant-security`,
  `radiant-blockchain`, `radiant-iot`)
- Tag v0.6.3 for real via `radiant release v0.6.3` (pipeline
  is dogfooded + interactive confirmation)