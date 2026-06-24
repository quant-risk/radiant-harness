# Skill Schema — Open Specification

**Version**: 1.0.0
**Status**: Draft (proposed for Sprint 10)
**License**: MIT (open spec; free to implement)

This document defines the canonical format for a **skill** in the radiant-harness workflow. The format is intentionally **vendor-neutral**: any LLM, IDE, or agent runtime can implement a parser without depending on a proprietary runtime, hook system, or namespace convention.

A reference implementation lives in `internal/scaffold/skills/` and is embedded in the `radiant` CLI binary.

---

## 1. Goals

1. **Universal**: any agent can consume a skill by reading the directory + YAML.
2. **Machine-readable contract**: inputs, outputs, gates, version are explicit (not implicit).
3. **Self-describing**: a skill carries enough metadata that an agent can decide in 5 seconds whether to use it.
4. **Versioned**: skills evolve; old projects can stay on old skills via `radiant update`.
5. **Embeddable**: the CLI bundles skills so `init` works offline, no network.
6. **Composable**: skills reference other skills explicitly (no implicit dependencies).

---

## 2. Directory layout

```
skills/<skill-name>/
├── SKILL.md                # required — human-readable instructions
├── frontmatter.yaml        # required — machine-readable contract
├── examples/               # optional — worked examples
│   ├── trivial.md
│   ├── medium.md
│   └── complex.md
├── scripts/                # optional — helper scripts the skill may invoke
│   └── <helper>.sh
└── CHANGELOG.md            # optional — per-skill version history
```

`<skill-name>` MUST be kebab-case (lowercase, hyphens), 1-32 chars, unique within a project.

---

## 3. `SKILL.md` structure

`SKILL.md` is plain Markdown. No proprietary frontmatter — the YAML contract lives in `frontmatter.yaml` (separate file) so the prose stays clean.

```markdown
# Skill: <name>

> One-paragraph orientation (2-4 sentences). Written for a busy
> agent who needs to decide in 5 seconds whether this skill
> applies. Reference the inputs from frontmatter.yaml.

## Decision tree

[ASCII or Mermaid diagram showing which paths apply in which cases]

## Workflow

[Step-by-step instructions. Reference the CLI commands from
frontmatter.yaml. Each step should be unambiguous enough that an
LLM can execute it without improvisation.]

### Step 1: <title>
[what to do, what file to read/write, what gate to validate]

### Step 2: <title>
[...]

### Step N: <title>
[...]

## Examples

### Example 1: trivial
[Input + Walkthrough + Output]

### Example 2: medium
[Input + Walkthrough + Output]

### Example 3: complex
[Input + Walkthrough + Output]

## Anti-patterns

[Concrete examples of what NOT to do, with the wrong output shown
alongside the corrected version. The wrong/correct pairs are the
highest-leverage teaching content.]

## Failure modes

[For each gate in frontmatter.yaml, describe what to do when it
fails. Include: how to detect the failure, how to retry, when to
escalate to a human or another skill.]

## Related skills

[Cross-references to other skills in `.radiant-harness/skills/`,
with one-line descriptions of when to chain them.]
```

---

## 4. `frontmatter.yaml` schema (canonical)

```yaml
# ─────────────────────────────────────────────────────────────
# Required fields
# ─────────────────────────────────────────────────────────────

# kebab-case identifier, unique within project
name: nova-feature

# semver, updated by `radiant update` when skills are revised
version: 1.0.0

# One-paragraph summary. Plain text, no Markdown.
description: |
  Inicia uma nova feature no pipeline SDD. Decide o tier,
  cria a pasta, conduz pelos gates, e entrega spec.md +
  tasks.md prontos para implementação.

# When to invoke this skill. Used by agents to decide if it's
# applicable. 1-3 sentences, plain text.
when_to_use: |
  O usuário pediu uma feature nova. Tem intenção clara mas
  ainda não decidiu se é trivial, feature ou architecture.

# Which tiers this skill can produce. Trivial/Feature/Architecture
# match the radiant tier system.
tier_eligible: [trivial, feature, architecture]

# ─────────────────────────────────────────────────────────────
# Contract: inputs
# ─────────────────────────────────────────────────────────────

inputs:
  - name: intent
    type: string              # string | number | enum | object | path
    required: true
    description: "O que o usuário quer construir (1-3 frases)"
  - name: context
    type: string
    required: false
    description: "Contexto adicional (projeto, restrições, links)"

# ─────────────────────────────────────────────────────────────
# Contract: outputs (artifacts this skill produces)
# ─────────────────────────────────────────────────────────────

outputs:
  - path: specs/<NNNN>-<slug>/spec.md
    type: artifact             # artifact | report | commit | pr | decision
    description: "Spec com ACs Given/When/Then"
  - path: specs/<NNNN>-<slug>/tasks.md
    type: artifact
    description: "Tabela de tasks com gate commands"

# ─────────────────────────────────────────────────────────────
# Contract: gates (validation steps before outputs are accepted)
# ─────────────────────────────────────────────────────────────

gates:
  - name: tier-decided
    description: "Tier escolhido (trivial/feature/architecture)"
    on_failure: |
      Re-abrir o skill /kickoff ou perguntar ao usuário até que
      o tier esteja claro. Não prosseguir sem tier definido.
  - name: ac-testable
    description: "Cada AC tem formato Given/When/Then verificável"
    on_failure: |
      Chamar a skill /clarificar para refinar ACs ambíguos.

# ─────────────────────────────────────────────────────────────
# Optional but recommended
# ─────────────────────────────────────────────────────────────

# Context files this skill reads from the project before executing
context_provides:
  - vision.md       # if exists
  - glossary.md     # always
  - state.md        # always (.radiant-harness/state.md)

# Equivalent CLI command — so non-agent users get the same
# capability through the binary
commands_available:
  - radiant spec <intent>

# Other skills this one references (for chaining)
related_skills:
  - clarificar
  - validar

# What this skill helps you AVOID (most valuable for review)
anti_patterns:
  - "Implementing without reading state.md (loses session context)"
  - "Writing ACs as 'should work correctly' (not testable)"

# Author + license metadata
author: radiant-harness contributors
license: MIT
```

---

## 5. Field reference

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | ✓ | kebab-case, 1-32 chars, unique within project |
| `version` | semver | ✓ | `MAJOR.MINOR.PATCH`. Bump MAJOR on contract change, MINOR on new optional field, PATCH on text-only updates |
| `description` | string | ✓ | One-paragraph summary. Used by agents to decide if the skill applies |
| `when_to_use` | string | ✓ | 1-3 sentences. Decision criterion for invocation |
| `tier_eligible` | array | ✓ | Subset of `[trivial, feature, architecture]` |
| `inputs[].name` | string | ✓ | kebab-case identifier within inputs |
| `inputs[].type` | enum | ✓ | `string \| number \| enum \| object \| path` |
| `inputs[].required` | bool | ✓ | Whether the skill fails without this input |
| `inputs[].description` | string | ✓ | Human-readable description |
| `outputs[].path` | glob | ✓ | Where the artifact lands. `<NNNN>` = sequence number, `<slug>` = kebab-case name |
| `outputs[].type` | enum | ✓ | `artifact \| report \| commit \| pr \| decision` |
| `outputs[].description` | string | ✓ | What the artifact is |
| `gates[].name` | string | ✓ | Identifier |
| `gates[].description` | string | ✓ | What is being validated |
| `gates[].on_failure` | string | ✗ | Recovery procedure. If omitted, the skill fails closed |
| `context_provides` | array | ✗ | Files the skill expects to read before executing |
| `commands_available` | array | ✗ | Equivalent CLI commands for non-agent users |
| `related_skills` | array | ✗ | Other skills this one references |
| `anti_patterns` | array | ✗ | Mistakes this skill helps avoid |
| `author` | string | ✗ | Skill author |
| `license` | string | ✗ | License identifier (SPDX) |

---

## 6. Validation rules

A skill is valid iff:

1. `name` matches `^[a-z][a-z0-9-]{0,31}$`
2. `version` matches `^\d+\.\d+\.\d+$`
3. `tier_eligible` is non-empty subset of `{trivial, feature, architecture}`
4. Every `inputs[].name` is unique within the skill
5. Every `outputs[].path` is unique within the skill
6. Every `gates[].name` is unique within the skill
7. All required fields are present
8. `SKILL.md` exists and is non-empty
9. The `name` in `frontmatter.yaml` matches the parent directory name
10. (Recommended) `SKILL.md` includes all section headers from §3

The reference validator is `radiant skills validate <skill-name>`, shipped with the CLI.

---

## 7. Distribution

Skills are bundled in the `radiant` CLI binary at build time via `//go:embed`. Distribution channels:

| Channel | Mechanism |
|---------|-----------|
| GitHub release | `radiant` binary + `radiant-skills.tar.gz` (extractable into a project) |
| `go install` | `go install github.com/quant-risk/radiant-harness/cmd/radiant@latest` |
| `brew install` | Homebrew tap |
| npm | `@quant-risk/radiant-harness` wraps the binary; provides the `npx @quant-risk/radiant-harness` entry |
| Direct download | `dist/radiant-{linux,darwin,windows}-{amd64,arm64}` per the Makefile |

Updating skills on an existing project:

```bash
$ radiant update
# Reads the bundled skills, compares against the project's
# .radiant-harness/skills/, prompts for each changed skill,
# updates frontmatter.yaml version + skill body, preserves
# any local customizations (skills the user wrote themselves).
```

---

## 8. Reference parsers

Parsers implementing this spec should:

1. Read the project root for `AGENTS.md` (the index) and `.radiant-harness/skills/` (the skills)
2. For each skill, parse `frontmatter.yaml` to extract the contract
3. Present the contract to the agent as structured metadata
4. When the agent invokes a skill, render `SKILL.md` as instructions
5. Validate outputs against the contract (`path` matches, gates satisfied) before considering the skill done

The CLI ships with a Go reference parser. Community parsers in other languages are welcome — please link them in the project README.

---

## 9. Versioning policy

- **PATCH**: text-only changes to SKILL.md (clarity, examples). No contract change.
- **MINOR**: new optional field in frontmatter.yaml (e.g. `inputs[].default`). Old parsers still work.
- **MAJOR**: rename or removal of a field, change in `tier_eligible` values, change in `inputs[].required`. Old parsers may break.

When `radiant update` runs, it warns on MAJOR bumps and refuses to auto-update — the user must explicitly opt in.

---

## 10. Example: minimal valid skill

**`skills/hello-world/frontmatter.yaml`**:

```yaml
name: hello-world
version: 1.0.0
description: |
  Greet the user with the project name and current state.
when_to_use: |
  Any greeting context (agent startup, user "hi", etc.).
tier_eligible: [trivial]
inputs: []
outputs:
  - path: -        # no artifact
    type: report
    description: "A greeting printed to stdout"
gates: []
```

**`skills/hello-world/SKILL.md`**:

```markdown
# Skill: hello-world

> Print a friendly greeting that includes the project name
> from `.radiant-harness/config.yaml` and the current state
> from `.radiant-harness/state.md`.

## Decision tree

Greeting? → Use this skill. Anything else? → Not applicable.

## Workflow

1. Read `.radiant-harness/config.yaml` → extract `project_name`
2. Read `.radiant-harness/state.md` → extract `current_feature`
3. Print: `Hello! Working on <project_name>, currently on <current_feature>.`

## Examples

### Example 1: trivial
Input: (none)
Output: `Hello! Working on my-app, currently on 0001-jwt-auth.`

## Anti-patterns
- Don't invent fields that don't exist in config.yaml.
- Don't print more than one line — this is a greeting, not a status report.

## Failure modes

- `config.yaml` missing → print: `Hello! (project name unknown — run radiant init)`
- `state.md` missing → print: `Hello! (no active feature)`

## Related skills
- (none)
```

---

## 11. License

This spec is released under MIT. Implementations are welcome in any language. The radiant-harness project provides a Go reference implementation; community implementations in TypeScript, Python, Rust, etc. are encouraged and will be linked from the README.