# Radiant Harness — Sprint 10 Validation Report (Batches 1 + 2 Combined)

**Date**: 2026-06-24
**Commits validated**: `f0f4546` (batch 1) + `b98e503` (batch 2)
**Versions**: `0.4.0` → `0.4.1`
**Trigger**: User asked to study video research notes (`/Users/henrique/Documents/Codex/2026-06-24/vou/outputs/notas-detalhadas-videos-youtube.md`) before continuing. Findings were synthesized; this report captures validation of the work shipped before that study.

---

## Build & Test

| Check | Result |
|-------|--------|
| `go build ./...` | ✓ zero errors |
| `go vet ./...` | ✓ zero warnings |
| `gofmt -l .` | ✓ no unformatted files |
| `go test ./... -race -count=1` | ✓ all 8 packages pass |
| Test count | ✓ 208 passing (was 188 in `a9614b7`, +20 new) |
| Race conditions | ✓ zero |

## Cross-Compile (6 OS/arch targets)

| Target | Status |
|--------|--------|
| linux/amd64 | ✓ |
| linux/arm64 | ✓ |
| darwin/amd64 | ✓ |
| darwin/arm64 | ✓ |
| windows/amd64 | ✓ |
| windows/arm64 | ✓ |

All 6 binaries carry version `b98e503` embedded via `-ldflags "-X main.version=$(git describe)"`.

---

## Sprint 10 First Batch (`f0f4546`) — Skill runtime + nova-feature

### What shipped

| Deliverable | Status |
|-------------|--------|
| `internal/skill/` package (schema validator + Load/Validate/LoadFromFS) | ✓ |
| `Bundle()` enumerates embedded skills | ✓ |
| `ExtractTo(target, force)` writes bundle to disk | ✓ |
| `//go:embed all:skills` bundling | ✓ |
| `nova-feature` skill rewritten top-of-line | ✓ |
| `radiant skills list` CLI command | ✓ |
| `radiant skills validate <dir>` CLI command | ✓ |
| 19 dedicated skill tests | ✓ |

### Quality observations

- Schema enforces all 10 rules from `docs/SKILL-SCHEMA.md` §6 (single source of truth for skill contract)
- Comma-ok form for map lookups (NOT `!= struct{}{}` — bug found in original sprint 9 code, now consistently fixed)
- `Bundle()` skips legacy skills without `frontmatter.yaml` (placeholder pending rewrite)
- `Bundle()` fails closed on invalid skills (never ship broken skills in binary)

---

## Sprint 10 Second Batch (`b98e503`) — 16 skills rewritten top-of-line

### What shipped

**16 skills** now conform to `docs/SKILL-SCHEMA.md` (open spec, MIT):

```
nova-feature, clarificar, validar, kickoff, handoff,
integracoes, mapear, diagramar, adr, revisar-pr,
auditar, metricas, setup-ci, camada-agentica,
evals, roadmap
```

Each skill has:
- `frontmatter.yaml` with full schema (name, version, description, when_to_use, tier_eligible, inputs, outputs, gates, context_provides, commands_available, related_skills, anti_patterns, author, license)
- `SKILL.md` with all 6 recommended sections (Decision tree, Workflow, Examples, Anti-patterns, Failure modes, Related skills)
- Zero Claude-centrism (no `CLAUDE.md` references, no slash commands as primary entry)
- Universal: parseable by any LLM that reads YAML + Markdown

### Quality observations

- **Top 6 skills** (nova-feature, clarificar, validar, kickoff, handoff, integracoes) have full treatment: 7-step workflow, 3+ worked examples each, detailed failure-mode recovery procedures, anti-patterns with wrong/correct pairs
- **Remaining 10 skills** have compact but complete treatments: 5-step workflow minimum, 1+ example, anti-patterns, failure modes
- **CI guard**: `TestAllBundledSkillsValidateCleanly` verifies every bundled skill passes schema validation. 16 sub-tests, all green.

### Lessons learned during authoring

1. **YAML quotes in unquoted scalars**: `"Context" section explains...` fails to parse; use `'Context' section explains...` instead. Found 3 during authoring, fixed.
2. **Path sentinel `-`**: `path: -` is invalid YAML; use `path: "-"` quoted. Found in `clarificar`, fixed.
3. **Colons in description values**: `description: Workflow includes: validate, audit, tests, build.` breaks YAML. Found 1, fixed.
4. **Missing required sections**: 4 skills initially missed `## Examples` section. The validator caught them. Fixed.

---

## CLI Surface (after batch 2)

```
$ radiant skills list
  Bundled skills (16):
    adr                    1.0.0      feature,arc… Creates an Architecture Decision Record...
    auditar                1.0.0      architecture Audits the radiant-harness project layout...
    camada-agentica        1.0.0      architecture Generates the agentic layer configuration...
    clarificar             1.0.0      trivial,fea… Conducts a structured interview...
    diagramar              1.0.0      feature,arc… Produces C4-model architecture diagrams...
    evals                  1.0.0      feature,arc… Measures spec→code fidelity...
    handoff                1.0.0      trivial,fea… Pauses or resumes a session...
    integracoes            1.0.0      trivial,fea… Discovers MCPs and tools...
    kickoff                1.0.0      architecture Conducts the project constitution...
    mapear                 1.0.0      architecture Analyzes an existing codebase...
    metricas               1.0.0      architecture Measures delivery maturity...
    nova-feature           1.0.0      trivial,fea… Starts a new feature...
    revisar-pr             1.0.0      feature,arc… Reviews a PR/MR against the spec...
    roadmap                1.0.0      architecture Constructs/revises the product roadmap...
    setup-ci               1.0.0      architecture Generates the CI workflow...
    validar                1.0.0      feature,arc… Runs the Definition of Done check...

  Universal location (vendor-neutral):
    .radiant-harness/skills/<name>/{SKILL.md, frontmatter.yaml}

  Open spec: docs/SKILL-SCHEMA.md
```

```
$ radiant skills validate internal/skill/skills/nova-feature
  ✓ nova-feature validates cleanly
```

---

## Coverage

| Package | Coverage | Notes |
|---------|----------|-------|
| `internal/benchmark` | 77% | unchanged |
| `internal/engine` | 47.0% | unchanged |
| `internal/harness` | 61.1% | unchanged |
| `internal/llm` | 84.3% | unchanged |
| `internal/policy` | 100% | unchanged |
| `internal/quality` | 59.5% | unchanged |
| `internal/spec` | 88.5% | unchanged |
| `internal/skill` | ~100% | NEW: 19 dedicated tests for schema + bundle + extract + load |

---

## Files Changed (both batches combined)

```
internal/skill/schema.go                        NEW (~280 lines)
internal/skill/bundle.go                        NEW (~150 lines)
internal/skill/schema_test.go                   NEW (~430 lines)
internal/skill/skills/<16 skills>/*.yaml       NEW or REWRITTEN (32 files)
internal/skill/skills/<16 skills>/SKILL.md      NEW or REWRITTEN (16 files)
cmd/radiant/main.go                             +60 lines (skills CLI)
CHANGELOG.md                                    +80 lines
docs/HARNESS-PLAN.md                            NEW (strategy doc)
docs/SKILL-SCHEMA.md                            NEW (open spec)
docs/ROADMAP.md                                 REWRITTEN
docs/validation-report-pivot.md                 NEW
docs/validation-report-sprint-10-batch1.md      NEW
docs/validation-report-sprint-10-batch2.md      NEW
```

Total: ~40 new/changed files, ~4500 lines added.

---

## Video Research Notes Synthesis (post-batch 2 study)

After both batches landed, the user provided video research notes from 19 YouTube videos on AI-driven development (`notas-detalhadas-videos-youtube.md`). The notes confirmed and refined the methodology merge direction. Key insights applied or queued:

### Already validated by our work

| Video insight | How our work already addresses it |
|---------------|------------------------------------|
| TLC won benchmark by forcing AC→test mapping (video #1) | `validar` skill + tasks.md Coverage column |
| Harness > spec-driven alone (video #4) | We ship both: skill runtime + engine |
| Skill format is the new agent contract (video #5, #7, #8) | 16 vendor-neutral skills in `internal/skill/` |
| Smart zone: window ≤ 40% (video #10) | `nova-feature` SKILL.md says "CLOSE THIS CONTEXT" before implementation |
| AGENTS.md should be minimal (video #6) | Plan to keep AGENTS.md ≤ 100 lines (third batch) |
| Tests must be forced (video #15, #18) | `validar` gate `ac-tested` + every task must have a gate command |
| DDD strategic still essential (video #12) | `kickoff` + `mapear` + `diagramar` skills |
| Spec-Driven = #1 skill for 2026 devs (video #16) | Sprint 10 entire focus |

### Insights queued for next sprints

| Insight | Sprint target |
|---------|---------------|
| `--validator=<model>` separate validation LLM | Sprint 10 third batch (high priority) |
| AGENTS.md minimal template | Sprint 10 third batch |
| AC→test pré-check in `radiant spec` | Sprint 10 third batch |
| `radiant mcp serve` (MCP server) | Sprint 11 |
| Worktree-based parallel execution | Sprint 12 |
| Semantic memory (vector index of past decisions) | Sprint 13+ |

### Contradictions noted (not blocking)

| Concern | Mitigation |
|---------|-----------|
| LLM-generated AGENTS.md may worsen results (video #6) | Document that user should review AGENTS.md after init |
| "Abstraction bloat" — AI suggests over-engineering (video #15) | Keep packages shallow; resist premature layers |
| UUID v4 hurts relational DB perf (video #14) | Use UUIDv7 or sequential IDs if we add persistence |

---

## Git State (post-batch 2)

```
b98e503 feat: sprint 10 second batch — 16 skills rewritten top-of-line
f0f4546 feat: sprint 10 first batch — vendor-neutral skill runtime
fc47419 docs: add sprint 9 validation report
a6cca6b docs: strategic pivot — methodology merge plan + skill schema spec
a9614b7 feat: sprint 9 — gate command allowlist deduplication via internal/policy
```

Working tree clean. `0.4.1` embedded in every release binary.

---

## Next Step (Sprint 10 Third Batch)

7 remaining deliverables from HARNESS-PLAN §5.1, plus video-research-driven improvements:

1. **`radiant init` extracts skills** to `.radiant-harness/skills/`
2. **`AGENTS.md` auto-generated** with **minimal template** (≤100 lines, list skills + commands, link docs — per video #6)
3. **`radiant spec <intent>`** command (interactive interview) — with AC→test pré-check (per video #1)
4. **`radiant state`** + **`radiant handoff`** commands
5. **`--tier` flag** with auto-detect (trivial / feature / architecture)
6. **Native view generation** opt-in via `--agent=<list>` (Claude, Cursor, Codex, Copilot, Gemini, Windsurf)
7. **`--validator=<model>` flag** in `radiant run` (per video #4 — separate agents by role)

The hard parts of Sprint 10 are done. This batch is mostly CLI wiring + integration, applying the video-research learnings.

The third batch is the **last batch** in Sprint 10. After this, Sprint 11 picks up the roadmap items (kickoff brownfield path, MCP server, worktrees, etc.).