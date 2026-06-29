---
name: agentic-layer
description: Map of rules/agents/skills/workflows. Pull when evolving the agentic layer.
alwaysApply: false
---

# Project Agentic Layer

The same kickoff inputs (stack, tools, process, domain) tune the layer that makes
**humans and agents operate the SDD pipeline**. Four artifact types, all versioned.

## 1. Rules — how the agent should behave
- **`CLAUDE.md`** — conventions, ubiquitous language, layer rule, Definition of Done.
- **`.claude/settings.json`** — permissions (allowlist) and **hooks**.

## 2. Docs — the knowledge (the constitution)
vision · mvp-canvas · design · domain · spec · ADRs · glossary · context-map · roadmap · integrations.

## 3. Agents (subagents) — on-demand specialists
`.claude/agents/<name>.md` (see `docs/engineering/_templates/subagent.template.md`).

## 4. Skills — reusable workflows
`.claude/skills/<name>/SKILL.md`. The 15 pipeline skills:

| Skill | Responsibility |
|---|---|
| `/kickoff` | orchestrates project constitution |
| `/integracoes` | team tools + MCPs |
| `/mapear` | brownfield as-is → assessment.md |
| `/diagramar` | high-level Mermaid architecture |
| `/roadmap` | builds/reviews roadmap with team |
| `/camada-agentica` | proposes/generates rules, subagents, skills, workflows/CI |
| `/nova-feature` | opens a feature in SDD pattern |
| `/clarificar` | relentless interview (one question at a time) |
| `/validar` | UAT: gates, AC→test, SPEC_DEVIATION, DoD |
| `/revisar-pr` | SDD conformity gate on PR/MR |
| `/setup-ci` | CI/CD pipeline that materializes gates |
| `/metricas` | Lead Time, Throughput, CD maturity |
| `/auditar` | validates pipeline conformity |
| `/evals` | spec→code fidelity |
| `/handoff` | pause/resume via docs/STATE.md |

## 5. Workflows — pipeline automation
- **Hooks** (`settings.json`): `SessionStart` → context injection.
- **CI/CD** (`/setup-ci`): fail PR that changes code without approved spec.
- **Pipeline conformity** (`.github/workflows/harness.yml` → `scripts/audit.ts`).
