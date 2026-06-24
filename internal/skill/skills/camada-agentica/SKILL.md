---
name: camada-agentica
description: Propose and generate the agentic layer for the project.
---

# Skill: Agentic Layer (Propose → Generate)

Proposes the agentic layer for the project, then generates only what the user approves.
Four artifact types: rules, subagents, skills, workflows/CI.

## Phase 1 — Analyze inputs (Research)

1. `read_file` `docs/engineering/agentic-layer.md` — existing layer map (if any).
2. `read_file` `docs/architecture/assessment.md` — detected stack, gaps, risks.
3. `read_file` `docs/product/vision.md` and `docs/product/roadmap.md` — what work is coming.
4. `read_file` `docs/engineering/TESTING.md` — existing gate commands.
5. Check `docs/engineering/integrations.md` — which MCPs are connected and who consumes them.
6. Read `CLAUDE.md` MCP table — currently connected tools.

> If inputs are sparse (early kickoff), the proposal is preliminary. Mark it as such.

## Phase 2 — Propose the layer (Plan)

Present a proposal table for each artifact type. Each row includes **justification**:

### 2a. Rules (behavioral conventions)
| Rule | Current state | Proposed change | Why |
|------|--------------|-----------------|-----|
| `CLAUDE.md` gates | <exists? empty?> | Fill DoD, layer rule, ubiquitous language | Source of truth for agent |
| `.claude/settings.json` | <exists?> | Add SessionStart hook, permissions allowlist | Deterministic context injection |

### 2b. Subagents (on-demand specialists)
| Subagent | Trigger | Receives | Returns |
|----------|---------|----------|---------|
| `researcher` | `/nova-feature` Phase 1 | task + codebase area | research.md summary |
| `test-runner` | gate execution | test command | pass/fail + output |

Only propose subagents that map to recurring patterns in the roadmap or TESTING.md.

### 2c. Skills (reusable workflows)
Review the 15 standard SDD skills. Flag any that need stack-specific customization
(e.g. `/setup-ci` needs to know the CI provider, `/metricas` needs the PM tool).

### 2d. Workflows/CI
| Workflow | What it does | Gate it enforces |
|----------|-------------|-----------------|
| `harness.yml` | Run audit script on PR | Spec exists for code changes |
| `SessionStart` hook | Inject base context | Deterministic context load |

## Phase 3 — Confirm scope (Plan)

1. Present the full proposal. Ask: "Which items should I generate now?"
2. Default: generate nothing without explicit approval per item.
3. For each approved item, note: file path, what it contains, what it depends on.

## Phase 4 — Generate approved artifacts (Implement)

For each approved item, generate the file:

1. **Rules:** edit `CLAUDE.md` (fill blanks), create/update `.claude/settings.json`.
2. **Subagents:** `write_file` to `.claude/agents/<name>.md` from `docs/engineering/_templates/subagent.template.md`.
3. **Workflows:** `write_file` to `.github/workflows/` or `.gitlab-ci.yml` — delegate CI specifics to `/setup-ci`.
4. Update `docs/engineering/agentic-layer.md` with what was generated and its status.

## Rules

- **Propose with justification, generate only approved.** Never silently create agent files.
- Each proposal cites the input that motivates it — stack, process, or domain need.
- Unapproved items become roadmap adoption items (suggest `/roadmap`).
- No secrets in generated files. CI workflows reference env vars, never inline tokens.
