# radiant-harness

> A vendor-neutral Spec-Driven Development harness for any LLM.
> Shipped as a single binary — works with Claude Code, Cursor,
> Codex, Copilot, Gemini CLI, and Windsurf.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Version](https://img.shields.io/badge/version-0.6.0-blue.svg)](CHANGELOG.md)
[![Tests](https://img.shields.io/badge/tests-298_pass-green.svg)](CHANGELOG.md)
[![Cross-compile](https://img.shields.io/badge/cross--compile-6_of_6-blueviolet.svg)](CHANGELOG.md)

---

## What it is

`radiant` is a CLI that implements an end-to-end Spec-Driven
Development methodology, designed to work with **any modern LLM**
through an open skill schema (no Claude-centrism, no vendor lock-in).
The CLI ships 18 commands and 17 vendor-neutral skills.

The methodology runs in five phases:

1. **Discover** — `radiant product` (Lean Inception)
2. **Specify** — `radiant spec` (AC→test mapping)
3. **Implement** — `radiant run` (the LLM agent drives this)
4. **Verify** — `radiant validate`, `radiant review-pr`, `radiant evals`, `radiant audit`
5. **Operate** — `radiant update`, `radiant release`, `radiant mcp serve`, `radiant setup-ci`

Plus companion commands: `radiant adr` (decisions), `radiant diagramar`
(C4 Mermaid), `radiant integrations list` (MCP discovery), `radiant views`
(native agent views), `radiant handoff` (session pause).

## Install

```bash
# from source
go install github.com/quant-risk/radiant-harness/cmd/radiant@latest

# or download a release binary
# https://github.com/quant-risk/radiant-harness/releases

# verify
radiant --version
```

Cross-platform: Linux (amd64, arm64), macOS (amd64, arm64),
Windows (amd64, arm64). All binaries are statically linked.

## Quick start

```bash
# 1. Initialize a project
mkdir my-saas && cd my-saas
radiant init . --all --yes

# 2. Start a product (Lean Inception)
radiant product "API observability for small dev teams"

# 3. Spec the first feature
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

## Skills (17 bundled)

The CLI ships these vendor-neutral skills in `.radiant-harness/skills/`:

**Core methodology:** `nova-feature`, `nova-product`, `kickoff`, `clarificar`

**Quality:** `validar`, `auditar`, `metricas`, `evals`, `revisar-pr`

**Architecture:** `adr`, `diagramar`, `mapear`, `camada-agentica`

**Operations:** `integracoes`, `setup-ci`, `update`, `handoff`, `roadmap`

Each skill is plain Markdown + YAML frontmatter — any LLM can
consume them. The open spec is at `docs/SKILL-SCHEMA.md`.

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