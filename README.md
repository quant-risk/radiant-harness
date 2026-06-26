# radiant-harness

> A vendor-neutral autonomous development harness for any LLM.
> Shipped as a single binary — works with Claude Code, Cursor,
> Codex, Copilot, Gemini CLI, and Windsurf.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Version](https://img.shields.io/badge/version-1.0.0-blue.svg)](CHANGELOG.md)
[![Tests](https://img.shields.io/badge/tests-155_pass-green.svg)](CHANGELOG.md)
[![Cross-compile](https://img.shields.io/badge/cross--compile-6_of_6-blueviolet.svg)](CHANGELOG.md)

---

## What it is (v2.0)

`radiant` is a CLI harness for **autonomous LLM-driven development**. It solves four problems:

1. **Token bloat** — instead of loading all 60 skills at session start (~55K tokens), the Context Engine detects your project domain and loads 3–10 skills (~300 tokens). **99% reduction**.
2. **No feedback loop** — the Loop Engine runs a crash-safe Discover→Plan→Execute→Verify→Persist cycle. The verifier is always a separate agent call; the executor never grades its own work.
3. **Static instructions** — the Self-Improvement Engine analyzes failure traces, proposes SKILL.md patches, and applies them only when improvement ≥ 5pp.
4. **Single-agent bottleneck** — the Fleet layer coordinates Planner + Implementer + Verifier + Summarizer agents with conflict-safe shared state.

Works with any OpenAI-compatible API. No vendor lock-in.

---

## Quick Start (v2.0) — zero to autonomous loop in 5 minutes

```bash
# 1. Install
go install github.com/quant-risk/radiant-harness/cmd/radiant@latest

# 2. Initialize a project
mkdir my-project && cd my-project
radiant init . --all --yes

# 3. Detect your project domain (optional — shown for transparency)
radiant context detect

# 4. Generate the minimal context file (~300 tokens)
radiant context assemble

# 5. Boot — emit a <500-token manifest any LLM can read
radiant boot

# 6. Start the autonomous loop
radiant loop start "add rate limiting to the /api/users endpoint"

# 7. Watch progress
radiant loop status
radiant trace show <run-id>
```

That's it. The loop runs Discover→Plan→Execute→Verify→Persist automatically.
It writes a checkpoint after each phase so it can resume after interruption.

---

## IDE Setup

```bash
# Generate native files for your IDE(s)
radiant views --agent=claude     # .claude/settings.json + skills
radiant views --agent=cursor     # .cursor/rules/*.mdc
radiant views --agent=copilot    # .github/copilot-instructions.md
radiant views --agent=gemini     # GEMINI.md + .gemini/commands/
radiant views --agent=windsurf   # .windsurfrules
radiant views --agent=codex      # AGENTS.md

# Or all at once
radiant views --agent=all --force

# Preview changes before writing
radiant views --agent=cursor --diff
```

---

## Command Reference

### Context & Boot
```bash
radiant boot                              # ≤500-token entry point for any LLM
radiant context detect [--json]           # detect domain + tier
radiant context assemble [--budget=N]     # build .radiant-harness/CONTEXT.md
radiant context compress --budget=2000    # compress to fit budget
radiant context summarize --phase=<name>  # compress a completed phase
```

### Loop Engine
```bash
radiant loop start "<goal>" [--profile=lean|standard|thorough]
radiant loop status
radiant loop resume
radiant trace show <run-id> [--json]
radiant trace list
```

### Token Budget
```bash
radiant budget estimate [spec-file] [--profile=standard]
radiant budget report <run-id>
```

### Multi-Agent Fleet
```bash
radiant fleet start "<goal>" [--agents=N]
radiant fleet status <run-id>
```

### Self-Improvement
```bash
radiant improve --from-traces [--skill=<id>] [--dry-run] [--apply]
radiant improve history
```

### IDE Views
```bash
radiant views --agent=<id> [--force] [--diff]
```

### Classic SDD workflow (still works)
```bash
radiant init . --all --yes
radiant product "API observability for small dev teams"
radiant spec "JWT auth so users stay logged in across restarts"

# 4. Run the implementation (the LLM does this)
radiant run specs/0001-jwt-auth --model <your-model>

# 5. Validate after implementation
radiant validate specs/0001-jwt-auth --gates

# 6. Audit + measure fidelity
radiant audit
radiant evals

# 7. Cut a release
radiant release v0.1.0
```

## Day-1 workflow (project setup)

| Step | Command | What it produces |
|------|---------|------------------|
| 1 | `radiant init .` | `.radiant-harness/skills/`, `AGENTS.md`, `state.md`, native views |
| 2 | `radiant product "..."` | `docs/product/inception.md` + `personas.md` |
| 3 | `radiant spec "..."` | `specs/0001-<slug>/spec.md` + `tasks.md` |
| 4 | `radiant run specs/0001-<slug>/` | implementation (LLM-driven) |
| 5 | `radiant validate specs/0001-<slug>/ --gates` | UAT report |
| 6 | `radiant audit` | project-wide conformity check |
| 7 | `radiant evals` | AC→test coverage metrics |
| 8 | `radiant release v0.1.0` | version bump + tests + cross-compile + tag |

## Upgrade workflow (existing project)

```bash
# 1. Pull the new binary
go install github.com/quant-risk/radiant-harness/cmd/radiant@latest

# 2. Refresh bundled skills + AGENTS.md (preserves user's docs)
radiant update

# 3. Regenerate native views for the new bundled skills
radiant views --agent=claude,cursor --force

# 4. Audit the agentic layer
radiant camada-agentica --fix

# 5. Measure fidelity after the upgrade
radiant evals
```

## Commands (18 total)

| Command | Version | Purpose |
|---------|---------|---------|
| `init` | 0.2.0+ | Scaffold the SDD pipeline |
| `config` | 0.2.0+ | Configure LLM provider/model |
| `run` | 0.2.0+ | Execute a spec end-to-end (LLM-driven) |
| `models` | 0.2.0+ | List model presets |
| `validate` | 0.2.0+ | Static spec→code→tests UAT |
| `eval` | 0.2.0+ | Latency/cost benchmark for one prompt × N runs |
| `bench` | 0.2.0+ | Compare against other frameworks |
| `doctor` | 0.2.0+ | Local environment diagnostic |
| `state` | 0.4.2 | Show current session state |
| `handoff` | 0.4.2 | Pause + write session state atomically |
| `spec` | 0.4.2 | Create spec.md + tasks.md from flag inputs |
| `skills list` / `skills validate` | 0.4.0 | Manage skills |
| `adr` | 0.4.3 | Create an Architecture Decision Record (Nygard) |
| `update` | 0.4.3 | Refresh bundled skills + AGENTS.md |
| `diagramar` | 0.4.3 | C4 Mermaid templates |
| `product` | 0.4.4 | Lean Inception scaffold |
| `integrations list` | 0.4.5 | Read-only MCP listing |
| `views` | 0.4.6 | Native agent views on demand |
| `review-pr` | 0.4.7 | PR review scaffold |
| `setup-ci` | 0.4.8 | CI workflow generator |
| `camada-agentica` | 0.4.9 | Agentic layer audit |
| `evals` | 0.5.0 | AC→test coverage metrics |
| `release` | 0.5.1 | Cut a release |
| `audit` | 0.6.0 | Project layout audit |
| `mcp serve` | 0.6.0 | MCP server (stdio) |

## Skills (60 bundled)

The CLI ships 60 vendor-neutral skills in `.radiant-harness/skills/`.
In v2.0, skills are lazy-loaded: only 3–10 relevant to your project
are included in context (see [docs/CONTEXT-ENGINE.md](docs/CONTEXT-ENGINE.md)).

**Core methodology (always loaded):** `nova-feature`, `nova-product`, `kickoff`, `clarificar`

**Quality:** `validar`, `auditar`, `metricas`, `evals`, `revisar-pr`

**Architecture:** `adr`, `diagramar`, `mapear`, `camada-agentica`, `handoff`, `roadmap`

**Finance & Risk:** `finance`, `credit-risk`, `credit-portfolio`, `market-risk`,
`liquidity-risk`, `operational-risk`, `model-risk`, `stress-test`, `regulatory`,
`actuarial`, `actuarial-solvency`, `accounting`, `controlling`, `tax`, `valuation`,
`aml-kyc`, `fraud-detection`, `capital-markets`

**ML & Data:** `ml`, `deep-learning`, `reinforcement-learning`, `causal`, `causal-ml`,
`bayesian`, `stats`, `econometrics`, `synthetic-data`, `evals`, `data`

**Engineering:** `api`, `cli`, `security`, `setup-ci`, `integracoes`, `update`, `incident`

**Domain:** `frontend`, `mobile`, `iot`, `game`, `blockchain`, `marketing`

**Science:** `biology`, `chemistry`, `physics`, `quantum-physics`, `quantum-ml`

Each skill is plain Markdown + YAML frontmatter — any LLM can
consume them. The open spec is at [docs/SKILL-SCHEMA.md](docs/SKILL-SCHEMA.md).

## Architecture

`radiant` is structured as:

```
cmd/radiant/main.go          ← CLI entrypoint (cobra commands)
internal/skill/              ← skill schema validator + bundle loader
internal/scaffold/           ← scaffold + native agent view generation
internal/engine/             ← SDD execution engine (planner/implementer/validator)
internal/harness/            ← quality gates + policy enforcement
internal/llm/                ← OpenAI + Anthropic + OpenRouter clients
internal/policy/             ← command allowlist + token estimator
internal/spec/               ← spec + task + ADR parsing
internal/quality/            ← fidelity scoring + drift detection
internal/benchmark/          ← cross-framework benchmark harness
```

The CLI binary embeds 17 skills via `//go:embed` — no external
dependencies at install time.

## Quality

Every commit passes the same battery:

- `go build ./...` clean
- `go vet ./...` clean
- `gofmt -l .` clean
- `CGO_ENABLED=0 go test ./... -count=1 -race` all green (298 tests)
- `make release` cross-compiles 6/6 targets

See `docs/METHODOLOGY-MERGE-FINAL.md` for the full history.

## Examples

The `internal/scaffold/templates/examples/pulse/` directory has a
worked example project ("Pulse — feedback collector") that
demonstrates every command end-to-end.

## Documentation

- `docs/HARNESS-PLAN.md` — the methodology merge plan
- `docs/SKILL-SCHEMA.md` — open MIT spec for the skill format
- `docs/METHODOLOGY-MERGE-FINAL.md` — consolidated report of Sprints 10-13
- `docs/ROADMAP.md` — current roadmap
- `docs/CHANGELOG.md` (top-level) — version history
- `docs/validation-report-*.md` — per-sprint validation reports

## License

MIT — see [LICENSE](LICENSE).

## Contributing

Open an issue or PR at
[github.com/quant-risk/radiant-harness](https://github.com/quant-risk/radiant-harness).
The CLI is built around the open skill schema; new skills can be
authored in any repo and consumed by `radiant` without recompilation.