# Radiant Harness — Strategic Pivot Validation Report

**Date**: 2026-06-24
**Commit**: (this commit)
**Trigger**: Comparison with [spec-driven](https://github.com/igoruehara/spec-driven) (137⭐, 45 forks, community-validated methodology) revealed a gap between radiant-harness's execution engine and the missing SDD methodology layer.

This report captures the **strategic pivot** documented in three new files:
- `docs/HARNESS-PLAN.md` — complete 4-phase plan (4 sprints, 24 deliverables)
- `docs/SKILL-SCHEMA.md` — open specification for vendor-neutral skill format
- `docs/ROADMAP.md` — updated with the new direction

---

## Build & Test

| Check | Result |
|-------|--------|
| `go build ./...` | ✓ zero errors (no code changes — only docs) |
| `go test ./... -count=1` | ✓ all 6 packages pass |
| `go vet ./...` | ✓ zero warnings |
| `gofmt -l .` | ✓ no unformatted files |
| Test count | ✓ 188 (unchanged — pure docs commit) |

No code regressions. This commit is documentation-only.

---

## What's in the three new documents

### `docs/HARNESS-PLAN.md`

**The strategic pivot** — capturing in one place:
- **Why we're doing this**: radiant-harness is strong on execution, weak on methodology. spec-driven is strong on methodology, weak on execution. The merge combines both.
- **The architectural principle (NON-NEGOTIABLE)**: "The format of skills and artifacts must be so good that any agent on the market can consume them without plugin, hook, or vendor namespace."
- **Target architecture**: `AGENTS.md` + `.radiant-harness/skills/` + opt-in native views (`.claude/`, `.cursor/`, etc.)
- **4 phases, 4 sprints, 24 deliverables**: Sprint 10 (foundation), Sprint 11 (discovery+design), Sprint 12 (governance), Sprint 13 (PR+views)
- **Acceptance criteria per sprint**: tests, race detector, coverage, cross-compile, skill schema compliance, AGENTS.md regeneration, **zero vendor lock-in** (the "test of fire"), validation report
- **Risk register + open questions**

### `docs/SKILL-SCHEMA.md`

**The open specification** — published as MIT, any agent author can implement:
- Directory layout (SKILL.md + frontmatter.yaml + examples/ + scripts/)
- SKILL.md structure (decision tree, workflow, examples, anti-patterns, failure modes, related skills)
- frontmatter.yaml schema (name, version, description, when_to_use, tier_eligible, inputs[], outputs[], gates[], context_provides, commands_available, related_skills, anti_patterns, author, license)
- Field reference table
- Validation rules (10 rules the reference parser enforces)
- Distribution channels (CLI binary, `go install`, brew, npm, direct download)
- Versioning policy (PATCH=text-only, MINOR=new optional field, MAJOR=contract change)
- Minimal valid skill example
- License (MIT)

### `docs/ROADMAP.md`

**Updated strategic roadmap**:
- Non-negotiable principles section (zero Claude-centrism, vendor-neutral LLM, cross-platform, no SDKs, skills as machine-readable contract, runtime detection)
- Sprints 0-9 marked as done with their commits + test counts
- Sprints 10-13 listed with deliverables
- Anti-backlog: items explicitly excluded (Claude preference, Claude hooks as required, CLAUDE.md as namespace, slash commands as only entry point, vendor lock-in)
- Metrics of success per sprint + per phase
- "Change of direction" section explaining the pivot

---

## The principle (test of fire)

> **If tomorrow we delete every native view (`.claude/`, `.cursor/`, `.windsurf/`, `.gemini/`, `.github/copilot-instructions.md`) and only `AGENTS.md` + `.radiant-harness/skills/` remain, any modern LLM IDE must be able to work on the project.**

Every sprint from 10 onwards has this in its acceptance criteria. We build toward a world where:

1. The PRIMARY format is universal (anyone reads it)
2. `AGENTS.md` consolidates everything for agents that prefer a single file
3. Native views are opt-in wrappers, generated from the universal source
4. If a new agent appears tomorrow, it doesn't need a plugin — it reads the universal format

---

## What this means for existing users

**Zero impact**:
- All existing CLI commands work unchanged
- All existing flags work unchanged
- All existing tests pass
- Existing users see no change in behavior
- The 188 tests, 6/6 cross-compile, security guarantees — all preserved

**New capabilities added incrementally**:
- Sprint 10: `radiant spec`, `radiant state`, `radiant handoff`, tier flag, bundled skills
- Sprint 11+: product discovery, ADRs, diagrams, update mechanism
- Sprint 12+: mapping, audit, metrics
- Sprint 13+: PR review, 6-agent views, CI generation

Each sprint is independently valuable and independently shippable.

---

## Open questions for the user (need answers before Sprint 10)

1. **Distribution name**: `@quant-risk/radiant-harness` (npm) + `radiant-harness` (go install) — confirm?
2. **Tier system language**: Portuguese names (Trivial/Pequeno/Arquitetural) or English (Trivial/Feature/Architecture)?
3. **Skill execution engine**: should the CLI itself execute skill instructions (for non-agent users), or only emit the skills for agents to read?
4. **Update channel**: stable + beta, or just latest?
5. **MCP integration depth**: just discover & list, or also auto-configure `.mcp.json`?

---

## Git State

```
(in this commit)  docs: strategic pivot — HARNESS-PLAN + SKILL-SCHEMA + updated ROADMAP
a9614b7  feat: sprint 9 — gate command allowlist deduplication via internal/policy
266eb9b  docs: add sprint 8 validation report
7fb5b54  feat: sprint 8 — gate command output cap via --max-gate-output
9f9a0f5  docs: add sprint 7 validation report
f20e94e  feat: sprint 7 — planner fires, JSONL trace, race fix, 6-target release
7fb5262  feat: sprint 6 — multi-agent routing + tracing + CodeLens
```

Working tree clean. No code changes. All 188 tests still pass.

---

## Next step

Awaiting user decisions on the 5 open questions, then start **Sprint 10** — the first sprint of the new direction. Sprint 10 will deliver:

- The skill schema implementation (Go validator for `frontmatter.yaml`)
- 3 skills written top-of-line (`nova-feature`, `clarificar`, `validar`)
- Skills bundled in the CLI binary via `//go:embed`
- `AGENTS.md` generated by `radiant init`
- `radiant spec <intent>` command (interactive interview)
- `--tier` flag with auto-detect
- `radiant state` + `radiant handoff`
- Native view generation opt-in via `--agent=<list>`

That's 8 deliverables in one sprint. Each one testable independently. Each one a step toward the "modern, intelligent, sophisticated, elegant" harness the user asked for.